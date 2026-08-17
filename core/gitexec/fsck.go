package gitexec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
)

const danglingCommitPrefix = "dangling commit "

// Orphans returns the commit hashes that no branch or tag reaches but that are still recoverable.
func (r *Runner) Orphans(ctx context.Context) (map[string]struct{}, error) {
	out, err := r.run(ctx, "fsck", "--no-reflogs")
	if err != nil {
		return nil, err
	}
	return parseDanglingCommits(out), nil
}

// Unreachable returns tip and every commit below it that no branch or tag reaches, newest
// first. A dangling tip is only the head of a lost chain; the commits under it are just as
// recoverable and just as worth searching.
func (r *Runner) Unreachable(ctx context.Context, tip string) ([]string, error) {
	out, err := r.run(ctx, "rev-list", tip, "--not", "--all")
	if err != nil {
		return nil, err
	}

	var hashes []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		if hash := strings.TrimSpace(scanner.Text()); hash != "" {
			hashes = append(hashes, hash)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading rev-list output: %w", err)
	}
	return hashes, nil
}

func parseDanglingCommits(out []byte) map[string]struct{} {
	orphans := make(map[string]struct{})

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		hash, ok := strings.CutPrefix(scanner.Text(), danglingCommitPrefix)
		if !ok {
			continue
		}
		if hash = strings.TrimSpace(hash); hash != "" {
			orphans[hash] = struct{}{}
		}
	}
	return orphans
}
