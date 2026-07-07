package gitexec

import (
	"bufio"
	"bytes"
	"context"
	"strings"
)

// danglingCommitPrefix is how "git fsck" labels an orphaned commit tip on its
// standard output.
const danglingCommitPrefix = "dangling commit "

// Orphans returns the set of dangling commit hashes: commits that, once reflogs
// are excluded as reachability roots, are reachable from no branch or tag. These
// are the recoverable "lost" commit tips git-rewind can offer to restore (for
// example the commit discarded by a reset --hard, or replaced by an amend).
//
// It runs "git fsck --no-reflogs"; without --no-reflogs the reflog would keep
// those commits reachable and they would not be reported.
func (r *Runner) Orphans(ctx context.Context) (map[string]struct{}, error) {
	out, err := r.run(ctx, "fsck", "--no-reflogs")
	if err != nil {
		return nil, err
	}
	return parseDanglingCommits(out), nil
}

// parseDanglingCommits collects the commit hashes from "dangling commit <hash>"
// lines, ignoring dangling blobs, trees, tags, and any progress notices.
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
