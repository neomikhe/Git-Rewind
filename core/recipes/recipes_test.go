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
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
	"github.com/neomikhe/git-rewind/internal/scenario"
)

var fixedNow = time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)

func TestAllRecipesAreWellFormed(t *testing.T) {
	seen := make(map[string]bool)
	for _, r := range All() {
		if r.Name() == "" || r.Title() == "" {
			t.Errorf("recipe %T has an empty name or title", r)
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

	res, err := safety.Apply(context.Background(), repo.Git, *plan, safety.Options{}) // dry run
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
	dir, _ := twoCommitRepo(t) // a healthy repo: no amend, reset, merge, or rebase
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

// TestResetRecipesRequireRecentMistake guards against the reset-based rescues
// moving an unrelated branch: after the mistake, switching branches must stop
// them from applying, because they would otherwise reset whatever branch is
// currently checked out.
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
	run("merge", "-q", "feature") // fast-forward: no merge commit is created
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

// --- test helpers ---

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
	events, err := timeline.Load(context.Background(), git)
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

// newRepo initializes an empty repository with a hermetic git environment and
// returns its path and a runner for further git commands.
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
	dir, run := newRepo(t)
	writeFile(t, dir, "one\n")
	run("add", "f.txt")
	run("commit", "-q", "-m", "first commit")
	first = run("rev-parse", "HEAD")
	writeFile(t, dir, "one\ntwo\n")
	run("commit", "-q", "-am", "second commit")
	return dir, first
}

func writeFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
