package safety

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

const (
	backupPrefix = "backup/rewind-"
	backupStamp  = "20060102-150405"
)

// Command is one git command a rescue will run, paired with a one-line reason.
type Command struct {
	Args    []string
	Explain string
}

// Plan is the ordered set of commands a rescue will run, its warnings, and whether it
// overwrites the working tree.
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
	if a == "" || strings.ContainsAny(a, " \t\"'") {
		return strconv.Quote(a)
	}
	return a
}

// Options controls how Apply carries out a plan; the zero value is a dry run.
type Options struct {
	Execute bool
	Now     time.Time
}

// Result reports what Apply did.
type Result struct {
	DryRun       bool
	BackupBranch string
	Commands     []string
}

// ApplyError reports a plan that failed partway through. It names the backup branch that
// already holds the previous state, so the caller can always tell the user where their work
// is even when the rescue itself went wrong.
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

// Apply previews a plan, or executes it after creating a backup branch that is never skipped.
func Apply(ctx context.Context, git *gitexec.Runner, plan Plan, opts Options) (Result, error) {
	preview := plan.Preview()
	if !opts.Execute {
		return Result{DryRun: true, Commands: preview}, nil
	}

	backup, err := Backup(ctx, git, opts.Now, "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("creating backup branch: %w", err)
	}

	for i, c := range plan.Commands {
		if _, err := git.Run(ctx, c.Args...); err != nil {
			return Result{BackupBranch: backup}, &ApplyError{
				Command:      preview[i],
				BackupBranch: backup,
				Err:          err,
			}
		}
	}
	return Result{BackupBranch: backup, Commands: preview}, nil
}

// Backup creates a branch backup/rewind-<timestamp> at ref and returns its name.
func Backup(ctx context.Context, git *gitexec.Runner, now time.Time, ref string) (string, error) {
	name := backupPrefix + now.UTC().Format(backupStamp)
	if _, err := git.Run(ctx, "branch", name, ref); err != nil {
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
	out, err := git.Run(ctx, "status", "--porcelain")
	if err != nil {
		return Status{}, err
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return Status{Clean: true}, nil
	}
	return Status{Clean: false, Changes: strings.Split(out, "\n")}, nil
}
