package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/timeline"
)

const orphanHash = "60f4edc0b1ab5048ca9f8e967c4deaeeb4228657"

func sampleSession() Session {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := timeline.FromReflog([]gitexec.ReflogEntry{
		{Index: 0, Ref: "HEAD", Time: base.Add(2 * time.Hour), Hash: "525df480d3eed5a1c12c2a6625e2aa08909c3447", ActorName: "Ada", Subject: "reset: moving to HEAD~1", Operation: "reset"},
		{Index: 1, Ref: "HEAD", Time: base.Add(1 * time.Hour), Hash: orphanHash, ActorName: "Ada", Subject: "checkout: moving from main to feature", Operation: "checkout"},
		{Index: 2, Ref: "HEAD", Time: base, Hash: "9f53d6dcf9d5ab63fa7110448ab00e3ed58886c6", ActorName: "Ada", Subject: "commit: first commit", Operation: "commit"},
	})
	events[0].Orphaned = []string{orphanHash}
	return Session{Events: events}
}

func TestTimelineViewShowsClassifiedEvents(t *testing.T) {
	out := newModel(sampleSession()).View()

	for _, want := range []string{
		"git-rewind timeline (3 events)",
		"HEAD@{0}",
		"[red]",
		"[yellow]",
		"[green]",
		"525df48",
		"Reset the branch to HEAD~1 (1 commit left unreachable, recoverable)",
		"Switched from main to feature",
		`Committed "first commit"`,
		"enter: details",
		"q, esc: quit",
		"?: help",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("timeline view is missing %q\n---\n%s", want, out)
		}
	}
}

func TestTimelineNavigationClamps(t *testing.T) {
	m := newModel(sampleSession())

	m = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("cursor after up at top = %d, want 0", m.cursor)
	}

	for i := 0; i < 5; i++ {
		m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if want := len(m.session.Events) - 1; m.cursor != want {
		t.Fatalf("cursor after repeated down = %d, want %d", m.cursor, want)
	}
}

func TestTimelineWindowKeepsCursorVisible(t *testing.T) {
	m := newModel(sampleSession())
	m.height = chromeHeight + 1
	m.cursor = 2

	start, end := m.window()
	if m.cursor < start || m.cursor >= end {
		t.Fatalf("window [%d,%d) does not contain the cursor %d", start, end, m.cursor)
	}
	if got := end - start; got != 1 {
		t.Fatalf("window shows %d rows, want 1", got)
	}
}

func TestRiskCellKeepsAFixedVisibleWidth(t *testing.T) {
	for _, r := range []timeline.Risk{timeline.RiskGreen, timeline.RiskYellow, timeline.RiskRed} {
		if got := lipgloss.Width(riskCell(r)); got != riskCellWidth {
			t.Errorf("risk cell for %s is %d columns wide, want %d — column alignment would break", r, got, riskCellWidth)
		}
	}
}

func TestEachRiskLevelHasItsOwnColour(t *testing.T) {
	seen := make(map[string]timeline.Risk)
	for _, r := range []timeline.Risk{timeline.RiskGreen, timeline.RiskYellow, timeline.RiskRed} {
		style, ok := riskStyles[r]
		if !ok {
			t.Fatalf("no style defined for risk %s", r)
		}
		colour := fmt.Sprint(style.GetForeground())
		if previous, duplicate := seen[colour]; duplicate {
			t.Errorf("risk %s shares colour %q with %s", r, colour, previous)
		}
		seen[colour] = r
	}
}

func TestRiskLabelSurvivesWithoutColour(t *testing.T) {
	out := newModel(sampleSession()).View()
	for _, want := range []string{"[red]", "[yellow]", "[green]"} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing the %q text label; colour must not be the only risk signal", want)
		}
	}
}

func update(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}
