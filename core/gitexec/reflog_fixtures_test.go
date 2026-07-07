package gitexec

import (
	"strings"
	"testing"
	"time"
)

// The reflog parser reads a fixed --format string, so the *structure* of git's
// output is stable across versions by design. What varies between git versions
// is the reflog subject wording (merge strategy names, interactive-rebase
// labels, the "initial" tag) and identity/selector edge cases. These fixtures
// capture that variation as exact bytes (the 0x1F field separator included) so
// the parser is proven against outputs from different git eras without needing
// several git binaries installed.

// unitSepFixture is the 0x1F field separator git emits between --format fields.
const unitSepFixture = "\x1f"

// entryLine assembles one raw reflog line exactly as git would print it under
// our --format string.
func entryLine(selector, hash, name, email, subject string) string {
	return strings.Join([]string{selector, hash, name, email, subject}, unitSepFixture)
}

func atUnix(sec int64) time.Time {
	return time.Unix(sec, 0).UTC()
}

func TestParseReflogFixtures(t *testing.T) {
	h1 := strings.Repeat("a", 40)
	h2 := strings.Repeat("b", 40)
	h3 := strings.Repeat("c", 40)

	cases := []struct {
		name string
		raw  string
		want []ReflogEntry
	}{
		{
			name: "modern git HEAD reflog with trailing blank line",
			raw: entryLine("HEAD@{1767286800}", h1, "Ada", "ada@example.invalid", "reset: moving to HEAD~1") + "\n" +
				entryLine("HEAD@{1767286740}", h2, "Ada", "ada@example.invalid", "commit: second commit") + "\n" +
				entryLine("HEAD@{1767286680}", h3, "Ada", "ada@example.invalid", "commit (initial): first commit") + "\n\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1767286800), Hash: h1, ActorName: "Ada", ActorEmail: "ada@example.invalid", Subject: "reset: moving to HEAD~1", Operation: "reset"},
				{Index: 1, Ref: "HEAD", Time: atUnix(1767286740), Hash: h2, ActorName: "Ada", ActorEmail: "ada@example.invalid", Subject: "commit: second commit", Operation: "commit"},
				{Index: 2, Ref: "HEAD", Time: atUnix(1767286680), Hash: h3, ActorName: "Ada", ActorEmail: "ada@example.invalid", Subject: "commit (initial): first commit", Operation: "commit (initial)"},
			},
		},
		{
			name: "pre-2.34 recursive merge strategy",
			raw:  entryLine("HEAD@{1700000000}", h1, "Bo", "bo@example.invalid", "merge topic: Merge made by the 'recursive' strategy.") + "\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1700000000), Hash: h1, ActorName: "Bo", ActorEmail: "bo@example.invalid", Subject: "merge topic: Merge made by the 'recursive' strategy.", Operation: "merge topic"},
			},
		},
		{
			name: "2.34+ ort merge strategy",
			raw:  entryLine("HEAD@{1700000000}", h1, "Bo", "bo@example.invalid", "merge topic: Merge made by the 'ort' strategy.") + "\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1700000000), Hash: h1, ActorName: "Bo", ActorEmail: "bo@example.invalid", Subject: "merge topic: Merge made by the 'ort' strategy.", Operation: "merge topic"},
			},
		},
		{
			name: "interactive rebase label",
			raw:  entryLine("HEAD@{1700000000}", h1, "Bo", "bo@example.invalid", "rebase -i (finish): returning to refs/heads/feature") + "\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1700000000), Hash: h1, ActorName: "Bo", ActorEmail: "bo@example.invalid", Subject: "rebase -i (finish): returning to refs/heads/feature", Operation: "rebase -i (finish)"},
			},
		},
		{
			name: "branch reflog selector, not HEAD",
			raw:  entryLine("feature@{1700000000}", h1, "Bo", "bo@example.invalid", "commit: work on feature") + "\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "feature", Time: atUnix(1700000000), Hash: h1, ActorName: "Bo", ActorEmail: "bo@example.invalid", Subject: "commit: work on feature", Operation: "commit"},
			},
		},
		{
			name: "empty committer email",
			raw:  entryLine("HEAD@{1700000000}", h1, "Anonymous", "", "commit: no email set") + "\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1700000000), Hash: h1, ActorName: "Anonymous", ActorEmail: "", Subject: "commit: no email set", Operation: "commit"},
			},
		},
		{
			name: "unicode identity",
			raw:  entryLine("HEAD@{1700000000}", h1, "José Muñoz", "jose@example.invalid", "commit: acentuación") + "\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1700000000), Hash: h1, ActorName: "José Muñoz", ActorEmail: "jose@example.invalid", Subject: "commit: acentuación", Operation: "commit"},
			},
		},
		{
			name: "message containing a colon",
			raw:  entryLine("HEAD@{1700000000}", h1, "Bo", "bo@example.invalid", "commit: fix: the parser bug") + "\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1700000000), Hash: h1, ActorName: "Bo", ActorEmail: "bo@example.invalid", Subject: "commit: fix: the parser bug", Operation: "commit"},
			},
		},
		{
			name: "CRLF line endings",
			raw:  entryLine("HEAD@{1700000000}", h1, "Bo", "bo@example.invalid", "commit: crlf output") + "\r\n",
			want: []ReflogEntry{
				{Index: 0, Ref: "HEAD", Time: atUnix(1700000000), Hash: h1, ActorName: "Bo", ActorEmail: "bo@example.invalid", Subject: "commit: crlf output", Operation: "commit"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseReflog([]byte(c.raw))
			if err != nil {
				t.Fatalf("parseReflog: %v", err)
			}
			assertEntries(t, got, c.want)
		})
	}
}

func TestParseReflogRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"too few fields":           "HEAD@{1700000000}" + unitSepFixture + "onlytwo\n",
		"selector without braces":  "HEADnobraces" + unitSepFixture + "h" + unitSepFixture + "n" + unitSepFixture + "e" + unitSepFixture + "commit: x\n",
		"selector with bad unix":   "HEAD@{notanumber}" + unitSepFixture + "h" + unitSepFixture + "n" + unitSepFixture + "e" + unitSepFixture + "commit: x\n",
		"selector missing closing": "HEAD@{1700000000" + unitSepFixture + "h" + unitSepFixture + "n" + unitSepFixture + "e" + unitSepFixture + "commit: x\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseReflog([]byte(raw)); err == nil {
				t.Errorf("expected an error, got nil")
			}
		})
	}
}

func assertEntries(t *testing.T, got, want []ReflogEntry) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Index != w.Index || g.Ref != w.Ref || g.Hash != w.Hash ||
			g.ActorName != w.ActorName || g.ActorEmail != w.ActorEmail ||
			g.Subject != w.Subject || g.Operation != w.Operation {
			t.Errorf("entry %d =\n  %+v\nwant\n  %+v", i, g, w)
		}
		if !g.Time.Equal(w.Time) {
			t.Errorf("entry %d Time = %s, want %s", i, g.Time, w.Time)
		}
	}
}
