package gitexec

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestParseCommit(t *testing.T) {
	hash := strings.Repeat("a", 40)
	parents := strings.Repeat("b", 40) + " " + strings.Repeat("c", 40)
	raw := strings.Join([]string{hash, parents, "Ada Lovelace", "1767286800", "add the parser", "with a longer\nbody\n"}, unitSep)

	got, err := parseCommit(raw + "\n")
	if err != nil {
		t.Fatalf("parseCommit: %v", err)
	}
	if got.Hash != hash || got.Author != "Ada Lovelace" || got.Subject != "add the parser" {
		t.Errorf("parseCommit = %+v", got)
	}
	if len(got.Parents) != 2 || got.Parents[0] != strings.Repeat("b", 40) {
		t.Errorf("Parents = %v, want the two parent hashes", got.Parents)
	}
	if want := time.Unix(1767286800, 0).UTC(); !got.When.Equal(want) {
		t.Errorf("When = %s, want %s", got.When, want)
	}
	if got.Body != "with a longer\nbody" {
		t.Errorf("Body = %q, want the trimmed multi-line body", got.Body)
	}
}

func TestParseCommitRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"too few fields": "onlyhash" + unitSep + "Ada",
		"bad time":       strings.Join([]string{"h", "", "Ada", "notanumber", "subject", ""}, unitSep),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCommit(raw); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestParseGrep(t *testing.T) {
	hash := strings.Repeat("b", 40)
	raw := hash + ":core/gitexec/reflog.go\x0016\x00\tunitSep = 1\n" +
		hash + ":odd:path/with:colons.go\x007\x00found here\n" +
		"someotherhash:ignored.go\x001\x00not ours\n"

	got, err := parseGrep([]byte(raw), hash, 0)
	if err != nil {
		t.Fatalf("parseGrep: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d matches, want 2: %+v", len(got), got)
	}
	if got[0].Path != "core/gitexec/reflog.go" || got[0].Line != 16 {
		t.Errorf("first match = %+v", got[0])
	}
	if got[1].Path != "odd:path/with:colons.go" || got[1].Line != 7 {
		t.Errorf("a path containing colons was mis-parsed: %+v", got[1])
	}
}

func TestParseGrepRespectsMax(t *testing.T) {
	hash := strings.Repeat("c", 40)
	var raw strings.Builder
	for i := 1; i <= 10; i++ {
		raw.WriteString(hash + ":f.go\x001\x00line\n")
	}

	got, err := parseGrep([]byte(raw.String()), hash, 3)
	if err != nil {
		t.Fatalf("parseGrep: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d matches, want the cap of 3", len(got))
	}
}

func TestCommitMetaAndGrepOnRealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	initRepo(t, dir)
	writeFile(t, dir, "func parseInvoiceTotal() int\n")
	runGit(t, dir, base, "add", workFile)
	runGit(t, dir, base, "commit", "-m", "add the invoice parser")

	ctx := context.Background()
	r := New(dir)
	head, err := r.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	head = strings.TrimSpace(head)

	commit, err := r.CommitMeta(ctx, head)
	if err != nil {
		t.Fatalf("CommitMeta: %v", err)
	}
	if commit.Hash != head || commit.Subject != "add the invoice parser" || commit.Author != "Test User" {
		t.Errorf("CommitMeta = %+v", commit)
	}

	hits, err := r.GrepCommit(ctx, head, "parseInvoiceTotal", 0)
	if err != nil {
		t.Fatalf("GrepCommit: %v", err)
	}
	if len(hits) != 1 || hits[0].Path != workFile || hits[0].Line != 1 {
		t.Fatalf("GrepCommit = %+v, want one hit in %s line 1", hits, workFile)
	}

	none, err := r.GrepCommit(ctx, head, "nothing-like-this-exists", 0)
	if err != nil {
		t.Fatalf("GrepCommit with no match must not be an error: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("got %d matches, want 0", len(none))
	}

	literal, err := r.GrepCommit(ctx, head, "parse.*Total", 0)
	if err != nil {
		t.Fatalf("GrepCommit: %v", err)
	}
	if len(literal) != 0 {
		t.Errorf("the query must be matched literally, not as a regular expression: %+v", literal)
	}
}

func TestHeadState(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	initRepo(t, dir)
	writeFile(t, dir, "one\n")
	runGit(t, dir, base, "add", workFile)
	runGit(t, dir, base, "commit", "-m", "first commit")

	ctx := context.Background()
	r := New(dir)

	attached, err := r.HeadState(ctx)
	if err != nil {
		t.Fatalf("HeadState: %v", err)
	}
	if attached.Detached || attached.Branch != "main" || len(attached.Hash) != 40 {
		t.Errorf("HeadState = %+v, want main, attached", attached)
	}

	runGit(t, dir, base.Add(time.Hour), "checkout", "-q", attached.Hash)

	detached, err := r.HeadState(ctx)
	if err != nil {
		t.Fatalf("HeadState detached: %v", err)
	}
	if !detached.Detached || detached.Branch != "" {
		t.Errorf("HeadState = %+v, want detached with no branch", detached)
	}
	if detached.Hash != attached.Hash {
		t.Errorf("Hash = %s, want the same commit %s", detached.Hash, attached.Hash)
	}
}
