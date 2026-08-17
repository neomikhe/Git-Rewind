package recipes

import (
	"context"
	"strings"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

const shortHashLen = 7

// Repo is the repository state a recipe inspects: a git runner plus the classified timeline.
type Repo struct {
	Git    *gitexec.Runner
	Events []timeline.Event
}

// Recipe is one rescue scenario and the project's extension point; recipes detect, never execute.
type Recipe interface {
	Name() string
	Title() string
	// Detect returns a plan when the recipe applies, or a nil plan when it does not.
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
		RestoreDroppedStash{},
		UndoLastCommit{},
		UndoLastCommitHard{},
	}
}

func findEvent(events []timeline.Event, pred func(timeline.Event) bool) (int, bool) {
	for i, e := range events {
		if pred(e) {
			return i, true
		}
	}
	return 0, false
}

func previousHash(events []timeline.Event, i int) string {
	if i+1 >= len(events) {
		return ""
	}
	return events[i+1].Entry.Hash
}

func commitExists(ctx context.Context, git *gitexec.Runner, rev string) bool {
	out, err := git.Run(ctx, "cat-file", "-t", rev)
	return err == nil && strings.TrimSpace(out) == "commit"
}

func refExists(ctx context.Context, git *gitexec.Runner, ref string) bool {
	_, err := git.Run(ctx, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

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

func looksLikeHash(s string) bool {
	if len(s) < shortHashLen {
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

func shortHash(h string) string {
	if len(h) > shortHashLen {
		return h[:shortHashLen]
	}
	return h
}
