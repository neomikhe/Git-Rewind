package recipes

import (
	"context"
	"strings"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

// Repo is the repository state a recipe inspects: a git runner plus the
// classified timeline (reflog events with orphaned commits attached).
type Repo struct {
	Git    *gitexec.Runner
	Events []timeline.Event
}

// Recipe is one rescue scenario and the project's extension point. A recipe
// inspects the repository and, when it applies, returns a safety.Plan describing
// exactly how to undo the mistake. Recipes never run destructive commands
// themselves; execution — backup creation and dry-run — is the safety package's
// job. New rescues are added by implementing this interface and registering the
// recipe in All, together with an integration test (scenario -> rescue ->
// verification).
type Recipe interface {
	// Name is a stable identifier, e.g. "undo-last-commit".
	Name() string
	// Title is a short label shown to the user.
	Title() string
	// Detect returns a plan when the recipe applies to repo, or a nil plan when
	// it does not.
	Detect(ctx context.Context, repo *Repo) (*safety.Plan, error)
}

// All returns every built-in recipe, most specific rescues first.
func All() []Recipe {
	return []Recipe{
		UndoAmend{},
		RecoverAfterResetHard{},
		RestoreDeletedBranch{},
		UndoMerge{},
		UndoRebase{},
		UndoLastCommit{},
		UndoLastCommitHard{},
	}
}

// findEvent returns the index of the first event (most recent first) that
// matches pred, or false when none do.
func findEvent(events []timeline.Event, pred func(timeline.Event) bool) (int, bool) {
	for i, e := range events {
		if pred(e) {
			return i, true
		}
	}
	return 0, false
}

// previousHash returns the ref value in effect before event i (the next-older
// entry's hash), or "" when i is the oldest event.
func previousHash(events []timeline.Event, i int) string {
	if i+1 >= len(events) {
		return ""
	}
	return events[i+1].Entry.Hash
}

// commitExists reports whether rev names an existing commit object, which is
// what makes an orphaned commit recoverable.
func commitExists(ctx context.Context, git *gitexec.Runner, rev string) bool {
	out, err := git.Run(ctx, "cat-file", "-t", rev)
	return err == nil && strings.TrimSpace(out) == "commit"
}

// refExists reports whether ref resolves. The --quiet check is
// language-independent (exit 1 with no output when the ref is missing).
func refExists(ctx context.Context, git *gitexec.Runner, ref string) bool {
	_, err := git.Run(ctx, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// parseCheckoutFrom extracts the source ref of a "checkout: moving from <from>
// to <to>" reflog subject.
func parseCheckoutFrom(subject string) (string, bool) {
	const marker = "moving from "
	i := strings.Index(subject, marker)
	if i < 0 {
		return "", false
	}
	from, _, ok := strings.Cut(subject[i+len(marker):], " to ")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(from), true
}

// looksLikeHash reports whether s is a hex object name rather than a branch
// name, so a detached-HEAD checkout is not mistaken for a deleted branch.
func looksLikeHash(s string) bool {
	if len(s) < 7 {
		return false
	}
	for _, c := range s {
		if !isHexDigit(c) {
			return false
		}
	}
	return true
}

func isHexDigit(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}

// shortHash abbreviates a hash for readable warnings.
func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
