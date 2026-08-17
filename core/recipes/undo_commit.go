package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
)

const parentRef = "HEAD~1"

// UndoLastCommit moves HEAD back one commit while keeping the changes staged.
type UndoLastCommit struct{}

func (UndoLastCommit) Name() string { return "undo-last-commit" }

func (UndoLastCommit) Title(p *i18n.Printer) string { return p.T(i18n.RecipeUndoCommitTitle) }

func (UndoLastCommit) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	if !refExists(ctx, repo.Git, parentRef) {
		return nil, nil
	}
	p := repo.Say()
	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--soft", parentRef},
			Explain: p.T(i18n.RecipeUndoCommitStep),
		}},
		Warnings: []string{p.T(i18n.RecipeUndoCommitWarning)},
	}, nil
}

// UndoLastCommitHard moves HEAD back one commit and discards the changes.
type UndoLastCommitHard struct{}

func (UndoLastCommitHard) Name() string { return "undo-last-commit-hard" }

func (UndoLastCommitHard) Title(p *i18n.Printer) string {
	return p.T(i18n.RecipeUndoCommitHardTitle)
}

func (UndoLastCommitHard) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	if !refExists(ctx, repo.Git, parentRef) {
		return nil, nil
	}
	p := repo.Say()
	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", parentRef},
			Explain: p.T(i18n.RecipeUndoCommitHardStep),
		}},
		Warnings:        []string{p.T(i18n.RecipeUndoCommitHardWarning)},
		DiscardsChanges: true,
	}, nil
}
