package search

import (
	"context"
	"sort"
	"strings"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

const (
	// DefaultMaxCommits bounds how many orphaned commits are inspected in one search.
	DefaultMaxCommits = 200
	// DefaultMaxFileHits bounds how many matching lines are reported per commit.
	DefaultMaxFileHits = 5
)

// Options tunes a search.
type Options struct {
	MessagesOnly bool
	MaxCommits   int
	MaxFileHits  int
}

// Match is one orphaned commit that matched, and where it matched.
type Match struct {
	Commit    gitexec.Commit
	InMessage bool
	InFiles   []gitexec.GrepMatch
}

// Result reports what a search found and how much of the repository it covered.
type Result struct {
	Query     string
	Matches   []Match
	Inspected int
	Orphans   int
}

// Truncated reports whether orphaned commits were left uninspected because of MaxCommits.
func (r Result) Truncated() bool { return r.Inspected < r.Orphans }

// Find returns the orphaned commits whose message or tree contains query, newest first.
func Find(ctx context.Context, git *gitexec.Runner, query string, opts Options) (Result, error) {
	opts = withDefaults(opts)
	result := Result{Query: query}
	if strings.TrimSpace(query) == "" {
		return result, nil
	}

	tips, err := git.Orphans(ctx)
	if err != nil {
		return result, err
	}
	candidates, err := lostChains(ctx, git, tips)
	if err != nil {
		return result, err
	}
	result.Orphans = len(candidates)

	commits, err := describeOrphans(ctx, git, candidates, opts.MaxCommits)
	if err != nil {
		return result, err
	}
	result.Inspected = len(commits)

	needle := strings.ToLower(query)
	for _, commit := range commits {
		match := Match{Commit: commit, InMessage: messageContains(commit, needle)}

		if !opts.MessagesOnly {
			hits, err := git.GrepCommit(ctx, commit.Hash, query, opts.MaxFileHits)
			if err != nil {
				return result, err
			}
			match.InFiles = hits
		}

		if match.InMessage || len(match.InFiles) > 0 {
			result.Matches = append(result.Matches, match)
		}
	}
	return result, nil
}

func withDefaults(opts Options) Options {
	if opts.MaxCommits <= 0 {
		opts.MaxCommits = DefaultMaxCommits
	}
	if opts.MaxFileHits <= 0 {
		opts.MaxFileHits = DefaultMaxFileHits
	}
	return opts
}

// lostChains expands each dangling tip into the whole run of commits below it that no branch
// or tag reaches. fsck only reports the tips, so searching those alone would miss every
// commit under them — and report work as gone when it is perfectly recoverable.
func lostChains(ctx context.Context, git *gitexec.Runner, tips map[string]struct{}) ([]string, error) {
	ordered := make([]string, 0, len(tips))
	for tip := range tips {
		ordered = append(ordered, tip)
	}
	sort.Strings(ordered)

	seen := make(map[string]struct{}, len(tips))
	var candidates []string
	for _, tip := range ordered {
		chain, err := git.Unreachable(ctx, tip)
		if err != nil {
			return nil, err
		}
		for _, hash := range chain {
			if _, duplicate := seen[hash]; duplicate {
				continue
			}
			seen[hash] = struct{}{}
			candidates = append(candidates, hash)
		}
	}
	return candidates, nil
}

func describeOrphans(ctx context.Context, git *gitexec.Runner, candidates []string, limit int) ([]gitexec.Commit, error) {
	// Bound the metadata reads before making them: each one is a git subprocess, and a long
	// lost chain would otherwise cost hundreds of them only to be truncated afterwards.
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	commits, err := git.CommitMetas(ctx, candidates)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(commits, func(i, j int) bool {
		if commits[i].When.Equal(commits[j].When) {
			return commits[i].Hash < commits[j].Hash
		}
		return commits[i].When.After(commits[j].When)
	})
	return commits, nil
}

func messageContains(commit gitexec.Commit, lowercaseNeedle string) bool {
	return strings.Contains(strings.ToLower(commit.Subject), lowercaseNeedle) ||
		strings.Contains(strings.ToLower(commit.Body), lowercaseNeedle)
}
