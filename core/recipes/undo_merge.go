package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// UndoMerge returns the branch to the commit it was on before the merge that is now HEAD.
type UndoMerge struct{}

func (UndoMerge) Name() string { return "undo-merge" }

func (UndoMerge) Title(p *i18n.Printer) string { return p.T(i18n.RecipeUndoMergeTitle) }

func (UndoMerge) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	if !refExists(ctx, repo.Git, "HEAD^2") {
		return nil, nil
	}
	p := repo.Say()
	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", "HEAD^1"},
			Explain: p.T(i18n.RecipeUndoMergeStep),
		}},
		Warnings:        []string{p.T(i18n.RecipeUndoMergeWarning)},
		DiscardsChanges: true,
	}, nil
}

// UndoRebase returns the branch to the tip it had before the most recent rebase.
type UndoRebase struct{}

func (UndoRebase) Name() string { return "undo-rebase" }

func (UndoRebase) Title(p *i18n.Printer) string { return p.T(i18n.RecipeUndoRebaseTitle) }

func (UndoRebase) Detect(_ context.Context, repo *Repo) (*safety.Plan, error) {
	if len(repo.Events) == 0 || repo.Events[0].Kind != timeline.KindRebase {
		return nil, nil
	}
	i, ok := findEvent(repo.Events, func(e timeline.Event) bool {
		return e.Kind == timeline.KindRebase && len(e.Orphaned) > 0
	})
	if !ok {
		return nil, nil
	}
	preRebase := repo.Events[i].Orphaned[0]
	p := repo.Say()

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", preRebase},
			Explain: p.T(i18n.RecipeUndoRebaseStep, shortHash(preRebase)),
		}},
		Warnings:        []string{p.T(i18n.RecipeUndoRebaseWarning)},
		DiscardsChanges: true,
	}, nil
}
