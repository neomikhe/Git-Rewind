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
	recordSep    = "\x1e"
	commitFormat = "%x1e%H" + unitSep + "%P" + unitSep + "%an" + unitSep + "%at" + unitSep + "%s" + unitSep + "%b"
	commitFields = 6
	grepFields   = 3
	batchSize    = 100
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
	commits, err := r.CommitMetas(ctx, []string{hash})
	if err != nil {
		return Commit{}, err
	}
	if len(commits) == 0 {
		return Commit{}, fmt.Errorf("commit %s: no metadata returned", hash)
	}
	return commits[0], nil
}

// CommitMetas reads many commits' metadata in as few git invocations as possible.
func (r *Runner) CommitMetas(ctx context.Context, hashes []string) ([]Commit, error) {
	commits := make([]Commit, 0, len(hashes))
	for start := 0; start < len(hashes); start += batchSize {
		end := min(start+batchSize, len(hashes))

		args := append([]string{"log", "--no-walk", "--format=" + commitFormat}, hashes[start:end]...)
		out, err := r.run(ctx, args...)
		if err != nil {
			return nil, err
		}
		batch, err := parseCommits(string(out))
		if err != nil {
			return nil, err
		}
		commits = append(commits, batch...)
	}
	return commits, nil
}

func parseCommits(out string) ([]Commit, error) {
	var commits []Commit
	for _, record := range strings.Split(out, recordSep) {
		if strings.TrimSpace(record) == "" {
			continue
		}
		commit, err := parseCommit(record)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}
	return commits, nil
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

// GrepCommits searches many commits' trees at once, returning matching lines keyed by hash.
func (r *Runner) GrepCommits(ctx context.Context, hashes []string, query string, limitPerCommit int) (map[string][]GrepMatch, error) {
	matches := make(map[string][]GrepMatch, len(hashes))

	for start := 0; start < len(hashes); start += batchSize {
		end := min(start+batchSize, len(hashes))

		args := []string{"grep", "-I", "-n", "-z", "-i", "-F", "-e", query}
		args = append(args, hashes[start:end]...)

		out, err := r.run(ctx, args...)
		if err != nil {
			if isNoMatch(err) {
				continue
			}
			return nil, err
		}
		if err := parseGrep(out, limitPerCommit, matches); err != nil {
			return nil, err
		}
	}
	return matches, nil
}

// GrepCommit returns the lines of one commit's tree containing query, matched literally.
func (r *Runner) GrepCommit(ctx context.Context, hash, query string, limit int) ([]GrepMatch, error) {
	matches, err := r.GrepCommits(ctx, []string{hash}, query, limit)
	if err != nil {
		return nil, err
	}
	return matches[hash], nil
}

func parseGrep(out []byte, limitPerCommit int, matches map[string][]GrepMatch) error {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		hash, rest, ok := strings.Cut(scanner.Text(), ":")
		if !ok || hash == "" {
			continue
		}
		if limitPerCommit > 0 && len(matches[hash]) >= limitPerCommit {
			continue
		}
		fields := strings.SplitN(rest, "\x00", grepFields)
		if len(fields) != grepFields {
			continue
		}
		number, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		matches[hash] = append(matches[hash], GrepMatch{Path: fields[0], Line: number, Text: fields[2]})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading grep output: %w", err)
	}
	return nil
}
