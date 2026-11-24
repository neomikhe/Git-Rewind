package main

import (
	"strings"
	"testing"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/search"
)

const terminalControls = "\x1b\x07\u202e"

func TestFindOutputCarriesNoTerminalControls(t *testing.T) {
	result := search.Result{
		Query:     "needle",
		Orphans:   1,
		Inspected: 1,
		Matches: []search.Match{{
			Commit: gitexec.Commit{
				Hash:    strings.Repeat("a", 40),
				Author:  "Mallory\x1b[2J",
				When:    time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
				Subject: "subject \x1b]52;c;ZXZpbA==\x07",
			},
			InFiles: []gitexec.GrepMatch{{Path: "evil\u202egnp.go", Line: 1, Text: "needle \x1b]52;c;ZXZpbA==\x07"}},
		}},
	}
	if out := describeSearch(i18n.New(i18n.EN), result); strings.ContainsAny(out, terminalControls) {
		t.Errorf("find output carries a terminal control character:\n%q", out)
	}
}

func TestExplainHeadCarriesNoTerminalControls(t *testing.T) {
	head := gitexec.Head{Hash: strings.Repeat("b", 40), Branch: "evil\u202egnp"}
	if out := headSummary(i18n.New(i18n.EN), head); strings.ContainsAny(out, terminalControls) {
		t.Errorf("explain HEAD line carries a terminal control character: %q", out)
	}
}
