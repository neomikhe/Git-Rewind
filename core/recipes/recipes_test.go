package recipes

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
	"github.com/neomikhe/git-rewind/internal/scenario"
)

var fixedNow = time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)

func TestAllRecipesAreWellFormed(t *testing.T) {
	seen := make(map[string]bool)
	for _, r := range All() {
		if r.Name() == "" {
			t.Errorf("recipe %T has an empty name", r)
		}
		for _, lang := range []i18n.Lang{i18n.EN, i18n.ES} {
			if r.Title(i18n.New(lang)) == "" {
				t.Errorf("recipe %q has no title in %q", r.Name(), lang)
			}
		}
		if seen[r.Name()] {
			t.Errorf("duplicate recipe name %q", r.Name())
		}
		seen[r.Name()] = true
	}
}

func TestUndoLastCommitSoftKeepsParent(t *testing.T) {
	dir, first := twoCommitRepo(t)
	repo := loadRepo(t, dir)

	runRecipe(t, repo, UndoLastCommit{})

	if head := revParse(t, repo.Git, "HEAD"); head != first {
		t.Errorf("HEAD = %s after undo, want first commit %s", head, first)
	}
}

func TestUndoLastCommitHardKeepsParent(t *testing.T) {
	dir, first := twoCommitRepo(t)
	repo := loadRepo(t, dir)

	runRecipe(t, repo, UndoLastCommitHard{})

	if head := revParse(t, repo.Git, "HEAD"); head != first {
		t.Errorf("HEAD = %s after undo, want first commit %s", head, first)
	}
}

func TestRestoreDroppedStashPutsItBackWithoutTouchingTheTree(t *testing.T) {
	built := buildScenario(t, "dropped-stash")
	repo := loadRepo(t, built.Dir)

	before := revParse(t, repo.Git, "HEAD")
	runRecipe(t, repo, RestoreDroppedStash{})

	list, err := repo.Git.Run(context.Background(), "stash", "list")
	if err != nil {
		t.Fatalf("stash list: %v", err)
	}
	if strings.TrimSpace(list) == "" {
		t.Fatal("the stash list is still empty after the rescue")
	}
	if got := revParse(t, repo.Git, "stash@{0}"); got != built.Anchors["stash"] {
		t.Errorf("stash@{0} = %s, want the dropped stash %s", got, built.Anchors["stash"])
	}
	if head := revParse(t, repo.Git, "HEAD"); head != before {
		t.Errorf("restoring a stash moved HEAD from %s to %s", before, head)
	}

	status, err := repo.Git.Run(context.Background(), "status", "--porcelain")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.TrimSpace(status) != "" {
		t.Errorf("restoring a stash must not touch the working tree, got:\n%s", status)
	}
}

func TestRestoreDroppedStashDoesNotApplyWithoutOne(t *testing.T) {
	dir, _ := twoCommitRepo(t)
	repo := loadRepo(t, dir)

	plan, err := RestoreDroppedStash{}.Detect(context.Background(), repo)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan != nil {
		t.Error("restore-dropped-stash applied to a repository with no dropped stash")
	}
}

func TestLooksLikeStashNeedsBothParentsAndSubject(t *testing.T) {
	cases := []struct {
		name   string
		commit gitexec.Commit
		want   bool
	}{
		{"stash created with a message", gitexec.Commit{Parents: []string{"a", "b"}, Subject: "On main: work"}, true},
		{"stash created without one", gitexec.Commit{Parents: []string{"a", "b"}, Subject: "WIP on main: 1234 base"}, true},
		{"a plain merge commit", gitexec.Commit{Parents: []string{"a", "b"}, Subject: "Merge branch feature"}, false},
		{"an ordinary commit that starts like a stash", gitexec.Commit{Parents: []string{"a"}, Subject: "On main: not a stash"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksLikeStash(c.commit); got != c.want {
				t.Errorf("looksLikeStash = %v, want %v", got, c.want)
			}
		})
	}
}

func TestUndoAmendRestoresOriginal(t *testing.T) {
	built := buildScenario(t, "amend")
	repo := loadRepo(t, built.Dir)

	runRecipe(t, repo, UndoAmend{})

	if head := revParse(t, repo.Git, "HEAD"); head != built.Anchors["original"] {
		t.Errorf("HEAD = %s after undo-amend, want original %s", head, built.Anchors["original"])
	}
}

