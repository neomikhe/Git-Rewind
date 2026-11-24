package recipes

import (
	"context"
	"strings"

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
	preRebase, ok := strandedByCurrentRebase(repo.Events)
	if !ok {
		return nil, nil
	}
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

func strandedByCurrentRebase(events []timeline.Event) (string, bool) {
	if len(events) == 0 || !isRebaseFinish(events[0]) {
		return "", false
	}
	for _, e := range events {
		if isRebaseStart(e) {
			if len(e.Orphaned) == 0 {
				return "", false
			}
			return e.Orphaned[0], true
		}
		if !happensDuringARebase(e.Kind) {
			return "", false
		}
	}
	return "", false
}

func isRebaseFinish(e timeline.Event) bool {
	return e.Kind == timeline.KindRebase && strings.Contains(e.Entry.Operation, "finish")
}

func isRebaseStart(e timeline.Event) bool {
	return e.Kind == timeline.KindRebase &&
		(strings.Contains(e.Entry.Operation, "(start)") ||
			strings.HasPrefix(e.Entry.Subject, "rebase: checkout "))
}

func happensDuringARebase(k timeline.Kind) bool {
	switch k {
	case timeline.KindRebase, timeline.KindCommit, timeline.KindAmend, timeline.KindReset:
		return true
	default:
		return false
	}
}
