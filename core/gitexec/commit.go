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
	commitFormat = "%H" + unitSep + "%P" + unitSep + "%an" + unitSep + "%at" + unitSep + "%s" + unitSep + "%b"
	commitFields = 6
	grepFields   = 3
)

// Commit is a commit's identifying metadata, whether or not any ref reaches it.
type Commit struct {
	Hash    string
	Parents []string
	Author  string
	When    time.Time
	Subject string
	Body    string
}

// GrepMatch is one matching line found inside a commit's tree.
type GrepMatch struct {
	Path string
	Line int
	Text string
}

// CommitMeta reads a single commit's metadata, including unreachable commits.
func (r *Runner) CommitMeta(ctx context.Context, hash string) (Commit, error) {
	out, err := r.run(ctx, "show", "-s", "--format="+commitFormat, hash)
	if err != nil {
		return Commit{}, err
	}
	return parseCommit(string(out))
}

// GrepCommit returns the lines of a commit's tree containing query, matched literally and
// case-insensitively, stopping once limit matches are collected.
func (r *Runner) GrepCommit(ctx context.Context, hash, query string, limit int) ([]GrepMatch, error) {
	out, err := r.run(ctx, "grep", "-I", "-n", "-z", "-i", "-F", "-e", query, hash)
	if err != nil {
		if isNoMatch(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseGrep(out, hash, limit)
}

func isNoMatch(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

func parseCommit(out string) (Commit, error) {
	fields := strings.SplitN(strings.TrimRight(out, "\n"), unitSep, commitFields)
	if len(fields) != commitFields {
		return Commit{}, fmt.Errorf("commit metadata: expected %d fields, got %d", commitFields, len(fields))
	}

	secs, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return Commit{}, fmt.Errorf("commit %s: invalid author time %q: %w", fields[0], fields[3], err)
	}

	return Commit{
		Hash:    fields[0],
		Parents: strings.Fields(fields[1]),
		Author:  fields[2],
		When:    time.Unix(secs, 0).UTC(),
		Subject: fields[4],
		Body:    strings.TrimSpace(fields[5]),
	}, nil
}

func parseGrep(out []byte, hash string, limit int) ([]GrepMatch, error) {
	var matches []GrepMatch
	prefix := hash + ":"

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		if limit > 0 && len(matches) >= limit {
			break
		}
		line, ok := strings.CutPrefix(scanner.Text(), prefix)
		if !ok {
			continue
		}
		fields := strings.SplitN(line, "\x00", grepFields)
		if len(fields) != grepFields {
			continue
		}
		number, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		matches = append(matches, GrepMatch{Path: fields[0], Line: number, Text: fields[2]})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading grep output: %w", err)
	}
	return matches, nil
}
