package tui

import (
	"fmt"
	"strings"
)

const labelWidth = 8

func (m model) detailView() string {
	e := m.session.Events[m.cursor]

	var b strings.Builder
	b.WriteString(titleStyle.Render("Event " + selectorOf(e.Entry)))
	b.WriteString("\n\n")
	b.WriteString(field("When", fmt.Sprintf("%s (%s)", e.Entry.Time.Format("2006-01-02 15:04:05 MST"), agoPhrase(e.Entry.Time, m.now))))
	b.WriteString(field("Kind", e.Kind.String()))
	b.WriteString(field("Risk", e.Risk.String()))
	b.WriteString(field("Commit", shortHash(e.Entry.Hash)))
	b.WriteString(field("Who", e.Entry.ActorName))
	b.WriteString(field("What", e.Describe()))
	b.WriteString(field("Reflog", e.Entry.Subject))

	if len(e.Orphaned) > 0 {
		b.WriteString("\n" + labelStyle.Render("  Commits left unreachable but still recoverable") + "\n")
		for _, hash := range e.Orphaned {
			b.WriteString("    " + shortHash(hash) + "\n")
		}
	}

	b.WriteString(m.footer("enter: available rescues  |  esc: back  |  q: quit"))
	return b.String()
}

func (m model) rescuesView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Available rescues (%d)", len(m.rescues))))
	b.WriteString("\n\n")

	if len(m.rescues) == 0 {
		b.WriteString("  No rescue applies to the current repository state.\n")
	}
	for i, r := range m.rescues {
		if i == m.choice {
			b.WriteString(selectedStyle.Render("> "+r.recipe.Title()) + "\n")
			continue
		}
		b.WriteString("  " + r.recipe.Title() + "\n")
	}

	if m.dirty {
		b.WriteString("\n  ! You have uncommitted changes. A backup branch does not preserve them.\n")
	}

	b.WriteString(m.footer("up/down: move  |  enter: review the commands  |  esc: back  |  q: quit"))
	return b.String()
}

func field(label, value string) string {
	return fmt.Sprintf("  %s %s\n", labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)), value)
}
