package scenario

import (
	"fmt"
	"strings"
)

// All returns every scenario git-rewind can reproduce, in a stable order.
func All() []Scenario {
	return []Scenario{
		{
			Name:        "reset-hard",
			Description: "git reset --hard HEAD~1 discarded the latest commit",
			Build:       buildResetHard,
			Verify:      verifyResetHard,
		},
		{
			Name:        "amend",
			Description: "git commit --amend replaced the last commit",
			Build:       buildAmend,
			Verify:      verifyAmend,
		},
		{
			Name:        "deleted-branch",
			Description: "git branch -D removed a branch that still had unmerged work",
			Build:       buildDeletedBranch,
			Verify:      verifyDeletedBranch,
		},
		{
			Name:        "rebase-rewrite",
			Description: "git rebase rewrote the branch, orphaning the pre-rebase commit",
			Build:       buildRebaseRewrite,
			Verify:      verifyRebaseRewrite,
		},
		{
			Name:        "pre-merge",
			Description: "a merge commit that should be undone back to its first parent",
			Build:       buildPreMerge,
			Verify:      verifyPreMerge,
		},
		{
			Name:        "detached-head",
			Description: "HEAD was checked out to a commit instead of a branch",
			Build:       buildDetachedHead,
			Verify:      verifyDetachedHead,
		},
		{
			Name:        "dropped-stash",
			Description: "git stash drop removed a stash still recoverable via fsck",
			Build:       buildDroppedStash,
			Verify:      verifyDroppedStash,
		},
	}
}

// buildResetHard sets "lost" to the commit discarded by reset --hard.
func buildResetHard(dir string) (Built, error) {
	r := newRepo(dir)
	r.commit("file.txt", "one\n", "first commit")
	lost := r.commit("file.txt", "one\ntwo\n", "second commit")
	r.git("reset", "--hard", "HEAD~1")
	return r.done(map[string]string{"lost": lost})
}

func verifyResetHard(b Built) error {
	lost := b.Anchors["lost"]
	if err := requireCommit(b.Dir, lost); err != nil {
		return err
	}
	if gitSucceeds(b.Dir, "merge-base", "--is-ancestor", lost, "HEAD") {
		return fmt.Errorf("commit %s is still reachable from HEAD", short(lost))
	}
	return nil
}

// buildAmend sets "original" (pre-amend commit) and "amended" (new HEAD).
func buildAmend(dir string) (Built, error) {
	r := newRepo(dir)
	r.commit("file.txt", "one\n", "first commit")
	original := r.commit("file.txt", "one\ntwo\n", "second commit")
	r.writeFile("file.txt", "one\ntwo-fixed\n")
	r.git("add", "file.txt")
	r.git("commit", "--amend", "-q", "-m", "second commit (fixed)")
	amended := r.git("rev-parse", "HEAD")
	return r.done(map[string]string{"original": original, "amended": amended})
}

func verifyAmend(b Built) error {
	original, amended := b.Anchors["original"], b.Anchors["amended"]
	if original == amended {
		return fmt.Errorf("amend did not change the commit hash")
	}
	if err := requireHead(b.Dir, amended); err != nil {
		return err
	}
	if err := requireCommit(b.Dir, original); err != nil {
		return err
	}
	if gitSucceeds(b.Dir, "merge-base", "--is-ancestor", original, "HEAD") {
		return fmt.Errorf("original commit %s is still reachable from HEAD", short(original))
	}
	return nil
}

// buildDeletedBranch sets "branch" (name) and "tip" (its last commit).
func buildDeletedBranch(dir string) (Built, error) {
	r := newRepo(dir)
	r.commit("base.txt", "base\n", "base commit")
	r.git("checkout", "-q", "-b", "feature")
	tip := r.commit("feature.txt", "work\n", "feature work")
	r.git("checkout", "-q", "main")
	r.git("branch", "-D", "feature")
	return r.done(map[string]string{"branch": "feature", "tip": tip})
}

func verifyDeletedBranch(b Built) error {
	if gitSucceeds(b.Dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+b.Anchors["branch"]) {
		return fmt.Errorf("branch %q still exists", b.Anchors["branch"])
	}
	return requireCommit(b.Dir, b.Anchors["tip"])
}

