package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// UndoAmend restores the commit as it was before the most recent amend.
type UndoAmend struct{}

func (UndoAmend) Name() string { return "undo-amend" }

func (UndoAmend) Title(p *i18n.Printer) string { return p.T(i18n.RecipeUndoAmendTitle) }

func (UndoAmend) Detect(_ context.Context, repo *Repo) (*safety.Plan, error) {
	if len(repo.Events) == 0 {
		return nil, nil
	}
	amend := repo.Events[0]
	if amend.Kind != timeline.KindAmend || len(amend.Orphaned) == 0 {
		return nil, nil
	}
	original := amend.Orphaned[0]
	p := repo.Say()

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--soft", original},
			Explain: p.T(i18n.RecipeUndoAmendStep),
		}},
		Warnings: []string{p.T(i18n.RecipeUndoAmendWarning, shortHash(original))},
	}, nil
}
