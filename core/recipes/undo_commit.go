package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/safety"
)

// parentRef is the previous commit, the target both undo-last-commit variants
// move HEAD back to.
const parentRef = "HEAD~1"

// UndoLastCommit moves HEAD back one commit while keeping the changes staged, so
// the commit can be redone.
type UndoLastCommit struct{}

// Name implements Recipe.
func (UndoLastCommit) Name() string { return "undo-last-commit" }

// Title implements Recipe.
func (UndoLastCommit) Title() string { return "Undo the last commit (keep the changes)" }

// Detect implements Recipe. It applies whenever HEAD has an earlier commit to
// move back to.
func (UndoLastCommit) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	if !refExists(ctx, repo.Git, parentRef) {
		return nil, nil
	}
	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--soft", parentRef},
			Explain: "move HEAD back one commit; the changes stay staged",
		}},
		Warnings: []string{
			"Your changes are kept staged so you can recommit them. To discard the changes too, use \"Undo the last commit (discard the changes)\".",
		},
	}, nil
}

// UndoLastCommitHard moves HEAD back one commit and discards the changes.
type UndoLastCommitHard struct{}

// Name implements Recipe.
func (UndoLastCommitHard) Name() string { return "undo-last-commit-hard" }

// Title implements Recipe.
func (UndoLastCommitHard) Title() string { return "Undo the last commit (discard the changes)" }

// Detect implements Recipe. It applies whenever HEAD has an earlier commit to
// move back to.
func (UndoLastCommitHard) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	if !refExists(ctx, repo.Git, parentRef) {
		return nil, nil
	}
	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", parentRef},
			Explain: "move HEAD back one commit and discard the changes",
		}},
		Warnings: []string{
			"This discards the last commit and any uncommitted changes in the working tree. A backup branch is created first, but it only preserves committed work.",
		},
		DiscardsChanges: true,
	}, nil
}
