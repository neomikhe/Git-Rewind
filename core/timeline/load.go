package timeline

import (
	"context"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// Load reads a repository's reflog and orphaned-commit set through git and
// returns fully classified events, most recent first, with recoverable orphaned
// commits attached to the events that produced them.
func Load(ctx context.Context, git *gitexec.Runner) ([]Event, error) {
	entries, err := git.Reflog(ctx)
	if err != nil {
		return nil, err
	}
	events := FromReflog(entries)

	orphans, err := git.Orphans(ctx)
	if err != nil {
		return nil, err
	}
	AttachOrphans(events, orphans)
	return events, nil
}
