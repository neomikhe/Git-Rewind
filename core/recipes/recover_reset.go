package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// RecoverAfterResetHard moves the branch back to the commit discarded by a
// reset --hard, bringing the lost work back onto the branch.
type RecoverAfterResetHard struct{}

// Name implements Recipe.
func (RecoverAfterResetHard) Name() string { return "recover-after-reset-hard" }

// Title implements Recipe.
func (RecoverAfterResetHard) Title() string { return "Recover commits discarded by reset --hard" }

// Detect implements Recipe. It applies only when the most recent operation was
// a reset that left a commit recoverable, so HEAD is still on the affected
// branch and the rescue cannot move an unrelated branch.
func (RecoverAfterResetHard) Detect(_ context.Context, repo *Repo) (*safety.Plan, error) {
	if len(repo.Events) == 0 {
		return nil, nil
	}
	reset := repo.Events[0]
	if reset.Kind != timeline.KindReset || len(reset.Orphaned) == 0 {
		return nil, nil
	}
	lost := reset.Orphaned[0]

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"reset", "--hard", lost},
			Explain: "move the branch back onto the recovered commit " + shortHash(lost),
		}},
		Warnings: []string{
			"Moves your branch back to the recovered commit, replacing the current state (which is saved to the backup branch first). Any uncommitted changes are discarded.",
		},
	}, nil
}
