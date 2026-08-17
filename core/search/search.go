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

	orphans, err := git.Orphans(ctx)
	if err != nil {
		return result, err
	}
	result.Orphans = len(orphans)

	commits, err := describeOrphans(ctx, git, orphans, opts.MaxCommits)
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

func describeOrphans(ctx context.Context, git *gitexec.Runner, orphans map[string]struct{}, limit int) ([]gitexec.Commit, error) {
	hashes := make([]string, 0, len(orphans))
	for hash := range orphans {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)

	commits := make([]gitexec.Commit, 0, len(hashes))
	for _, hash := range hashes {
		commit, err := git.CommitMeta(ctx, hash)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}

	sort.SliceStable(commits, func(i, j int) bool {
		if commits[i].When.Equal(commits[j].When) {
			return commits[i].Hash < commits[j].Hash
		}
		return commits[i].When.After(commits[j].When)
	})

	if len(commits) > limit {
		commits = commits[:limit]
	}
	return commits, nil
}

func messageContains(commit gitexec.Commit, lowercaseNeedle string) bool {
	return strings.Contains(strings.ToLower(commit.Subject), lowercaseNeedle) ||
		strings.Contains(strings.ToLower(commit.Body), lowercaseNeedle)
}
