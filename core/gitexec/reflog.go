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
	unitSep      = "\x1f"
	reflogFormat = "%gd" + unitSep + "%H" + unitSep + "%gn" + unitSep + "%ge" + unitSep + "%gs"
	reflogFields = 5
	maxLineBytes = 1024 * 1024
)

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

// Reflog returns up to limit reflog entries, newest first; a non-positive limit returns all.
func (r *Runner) Reflog(ctx context.Context, limit int) ([]ReflogEntry, error) {
	born, err := r.hasCommits(ctx)
	if err != nil {
		return nil, err
	}
	if !born {
		return nil, nil
	}

	args := []string{"reflog"}
	if limit > 0 {
		args = append(args, "-n", strconv.Itoa(limit))
	}
	args = append(args, "--no-show-signature", "--date=unix", "--format="+reflogFormat)

	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseReflog(out)
}

// ReflogHashes returns every commit HEAD's reflog records, however far back, as a set.
func (r *Runner) ReflogHashes(ctx context.Context) (map[string]bool, error) {
	hashes := make(map[string]bool)
	born, err := r.hasCommits(ctx)
	if err != nil || !born {
		return hashes, err
	}

	out, err := r.run(ctx, "reflog", "show", "--no-show-signature", "--format=%H", "HEAD")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		hash := strings.TrimSpace(line)
		if hash == "" {
			continue
		}
		if !isFullHash(hash) {
			return nil, fmt.Errorf("reflog returned %q, not a commit hash", hash)
		}
		hashes[hash] = true
	}
	return hashes, nil
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

func parseReflog(out []byte) ([]ReflogEntry, error) {
	var entries []ReflogEntry

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, unitSep, reflogFields)
		if len(fields) != reflogFields {
			return nil, fmt.Errorf("reflog entry %d: expected %d fields, got %d", len(entries), reflogFields, len(fields))
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
