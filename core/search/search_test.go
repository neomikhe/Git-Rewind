package search

import (
	"context"
	"os/exec"
	"testing"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/internal/scenario"
)

func resetHardRepo(t *testing.T) (*gitexec.Runner, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	for _, s := range scenario.All() {
		if s.Name != "reset-hard" {
			continue
		}
		built, err := s.Build(t.TempDir())
		if err != nil {
			t.Fatalf("build reset-hard: %v", err)
		}
		return gitexec.New(built.Dir), built.Anchors["lost"]
	}
	t.Fatal("reset-hard scenario not found")
	return nil, ""
}

func TestFindMatchesTheCommitMessage(t *testing.T) {
	git, lost := resetHardRepo(t)

	got, err := Find(context.Background(), git, "second commit", Options{})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got.Matches) != 1 {
		t.Fatalf("got %d matches, want 1: %+v", len(got.Matches), got.Matches)
	}
	m := got.Matches[0]
	if m.Commit.Hash != lost {
		t.Errorf("matched %s, want the lost commit %s", m.Commit.Hash, lost)
	}
	if !m.InMessage {
		t.Error("InMessage = false, want true for a message match")
	}
}

func TestFindMatchesFileContents(t *testing.T) {
	git, lost := resetHardRepo(t)

	got, err := Find(context.Background(), git, "two", Options{})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got.Matches) != 1 {
		t.Fatalf("got %d matches, want 1: %+v", len(got.Matches), got.Matches)
	}
	m := got.Matches[0]
	if m.Commit.Hash != lost {
		t.Errorf("matched %s, want the lost commit %s", m.Commit.Hash, lost)
	}
	if len(m.InFiles) == 0 {
		t.Fatal("InFiles is empty, want the matching line from the lost commit's tree")
	}
	if m.InFiles[0].Path != "file.txt" {
		t.Errorf("matched %s, want file.txt", m.InFiles[0].Path)
	}
}

func TestMessagesOnlySkipsFileContents(t *testing.T) {
	git, _ := resetHardRepo(t)

	got, err := Find(context.Background(), git, "two", Options{MessagesOnly: true})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got.Matches) != 0 {
		t.Errorf("got %d matches, want 0: contents must not be searched with MessagesOnly", len(got.Matches))
	}
}

func TestFindReportsNoMatch(t *testing.T) {
	git, _ := resetHardRepo(t)

	got, err := Find(context.Background(), git, "nothing-like-this-exists", Options{})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got.Matches) != 0 {
		t.Errorf("got %d matches, want 0", len(got.Matches))
	}
	if got.Orphans == 0 {
		t.Error("Orphans = 0, want the scenario's unreachable commit to have been counted")
	}
	if got.Truncated() {
		t.Error("Truncated = true with the default limit and one orphan")
	}
}

func TestEmptyQueryFindsNothingAndRunsNoGit(t *testing.T) {
	got, err := Find(context.Background(), nil, "   ", Options{})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got.Matches) != 0 || got.Orphans != 0 {
		t.Errorf("an empty query must short-circuit before touching git: %+v", got)
	}
}

func TestMaxCommitsTruncates(t *testing.T) {
	git, _ := resetHardRepo(t)

	got, err := Find(context.Background(), git, "second commit", Options{MaxCommits: 0})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Inspected != got.Orphans {
		t.Errorf("a non-positive MaxCommits must fall back to the default, not to zero: %+v", got)
	}
}
