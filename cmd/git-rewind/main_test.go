package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// TestRunEmptyRepoPrintsNotice covers the non-interactive path: a repository
// with no history prints a notice and returns without launching the TUI.
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
	if err := run(dir, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(buf.String(), "no repository history") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}
