package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// RestoreDeletedBranch recreates a deleted branch at the last commit it held.
type RestoreDeletedBranch struct{}

func (RestoreDeletedBranch) Name() string { return "restore-deleted-branch" }

func (RestoreDeletedBranch) Title() string { return "Restore a deleted branch" }

func (RestoreDeletedBranch) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	for i, e := range repo.Events {
		if e.Kind != timeline.KindCheckout {
			continue
		}
		from, ok := parseCheckoutFrom(e.Entry.Subject)
		if !ok || from == "" || looksLikeHash(from) {
			continue
		}
		if refExists(ctx, repo.Git, "refs/heads/"+from) {
			continue
		}
		tip := previousHash(repo.Events, i)
		if tip == "" || !commitExists(ctx, repo.Git, tip) {
			continue
		}

		return &safety.Plan{
			Commands: []safety.Command{{
				Args:    []string{"branch", from, tip},
				Explain: "recreate the branch at its last known commit " + shortHash(tip),
			}},
			Warnings: []string{
				"Recreates branch \"" + from + "\" at " + shortHash(tip) + ". This only adds a branch ref; nothing else changes.",
			},
		}, nil
	}
	return nil, nil
}
