package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

func sampleEntries() []gitexec.ReflogEntry {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	return []gitexec.ReflogEntry{
		{Index: 0, Ref: "HEAD", Time: base.Add(2 * time.Hour), Hash: "525df480d3eed5a1c12c2a6625e2aa08909c3447", Subject: "reset: moving to HEAD~1", Operation: "reset"},
		{Index: 1, Ref: "HEAD", Time: base.Add(1 * time.Hour), Hash: "60f4edc0b1ab5048ca9f8e967c4deaeeb4228657", Subject: "checkout: moving from main to feature", Operation: "checkout"},
		{Index: 2, Ref: "HEAD", Time: base, Hash: "9f53d6dcf9d5ab63fa7110448ab00e3ed58886c6", Subject: "commit: first commit", Operation: "commit"},
	}
}

func TestViewShowsEntries(t *testing.T) {
	out := newModel(sampleEntries()).View()

	for _, want := range []string{
		"git-rewind timeline (3 events)",
		"HEAD@{0}",
		"reset: moving to HEAD~1",
		"checkout: moving from main to feature",
		"commit: first commit",
		"525df48", // short hash
		"q: quit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("View() is missing %q\n---\n%s", want, out)
		}
	}
}

func TestUpdateNavigationClamps(t *testing.T) {
	m := newModel(sampleEntries())

	// Up at the top stays at 0.
	m = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("cursor after up at top = %d, want 0", m.cursor)
	}

	// Down moves through the list and clamps at the last entry.
	for i := 0; i < 5; i++ {
		m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if want := len(m.entries) - 1; m.cursor != want {
		t.Fatalf("cursor after repeated down = %d, want %d", m.cursor, want)
	}
}

func TestUpdateQuitKeys(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyCtrlC},
	} {
		_, cmd := newModel(sampleEntries()).Update(key)
		if cmd == nil {
			t.Fatalf("key %q produced no command, want quit", key.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("key %q produced %T, want tea.QuitMsg", key.String(), cmd())
		}
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

// update applies a message and returns the concrete model type for assertions.
func update(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}
