package timeline

import (
	"context"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// Load returns classified events for up to limit reflog entries; a non-positive limit loads all.
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
