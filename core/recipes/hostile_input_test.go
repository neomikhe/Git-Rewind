package recipes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreDroppedStashIgnoresAMergeThatOnlyLooksLikeOne(t *testing.T) {
	dir, first, run := twoCommitRepoWithRunner(t)
	tree := run("rev-parse", "HEAD^{tree}")
	run("commit-tree", tree, "-p", "HEAD", "-p", first, "-m", "On main: not a stash")

	assertNoStashOffered(t, dir)
}

func TestRestoreDroppedStashIgnoresAStashShapeOverABaseNeverCheckedOut(t *testing.T) {
	dir, _, run := twoCommitRepoWithRunner(t)
	tree := run("rev-parse", "HEAD^{tree}")
	base := run("commit-tree", tree, "-p", "HEAD", "-m", "never checked out")
	index := run("commit-tree", tree, "-p", base, "-m", "index on main: 1234567 never checked out")
	run("commit-tree", tree, "-p", base, "-p", index, "-m", "On main: crafted")

	assertNoStashOffered(t, dir)
}

func TestRestoreDroppedStashRecognisesAStashWithUntrackedFiles(t *testing.T) {
	dir, _, run := twoCommitRepoWithRunner(t)
	writeFile(t, dir, "changed\n")
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("stash", "push", "-u", "-m", "with untracked")
	run("stash", "drop")

	plan, err := RestoreDroppedStash{}.Detect(context.Background(), loadRepo(t, dir))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan == nil {
		t.Fatal("a dropped stash with untracked files was not offered")
	}
}

func TestRestoreDroppedStashKeepsControlCharactersOutOfTheCommand(t *testing.T) {
	dir, _, run := twoCommitRepoWithRunner(t)
	writeFile(t, dir, "changed\n")
	run("stash", "push", "-m", "colour \x1b]52;c;ZXZpbA==\x07 red")
	run("stash", "drop")

	plan, err := RestoreDroppedStash{}.Detect(context.Background(), loadRepo(t, dir))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan == nil {
		t.Fatal("the dropped stash was not offered")
	}
	shown := append(append([]string{}, plan.Commands[0].Args...), plan.Warnings...)
	for _, text := range shown {
		if strings.ContainsAny(text, "\x1b\x07") {
			t.Errorf("plan text carries a terminal control character: %q", text)
		}
	}
}

func TestRestoreDeletedBranchRefusesUnprintableNames(t *testing.T) {
	for _, name := range []string{"evil\u202egnp", "tab\u2066bed", "bell\x07"} {
		t.Run(name, func(t *testing.T) {
			dir, first, run := twoCommitRepoWithRunner(t)
			run("update-ref", "-m", "checkout: moving from "+name+" to main", "HEAD", first)

			plan, err := RestoreDeletedBranch{}.Detect(context.Background(), loadRepo(t, dir))
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if plan != nil {
				t.Errorf("offered %q, want no plan for a name a terminal would render deceptively", plan.Commands[0].Args)
			}
		})
	}
}

func assertNoStashOffered(t *testing.T, dir string) {
	t.Helper()
	plan, err := RestoreDroppedStash{}.Detect(context.Background(), loadRepo(t, dir))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan != nil {
		t.Errorf("offered %q as a dropped stash, want no plan", plan.Commands[0].Args)
	}
}

func TestRestoreDroppedStashDoesNotDependOnTheLoadedPage(t *testing.T) {
	dir, _, run := twoCommitRepoWithRunner(t)
	writeFile(t, dir, "changed\n")
	run("stash", "push", "-m", "work in progress")
	run("stash", "drop")
	commitNamedFile(t, dir, run, "later1.txt", "later one")
	commitNamedFile(t, dir, run, "later2.txt", "later two")

	repo := loadRepo(t, dir)
	repo.Events = repo.Events[:1]

	plan, err := RestoreDroppedStash{}.Detect(context.Background(), repo)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if plan == nil {
		t.Error("a dropped stash whose base is past the loaded page was not offered")
	}
}
