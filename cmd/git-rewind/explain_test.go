package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestExplainOnABrokenRepo(t *testing.T) {
	dir, _ := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"explain"}, dir, &buf); err != nil {
		t.Fatalf("run explain: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Repository state",
		"HEAD",
		"on branch main",
		"Working tree",
		"clean",
		"Last event",
		"Reset the branch",
		"Unreachable",
		"1 commit no branch or tag reaches",
		"Something can be undone: Recover commits discarded by reset --hard",
		"git rewind find",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain output is missing %q\n---\n%s", want, out)
		}
	}
}

func TestExplainReportsADirtyTree(t *testing.T) {
	dir, _ := resetHardRepo(t)
	writeFile(t, dir, "file.txt", "one\ndirty\n")

	var buf bytes.Buffer
	if err := run([]string{"explain"}, dir, &buf); err != nil {
		t.Fatalf("run explain: %v", err)
	}
	if !strings.Contains(buf.String(), "uncommitted change") {
		t.Errorf("explain should report uncommitted changes\n---\n%s", buf.String())
	}
}

func TestExplainOnAHealthyRepo(t *testing.T) {
	dir := healthyRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"explain"}, dir, &buf); err != nil {
		t.Fatalf("run explain: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Nothing looks wrong") {
		t.Errorf("explain should report a healthy repository\n---\n%s", out)
	}
	if !strings.Contains(out, "nothing — no commit is missing") {
		t.Errorf("explain should report nothing unreachable\n---\n%s", out)
	}
	if strings.Contains(out, "git rewind find") {
		t.Errorf("explain should not suggest find when nothing is unreachable\n---\n%s", out)
	}
}

func TestExplainOnDetachedHead(t *testing.T) {
	built := detachedHeadRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"explain"}, built, &buf); err != nil {
		t.Fatalf("run explain: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "detached at") {
		t.Errorf("explain should report a detached HEAD\n---\n%s", out)
	}
	if strings.Contains(out, "Nothing looks wrong") {
		t.Errorf("a detached HEAD is worth flagging, not calling healthy\n---\n%s", out)
	}
}

func TestExplainOnAnEmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	hermeticGit(t, dir)("init", "-q", "-b", "main")

	var buf bytes.Buffer
	if err := run([]string{"explain"}, dir, &buf); err != nil {
		t.Fatalf("run explain: %v", err)
	}
	if !strings.Contains(buf.String(), "no repository history") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func healthyRepo(t *testing.T) string {
	t.Helper()
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
	return dir
}

func detachedHeadRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	git := hermeticGit(t, dir)
	git("init", "-q", "-b", "main")
	git("config", "commit.gpgsign", "false")
	writeFile(t, dir, "f.txt", "one\n")
	git("add", "f.txt")
	git("commit", "-q", "-m", "first commit")
	first := strings.TrimSpace(git("rev-parse", "HEAD"))
	writeFile(t, dir, "f.txt", "one\ntwo\n")
	git("commit", "-q", "-am", "second commit")
	git("checkout", "-q", first)
	return dir
}
