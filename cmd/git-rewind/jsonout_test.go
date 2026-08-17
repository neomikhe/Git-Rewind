package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func decode[T any](t *testing.T, args []string, dir string) T {
	t.Helper()
	var buf bytes.Buffer
	if err := run(args, dir, &buf); err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	var out T
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("run %v produced invalid JSON: %v\n---\n%s", args, err, buf.String())
	}
	return out
}

func TestExplainJSON(t *testing.T) {
	dir, _ := resetHardRepo(t)
	got := decode[jsonExplain](t, []string{"explain", "--json"}, dir)

	if got.Schema != jsonSchema || got.Command != "explain" {
		t.Errorf("schema/command = %d/%q", got.Schema, got.Command)
	}
	if got.Head.Branch != "main" || got.Head.Detached || len(got.Head.Hash) != 40 {
		t.Errorf("head = %+v", got.Head)
	}
	if !got.WorkingTree.Clean || got.WorkingTree.Changes != 0 {
		t.Errorf("workingTree = %+v, want clean", got.WorkingTree)
	}
	if got.LastEvent == nil {
		t.Fatal("lastEvent is null, want the reset")
	}
	if got.LastEvent.Kind != "reset" || got.LastEvent.Risk != "red" {
		t.Errorf("lastEvent = %+v, want a red reset", got.LastEvent)
	}
	if len(got.LastEvent.Orphaned) != 1 {
		t.Errorf("lastEvent.orphaned = %v, want the discarded commit", got.LastEvent.Orphaned)
	}
	if got.Unreachable != 1 {
		t.Errorf("unreachableCommits = %d, want 1", got.Unreachable)
	}
	if got.Rescue == nil || got.Rescue.Name != "recover-after-reset-hard" {
		t.Errorf("rescue = %+v", got.Rescue)
	}
}

func TestExplainJSONOnAnEmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	hermeticGit(t, dir)("init", "-q", "-b", "main")

	got := decode[jsonExplain](t, []string{"explain", "--json"}, dir)
	if got.Schema != jsonSchema || got.Command != "explain" {
		t.Errorf("an empty repository must still produce a valid document: %+v", got)
	}
	if got.LastEvent != nil {
		t.Errorf("lastEvent = %+v, want null", got.LastEvent)
	}
}

func TestFindJSON(t *testing.T) {
	dir, lost := resetHardRepo(t)
	got := decode[jsonFind](t, []string{"find", "two", "--json"}, dir)

	if got.Schema != jsonSchema || got.Command != "find" || got.Query != "two" {
		t.Errorf("schema/command/query = %d/%q/%q", got.Schema, got.Command, got.Query)
	}
	if got.Truncated {
		t.Error("truncated = true with one orphan and the default limit")
	}
	if len(got.Matches) != 1 {
		t.Fatalf("got %d matches, want 1: %+v", len(got.Matches), got.Matches)
	}

	m := got.Matches[0]
	if m.Hash != lost || m.ShortHash != shortHash(lost) {
		t.Errorf("match hash = %s/%s, want %s", m.Hash, m.ShortHash, lost)
	}
	if len(m.Files) == 0 || m.Files[0].Path != "file.txt" {
		t.Errorf("files = %+v, want the matching line in file.txt", m.Files)
	}
	want := []string{"git", "branch", rescuePrefix + shortHash(lost), lost}
	if strings.Join(m.KeepCommand, " ") != strings.Join(want, " ") {
		t.Errorf("keepCommand = %v, want %v", m.KeepCommand, want)
	}
}

func TestFindJSONWithNoMatchesIsAnEmptyList(t *testing.T) {
	dir, _ := resetHardRepo(t)
	got := decode[jsonFind](t, []string{"find", "nothing-like-this", "--json"}, dir)

	if got.Matches == nil {
		t.Error("matches is null; an empty result must marshal as [] so consumers can range over it")
	}
	if len(got.Matches) != 0 {
		t.Errorf("got %d matches, want 0", len(got.Matches))
	}
}

