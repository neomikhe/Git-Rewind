package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// UndoAmend restores the commit as it was before the most recent amend, keeping
// the amended changes staged.
type UndoAmend struct{}

// Name implements Recipe.
func (UndoAmend) Name() string { return "undo-amend" }

// Title implements Recipe.
func (UndoAmend) Title() string { return "Undo the last amend" }

// Detect implements Recipe. It applies only when the most recent operation was
// an amend that left the original commit recoverable, so HEAD is still on the
// affected branch and the rescue cannot move an unrelated branch.
func (UndoAmend) Detect(_ context.Context, repo *Repo) (*safety.Plan, error) {
	if len(repo.Events) == 0 {
		return nil, nil
	}
	amend := repo.Events[0]
	if amend.Kind != timeline.KindAmend || len(amend.Orphaned) == 0 {
		return nil, nil
	}
	original := amend.Orphaned[0]

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--soft", original},
			Explain: "restore the pre-amend commit; the amended changes stay staged",
		}},
		Warnings: []string{
			"Restores the original commit " + shortHash(original) + " as HEAD and keeps the amended changes staged, so you can recommit them as you meant to.",
		},
	}, nil
}
