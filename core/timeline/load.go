package timeline

import (
	"context"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// Load reads a repository's reflog and orphans and returns fully classified events.
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
