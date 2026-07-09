package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/internal/scenario"
)

// TestRunEmptyRepoPrintsNotice covers the non-interactive path of the default
// command: a repository with no history prints a notice instead of launching
// the TUI.
func TestRunEmptyRepoPrintsNotice(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	//nolint:gosec // G204: fixed "git" command with a test-controlled directory.
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	var buf bytes.Buffer
	if err := run(nil, dir, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "no repository history") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"bogus"}, ".", &buf)
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}
}

func TestLastDryRunShowsPlanWithoutApplying(t *testing.T) {
	dir, lost := resetHardRepo(t)
	before := headHash(t, dir)

	var buf bytes.Buffer
	if err := run([]string{"last"}, dir, &buf); err != nil {
		t.Fatalf("run last: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Rescue:", "git reset --hard", "Dry run"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q\n%s", want, out)
		}
	}
	if head := headHash(t, dir); head != before {
		t.Errorf("dry run moved HEAD to %s, want unchanged %s", head, before)
	}
	if lost == before {
		t.Fatal("scenario setup error: lost commit equals HEAD")
	}
}

func TestLastYesAppliesWithBackup(t *testing.T) {
	dir, lost := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"last", "--yes"}, dir, &buf); err != nil {
		t.Fatalf("run last --yes: %v", err)
	}

	if head := headHash(t, dir); head != lost {
		t.Errorf("HEAD = %s after rescue, want recovered commit %s", head, lost)
	}
	if !strings.Contains(buf.String(), "backup/rewind-") {
		t.Errorf("output does not mention the backup branch:\n%s", buf.String())
	}
}

func TestLastNothingToUndo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	git := hermeticGit(t, dir)
	git("init", "-q", "-b", "main")
	git("config", "commit.gpgsign", "false")
	writeFile(t, dir, "f.txt", "only\n")
	git("add", "f.txt")
	git("commit", "-q", "-m", "only commit")

	var buf bytes.Buffer
	if err := run([]string{"last"}, dir, &buf); err != nil {
		t.Fatalf("run last: %v", err)
	}
	if !strings.Contains(buf.String(), "nothing to undo") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestLastAbortsOnDirtyTree(t *testing.T) {
	dir, lost := resetHardRepo(t)
	before := headHash(t, dir)
	writeFile(t, dir, "file.txt", "one\ndirty\n") // modify the tracked file a reset --hard would discard

	// --yes without --force must refuse and change nothing.
	var buf bytes.Buffer
	if err := run([]string{"last", "--yes"}, dir, &buf); err == nil {
		t.Fatal("expected an error for a dirty working tree")
	}
	if head := headHash(t, dir); head != before {
		t.Errorf("aborted rescue still moved HEAD to %s, want %s", head, before)
	}

	// --force proceeds.
	var buf2 bytes.Buffer
	if err := run([]string{"last", "--yes", "--force"}, dir, &buf2); err != nil {
		t.Fatalf("run last --yes --force: %v", err)
	}
	if head := headHash(t, dir); head != lost {
		t.Errorf("HEAD = %s after forced rescue, want %s", head, lost)
	}
}

// --- test helpers ---

// resetHardRepo builds the reset-hard scenario and returns its path and the
// hash of the commit the reset discarded.
func resetHardRepo(t *testing.T) (dir, lost string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	for _, s := range scenario.All() {
		if s.Name == "reset-hard" {
			built, err := s.Build(t.TempDir())
			if err != nil {
				t.Fatalf("build reset-hard: %v", err)
			}
			return built.Dir, built.Anchors["lost"]
		}
	}
	t.Fatal("reset-hard scenario not found")
	return "", ""
}

func hermeticGit(t *testing.T, dir string) func(args ...string) string {
	t.Helper()
	return func(args ...string) string {
		cmd := exec.Command("git", args...) //nolint:gosec // G204: fixed "git" command with test-controlled arguments.
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+os.DevNull,
			"GIT_CONFIG_SYSTEM="+os.DevNull,
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.invalid",
			"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func headHash(t *testing.T, dir string) string {
	t.Helper()
	out, err := gitexec.New(dir).Run(context.Background(), "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(out)
}