// buildRebaseRewrite sets "preRebase" (original tip) and "newTip" (post-rebase).
func buildRebaseRewrite(dir string) (Built, error) {
	r := newRepo(dir)
	r.commit("base.txt", "base\n", "base commit")
	r.git("checkout", "-q", "-b", "feature")
	preRebase := r.commit("feature.txt", "f\n", "feature commit")
	r.git("checkout", "-q", "main")
	r.commit("main.txt", "m\n", "main commit")
	r.git("checkout", "-q", "feature")
	r.git("rebase", "-q", "main")
	newTip := r.git("rev-parse", "HEAD")
	return r.done(map[string]string{"preRebase": preRebase, "newTip": newTip})
}

func verifyRebaseRewrite(b Built) error {
	pre, newTip := b.Anchors["preRebase"], b.Anchors["newTip"]
	if pre == newTip {
		return fmt.Errorf("rebase did not rewrite the commit")
	}
	if err := requireHead(b.Dir, newTip); err != nil {
		return err
	}
	if err := requireCommit(b.Dir, pre); err != nil {
		return err
	}
	if gitSucceeds(b.Dir, "merge-base", "--is-ancestor", pre, "HEAD") {
		return fmt.Errorf("pre-rebase commit %s is still reachable from HEAD", short(pre))
	}
	return nil
}

// buildPreMerge sets "preMerge" (first parent) and "merge" (the merge commit).
func buildPreMerge(dir string) (Built, error) {
	r := newRepo(dir)
	r.commit("base.txt", "base\n", "base commit")
	r.git("checkout", "-q", "-b", "feature")
	r.commit("feature.txt", "f\n", "feature commit")
	r.git("checkout", "-q", "main")
	preMerge := r.commit("main.txt", "m\n", "main commit")
	r.git("merge", "--no-edit", "-m", "merge feature into main", "feature")
	merge := r.git("rev-parse", "HEAD")
	return r.done(map[string]string{"preMerge": preMerge, "merge": merge})
}

func verifyPreMerge(b Built) error {
	ps, err := parents(b.Dir, "HEAD")
	if err != nil {
		return err
	}
	if len(ps) != 2 {
		return fmt.Errorf("HEAD has %d parents, want 2 (not a merge commit)", len(ps))
	}
	if ps[0] != b.Anchors["preMerge"] {
		return fmt.Errorf("first parent %s != pre-merge %s", short(ps[0]), short(b.Anchors["preMerge"]))
	}
	return nil
}

// buildDetachedHead sets "commit" to the detached commit HEAD points at.
func buildDetachedHead(dir string) (Built, error) {
	r := newRepo(dir)
	first := r.commit("file.txt", "one\n", "first commit")
	r.commit("file.txt", "one\ntwo\n", "second commit")
	r.git("checkout", "-q", first)
	return r.done(map[string]string{"commit": first})
}

func verifyDetachedHead(b Built) error {
	if gitSucceeds(b.Dir, "symbolic-ref", "-q", "HEAD") {
		return fmt.Errorf("HEAD is attached to a branch, want detached")
	}
	return requireHead(b.Dir, b.Anchors["commit"])
}

// buildDroppedStash sets "stash" to the dropped stash commit.
func buildDroppedStash(dir string) (Built, error) {
	r := newRepo(dir)
	r.commit("file.txt", "base\n", "base commit")
	r.writeFile("file.txt", "base\nwip\n")
	r.git("stash", "push", "-q", "-m", "wip changes")
	stash := r.git("rev-parse", "stash@{0}")
	r.git("stash", "drop", "-q", "stash@{0}")
	return r.done(map[string]string{"stash": stash})
}

func verifyDroppedStash(b Built) error {
	list, err := gitOut(b.Dir, "stash", "list")
	if err != nil {
		return err
	}
	if strings.TrimSpace(list) != "" {
		return fmt.Errorf("stash list is not empty: %q", list)
	}
	return requireCommit(b.Dir, b.Anchors["stash"])
}

// requireHead returns an error unless HEAD resolves to want.
func requireHead(dir, want string) error {
	head, err := gitOut(dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != want {
		return fmt.Errorf("HEAD is %s, want %s", short(head), short(want))
	}
	return nil
}
