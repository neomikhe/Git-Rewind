package tui

import (
	"fmt"
	"strings"
)

type binding struct {
	keys    string
	short   string
	help    string
	primary bool
}

var (
	quitBinding = binding{keys: "q", short: "quit", help: "quit git-rewind", primary: true}
	helpBinding = binding{keys: "?", short: "help", help: "show this help", primary: true}
)

func (m model) bindings() []binding {
	switch m.screen {
	case screenDetail:
		return []binding{
			{keys: "enter", short: "rescues", help: "list the rescues that apply", primary: true},
			{keys: "esc", short: "back", help: "back to the timeline", primary: true},
			quitBinding,
			helpBinding,
		}
	case screenRescues:
		return []binding{
			{keys: "up/down, j/k", short: "move", help: "move through the rescues", primary: true},
			{keys: "enter", short: "review", help: "review the exact commands", primary: true},
			{keys: "esc", short: "back", help: "back to the event detail", primary: true},
			quitBinding,
			helpBinding,
		}
	case screenConfirm:
		return m.confirmBindings()
	case screenResult:
		return []binding{quitBinding, helpBinding}
	default:
		return m.timelineBindings()
	}
}

func (m model) timelineBindings() []binding {
	b := []binding{
		{keys: "up/down, j/k", short: "move", help: "move through the timeline", primary: true},
		{keys: "enter", short: "details", help: "open the event detail", primary: true},
	}
	if m.hasMore() {
		b = append(b, binding{keys: "m", short: "more", help: "load older events", primary: true})
	}
	return append(b,
		binding{keys: "q, esc", short: "quit", help: "quit git-rewind", primary: true},
		helpBinding,
	)
}

func (m model) confirmBindings() []binding {
	apply := binding{keys: "y", short: "apply", help: "apply this rescue", primary: true}
	if m.discardsUncommitted() {
		apply = binding{
			keys:    "f",
			short:   "apply, discarding changes",
			help:    "apply and discard your uncommitted changes",
			primary: true,
		}
	}
	return []binding{
		apply,
		{keys: "esc", short: "back", help: "back to the rescue list", primary: true},
		quitBinding,
		helpBinding,
	}
}

func (m model) footerHint() string {
	bindings := m.bindings()
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if b.primary {
			parts = append(parts, b.keys+": "+b.short)
		}
	}
	return strings.Join(parts, "  |  ")
}

func (m model) helpView() string {
	bindings := m.bindings()

	width := 0
	for _, b := range bindings {
		if len(b.keys) > width {
			width = len(b.keys)
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Keys — " + m.screen.String()))
	b.WriteString("\n\n")
	for _, k := range bindings {
		fmt.Fprintf(&b, "  %s  %s\n", keyStyle.Render(fmt.Sprintf("%-*s", width, k.keys)), k.help)
	}

	b.WriteString("\n" + labelStyle.Render("  Nothing runs until you confirm it, and your current state is always saved"))
	b.WriteString("\n" + labelStyle.Render("  to a backup/rewind-<timestamp> branch first.") + "\n\n")
	b.WriteString(helpStyle.Render("?, esc: close this help  |  q: quit"))
	return b.String()
}

func (s screen) String() string {
	switch s {
	case screenDetail:
		return "event detail"
	case screenRescues:
		return "available rescues"
	case screenConfirm:
		return "confirmation"
	case screenResult:
		return "result"
	default:
		return "timeline"
	}
}
