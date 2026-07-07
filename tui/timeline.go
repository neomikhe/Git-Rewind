package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// chromeHeight is the number of non-list lines the view always draws (title,
// two blank lines, and the help footer). It is subtracted from the terminal
// height to size the scrolling window.
const chromeHeight = 4

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
)

// Run launches the timeline TUI for the given reflog entries and blocks until
// the user quits.
func Run(entries []gitexec.ReflogEntry) error {
	_, err := tea.NewProgram(newModel(entries), tea.WithAltScreen()).Run()
	return err
}

// model is the Bubble Tea model backing the navigable reflog timeline.
type model struct {
	entries []gitexec.ReflogEntry
	cursor  int
	height  int // terminal height in rows; 0 until the first WindowSizeMsg.
	now     time.Time
}

func newModel(entries []gitexec.ReflogEntry) model {
	return model{entries: entries, now: time.Now()}
}

// Init implements tea.Model; the timeline has no startup command.
func (m model) Init() tea.Cmd { return nil }

// Update implements tea.Model, handling navigation and quit keys.
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

// View implements tea.Model, rendering the title, a scrolling window of entries
// with the cursor row highlighted, and a help footer.
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

// renderRow formats a single reflog entry, marking and highlighting the row at
// the cursor.
func (m model) renderRow(i int) string {
	e := m.entries[i]

	short := e.Hash
	if len(short) > 7 {
		short = short[:7]
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

// window returns the half-open range of entry indices to display, keeping the
// cursor roughly centered when the list is taller than the terminal.
func (m model) window() (start, end int) {
	visible := len(m.entries)
	if m.height > 0 {
		visible = m.height - chromeHeight
		if visible < 1 {
			visible = 1
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

// relativeTime renders how long before now the event happened, in a compact
// form (now, 5m, 3h, 2d). Times in the future clamp to "now".
func relativeTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
