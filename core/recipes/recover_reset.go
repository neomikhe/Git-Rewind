package recipes

import (
	"context"

	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// RecoverAfterResetHard moves the branch back onto the commit a reset --hard discarded.
type RecoverAfterResetHard struct{}

func (RecoverAfterResetHard) Name() string { return "recover-after-reset-hard" }

func (RecoverAfterResetHard) Title() string { return "Recover commits discarded by reset --hard" }

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
		DiscardsChanges: true,
	}, nil
}
