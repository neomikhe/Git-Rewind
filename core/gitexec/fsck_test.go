package gitexec

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestParseDanglingCommits(t *testing.T) {
	out := []byte("" +
		"dangling commit 1ad7e4951deafc5d36c38a5c06c02cd494f52768\n" +
		"dangling blob e69de29bb2d1d6434b8b29ae775ad8c2e48c5391\n" +
		"dangling tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n" +
		"dangling commit 60f4edc0b1ab5048ca9f8e967c4deaeeb4228657\n" +
		"notice: HEAD points to an unborn branch\n")

	got := parseDanglingCommits(out)
	if len(got) != 2 {
		t.Fatalf("got %d dangling commits, want 2: %v", len(got), got)
	}
	for _, want := range []string{
		"1ad7e4951deafc5d36c38a5c06c02cd494f52768",
		"60f4edc0b1ab5048ca9f8e967c4deaeeb4228657",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing dangling commit %s", want)
		}
	}
}

// TestOrphansFindsResetHardCommit builds a repo, discards a commit with
// reset --hard, and checks Orphans reports that commit as recoverable.
func TestOrphansFindsResetHardCommit(t *testing.T) {
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

	r := New(dir)
	before, err := r.Reflog(context.Background())
	if err != nil {
		t.Fatalf("Reflog: %v", err)
	}
	lost := before[0].Hash // the second commit is the current HEAD

	runGit(t, dir, base.Add(2*time.Hour), "reset", "--hard", "HEAD~1")

	orphans, err := r.Orphans(context.Background())
	if err != nil {
		t.Fatalf("Orphans: %v", err)
	}
	if _, ok := orphans[lost]; !ok {
		t.Fatalf("orphaned commit %s not reported by Orphans: %v", lost, orphans)
	}
}
