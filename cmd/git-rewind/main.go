// Command git-rewind turns a Git repository's reflog into a readable timeline of recent
// events and offers safe, backed-up rescue actions to undo Git mistakes. Installed on
// PATH as "git-rewind", it is invoked natively as "git rewind".
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
	"github.com/neomikhe/git-rewind/tui"
)

const (
	allEvents    = 0
	shortHashLen = 7
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:], ".", os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "git-rewind:", err)
		os.Exit(1)
	}
}

func run(args []string, dir string, stdout io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "last":
			return runLast(args[1:], dir, stdout)
		case "find":
			return runFind(args[1:], dir, stdout)
		case "explain":
			return runExplain(args[1:], dir, stdout)
		case "version", "--version", "-v":
			return flush(stdout, versionLine())
		default:
			return fmt.Errorf("unknown command %q (try \"git rewind\", or its \"last\", \"find\", \"explain\" and \"version\" subcommands)", args[0])
		}
	}

	git := gitexec.New(dir)
	events, err := timeline.Load(context.Background(), git, tui.PageSize)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return flush(stdout, "git-rewind: no repository history to show yet.\n")
	}
	return tui.Run(tui.Session{Git: git, Events: events, Limit: tui.PageSize})
}

func runLast(args []string, dir string, stdout io.Writer) error {
	fs := flag.NewFlagSet("last", flag.ContinueOnError)
	fs.SetOutput(stdout)
	apply := fs.Bool("yes", false, "apply the rescue; without this it is a dry run that only prints the commands")
	force := fs.Bool("force", false, "apply even when a reset would discard uncommitted changes")
	asJSON := fs.Bool("json", false, "print the result as JSON instead of text")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	git := gitexec.New(dir)
	events, err := timeline.Load(ctx, git, allEvents)
	if err != nil {
		return err
	}
	repo := &recipes.Repo{Git: git, Events: events}

	recipe, plan, err := chooseRescue(ctx, repo)
	if err != nil {
		return err
	}
	if plan == nil {
		if *asJSON {
			return writeJSON(stdout, jsonLast{Schema: jsonSchema, Command: "last", DryRun: !*apply})
		}
		return flush(stdout, "git-rewind: nothing to undo — no rescue applies to the recent history.\n")
	}

	status, err := safety.WorkingTreeStatus(ctx, git)
	if err != nil {
		return err
	}
	dirtyRisk := plan.DiscardsChanges && !status.Clean

	if !*asJSON {
		if err := flush(stdout, describePlan(recipe, plan, dirtyRisk)); err != nil {
			return err
		}
	}

	if !*apply {
		if *asJSON {
			return writeJSON(stdout, lastResult(recipe, plan, status, true, ""))
		}
		return flush(stdout, "\nDry run. Re-run with --yes to apply it; a backup branch is always created first.\n")
	}
	if dirtyRisk && !*force {
		return errors.New("aborted: uncommitted changes would be discarded; commit or stash them, or re-run with --force")
	}

	res, err := safety.Apply(ctx, git, *plan, safety.Options{Execute: true, Now: time.Now()})
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(stdout, lastResult(recipe, plan, status, false, res.BackupBranch))
	}
	return flush(stdout, fmt.Sprintf("\nDone. The previous state is saved on branch %s.\n", res.BackupBranch))
}

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

func versionLine() string {
	return fmt.Sprintf("git-rewind %s (commit %s, built %s, %s/%s)\n",
		version, commit, date, runtime.GOOS, runtime.GOARCH)
}

func flush(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
