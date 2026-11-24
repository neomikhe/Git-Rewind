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
	stashMinParents      = 2
	stashMaxParents      = 3
	stashPrefixWIP       = "WIP on "
	stashPrefixPlain     = "On "
	stashIndexPrefix     = "index on "
	stashUntrackedPrefix = "untracked files on "
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
	message := gitexec.Printable(stash.Subject)
	p := repo.Say()

	return &safety.Plan{
		Commands: []safety.Command{{
			Args:    []string{"stash", "store", "-m", message, stash.Hash},
			Explain: p.T(i18n.RecipeRestoreStashStep),
		}},
		Warnings: []string{p.T(i18n.RecipeRestoreStashWarning, message)},
	}, nil
}

func newestStash(ctx context.Context, git *gitexec.Runner, orphans map[string]struct{}) (gitexec.Commit, bool, error) {
	candidates, err := stashCandidates(ctx, git, orphans)
	if err != nil {
		return gitexec.Commit{}, false, err
	}
	for _, c := range candidates {
		ok, err := hasStashParents(ctx, git, c)
		if err != nil {
			return gitexec.Commit{}, false, err
		}
		if ok {
			return c, true, nil
		}
	}
	return gitexec.Commit{}, false, nil
}

func looksLikeStash(commit gitexec.Commit) bool {
	if len(commit.Parents) < stashMinParents || len(commit.Parents) > stashMaxParents {
		return false
	}
	return strings.HasPrefix(commit.Subject, stashPrefixWIP) ||
		strings.HasPrefix(commit.Subject, stashPrefixPlain)
}

func hasStashParents(ctx context.Context, git *gitexec.Runner, stash gitexec.Commit) (bool, error) {
	parents, err := git.CommitMetas(ctx, stash.Parents[1:])
	if err != nil {
		return false, err
	}
	byHash := make(map[string]gitexec.Commit, len(parents))
	for _, c := range parents {
		byHash[c.Hash] = c
	}

	index, ok := byHash[stash.Parents[1]]
	if !ok || !strings.HasPrefix(index.Subject, stashIndexPrefix) ||
		len(index.Parents) != 1 || index.Parents[0] != stash.Parents[0] {
		return false, nil
	}
	if len(stash.Parents) < stashMaxParents {
		return true, nil
	}
	untracked, ok := byHash[stash.Parents[2]]
	return ok && strings.HasPrefix(untracked.Subject, stashUntrackedPrefix) && len(untracked.Parents) == 0, nil
}

func stashCandidates(ctx context.Context, git *gitexec.Runner, orphans map[string]struct{}) ([]gitexec.Commit, error) {
	hashes := make([]string, 0, len(orphans))
	for hash := range orphans {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)

	commits, err := git.CommitMetas(ctx, hashes)
	if err != nil {
		return nil, err
	}
	shaped := commits[:0]
	for _, c := range commits {
		if looksLikeStash(c) {
			shaped = append(shaped, c)
		}
	}
	if len(shaped) == 0 {
		return nil, nil
	}

	visited, err := git.ReflogHashes(ctx)
	if err != nil {
		return nil, err
	}
	candidates := shaped[:0]
	for _, c := range shaped {
		if visited[c.Parents[0]] {
			candidates = append(candidates, c)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].When.Equal(candidates[j].When) {
			return candidates[i].Hash < candidates[j].Hash
		}
		return candidates[i].When.After(candidates[j].When)
	})
	return candidates, nil
}
