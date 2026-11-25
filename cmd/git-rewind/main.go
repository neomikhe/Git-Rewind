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
	"runtime/debug"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/config"
	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
	"github.com/neomikhe/git-rewind/tui"
)

const (
	allEvents    = 0
	shortHashLen = 7
	devVersion   = "dev"
	develVersion = "(devel)"
)

var (
	version = devVersion
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
	p := newPrinter()

	if len(args) > 0 {
		switch args[0] {
		case "last":
			return runLast(args[1:], dir, stdout, p)
		case "find":
			return runFind(args[1:], dir, stdout, p)
		case "explain":
			return runExplain(args[1:], dir, stdout, p)
		case "version", "--version", "-v":
			return flush(stdout, versionLine())
		default:
			return fmt.Errorf(p.T(i18n.RootUnknownCommand), args[0])
		}
	}

	git := gitexec.New(dir)
	head, events, err := timeline.LoadWithHead(context.Background(), git, tui.PageSize)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return flush(stdout, p.T(i18n.RootNoHistory))
	}
	return tui.Run(tui.Session{Git: git, Head: head, Events: events, Limit: tui.PageSize, Printer: p})
}

func newPrinter() *i18n.Printer {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Config{}
	}
	return i18n.New(i18n.Resolve("", cfg.Language, os.Getenv))
}

func runLast(args []string, dir string, stdout io.Writer, p *i18n.Printer) error {
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
	head, events, err := timeline.LoadWithHead(ctx, git, allEvents)
	if err != nil {
		return err
	}
	repo := &recipes.Repo{Git: git, Events: events, Printer: p}

	recipe, plan, err := chooseRescue(ctx, repo)
	if err != nil {
		return err
	}
	if plan == nil {
		if *asJSON {
			return writeJSON(stdout, jsonLast{Schema: jsonSchema, Command: "last", DryRun: !*apply})
		}
		return flush(stdout, p.T(i18n.LastNothingToUndo))
	}

	status, err := safety.WorkingTreeStatus(ctx, git)
	if err != nil {
		return err
	}
	dirtyRisk := plan.DiscardsChanges && !status.Clean

	if !*asJSON {
		if err := flush(stdout, describePlan(p, recipe, plan, dirtyRisk)); err != nil {
			return err
		}
	}

	if !*apply {
		if *asJSON {
			return writeJSON(stdout, lastResult(p, recipe, plan, status, true, ""))
		}
		return flush(stdout, p.T(i18n.LastDryRun))
	}

	res, err := safety.Apply(ctx, git, *plan, safety.Options{
		Execute:      true,
		AllowDiscard: *force,
		Expected:     head,
		Now:          time.Now(),
	})
	if err != nil {
		return describeApplyFailure(p, err)
	}
	if *asJSON {
		return writeJSON(stdout, lastResult(p, recipe, plan, status, false, res.BackupBranch))
	}
	return flush(stdout, p.T(i18n.LastDone, res.BackupBranch))
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

func describePlan(p *i18n.Printer, recipe recipes.Recipe, plan *safety.Plan, dirtyRisk bool) string {
	var b strings.Builder
	b.WriteString(p.T(i18n.LastRescueHeading, recipe.Title(p)))
	b.WriteString(p.T(i18n.LastWillRun))
	for _, cmd := range plan.Preview() {
		b.WriteString("  " + cmd + "\n")
	}
	for _, w := range plan.Warnings {
		b.WriteString("\n  " + w + "\n")
	}
	if dirtyRisk {
		b.WriteString(p.T(i18n.LastDirtyWarning))
	}
	return b.String()
}

func describeApplyFailure(p *i18n.Printer, err error) error {
	switch {
	case errors.Is(err, safety.ErrDirtyTree):
		return errors.New(p.T(i18n.LastAbortedDirty))
	case errors.Is(err, safety.ErrHeadMoved):
		return errors.New(p.T(i18n.LastAbortedHeadMoved))
	}
	var applyErr *safety.ApplyError
	if !errors.As(err, &applyErr) {
		return err
	}
	return fmt.Errorf(p.T(i18n.LastFailedWithBackup),
		applyErr.Command, applyErr.Err, applyErr.BackupBranch)
}

func versionLine() string {
	v, c, d := version, commit, date
	if info, ok := debug.ReadBuildInfo(); ok && v == devVersion {
		v, c, d = fromBuildInfo(info, v, c, d)
	}
	return fmt.Sprintf("git-rewind %s (commit %s, built %s, %s/%s)\n",
		v, c, d, runtime.GOOS, runtime.GOARCH)
}

func fromBuildInfo(info *debug.BuildInfo, v, c, d string) (string, string, string) {
	if mv := info.Main.Version; mv != "" && mv != develVersion {
		v = strings.TrimPrefix(mv, "v")
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			c = s.Value
		case "vcs.time":
			d = s.Value
		}
	}
	return v, c, d
}

func flush(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