func TestLastJSONDryRun(t *testing.T) {
	dir, _ := resetHardRepo(t)
	before := headHash(t, dir)

	got := decode[jsonLast](t, []string{"last", "--json"}, dir)

	if !got.DryRun || got.Applied {
		t.Errorf("dryRun/applied = %v/%v, want true/false", got.DryRun, got.Applied)
	}
	if got.BackupBranch != "" {
		t.Errorf("backupBranch = %q, want empty on a dry run", got.BackupBranch)
	}
	if got.Rescue == nil || got.Rescue.Name != "recover-after-reset-hard" {
		t.Errorf("rescue = %+v", got.Rescue)
	}
	if len(got.Commands) != 1 || !strings.HasPrefix(got.Commands[0].Preview, "git reset --hard") {
		t.Errorf("commands = %+v", got.Commands)
	}
	if !got.DiscardsChanges {
		t.Error("discardsChanges = false, want true for a reset --hard rescue")
	}
	if head := headHash(t, dir); head != before {
		t.Errorf("a JSON dry run moved HEAD to %s", head)
	}
}

func TestLastJSONApplied(t *testing.T) {
	dir, lost := resetHardRepo(t)

	got := decode[jsonLast](t, []string{"last", "--yes", "--json"}, dir)

	if got.DryRun || !got.Applied {
		t.Errorf("dryRun/applied = %v/%v, want false/true", got.DryRun, got.Applied)
	}
	if !strings.HasPrefix(got.BackupBranch, "backup/rewind-") {
		t.Errorf("backupBranch = %q, want a backup branch name", got.BackupBranch)
	}
	if head := headHash(t, dir); head != lost {
		t.Errorf("HEAD = %s after the rescue, want %s", head, lost)
	}
}

func TestLastJSONWhenNothingApplies(t *testing.T) {
	dir := healthyRepo(t)
	got := decode[jsonLast](t, []string{"last", "--json"}, dir)

	if got.Rescue != nil {
		t.Errorf("rescue = %+v, want null when nothing applies", got.Rescue)
	}
	if got.Schema != jsonSchema || got.Command != "last" {
		t.Errorf("schema/command = %d/%q", got.Schema, got.Command)
	}
}

func TestJSONFlagIsAcceptedBeforeOrAfterTheQuery(t *testing.T) {
	dir, _ := resetHardRepo(t)

	before := decode[jsonFind](t, []string{"find", "--json", "two"}, dir)
	after := decode[jsonFind](t, []string{"find", "two", "--json"}, dir)

	if before.Query != "two" || after.Query != "two" {
		t.Errorf("query = %q before and %q after; a flag must never end up in the search text",
			before.Query, after.Query)
	}
	if len(before.Matches) != len(after.Matches) {
		t.Errorf("flag position changed the result: %d vs %d matches", len(before.Matches), len(after.Matches))
	}
}

func TestSplitLeadingText(t *testing.T) {
	cases := []struct {
		name string
		args []string
		text string
		rest []string
	}{
		{"text then flags", []string{"foo", "--json"}, "foo", []string{"--json"}},
		{"flags then text", []string{"--json", "foo"}, "", []string{"--json", "foo"}},
		{"text with a flag value after", []string{"foo", "--limit", "50"}, "foo", []string{"--limit", "50"}},
		{"only text", []string{"foo", "bar"}, "foo bar", nil},
		{"terminator", []string{"--", "--literal"}, "", []string{"--", "--literal"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, rest := splitLeadingText(c.args)
			if strings.Join(text, " ") != c.text {
				t.Errorf("text = %v, want %q", text, c.text)
			}
			if strings.Join(rest, " ") != strings.Join(c.rest, " ") {
				t.Errorf("rest = %v, want %v", rest, c.rest)
			}
		})
	}
}
