package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/timeline"
)

const (
	chromeHeight  = 4
	shortHashLen  = 7
	hoursPerDay   = 24
	minVisibleRow = 1
	riskCellWidth = 8
)

var riskStyles = map[timeline.Risk]lipgloss.Style{
	timeline.RiskGreen:  lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	timeline.RiskYellow: lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
	timeline.RiskRed:    lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
}

func (m model) timelineView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(m.timelineTitle()))
	b.WriteString("\n\n")

	start, end := m.window()
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(i))
		b.WriteByte('\n')
	}

	b.WriteString(m.footer())
	return b.String()
}

func (m model) timelineTitle() string {
	if m.hasMore() {
		return fmt.Sprintf("git-rewind timeline (%d events, more available)", len(m.session.Events))
	}
	return fmt.Sprintf("git-rewind timeline (%d events)", len(m.session.Events))
}

func (m model) renderRow(i int) string {
	e := m.session.Events[i]

	head := fmt.Sprintf("%-10s %5s  ", selectorOf(e.Entry), relativeTime(e.Entry.Time, m.now))
	tail := fmt.Sprintf("%s  %s", shortHash(e.Entry.Hash), e.Describe())

	marker := "  "
	if i == m.cursor {
		marker = selectedStyle.Render("> ")
		head = selectedStyle.Render(head)
		tail = selectedStyle.Render(tail)
	}
	return marker + head + riskCell(e.Risk) + " " + tail
}

func (m model) window() (start, end int) {
	visible := len(m.session.Events)
	if m.height > 0 {
		visible = m.height - chromeHeight
		if visible < minVisibleRow {
			visible = minVisibleRow
		}
	}
	if visible >= len(m.session.Events) {
		return 0, len(m.session.Events)
	}

	start = m.cursor - visible/2
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > len(m.session.Events) {
		end = len(m.session.Events)
		start = end - visible
	}
	return start, end
}

func selectorOf(e gitexec.ReflogEntry) string {
	return fmt.Sprintf("%s@{%d}", e.Ref, e.Index)
}

func riskLabel(r timeline.Risk) string {
	return "[" + r.String() + "]"
}

func riskCell(r timeline.Risk) string {
	return riskStyle(r).Render(fmt.Sprintf("%-*s", riskCellWidth, riskLabel(r)))
}

func riskStyle(r timeline.Risk) lipgloss.Style {
	if style, ok := riskStyles[r]; ok {
		return style
	}
	return lipgloss.NewStyle()
}

func shortHash(hash string) string {
	if len(hash) > shortHashLen {
		return hash[:shortHashLen]
	}
	return hash
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

func agoPhrase(t, now time.Time) string {
	rel := relativeTime(t, now)
	if rel == "now" {
		return "just now"
	}
	return rel + " ago"
}
