package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

const (
	chromeHeight  = 4
	shortHashLen  = 7
	hoursPerDay   = 24
	minVisibleRow = 1
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
)

// Run launches the timeline TUI and blocks until the user quits.
func Run(entries []gitexec.ReflogEntry) error {
	_, err := tea.NewProgram(newModel(entries), tea.WithAltScreen()).Run()
	return err
}

type model struct {
	entries []gitexec.ReflogEntry
	cursor  int
	height  int
	now     time.Time
}

func newModel(entries []gitexec.ReflogEntry) model {
	return model{entries: entries, now: time.Now()}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("git-rewind timeline (%d events)", len(m.entries))))
	b.WriteString("\n\n")

	start, end := m.window()
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(i))
		b.WriteByte('\n')
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("up/down or j/k: move  |  q: quit"))
	return b.String()
}

func (m model) renderRow(i int) string {
	e := m.entries[i]

	short := e.Hash
	if len(short) > shortHashLen {
		short = short[:shortHashLen]
	}

	line := fmt.Sprintf("%-10s %5s  %s  %s",
		fmt.Sprintf("%s@{%d}", e.Ref, e.Index),
		relativeTime(e.Time, m.now),
		short,
		e.Subject,
	)

	if i == m.cursor {
		return selectedStyle.Render("> " + line)
	}
	return "  " + line
}

func (m model) window() (start, end int) {
	visible := len(m.entries)
	if m.height > 0 {
		visible = m.height - chromeHeight
		if visible < minVisibleRow {
			visible = minVisibleRow
		}
	}
	if visible >= len(m.entries) {
		return 0, len(m.entries)
	}

	start = m.cursor - visible/2
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > len(m.entries) {
		end = len(m.entries)
		start = end - visible
	}
	return start, end
}

func relativeTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < hoursPerDay*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/hoursPerDay))
	}
}
