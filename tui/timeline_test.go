package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
		"q: quit",
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

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{"seconds ago", now.Add(-30 * time.Second), "now"},
		{"minutes ago", now.Add(-5 * time.Minute), "5m"},
		{"hours ago", now.Add(-3 * time.Hour), "3h"},
		{"days ago", now.Add(-48 * time.Hour), "2d"},
		{"future clamps", now.Add(1 * time.Hour), "now"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := relativeTime(c.t, now); got != c.want {
				t.Errorf("relativeTime = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAgoPhrase(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	if got := agoPhrase(now.Add(-10*time.Second), now); got != "just now" {
		t.Errorf("agoPhrase = %q, want %q", got, "just now")
	}
	if got := agoPhrase(now.Add(-3*time.Hour), now); got != "3h ago" {
		t.Errorf("agoPhrase = %q, want %q", got, "3h ago")
	}
}

func update(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}
