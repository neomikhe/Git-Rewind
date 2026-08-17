package gitexec

import (
	"bufio"
	"bytes"
	"context"
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
