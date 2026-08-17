package timeline

import (
	"strings"
	"testing"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
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
	english, spanish := i18n.New(i18n.EN), i18n.New(i18n.ES)

	cases := []struct {
		name    string
		printer *i18n.Printer
		at      time.Time
		want    string
	}{
		{"english just now", english, now.Add(-10 * time.Second), "just now"},
		{"english hours", english, now.Add(-3 * time.Hour), "3h ago"},
		{"spanish just now", spanish, now.Add(-10 * time.Second), "ahora mismo"},
		{"spanish hours", spanish, now.Add(-3 * time.Hour), "hace 3h"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AgoPhrase(c.printer, c.at, now); got != c.want {
				t.Errorf("AgoPhrase = %q, want %q", got, c.want)
			}
		})
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
			if got := e.Describe(i18n.New(i18n.EN)); got != c.want {
				t.Errorf("Describe() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDescribeInSpanish(t *testing.T) {
	spanish := i18n.New(i18n.ES)
	cases := []struct {
		subject  string
		orphaned []string
		want     string
	}{
		{"reset: moving to HEAD~1", nil, "Moviste la rama a HEAD~1"},
		{"checkout: moving from main to feature", nil, "Cambiaste de main a feature"},
		{"commit: add the parser", nil, `Hiciste commit "add the parser"`},
		{"reset: moving to HEAD~1", []string{"abc"}, "Moviste la rama a HEAD~1 (1 commit quedó sin rama, se puede recuperar)"},
		{"reset: moving to HEAD~2", []string{"abc", "def"}, "Moviste la rama a HEAD~2 (2 commits quedaron sin rama, se pueden recuperar)"},
	}

	for _, c := range cases {
		t.Run(c.subject, func(t *testing.T) {
			events := FromReflog([]gitexec.ReflogEntry{{Subject: c.subject, Operation: operationOf(c.subject)}})
			e := events[0]
			e.Orphaned = c.orphaned
			if got := e.Describe(spanish); got != c.want {
				t.Errorf("Describe(es) = %q, want %q", got, c.want)
			}
		})
	}
}

func TestUnknownOperationKeepsGitsOwnWordsInEveryLanguage(t *testing.T) {
	const subject = "gc: prune expired objects"
	events := FromReflog([]gitexec.ReflogEntry{{Subject: subject, Operation: operationOf(subject)}})

	for _, lang := range []i18n.Lang{i18n.EN, i18n.ES} {
		if got := events[0].Describe(i18n.New(lang)); got != subject {
			t.Errorf("Describe(%s) = %q, want git's own words %q", lang, got, subject)
		}
	}
}

func operationOf(subject string) string {
	before, _, _ := strings.Cut(subject, ":")
	return strings.TrimSpace(before)
}
