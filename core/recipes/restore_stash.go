package recipes

import (
	"context"
	"sort"
	"strings"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
)

const (
	stashMinParents  = 2
	stashPrefixWIP   = "WIP on "
	stashPrefixPlain = "On "
)

// RestoreDroppedStash puts a dropped stash back on the stash list.
type RestoreDroppedStash struct{}

func (RestoreDroppedStash) Name() string { return "restore-dropped-stash" }

func (RestoreDroppedStash) Title(p *i18n.Printer) string { return p.T(i18n.RecipeRestoreStashTitle) }

func (RestoreDroppedStash) Detect(ctx context.Context, repo *Repo) (*safety.Plan, error) {
	orphans, err := repo.Git.Orphans(ctx)
	if err != nil {
		return nil, err
	}
	stash, found, err := newestStash(ctx, repo.Git, orphans)
	if err != nil || !found {
		return nil, err
	}
	p := repo.Say()

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"stash", "store", "-m", stash.Subject, stash.Hash},
			Explain: p.T(i18n.RecipeRestoreStashStep),
		}},
		Warnings: []string{p.T(i18n.RecipeRestoreStashWarning, stash.Subject)},
	}, nil
}

func newestStash(ctx context.Context, git *gitexec.Runner, orphans map[string]struct{}) (gitexec.Commit, bool, error) {
	hashes := make([]string, 0, len(orphans))
	for hash := range orphans {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)

	var newest gitexec.Commit
	found := false
	for _, hash := range hashes {
		commit, err := git.CommitMeta(ctx, hash)
		if err != nil {
			return gitexec.Commit{}, false, err
		}
		if !looksLikeStash(commit) {
			continue
		}
		if !found || commit.When.After(newest.When) {
			newest, found = commit, true
		}
	}
	return newest, found, nil
}

func looksLikeStash(commit gitexec.Commit) bool {
	if len(commit.Parents) < stashMinParents {
		return false
	}
	return strings.HasPrefix(commit.Subject, stashPrefixWIP) ||
		strings.HasPrefix(commit.Subject, stashPrefixPlain)
}
