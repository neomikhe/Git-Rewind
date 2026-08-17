package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// RestoreDeletedBranch recreates a deleted branch at the last commit it held.
type RestoreDeletedBranch struct{}

func (RestoreDeletedBranch) Name() string { return "restore-deleted-branch" }

func (RestoreDeletedBranch) Title(p *i18n.Printer) string {
	return p.T(i18n.RecipeRestoreBranchTitle)
}

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
		p := repo.Say()

		return &safety.Plan{
			Commands: []safety.Command{{
				Args:    []string{"branch", from, tip},
				Explain: p.T(i18n.RecipeRestoreBranchStep, shortHash(tip)),
			}},
			Warnings: []string{p.T(i18n.RecipeRestoreBranchWarning, from, shortHash(tip))},
		}, nil
	}
	return nil, nil
}
