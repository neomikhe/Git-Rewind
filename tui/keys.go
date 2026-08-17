package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/neomikhe/git-rewind/core/i18n"
)

type binding struct {
	keys    string
	short   string
	help    string
	primary bool
}

func (m model) quitBinding() binding {
	return binding{keys: "q", short: m.say(i18n.KeyQuit), help: m.say(i18n.KeyQuitLong), primary: true}
}

func (m model) helpBinding() binding {
	return binding{keys: "?", short: m.say(i18n.KeyHelp), help: m.say(i18n.KeyHelpLong), primary: true}
}

func (m model) say(key i18n.Key, args ...any) string {
	return m.session.Printer.T(key, args...)
}

func (m model) bindings() []binding {
	switch m.screen {
	case screenDetail:
		return []binding{
			{keys: "enter", short: m.say(i18n.KeyRescues), help: m.say(i18n.KeyListRescues), primary: true},
			{keys: "esc", short: m.say(i18n.KeyBack), help: m.say(i18n.KeyBackTimeline), primary: true},
			m.quitBinding(),
			m.helpBinding(),
		}
	case screenRescues:
		return []binding{
			{keys: "up/down, j/k", short: m.say(i18n.KeyMove), help: m.say(i18n.KeyMoveRescues), primary: true},
			{keys: "enter", short: m.say(i18n.KeyReview), help: m.say(i18n.KeyReviewCommands), primary: true},
			{keys: "esc", short: m.say(i18n.KeyBack), help: m.say(i18n.KeyBackDetail), primary: true},
			m.quitBinding(),
			m.helpBinding(),
		}
	case screenConfirm:
		return m.confirmBindings()
	case screenResult:
		return []binding{m.quitBinding(), m.helpBinding()}
	default:
		return m.timelineBindings()
	}
}

func (m model) timelineBindings() []binding {
	b := []binding{
		{keys: "up/down, j/k", short: m.say(i18n.KeyMove), help: m.say(i18n.KeyMoveTimeline), primary: true},
		{keys: "enter", short: m.say(i18n.KeyDetails), help: m.say(i18n.KeyOpenDetail), primary: true},
	}
	if m.hasMore() {
		b = append(b, binding{keys: "m", short: m.say(i18n.KeyMore), help: m.say(i18n.KeyLoadOlder), primary: true})
	}
	return append(b,
		binding{keys: "q, esc", short: m.say(i18n.KeyQuit), help: m.say(i18n.KeyQuitLong), primary: true},
		m.helpBinding(),
	)
}

func (m model) confirmBindings() []binding {
	apply := binding{keys: "y", short: m.say(i18n.KeyApply), help: m.say(i18n.KeyApplyRescue), primary: true}
	if m.discardsUncommitted() {
		apply = binding{
			keys:    "f",
			short:   m.say(i18n.KeyApplyDiscard),
			help:    m.say(i18n.KeyApplyDiscardLong),
			primary: true,
		}
	}
	return []binding{
		apply,
		{keys: "esc", short: m.say(i18n.KeyBack), help: m.say(i18n.KeyBackRescues), primary: true},
		m.quitBinding(),
		m.helpBinding(),
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
		if n := utf8.RuneCountInString(b.keys); n > width {
			width = n
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.say(i18n.TuiHelpTitle, m.screenName())))
	b.WriteString("\n\n")
	for _, k := range bindings {
		padding := strings.Repeat(" ", width-utf8.RuneCountInString(k.keys))
		fmt.Fprintf(&b, "  %s  %s\n", keyStyle.Render(k.keys+padding), k.help)
	}

	b.WriteString("\n" + labelStyle.Render(m.say(i18n.TuiHelpSafety)) + "\n\n")
	b.WriteString(helpStyle.Render(m.say(i18n.TuiHelpFooter)))
	return b.String()
}

func (m model) screenName() string {
	switch m.screen {
	case screenDetail:
		return m.say(i18n.TuiScreenDetail)
	case screenRescues:
		return m.say(i18n.TuiScreenRescues)
	case screenConfirm:
		return m.say(i18n.TuiScreenConfirm)
	case screenResult:
		return m.say(i18n.TuiScreenResult)
	default:
		return m.say(i18n.TuiScreenTimeline)
	}
}
