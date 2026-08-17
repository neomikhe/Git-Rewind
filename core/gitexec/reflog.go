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

const (
	unitSep        = "\x1f"
	reflogFormat   = "%gd" + unitSep + "%H" + unitSep + "%gn" + unitSep + "%ge" + unitSep + "%gs"
	reflogFields   = 5
	defaultTimeout = 10 * time.Second
	maxLineBytes   = 1024 * 1024
)

// Runner executes git plumbing commands inside one repository.
type Runner struct {
	dir     string
	timeout time.Duration
}

// New returns a Runner that runs git in dir, a repository root or any path inside it.
func New(dir string) *Runner {
	return &Runner{dir: dir, timeout: defaultTimeout}
}

// ReflogEntry is a single parsed entry from a repository's reflog.
type ReflogEntry struct {
	Index      int
	Ref        string
	Time       time.Time
	Hash       string
	ActorName  string
	ActorEmail string
	Subject    string
	Operation  string
}

// Reflog returns the repository's reflog entries, most recent first; an unborn HEAD yields none.
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

// Run executes an arbitrary git command in the repository and returns its standard output.
func (r *Runner) Run(ctx context.Context, args ...string) (string, error) {
	out, err := r.run(ctx, args...)
	return string(out), err
}

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

func (r *Runner) run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

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

func parseReflog(out []byte) ([]ReflogEntry, error) {
	var entries []ReflogEntry

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Split(line, unitSep)
		if len(fields) != reflogFields {
			return nil, fmt.Errorf("reflog entry %d: expected %d fields, got %d: %q", len(entries), reflogFields, len(fields), line)
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

func operationOf(subject string) string {
	if i := strings.IndexByte(subject, ':'); i >= 0 {
		return strings.TrimSpace(subject[:i])
	}
	return strings.TrimSpace(subject)
}
