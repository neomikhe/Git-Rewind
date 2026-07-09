// Command git-rewind reads a Git repository's reflog, fsck, and working-tree
// state, translates it into a human-readable timeline of recent events, and
// offers safe, reversible "rescue" actions to undo Git mistakes.
//
// When installed on PATH as "git-rewind", Git invokes it as the native
// subcommand "git rewind".
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
	"github.com/neomikhe/git-rewind/tui"
)

func main() {
	if err := run(os.Args[1:], ".", os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "git-rewind:", err)
		os.Exit(1)
	}
}

// run dispatches subcommands. With no arguments it launches the interactive
// timeline; "last" runs the non-interactive rescue for the most recent mistake.
func run(args []string, dir string, stdout io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "last":
			return runLast(args[1:], dir, stdout)
		default:
			return fmt.Errorf("unknown command %q (try \"git rewind\" or \"git rewind last\")", args[0])
		}
	}

	entries, err := gitexec.New(dir).Reflog(context.Background())
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return flush(stdout, "git-rewind: no repository history to show yet.\n")
	}
	return tui.Run(entries)
}

// runLast finds the rescue for the most recent mistake, prints exactly what it
// would run, and applies it only with --yes. A backup branch is always created
// before anything runs, and a reset that would discard uncommitted changes is
// refused on a dirty working tree unless --force is given.
func runLast(args []string, dir string, stdout io.Writer) error {
	fs := flag.NewFlagSet("last", flag.ContinueOnError)
	fs.SetOutput(stdout)
	apply := fs.Bool("yes", false, "apply the rescue; without this it is a dry run that only prints the commands")
	force := fs.Bool("force", false, "apply even when a reset would discard uncommitted changes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	git := gitexec.New(dir)
	events, err := timeline.Load(ctx, git)
	if err != nil {
		return err
	}
	repo := &recipes.Repo{Git: git, Events: events}

	recipe, plan, err := chooseRescue(ctx, repo)
	if err != nil {
		return err
	}
	if plan == nil {
		return flush(stdout, "git-rewind: nothing to undo — no rescue applies to the recent history.\n")
	}

	status, err := safety.WorkingTreeStatus(ctx, git)
	if err != nil {
		return err
	}
	dirtyRisk := plan.DiscardsChanges && !status.Clean

	if err := flush(stdout, describePlan(recipe, plan, dirtyRisk)); err != nil {
		return err
	}

	if !*apply {
		return flush(stdout, "\nDry run. Re-run with --yes to apply it; a backup branch is always created first.\n")
	}
	if dirtyRisk && !*force {
		return errors.New("aborted: uncommitted changes would be discarded; commit or stash them, or re-run with --force")
	}

	res, err := safety.Apply(ctx, git, *plan, safety.Options{Execute: true, Now: time.Now()})
	if err != nil {
		return err
	}
	return flush(stdout, fmt.Sprintf("\nDone. The previous state is saved on branch %s.\n", res.BackupBranch))
}

// chooseRescue returns the first applicable recipe and its plan, or a nil plan
// when nothing applies.
func chooseRescue(ctx context.Context, repo *recipes.Repo) (recipes.Recipe, *safety.Plan, error) {
	for _, r := range recipes.All() {
		plan, err := r.Detect(ctx, repo)
		if err != nil {
			return nil, nil, err
		}
		if plan != nil {
			return r, plan, nil
		}
	}
	return nil, nil, nil
}

// describePlan renders the rescue, its exact commands, warnings, and the
// dirty-tree caution the user should read before applying.
func describePlan(recipe recipes.Recipe, plan *safety.Plan, dirtyRisk bool) string {
	var b strings.Builder
	b.WriteString("Rescue: " + recipe.Title() + "\n\nWill run:\n")
	for _, cmd := range plan.Preview() {
		b.WriteString("  " + cmd + "\n")
	}
	for _, w := range plan.Warnings {
		b.WriteString("\n  " + w + "\n")
	}
	if dirtyRisk {
		b.WriteString("\n  ! You have uncommitted changes. They are NOT saved to the backup and would be lost.\n")
	}
	return b.String()
}

// flush writes s to w and returns any write error.
func flush(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
