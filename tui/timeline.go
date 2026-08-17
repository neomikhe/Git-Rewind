package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/timeline"
)

const (
	chromeHeight  = 4
	shortHashLen  = 7
	hoursPerDay   = 24
	minVisibleRow = 1
)

func (m model) timelineView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("git-rewind timeline (%d events)", len(m.session.Events))))
	b.WriteString("\n\n")

	start, end := m.window()
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(i))
		b.WriteByte('\n')
	}

	b.WriteString(m.footer("up/down or j/k: move  |  enter: details  |  q: quit"))
	return b.String()
}

func (m model) renderRow(i int) string {
	e := m.session.Events[i]

	line := fmt.Sprintf("%-10s %5s  %-8s %s  %s",
		selectorOf(e.Entry),
		relativeTime(e.Entry.Time, m.now),
		riskLabel(e.Risk),
		shortHash(e.Entry.Hash),
		e.Describe(),
	)

	if i == m.cursor {
		return selectedStyle.Render("> " + line)
	}
	return "  " + line
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
