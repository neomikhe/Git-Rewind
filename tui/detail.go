package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/timeline"
)

func (m model) detailView() string {
	e := m.session.Events[m.cursor]
	p := m.session.Printer

	fields := [][2]string{
		{m.say(i18n.TuiFieldWhen), fmt.Sprintf("%s (%s)", e.Entry.Time.Format("2006-01-02 15:04:05 MST"), timeline.AgoPhrase(p, e.Entry.Time, m.now))},
		{m.say(i18n.TuiFieldKind), e.Kind.String()},
		{m.say(i18n.TuiFieldRisk), riskStyle(e.Risk).Render(e.Risk.String())},
		{m.say(i18n.TuiFieldCommit), shortHash(e.Entry.Hash)},
		{m.say(i18n.TuiFieldWho), e.Entry.ActorName},
		{m.say(i18n.TuiFieldWhat), e.Describe(p)},
		{m.say(i18n.TuiFieldReflog), e.Entry.Subject},
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.say(i18n.TuiEventTitle, selectorOf(e.Entry))))
	b.WriteString("\n\n")
	for _, f := range fields {
		b.WriteString(field(f[0], f[1], labelWidth(fields)))
	}

	if len(e.Orphaned) > 0 {
		b.WriteString("\n" + labelStyle.Render(m.say(i18n.TuiRecoverableHeading)) + "\n")
		for _, hash := range e.Orphaned {
			b.WriteString("    " + shortHash(hash) + "\n")
		}
	}

	b.WriteString(m.footer())
	return b.String()
}

func (m model) rescuesView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.say(i18n.TuiRescuesTitle, len(m.rescues))))
	b.WriteString("\n\n")

	if len(m.rescues) == 0 {
		b.WriteString(m.say(i18n.TuiNoRescues))
	}
	for i, r := range m.rescues {
		title := r.recipe.Title(m.session.Printer)
		if i == m.choice {
			b.WriteString(selectedStyle.Render("> "+title) + "\n")
			continue
		}
		b.WriteString("  " + title + "\n")
	}

	if m.dirty {
		b.WriteString("\n" + warnStyle.Render(m.say(i18n.TuiDirtyNotice)) + "\n")
	}

	b.WriteString(m.footer())
	return b.String()
}

func labelWidth(fields [][2]string) int {
	width := 0
	for _, f := range fields {
		if n := utf8.RuneCountInString(f[0]); n > width {
			width = n
		}
	}
	return width
}

func field(label, value string, width int) string {
	padding := strings.Repeat(" ", width-utf8.RuneCountInString(label))
	return "  " + label + padding + "  " + value + "\n"
}
