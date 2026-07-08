package gitexec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// unitSep is the ASCII Unit Separator (0x1F). It is used as the field delimiter
// inside git's --format string because it cannot appear in commit hashes, refs,
// actor identities, or reflog subjects, making it a collision-free separator.
const unitSep = "\x1f"

// reflogFormat asks git for the fields git-rewind needs from each reflog entry.
// Combined with --date=unix, the %gd selector renders as "<ref>@{<unix>}", which
// carries the reflog entry's own timestamp (not the commit date, which differs
// for operations such as reset and checkout).
const reflogFormat = "%gd" + unitSep + "%H" + unitSep + "%gn" + unitSep + "%ge" + unitSep + "%gs"

// defaultTimeout bounds a single git invocation so a hung or pathological
// repository can never block git-rewind indefinitely.
const defaultTimeout = 10 * time.Second

// Runner executes git plumbing commands inside a specific repository. It shells
// out to the system git binary rather than reimplementing any git internals.
type Runner struct {
	dir     string
	timeout time.Duration
}

// New returns a Runner that executes git commands in dir (a repository root or
// any path inside it).
func New(dir string) *Runner {
	return &Runner{dir: dir, timeout: defaultTimeout}
}

// ReflogEntry is a single parsed entry from a repository's reflog.
type ReflogEntry struct {
	// Index is the reflog position; 0 is the most recent entry (e.g. HEAD@{0}).
	Index int
	// Ref is the reference the entry belongs to, e.g. "HEAD".
	Ref string
	// Time is when the ref update was recorded (the reflog entry's own time).
	Time time.Time
	// Hash is the full object name the ref pointed to after this update.
	Hash string
	// ActorName and ActorEmail identify who performed the update.
	ActorName  string
	ActorEmail string
	// Subject is the raw reflog message, e.g. "reset: moving to HEAD~1".
	Subject string
	// Operation is the leading segment of Subject (the text before the first
	// colon), e.g. "reset", "checkout", or "commit (amend)". It is a coarse
	// action label; richer classification belongs to the timeline package.
	Operation string
}

// Reflog runs "git reflog" and returns its entries, most recent first.
// A freshly initialized repository (an unborn branch with no commits) has no
// HEAD reflog; that is reported as an empty history, not an error.
func (r *Runner) Reflog(ctx context.Context) ([]ReflogEntry, error) {
	born, err := r.hasCommits(ctx)
	if err != nil {
		return nil, err
	}
	if !born {
		return nil, nil
	}

	out, err := r.run(ctx, "reflog", "--date=unix", "--format="+reflogFormat)
	if err != nil {
		return nil, err
	}
	return parseReflog(out)
}

// Run executes an arbitrary git command in the repository and returns its
// standard output verbatim. Prefer the typed reads (Reflog, Orphans) where they
// apply; mutating operations should go through the safety package so backups and
// dry-run are enforced.
func (r *Runner) Run(ctx context.Context, args ...string) (string, error) {
	out, err := r.run(ctx, args...)
	return string(out), err
}

// hasCommits reports whether HEAD resolves to a commit. It is false for a
// repository whose current branch is still unborn (no commits yet). The
// exit-code check is language-independent: "git rev-parse --verify --quiet HEAD"
// exits 1 with no diagnostics for an unborn HEAD, and non-1 for real failures
// such as "not a git repository".
func (r *Runner) hasCommits(ctx context.Context) (bool, error) {
	_, err := r.run(ctx, "rev-parse", "--verify", "--quiet", "HEAD")
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// run executes a git subcommand in the runner's directory, bounded by the
// runner's timeout, and returns its standard output.
func (r *Runner) run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// The command is the constant "git" and every argument is built from
	// git-rewind's own constants, never from shell-interpreted user input.
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // G204: fixed "git" command, non-shell args controlled by this package.
	cmd.Dir = r.dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// parseReflog converts the raw --format output into typed entries. The reflog is
// emitted newest first, so each entry's Index is its line position.
func parseReflog(out []byte) ([]ReflogEntry, error) {
	var entries []ReflogEntry

	scanner := bufio.NewScanner(bytes.NewReader(out))
	// Reflog subjects are short, but allow generous lines to be safe.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Split(line, unitSep)
		if len(fields) != 5 {
			return nil, fmt.Errorf("reflog entry %d: expected 5 fields, got %d: %q", len(entries), len(fields), line)
		}

		ref, when, err := parseSelector(fields[0])
		if err != nil {
			return nil, fmt.Errorf("reflog entry %d: %w", len(entries), err)
		}

		subject := fields[4]
		entries = append(entries, ReflogEntry{
			Index:      len(entries),
			Ref:        ref,
			Time:       when,
			Hash:       fields[1],
			ActorName:  fields[2],
			ActorEmail: fields[3],
			Subject:    subject,
			Operation:  operationOf(subject),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading reflog output: %w", err)
	}
	return entries, nil
}

// parseSelector splits a unix-dated reflog selector such as "HEAD@{1767373200}"
// into its ref name and timestamp.
func parseSelector(selector string) (ref string, when time.Time, err error) {
	open := strings.Index(selector, "@{")
	if open < 0 || !strings.HasSuffix(selector, "}") {
		return "", time.Time{}, fmt.Errorf("malformed reflog selector %q", selector)
	}

	ref = selector[:open]
	inner := selector[open+2 : len(selector)-1]

	secs, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("reflog selector %q: invalid unix time %q: %w", selector, inner, err)
	}
	return ref, time.Unix(secs, 0).UTC(), nil
}

// operationOf extracts the leading action label from a reflog subject: the text
// before the first colon, or the whole subject when there is none.
func operationOf(subject string) string {
	if i := strings.IndexByte(subject, ':'); i >= 0 {
		return strings.TrimSpace(subject[:i])
	}
	return strings.TrimSpace(subject)
}
