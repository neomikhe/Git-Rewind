package timeline

import (
	"context"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// Load returns fully classified events for at most limit reflog entries, or all of them
// when limit is not positive.
func Load(ctx context.Context, git *gitexec.Runner, limit int) ([]Event, error) {
	entries, err := git.Reflog(ctx, limit)
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
