package timeline

import (
	"context"
	"os/exec"
	"testing"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/internal/scenario"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		operation string
		want      Kind
	}{
		{"commit", KindCommit},
		{"commit (initial)", KindInitialCommit},
		{"commit (amend)", KindAmend},
		{"reset", KindReset},
		{"checkout", KindCheckout},
		{"merge feature", KindMerge},
		{"rebase (finish)", KindRebase},
		{"rebase -i (finish)", KindRebase},
		{"pull", KindPull},
		{"pull --rebase", KindPull},
		{"clone", KindClone},
		{"branch", KindBranch},
		{"cherry-pick", KindCherryPick},
		{"revert", KindRevert},
		{"gc", KindOther},
		{"", KindOther},
	}
	for _, c := range cases {
		if got := classify(c.operation); got != c.want {
			t.Errorf("classify(%q) = %v, want %v", c.operation, got, c.want)
		}
	}
}

func TestRiskOf(t *testing.T) {
	cases := []struct {
		kind Kind
		want Risk
	}{
		{KindReset, RiskRed},
		{KindRebase, RiskRed},
		{KindAmend, RiskRed},
		{KindCheckout, RiskYellow},
		{KindMerge, RiskYellow},
		{KindPull, RiskYellow},
		{KindCommit, RiskGreen},
		{KindInitialCommit, RiskGreen},
		{KindBranch, RiskGreen},
		{KindCherryPick, RiskGreen},
		{KindRevert, RiskGreen},
		{KindOther, RiskGreen},
	}
	for _, c := range cases {
		if got := riskOf(c.kind); got != c.want {
			t.Errorf("riskOf(%v) = %v, want %v", c.kind, got, c.want)
		}
	}
}

func TestRiskString(t *testing.T) {
	cases := map[Risk]string{RiskGreen: "green", RiskYellow: "yellow", RiskRed: "red", Risk(99): "unknown"}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("Risk(%d).String() = %q, want %q", r, got, want)
		}
	}
}

func TestFromReflogPreservesOrderAndData(t *testing.T) {
	entries := []gitexec.ReflogEntry{
		{Index: 0, Operation: "reset", Hash: "aaa"},
		{Index: 1, Operation: "commit (amend)", Hash: "bbb"},
		{Index: 2, Operation: "commit", Hash: "ccc"},
	}
	events := FromReflog(entries)

	if len(events) != len(entries) {
		t.Fatalf("got %d events, want %d", len(events), len(entries))
	}
	want := []struct {
		kind Kind
		risk Risk
	}{
		{KindReset, RiskRed},
		{KindAmend, RiskRed},
		{KindCommit, RiskGreen},
	}
	for i, w := range want {
		if events[i].Kind != w.kind || events[i].Risk != w.risk {
			t.Errorf("event %d = {%v, %v}, want {%v, %v}", i, events[i].Kind, events[i].Risk, w.kind, w.risk)
		}
		if events[i].Entry.Hash != entries[i].Hash {
			t.Errorf("event %d dropped its entry: hash %q, want %q", i, events[i].Entry.Hash, entries[i].Hash)
		}
	}
}

// TestFromReflogOnRealResetHard ties the parser and classifier together on a
// real repository: the reset-hard scenario must surface a red reset as the most
// recent event.
func TestFromReflogOnRealResetHard(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	var resetHard scenario.Scenario
	for _, s := range scenario.All() {
		if s.Name == "reset-hard" {
			resetHard = s
		}
	}
	if resetHard.Build == nil {
		t.Fatal("reset-hard scenario not found in registry")
	}

	built, err := resetHard.Build(t.TempDir())
	if err != nil {
		t.Fatalf("build reset-hard: %v", err)
	}

	entries, err := gitexec.New(built.Dir).Reflog(context.Background())
	if err != nil {
		t.Fatalf("Reflog: %v", err)
	}

	events := FromReflog(entries)
	if len(events) == 0 {
		t.Fatal("expected reflog events, got none")
	}
	if events[0].Kind != KindReset {
		t.Errorf("most recent event Kind = %v, want %v", events[0].Kind, KindReset)
	}
	if events[0].Risk != RiskRed {
		t.Errorf("most recent event Risk = %v, want %v", events[0].Risk, RiskRed)
	}
}
