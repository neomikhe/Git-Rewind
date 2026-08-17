package timeline

import (
	"strings"
	"testing"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{"seconds ago", now.Add(-30 * time.Second), "now"},
		{"minutes ago", now.Add(-5 * time.Minute), "5m"},
		{"hours ago", now.Add(-3 * time.Hour), "3h"},
		{"days ago", now.Add(-48 * time.Hour), "2d"},
		{"future clamps", now.Add(1 * time.Hour), "now"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RelativeTime(c.t, now); got != c.want {
				t.Errorf("RelativeTime = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAgoPhrase(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	if got := AgoPhrase(now.Add(-10*time.Second), now); got != "just now" {
		t.Errorf("AgoPhrase = %q, want %q", got, "just now")
	}
	if got := AgoPhrase(now.Add(-3*time.Hour), now); got != "3h ago" {
		t.Errorf("AgoPhrase = %q, want %q", got, "3h ago")
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		name     string
		subject  string
		orphaned []string
		want     string
	}{
		{"commit", "commit: add feature", nil, `Committed "add feature"`},
		{"initial commit", "commit (initial): first", nil, `Made the first commit "first"`},
		{"amend", "commit (amend): fix typo", nil, `Amended the last commit "fix typo"`},
		{"checkout", "checkout: moving from main to feature", nil, "Switched from main to feature"},
		{"merge", "merge feature: Fast-forward", nil, "Merged feature"},
		{"rebase", "rebase (finish): returning to refs/heads/feature", nil, "Rebased the branch"},
		{"pull", "pull: Fast-forward", nil, "Pulled from the remote"},
		{"branch", "branch: Created from HEAD", nil, "Created a branch"},
		{"clone", "clone: from https://example.invalid/repo.git", nil, "Cloned the repository"},
		{"cherry-pick", "cherry-pick: pick this", nil, `Cherry-picked "pick this"`},
		{"revert", "revert: undo that", nil, `Reverted "undo that"`},
		{"unknown falls back to subject", "gc: prune expired objects", nil, "gc: prune expired objects"},
		{"reset with one orphan", "reset: moving to HEAD~1", []string{"abc"}, "Reset the branch to HEAD~1 (1 commit left unreachable, recoverable)"},
		{"reset with two orphans", "reset: moving to HEAD~2", []string{"abc", "def"}, "Reset the branch to HEAD~2 (2 commits left unreachable, recoverable)"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			events := FromReflog([]gitexec.ReflogEntry{{Subject: c.subject, Operation: operationOf(c.subject)}})
			e := events[0]
			e.Orphaned = c.orphaned
			if got := e.Describe(); got != c.want {
				t.Errorf("Describe() = %q, want %q", got, c.want)
			}
		})
	}
}

func operationOf(subject string) string {
	before, _, _ := strings.Cut(subject, ":")
	return strings.TrimSpace(before)
}
