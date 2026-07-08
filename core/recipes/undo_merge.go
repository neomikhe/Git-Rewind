package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// UndoMerge returns the branch to the commit it was on before the most recent
// merge, undoing the merge.
type UndoMerge struct{}

// Name implements Recipe.
func (UndoMerge) Name() string { return "undo-merge" }

// Title implements Recipe.
func (UndoMerge) Title() string { return "Undo a merge (back to before the merge)" }

// Detect implements Recipe. It applies only when HEAD itself is a merge commit
// (it has a second parent) — the merge you just made and are still on. Keying on
// HEAD rather than the reflog means the rescue resets to the merge's own first
// parent and can never move an unrelated branch.
func (UndoMerge) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	if !refExists(ctx, repo.Git, "HEAD^2") {
		return nil, nil
	}
	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", "HEAD^1"},
			Explain: "move the branch back to the first parent, the commit before the merge",
		}},
		Warnings: []string{
			"Undoes the merge, restoring the state before it (saved to the backup branch first). Any uncommitted changes are discarded.",
		},
	}, nil
}

// UndoRebase returns the branch to the commit it was on before the most recent
// rebase, undoing the rebase.
type UndoRebase struct{}

// Name implements Recipe.
func (UndoRebase) Name() string { return "undo-rebase" }

// Title implements Recipe.
func (UndoRebase) Title() string { return "Undo a rebase (back to before the rebase)" }

// Detect implements Recipe. It applies only right after a rebase (the most
// recent operation), so HEAD is still the rebased branch's new tip and the
// rescue cannot move an unrelated branch.
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

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", preRebase},
			Explain: "move the branch back to " + shortHash(preRebase) + ", the tip before the rebase",
		}},
		Warnings: []string{
			"Discards the rebased commits and restores the pre-rebase tip (saved to the backup branch first). Any uncommitted changes are discarded.",
		},
	}, nil
}
