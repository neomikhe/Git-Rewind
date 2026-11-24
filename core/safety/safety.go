package safety

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

const (
	backupPrefix   = "backup/rewind-"
	backupStamp    = "20060102-150405"
	shellSafeChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._/:+,-"
	firstQuoteMark = '\u2018'
	lastQuoteMark  = '\u201b'
)

// Command is one git command a rescue will run, paired with a one-line reason.
type Command struct {
	Args    []string
	Explain string
}

// Plan is a rescue's ordered commands, its warnings, and whether it overwrites the working tree.
type Plan struct {
	Commands        []Command
	Warnings        []string
	DiscardsChanges bool
}

// Preview renders each command exactly as it would be typed, ready to copy and paste.
func (p Plan) Preview() []string {
	lines := make([]string, len(p.Commands))
	for i, c := range p.Commands {
		parts := make([]string, 0, len(c.Args)+1)
		parts = append(parts, "git")
		for _, a := range c.Args {
			parts = append(parts, quoteArg(a))
		}
		lines[i] = strings.Join(parts, " ")
	}
	return lines
}

func quoteArg(a string) string {
	if a != "" && strings.Trim(a, shellSafeChars) == "" {
		return a
	}
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range a {
		if isSingleQuoteMark(r) {
			b.WriteString(`'"`)
			b.WriteRune(r)
			b.WriteString(`"'`)
			continue
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

func isSingleQuoteMark(r rune) bool {
	return r == '\'' || (r >= firstQuoteMark && r <= lastQuoteMark)
}

// Errors Apply returns before anything runs, when a plan is no longer safe to apply.
var (
	ErrHeadMoved = errors.New("HEAD moved after the rescue was worked out, so it may target the wrong branch")
	ErrDirtyTree = errors.New("the working tree has uncommitted changes this rescue would discard")
)

// Options controls how Apply carries out a plan; the zero value is a dry run.
type Options struct {
	Execute      bool
	AllowDiscard bool
	Expected     gitexec.Head
	Now          time.Time
}

// Result reports what Apply did.
type Result struct {
	DryRun       bool
	BackupBranch string
	Commands     []string
}

// ApplyError reports a plan that failed partway through, naming the backup branch already made.
type ApplyError struct {
	Command      string
	BackupBranch string
	Err          error
}

func (e *ApplyError) Error() string {
	return fmt.Sprintf("running %q: %v (your previous state is saved on branch %s)",
		e.Command, e.Err, e.BackupBranch)
}

func (e *ApplyError) Unwrap() error { return e.Err }

// Apply previews a plan, or re-checks it is still safe, creates the backup branch and executes it.
func Apply(ctx context.Context, git *gitexec.Runner, plan Plan, opts Options) (Result, error) {
	preview := plan.Preview()
	if !opts.Execute {
		return Result{DryRun: true, Commands: preview}, nil
	}
	if err := stillSafe(ctx, git, plan, opts); err != nil {
		return Result{}, err
	}

	backup, err := Backup(ctx, git, opts.Now, "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("creating backup branch: %w", err)
	}

	for i, c := range plan.Commands {
		if _, err := git.Mutate(ctx, c.Args...); err != nil {
			return Result{BackupBranch: backup}, &ApplyError{
				Command:      preview[i],
				BackupBranch: backup,
				Err:          err,
			}
		}
	}
	return Result{BackupBranch: backup, Commands: preview}, nil
}

func stillSafe(ctx context.Context, git *gitexec.Runner, plan Plan, opts Options) error {
	head, err := git.HeadState(ctx)
	if err != nil {
		return fmt.Errorf("reading HEAD before applying: %w", err)
	}
	if head != opts.Expected {
		return ErrHeadMoved
	}
	if !plan.DiscardsChanges || opts.AllowDiscard {
		return nil
	}
	status, err := WorkingTreeStatus(ctx, git)
	if err != nil {
		return err
	}
	if !status.Clean {
		return ErrDirtyTree
	}
	return nil
}

// Backup creates a branch backup/rewind-<timestamp> at ref and returns its name.
func Backup(ctx context.Context, git *gitexec.Runner, now time.Time, ref string) (string, error) {
	name := backupPrefix + now.UTC().Format(backupStamp)
	if _, err := git.Mutate(ctx, "branch", name, ref); err != nil {
		return "", err
	}
	return name, nil
}

// Status describes whether the working tree has uncommitted changes.
type Status struct {
	Clean   bool
	Changes []string
}

// WorkingTreeStatus reports the working tree's cleanliness and its porcelain status lines.
func WorkingTreeStatus(ctx context.Context, git *gitexec.Runner) (Status, error) {
	out, err := git.Run(ctx, "status", "--porcelain", "--untracked-files=normal", "--ignore-submodules=none")
	if err != nil {
		return Status{}, err
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return Status{Clean: true}, nil
	}
	return Status{Clean: false, Changes: strings.Split(out, "\n")}, nil
}