func TestRecoverAfterResetHardRestoresLostCommit(t *testing.T) {
	built := buildScenario(t, "reset-hard")
	repo := loadRepo(t, built.Dir)

	runRecipe(t, repo, RecoverAfterResetHard{})

	if head := revParse(t, repo.Git, "HEAD"); head != built.Anchors["lost"] {
		t.Errorf("HEAD = %s after recovery, want lost commit %s", head, built.Anchors["lost"])
	}
}

func TestRestoreDeletedBranchRecreatesRef(t *testing.T) {
	built := buildScenario(t, "deleted-branch")
	repo := loadRepo(t, built.Dir)

	runRecipe(t, repo, RestoreDeletedBranch{})

	tip := revParse(t, repo.Git, "refs/heads/"+built.Anchors["branch"])
	if tip != built.Anchors["tip"] {
		t.Errorf("restored branch points at %s, want tip %s", tip, built.Anchors["tip"])
	}
}

func TestRestoreDeletedBranchRefusesFlagLikeNames(t *testing.T) {
	cases := []struct {
		name     string
		checkout []string
		want     string
	}{
		{"ordinary name", []string{"gone"}, "gone"},
		{"long flag", []string{"--orphan"}, ""},
		{"short flag", []string{"-D"}, ""},
		{"flag over a real deletion", []string{"gone", "-D"}, "gone"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, first, run := twoCommitRepoWithRunner(t)
			targets := []string{first, run("rev-parse", "HEAD")}
			for i, from := range c.checkout {
				run("update-ref", "-m", "checkout: moving from "+from+" to main", "HEAD", targets[i%2])
			}

			plan, err := RestoreDeletedBranch{}.Detect(context.Background(), loadRepo(t, dir))
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := ""
			if plan != nil {
				got = plan.Commands[0].Args[1]
			}
			if got != c.want {
				t.Errorf("plan restores %q, want %q", got, c.want)
			}
		})
	}
}

func TestUndoMergeRestoresPreMerge(t *testing.T) {
	built := buildScenario(t, "pre-merge")
	repo := loadRepo(t, built.Dir)

	runRecipe(t, repo, UndoMerge{})

	if head := revParse(t, repo.Git, "HEAD"); head != built.Anchors["preMerge"] {
		t.Errorf("HEAD = %s after undo-merge, want preMerge %s", head, built.Anchors["preMerge"])
	}
}

func TestUndoRebaseRestoresPreRebase(t *testing.T) {
	built := buildScenario(t, "rebase-rewrite")
	repo := loadRepo(t, built.Dir)

	runRecipe(t, repo, UndoRebase{})

	if head := revParse(t, repo.Git, "HEAD"); head != built.Anchors["preRebase"] {
		t.Errorf("HEAD = %s after undo-rebase, want preRebase %s", head, built.Anchors["preRebase"])
	}
}

func TestUndoRebaseIgnoresAnOlderRebaseOnAnotherBranch(t *testing.T) {
	dir, run := newRepo(t)
	commitNamedFile(t, dir, run, "base.txt", "base")
	run("checkout", "-q", "-b", "old")
	commitNamedFile(t, dir, run, "old.txt", "old work")
	run("checkout", "-q", "main")
	commitNamedFile(t, dir, run, "main1.txt", "main one")
	run("checkout", "-q", "old")
	run("rebase", "-q", "main")
	run("checkout", "-q", "-b", "feature", "main")
	commitNamedFile(t, dir, run, "feature.txt", "feature work")
	run("branch", "keep")
	run("checkout", "-q", "main")
	commitNamedFile(t, dir, run, "main2.txt", "main two")
	run("checkout", "-q", "feature")
	run("rebase", "-q", "main")

	plan, err := UndoRebase{}.Detect(context.Background(), loadRepo(t, dir))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan != nil {
		t.Errorf("offered %v after a rebase that stranded nothing, want no plan", plan.Commands[0].Args)
	}
}

