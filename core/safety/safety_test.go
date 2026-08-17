package safety

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

func TestPreviewQuotesArguments(t *testing.T) {
	plan := Plan{Commands: []Command{
		{Args: []string{"reset", "--hard", "HEAD@{1}"}},
		{Args: []string{"commit", "-m", "two words"}},
	}}
	want := []string{
		"git reset --hard HEAD@{1}",
		`git commit -m "two words"`,
	}
	if got := plan.Preview(); !reflect.DeepEqual(got, want) {
		t.Errorf("Preview() = %q, want %q", got, want)
	}
}

func TestWorkingTreeStatus(t *testing.T) {
	dir, _, _ := twoCommitRepo(t)
	git := gitexec.New(dir)
	ctx := context.Background()

	st, err := WorkingTreeStatus(ctx, git)
	if err != nil {
		t.Fatalf("WorkingTreeStatus: %v", err)
	}
	if !st.Clean {
		t.Fatalf("fresh repo reported dirty: %+v", st)
	}

	writeFile(t, dir, "one\ntwo\ndirty\n")
	st, err = WorkingTreeStatus(ctx, git)
	if err != nil {
		t.Fatalf("WorkingTreeStatus: %v", err)
	}
	if st.Clean || len(st.Changes) == 0 {
		t.Fatalf("modified repo reported clean: %+v", st)
	}
}

func TestBackupCreatesBranchAtHead(t *testing.T) {
	dir, _, second := twoCommitRepo(t)
	git := gitexec.New(dir)
	ctx := context.Background()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	name, err := Backup(ctx, git, now, "HEAD")
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if want := "backup/rewind-20260707-120000"; name != want {
		t.Fatalf("backup name = %q, want %q", name, want)
	}
	if tip := revParse(t, git, name); tip != second {
		t.Fatalf("backup points at %s, want HEAD %s", tip, second)
	}
}

func TestApplyDryRunChangesNothing(t *testing.T) {
	dir, _, second := twoCommitRepo(t)
	git := gitexec.New(dir)
	ctx := context.Background()

	plan := Plan{Commands: []Command{{Args: []string{"reset", "--hard", "HEAD~1"}, Explain: "discard the last commit"}}}

	res, err := Apply(ctx, git, plan, Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.DryRun {
		t.Error("expected a dry run")
	}
	if want := []string{"git reset --hard HEAD~1"}; !reflect.DeepEqual(res.Commands, want) {
		t.Errorf("Commands = %q, want %q", res.Commands, want)
	}
	if head := revParse(t, git, "HEAD"); head != second {
		t.Errorf("dry run moved HEAD to %s, want %s", head, second)
	}
	branches, err := git.Run(ctx, "branch", "--list", backupPrefix+"*")
	if err != nil {
		t.Fatalf("listing backup branches: %v", err)
	}
	if strings.TrimSpace(branches) != "" {
		t.Errorf("dry run created a backup branch: %q", branches)
	}
}

func TestApplyExecutesAfterBackup(t *testing.T) {
	dir, first, second := twoCommitRepo(t)
	git := gitexec.New(dir)
	ctx := context.Background()
	now := time.Date(2026, 7, 7, 9, 30, 0, 0, time.UTC)

	plan := Plan{Commands: []Command{{Args: []string{"reset", "--hard", "HEAD~1"}, Explain: "discard the last commit"}}}

	res, err := Apply(ctx, git, plan, Options{Execute: true, Now: now})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.DryRun {
		t.Error("expected a real run")
	}
	if want := "backup/rewind-20260707-093000"; res.BackupBranch != want {
		t.Fatalf("BackupBranch = %q, want %q", res.BackupBranch, want)
	}
	if tip := revParse(t, git, res.BackupBranch); tip != second {
		t.Errorf("backup points at %s, want pre-reset HEAD %s", tip, second)
	}
	if head := revParse(t, git, "HEAD"); head != first {
		t.Errorf("HEAD is %s after reset, want %s", head, first)
	}
}

// TestApplyNamesTheBackupWhenAPlanFailsPartway is the guard on git-rewind's core promise. A
// rescue that dies halfway has already created the backup branch, and the user must be told
// where it is — that is the exact moment they most need to know their work is safe.
func TestApplyReportsTheBackupWhenAPlanFailsPartway(t *testing.T) {
	dir, _, _ := twoCommitRepo(t)
	git := gitexec.New(dir)

	plan := Plan{
		Commands: []Command{
			{Args: []string{"branch", "applied-before-the-failure"}, Explain: "succeeds"},
			{Args: []string{"branch", "--no-such-flag"}, Explain: "fails"},
		},
		Warnings: []string{"test plan"},
	}

	now := time.Date(2026, 7, 7, 14, 15, 0, 0, time.UTC)
	res, err := Apply(context.Background(), git, plan, Options{Execute: true, Now: now})
	if err == nil {
		t.Fatal("expected the second command to fail")
	}

	var applyErr *ApplyError
	if !errors.As(err, &applyErr) {
		t.Fatalf("error is %T, want *ApplyError so callers can name the backup branch", err)
	}
	if applyErr.BackupBranch == "" || applyErr.BackupBranch != res.BackupBranch {
		t.Errorf("ApplyError.BackupBranch = %q, Result.BackupBranch = %q; both must name the backup",
			applyErr.BackupBranch, res.BackupBranch)
	}
	if !strings.Contains(applyErr.Error(), applyErr.BackupBranch) {
		t.Errorf("the error text does not mention the backup branch: %q", applyErr.Error())
	}
	if !strings.Contains(applyErr.Command, "--no-such-flag") {
		t.Errorf("ApplyError.Command = %q, want the command that failed", applyErr.Command)
	}
	if revParse(t, git, applyErr.BackupBranch) == "" {
		t.Error("the backup branch named in the error does not resolve")
	}
}

func twoCommitRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir = t.TempDir()
	run := func(args ...string) string {
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

	run("init", "-q", "-b", "main")
	run("config", "commit.gpgsign", "false")
	writeFile(t, dir, "one\n")
	run("add", "f.txt")
	run("commit", "-q", "-m", "first commit")
	first = run("rev-parse", "HEAD")
	writeFile(t, dir, "one\ntwo\n")
	run("commit", "-q", "-am", "second commit")
	second = run("rev-parse", "HEAD")
	return dir, first, second
}

func writeFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func revParse(t *testing.T, git *gitexec.Runner, ref string) string {
	t.Helper()
	out, err := git.Run(context.Background(), "rev-parse", ref)
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(out)
}
