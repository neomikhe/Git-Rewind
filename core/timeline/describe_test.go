package timeline

import (
	"strings"
	"testing"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

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

// operationOf mirrors gitexec's reflog parsing (the operation is the text before
// the first colon) so these tests build events the way real reflog entries do.
func operationOf(subject string) string {
	before, _, _ := strings.Cut(subject, ":")
	return strings.TrimSpace(before)
}
