package gitexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func commitRecord(fields ...string) string {
	return strings.Join(fields, nulSep) + nulSep
}

func TestParseCommit(t *testing.T) {
	hash := strings.Repeat("a", 40)
	parents := strings.Repeat("b", 40) + " " + strings.Repeat("c", 40)
	raw := commitRecord(hash, parents, "Ada Lovelace", "1767286800", "add the parser", "with a longer\nbody\n")

	commits, err := parseCommits(raw, []string{hash})
	if err != nil {
		t.Fatalf("parseCommits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(commits))
	}
	got := commits[0]
	if got.Hash != hash || got.Author != "Ada Lovelace" || got.Subject != "add the parser" {
		t.Errorf("parseCommits = %+v", got)
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

func TestParseCommitsRejectsMalformed(t *testing.T) {
	hash := strings.Repeat("a", 40)
	other := strings.Repeat("d", 40)
	valid := commitRecord(hash, "", "Ada", "1767286800", "subject", "")
	cases := map[string]string{
		"partial record":       hash + nulSep + "Ada" + nulSep,
		"bad time":             commitRecord(hash, "", "Ada", "notanumber", "subject", ""),
		"hash that is a flag":  commitRecord("-Osh -c id", "", "Ada", "1767286800", "subject", ""),
		"parent that is not":   commitRecord(hash, "--all", "Ada", "1767286800", "subject", ""),
		"commit not requested": commitRecord(other, "", "Ada", "1767286800", "subject", ""),
		"commit twice":         valid + valid,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCommits(raw, []string{hash}); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestParseGrepKeysMatchesByCommit(t *testing.T) {
	a := strings.Repeat("b", 40)
	b := strings.Repeat("c", 40)
	raw := a + ":core/gitexec/reflog.go\x0016\x00\tunitSep = 1\n" +
		a + ":odd:path/with:colons.go\x007\x00found here\n" +
		b + ":other.go\x001\x00another commit\n"

	matches := make(map[string][]GrepMatch)
	if err := parseGrep([]byte(raw), 0, matches); err != nil {
		t.Fatalf("parseGrep: %v", err)
	}

	if len(matches[a]) != 2 {
		t.Fatalf("commit a has %d matches, want 2: %+v", len(matches[a]), matches[a])
	}
	if matches[a][0].Path != "core/gitexec/reflog.go" || matches[a][0].Line != 16 {
		t.Errorf("first match = %+v", matches[a][0])
	}
	if matches[a][1].Path != "odd:path/with:colons.go" || matches[a][1].Line != 7 {
		t.Errorf("a path containing colons was mis-parsed: %+v", matches[a][1])
	}
	if len(matches[b]) != 1 || matches[b][0].Path != "other.go" {
		t.Errorf("a second commit in the same batch was lost: %+v", matches[b])
	}
}

func TestParseGrepLimitsPerCommitNotOverall(t *testing.T) {
	a := strings.Repeat("d", 40)
	b := strings.Repeat("e", 40)

	var raw strings.Builder
	for i := 0; i < 10; i++ {
		raw.WriteString(a + ":f.go\x001\x00line\n")
		raw.WriteString(b + ":g.go\x001\x00line\n")
	}

	matches := make(map[string][]GrepMatch)
	if err := parseGrep([]byte(raw.String()), 3, matches); err != nil {
		t.Fatalf("parseGrep: %v", err)
	}
	if len(matches[a]) != 3 || len(matches[b]) != 3 {
		t.Errorf("got %d and %d matches, want the cap of 3 applied to each commit separately",
			len(matches[a]), len(matches[b]))
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
	if commit.Hash != head || commit.Subject != "add the invoice parser" || commit.Author != "Rewind Test" {
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

func TestCommitMetasIgnoresRecordsForgedInsideAMessage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	initRepo(t, dir)
	writeFile(t, dir, "x\n")
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	runGit(t, dir, base, "add", workFile)
	runGit(t, dir, base, "commit", "-m", "base")

	forged := "\x1e-Osh -c id\x1f\x1fMallory\x1f9999999999\x1fforged\x1fforged body"
	message := "innocent subject\n\nbody" + forged + forged + "\n"
	messageFile := filepath.Join(t.TempDir(), "message")
	if err := os.WriteFile(messageFile, []byte(message), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	r := New(dir)
	out, err := r.Run(ctx, "commit-tree", "HEAD^{tree}", "-F", messageFile)
	if err != nil {
		t.Fatalf("commit-tree: %v", err)
	}
	hash := strings.TrimSpace(out)

	commits, err := r.CommitMetas(ctx, []string{hash})
	if err != nil {
		t.Fatalf("CommitMetas: %v", err)
	}
	if len(commits) != 1 || commits[0].Hash != hash {
		t.Fatalf("CommitMetas returned %d commits %+v, want only %s", len(commits), commits, hash)
	}
	if commits[0].Subject != "innocent subject" || commits[0].Author != "Rewind Test" {
		t.Errorf("metadata = %+v, want the real subject and author", commits[0])
	}
}

func TestParsersIgnoreOutputSettingsInUserConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not found in PATH")
	}
	key := filepath.Join(t.TempDir(), "signing")
	keygen := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key) //nolint:gosec // G204: fixed "ssh-keygen" command with a test-created key path.
	if out, err := keygen.CombinedOutput(); err != nil {
		t.Skipf("cannot create a signing key: %v\n%s", err, out)
	}

	dir := t.TempDir()
	initRepo(t, dir)
	for _, setting := range [][2]string{
		{"gpg.format", "ssh"}, {"user.signingkey", key}, {"commit.gpgsign", "true"},
		{"log.showSignature", "true"}, {"grep.column", "true"}, {"color.ui", "always"},
	} {
		runGitEnv(t, dir, nil, "config", setting[0], setting[1])
	}
	writeFile(t, dir, "func signedWork() int\n")
	runGitEnv(t, dir, nil, "add", workFile)
	if out, err := gitOutput(dir, "commit", "-q", "-m", "signed work"); err != nil {
		t.Skipf("cannot sign a commit here: %v\n%s", err, out)
	}

	ctx := context.Background()
	r := New(dir)
	entries, err := r.Reflog(ctx, 0)
	if err != nil {
		t.Fatalf("Reflog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d reflog entries, want 1", len(entries))
	}
	head := entries[0].Hash

	commits, err := r.CommitMetas(ctx, []string{head})
	if err != nil {
		t.Fatalf("CommitMetas: %v", err)
	}
	if len(commits) != 1 || commits[0].Subject != "signed work" {
		t.Errorf("CommitMetas = %+v, want the signed commit", commits)
	}

	hits, err := r.GrepCommit(ctx, head, "signedWork", 0)
	if err != nil {
		t.Fatalf("GrepCommit: %v", err)
	}
	if len(hits) != 1 || hits[0].Text != "func signedWork() int" {
		t.Errorf("GrepCommit = %+v, want the plain matching line", hits)
	}
}

func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // G204: fixed "git" command with test-controlled arguments.
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func TestGrepCommitsRefusesArgumentsThatAreNotHashes(t *testing.T) {
	for _, bad := range []string{"-Osh -c id", "--no-index", "HEAD", "abc123", ""} {
		_, err := New(t.TempDir()).GrepCommits(context.Background(), []string{bad}, "query", 0)
		if err == nil {
			t.Errorf("GrepCommits accepted %q as a commit hash", bad)
		}
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
