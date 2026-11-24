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

// LoadWithHead returns the events together with the HEAD read just before them, for Apply to check against.
func LoadWithHead(ctx context.Context, git *gitexec.Runner, limit int) (gitexec.Head, []Event, error) {
	head, headErr := git.HeadState(ctx)
	events, err := Load(ctx, git, limit)
	if err != nil {
		return gitexec.Head{}, nil, err
	}
	if headErr != nil {
		if len(events) == 0 {
			return gitexec.Head{}, nil, nil
		}
		return gitexec.Head{}, nil, headErr
	}
	return head, events, nil
}
