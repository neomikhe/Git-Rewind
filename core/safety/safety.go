package safety

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// backupPrefix is the branch namespace git-rewind uses for its safety backups.
const backupPrefix = "backup/rewind-"

// Command is a single git command a rescue will run, paired with a short reason
// so the dry-run preview can teach what each step does.
type Command struct {
	// Args are the git arguments, without the leading "git".
	Args []string
	// Explain is a one-line reason the command is run.
	Explain string
}

// Plan is the ordered set of commands a rescue will run, plus any warnings the
// user should read first. Recipes produce a Plan; the safety layer carries it
// out with a backup and dry-run.
type Plan struct {
	Commands []Command
	Warnings []string
	// DiscardsChanges is true when carrying out the plan overwrites the working
	// tree (for example a reset --hard), so uncommitted changes — which the
	// backup does not capture — would be lost. Callers use it to guard against a
	// dirty working tree.
	DiscardsChanges bool
}

// Preview renders each command exactly as it would be run ("git reset --hard
// HEAD@{1}"). It is what the dry run shows and what teaches the user the real
// git commands behind a rescue.
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

// quoteArg quotes an argument that contains whitespace or quotes so the preview
// reads as a copy-pasteable command.
func quoteArg(a string) string {
	if a == "" || strings.ContainsAny(a, " \t\"'") {
		return strconv.Quote(a)
	}
	return a
}

// Options controls how Apply carries out a plan.
type Options struct {
	// Execute runs the plan for real. When false (the zero value, and the safe
	// default), Apply performs a dry run: it returns the exact commands without
	// running anything and without creating a backup.
	Execute bool
	// Now is the timestamp used to name the backup branch. Callers pass
	// time.Now(); tests pass a fixed time. Only used when Execute is true.
	Now time.Time
}

// Result reports what Apply did.
type Result struct {
	// DryRun is true when nothing was run.
	DryRun bool
	// BackupBranch is the safety branch created before executing (empty on a
	// dry run).
	BackupBranch string
	// Commands are the exact git commands previewed or run.
	Commands []string
}

// Apply carries out a plan. On a dry run it returns the exact commands and
// changes nothing. Otherwise it first creates a backup branch at HEAD, then runs
// each command in order; a backup is always created before anything is executed
// and is never skipped.
//
// Apply does not inspect the working tree: a plan that discards uncommitted
// changes (such as reset --hard) only preserves committed state in the backup,
// so callers must warn on a dirty tree using WorkingTreeStatus beforehand.
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
			return Result{BackupBranch: backup}, fmt.Errorf("running %q: %w", preview[i], err)
		}
	}
	return Result{BackupBranch: backup, Commands: preview}, nil
}

// Backup creates a branch backup/rewind-<timestamp> pointing at ref and returns
// its name. It only writes a ref, never touching the working tree. If a backup
// with the same second already exists, git reports the collision as an error.
func Backup(ctx context.Context, git *gitexec.Runner, now time.Time, ref string) (string, error) {
	name := backupPrefix + now.UTC().Format("20060102-150405")
	if _, err := git.Run(ctx, "branch", name, ref); err != nil {
		return "", err
	}
	return name, nil
}

// Status describes whether the working tree has uncommitted changes.
type Status struct {
	// Clean is true when there are no staged, unstaged, or untracked changes.
	Clean bool
	// Changes are the porcelain status lines (empty when Clean).
	Changes []string
}

// WorkingTreeStatus reports the working tree's cleanliness so callers can warn
// before an operation that would touch uncommitted work.
func WorkingTreeStatus(ctx context.Context, git *gitexec.Runner) (Status, error) {
	out, err := git.Run(ctx, "status", "--porcelain")
	if err != nil {
		return Status{}, err
	}
	// Keep leading status characters intact; only drop the trailing newline.
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return Status{Clean: true}, nil
	}
	return Status{Clean: false, Changes: strings.Split(out, "\n")}, nil
}
