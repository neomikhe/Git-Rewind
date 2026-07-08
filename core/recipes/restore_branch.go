package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// RestoreDeletedBranch recreates a branch that was deleted, pointing it at the
// last commit it held. It reads the reflog for a "checkout: moving from <branch>
// to ..." entry whose branch no longer exists but whose tip is still recoverable.
type RestoreDeletedBranch struct{}

// Name implements Recipe.
func (RestoreDeletedBranch) Name() string { return "restore-deleted-branch" }

// Title implements Recipe.
func (RestoreDeletedBranch) Title() string { return "Restore a deleted branch" }

// Detect implements Recipe. It applies when the reflog shows a branch we left
// that no longer exists and whose tip commit is still present.
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
			continue // the branch still exists; nothing to restore
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