func TestUndoRebaseTrustsOnlyTheOrphanOfTheRebaseStart(t *testing.T) {
	const keepTodo, stopAtEnd = "sequence.editor=echo >>", "sequence.editor=echo break >>"
	cases := []struct {
		name     string
		rebase   func(run func(args ...string) string)
		wantPlan bool
	}{
		{"fixup squashed during the rebase", func(run func(args ...string) string) {
			run("-c", keepTodo, "rebase", "-q", "-i", "--autosquash", "main")
		}, true},
		{"commit amended while the rebase was stopped", func(run func(args ...string) string) {
			run("-c", stopAtEnd, "rebase", "-q", "-i", "main")
			run("commit", "-q", "--amend", "-m", "amended while the rebase was stopped")
			run("rebase", "--continue")
		}, true},
		{"rebase aborted", func(run func(args ...string) string) {
			run("-c", stopAtEnd, "rebase", "-q", "-i", "main")
			run("rebase", "--abort")
		}, false},
		{"rebase still in progress", func(run func(args ...string) string) {
			run("-c", stopAtEnd, "rebase", "-q", "-i", "main")
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, run := newRepo(t)
			commitNamedFile(t, dir, run, "base.txt", "base")
			run("checkout", "-q", "-b", "feature")
			commitNamedFile(t, dir, run, "a.txt", "feature A")
			commitNamedFile(t, dir, run, "b.txt", "feature B")
			commitNamedFile(t, dir, run, "a.txt", "fixup! feature A")
			run("checkout", "-q", "main")
			commitNamedFile(t, dir, run, "main.txt", "main moved on")
			run("checkout", "-q", "feature")
			preRebase := run("rev-parse", "HEAD")

			c.rebase(run)

			plan, err := UndoRebase{}.Detect(context.Background(), loadRepo(t, dir))
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			switch {
			case !c.wantPlan && plan != nil:
				t.Errorf("offered %v, want no plan", plan.Commands[0].Args)
			case c.wantPlan && plan == nil:
				t.Error("no plan offered, want a reset to the pre-rebase tip")
			case c.wantPlan && plan.Commands[0].Args[2] != preRebase:
				t.Errorf("plan resets to %s, want the pre-rebase tip %s", plan.Commands[0].Args[2], preRebase)
			}
		})
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	built := buildScenario(t, "reset-hard")
	repo := loadRepo(t, built.Dir)
	before := revParse(t, repo.Git, "HEAD")

	plan, err := RecoverAfterResetHard{}.Detect(context.Background(), repo)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan == nil {
		t.Fatal("recipe did not apply to its scenario")
	}

	res, err := safety.Apply(context.Background(), repo.Git, *plan, safety.Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.DryRun {
		t.Error("expected a dry run")
	}
	if head := revParse(t, repo.Git, "HEAD"); head != before {
		t.Errorf("dry run moved HEAD from %s to %s", before, head)
	}
}

func TestDetectReturnsNilWhenNotApplicable(t *testing.T) {
	dir, _ := twoCommitRepo(t)
	repo := loadRepo(t, dir)

	for _, r := range []Recipe{UndoAmend{}, RecoverAfterResetHard{}, UndoMerge{}, UndoRebase{}, RestoreDeletedBranch{}} {
		plan, err := r.Detect(context.Background(), repo)
		if err != nil {
			t.Fatalf("%s Detect: %v", r.Name(), err)
		}
		if plan != nil {
			t.Errorf("%s applied to a healthy repo, want no plan", r.Name())
		}
	}
}

func TestResetRecipesRequireRecentMistake(t *testing.T) {
	cases := []struct {
		scenario string
		recipe   Recipe
	}{
		{"reset-hard", RecoverAfterResetHard{}},
		{"amend", UndoAmend{}},
		{"rebase-rewrite", UndoRebase{}},
	}
	for _, c := range cases {
		t.Run(c.recipe.Name(), func(t *testing.T) {
			built := buildScenario(t, c.scenario)
			git := gitexec.New(built.Dir)
			if _, err := git.Run(context.Background(), "checkout", "-b", "somewhere-else"); err != nil {
				t.Fatalf("checkout: %v", err)
			}

			repo := loadRepo(t, built.Dir)
			plan, err := c.recipe.Detect(context.Background(), repo)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if plan != nil {
				t.Errorf("%s applied after switching branches; want no plan", c.recipe.Name())
			}
		})
	}
}

func TestUndoLastCommitNeedsAParent(t *testing.T) {
	dir, run := newRepo(t)
	writeFile(t, dir, "only\n")
	run("add", "f.txt")
	run("commit", "-q", "-m", "only commit")
	repo := loadRepo(t, dir)

	for _, r := range []Recipe{UndoLastCommit{}, UndoLastCommitHard{}} {
		plan, err := r.Detect(context.Background(), repo)
		if err != nil {
			t.Fatalf("%s Detect: %v", r.Name(), err)
		}
		if plan != nil {
			t.Errorf("%s applied to a root-only repo; want no plan", r.Name())
		}
	}
}

func TestUndoMergeIgnoresFastForward(t *testing.T) {
	dir, run := newRepo(t)
	writeFile(t, dir, "a\n")
	run("add", "f.txt")
	run("commit", "-q", "-m", "A")
	run("checkout", "-q", "-b", "feature")
	writeFile(t, dir, "a\nb\n")
	run("commit", "-q", "-am", "B")
	run("checkout", "-q", "main")
	run("merge", "-q", "feature")
	repo := loadRepo(t, dir)

	plan, err := UndoMerge{}.Detect(context.Background(), repo)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan != nil {
		t.Error("undo-merge applied to a fast-forward merge; want no plan")
	}
}

func TestRescueBacksUpPreRescueState(t *testing.T) {
	built := buildScenario(t, "reset-hard")
	repo := loadRepo(t, built.Dir)
	before := revParse(t, repo.Git, "HEAD")

	plan, err := RecoverAfterResetHard{}.Detect(context.Background(), repo)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan == nil {
		t.Fatal("recipe did not apply to its scenario")
	}
	res, err := safety.Apply(context.Background(), repo.Git, *plan, safety.Options{Execute: true, Now: fixedNow})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if backup := revParse(t, repo.Git, res.BackupBranch); backup != before {
		t.Errorf("backup captured %s, want the pre-rescue HEAD %s", backup, before)
	}
	if head := revParse(t, repo.Git, "HEAD"); head != built.Anchors["lost"] {
		t.Errorf("HEAD = %s after recovery, want %s", head, built.Anchors["lost"])
	}
}

func runRecipe(t *testing.T, repo *Repo, r Recipe) {
	t.Helper()
	plan, err := r.Detect(context.Background(), repo)
	if err != nil {
		t.Fatalf("%s Detect: %v", r.Name(), err)
	}
	if plan == nil {
		t.Fatalf("%s did not apply to its scenario", r.Name())
	}
	res, err := safety.Apply(context.Background(), repo.Git, *plan, safety.Options{Execute: true, Now: fixedNow})
	if err != nil {
		t.Fatalf("%s Apply: %v", r.Name(), err)
	}
	if res.BackupBranch == "" {
		t.Errorf("%s executed without creating a backup branch", r.Name())
	}
}

func buildScenario(t *testing.T, name string) scenario.Built {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	for _, s := range scenario.All() {
		if s.Name == name {
			built, err := s.Build(t.TempDir())
			if err != nil {
				t.Fatalf("build scenario %q: %v", name, err)
			}
			return built
		}
	}
	t.Fatalf("scenario %q not found", name)
	return scenario.Built{}
}

func loadRepo(t *testing.T, dir string) *Repo {
	t.Helper()
	git := gitexec.New(dir)
	events, err := timeline.Load(context.Background(), git, 0)
	if err != nil {
		t.Fatalf("load timeline: %v", err)
	}
	return &Repo{Git: git, Events: events}
}

func revParse(t *testing.T, git *gitexec.Runner, ref string) string {
	t.Helper()
	out, err := git.Run(context.Background(), "rev-parse", ref)
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(out)
}

func newRepo(t *testing.T) (dir string, run func(args ...string) string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir = t.TempDir()
	run = func(args ...string) string {
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
	return dir, run
}

func twoCommitRepo(t *testing.T) (dir, first string) {
	t.Helper()
	dir, first, _ = twoCommitRepoWithRunner(t)
	return dir, first
}

func twoCommitRepoWithRunner(t *testing.T) (dir, first string, run func(args ...string) string) {
	t.Helper()
	dir, run = newRepo(t)
	writeFile(t, dir, "one\n")
	run("add", "f.txt")
	run("commit", "-q", "-m", "first commit")
	first = run("rev-parse", "HEAD")
	writeFile(t, dir, "one\ntwo\n")
	run("commit", "-q", "-am", "second commit")
	return dir, first, run
}

func commitNamedFile(t *testing.T, dir string, run func(args ...string) string, name, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(msg+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", name)
	run("commit", "-q", "-m", msg)
}

func writeFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
