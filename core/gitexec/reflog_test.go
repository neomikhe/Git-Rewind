package gitexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestReflogParsesRealRepo builds a real repository with a known sequence of
// operations (commit, amend, checkout, reset --hard) at deterministic times and
// verifies that Reflog parses every field of every entry correctly.
func TestReflogParsesRealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	initRepo(t, dir)

	writeFile(t, dir, "a\n")
	runGit(t, dir, base, "add", workFile)
	runGit(t, dir, base, "commit", "-m", "first commit")

	writeFile(t, dir, "a\nb\n")
	runGit(t, dir, base.Add(1*time.Hour), "commit", "-am", "second commit")

	runGit(t, dir, base.Add(2*time.Hour), "commit", "--amend", "-m", "second commit (amended)")

	runGit(t, dir, base.Add(3*time.Hour), "checkout", "-b", "feature")

	runGit(t, dir, base.Add(4*time.Hour), "reset", "--hard", "HEAD~1")

	entries, err := New(dir).Reflog(context.Background())
	if err != nil {
		t.Fatalf("Reflog: %v", err)
	}

	// Reflog is newest first, so the reset is entry 0 and the initial commit
	// is last.
	want := []struct {
		operation string
		when      time.Time
	}{
		{"reset", base.Add(4 * time.Hour)},
		{"checkout", base.Add(3 * time.Hour)},
		{"commit (amend)", base.Add(2 * time.Hour)},
		{"commit", base.Add(1 * time.Hour)},
		{"commit (initial)", base},
	}

	if len(entries) != len(want) {
		t.Fatalf("got %d reflog entries, want %d: %+v", len(entries), len(want), entries)
	}

	for i, w := range want {
		e := entries[i]
		if e.Index != i {
			t.Errorf("entry %d: Index = %d, want %d", i, e.Index, i)
		}
		if e.Ref != "HEAD" {
			t.Errorf("entry %d: Ref = %q, want \"HEAD\"", i, e.Ref)
		}
		if e.Operation != w.operation {
			t.Errorf("entry %d: Operation = %q, want %q (subject %q)", i, e.Operation, w.operation, e.Subject)
		}
		if !e.Time.Equal(w.when) {
			t.Errorf("entry %d: Time = %s, want %s", i, e.Time, w.when)
		}
		if len(e.Hash) != 40 {
			t.Errorf("entry %d: Hash = %q, want 40 hex chars", i, e.Hash)
		}
		if e.ActorName != "Test User" {
			t.Errorf("entry %d: ActorName = %q, want \"Test User\"", i, e.ActorName)
		}
	}
}

// TestReflogEmptyRepo confirms that a repository with no reflog yields no
// entries and no error.
func TestReflogEmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	initRepo(t, dir)

	entries, err := New(dir).Reflog(context.Background())
	if err != nil {
		t.Fatalf("Reflog: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0: %+v", len(entries), entries)
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	runGitEnv(t, dir, nil, "init", "-q", "-b", "main")
	runGitEnv(t, dir, nil, "config", "user.name", "Test User")
	runGitEnv(t, dir, nil, "config", "user.email", "test@example.com")
}

// runGit runs a git command whose author and committer dates (and therefore its
// reflog timestamp) are pinned to when, so the reflog is reproducible.
func runGit(t *testing.T, dir string, when time.Time, args ...string) {
	t.Helper()
	stamp := when.Format("2006-01-02T15:04:05 -0700")
	runGitEnv(t, dir, []string{
		"GIT_AUTHOR_DATE=" + stamp,
		"GIT_COMMITTER_DATE=" + stamp,
	}, args...)
}

func runGitEnv(t *testing.T, dir string, extraEnv []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // G204: fixed "git" command with test-controlled arguments.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// workFile is the single tracked file the gitexec tests create and mutate.
const workFile = "f.txt"

func writeFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, workFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
