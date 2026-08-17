package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// RecoverAfterResetHard moves the branch back onto the commit a reset --hard discarded.
type RecoverAfterResetHard struct{}

func (RecoverAfterResetHard) Name() string { return "recover-after-reset-hard" }

func (RecoverAfterResetHard) Title(p *i18n.Printer) string {
	return p.T(i18n.RecipeRecoverResetTitle)
}

func (RecoverAfterResetHard) Detect(_ context.Context, repo *Repo) (*safety.Plan, error) {
	if len(repo.Events) == 0 {
		return nil, nil
	}
	reset := repo.Events[0]
	if reset.Kind != timeline.KindReset || len(reset.Orphaned) == 0 {
		return nil, nil
	}
	lost := reset.Orphaned[0]
	p := repo.Say()

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", lost},
			Explain: p.T(i18n.RecipeRecoverResetStep, shortHash(lost)),
		}},
		Warnings:        []string{p.T(i18n.RecipeRecoverResetWarning)},
		DiscardsChanges: true,
	}, nil
}
