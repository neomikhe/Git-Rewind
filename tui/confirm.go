package tui

import (
	"strings"

	"github.com/neomikhe/git-rewind/core/i18n"
)

func (m model) confirmView() string {
	if len(m.rescues) == 0 {
		return m.rescuesView()
	}
	selected := m.rescues[m.choice]

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.say(i18n.TuiConfirmTitle, selected.recipe.Title(m.session.Printer))))
	b.WriteString(m.say(i18n.TuiWillRun))

	for i, command := range selected.plan.Preview() {
		b.WriteString("  " + command + "\n")
		if explain := selected.plan.Commands[i].Explain; explain != "" {
			b.WriteString("      " + labelStyle.Render(explain) + "\n")
		}
	}

	b.WriteString(m.say(i18n.TuiBackupPromise))
	for _, warning := range selected.plan.Warnings {
		b.WriteString("\n  " + warning + "\n")
	}
	if m.discardsUncommitted() {
		b.WriteString("\n" + warnStyle.Render(m.say(i18n.TuiConfirmDirty)) + "\n")
	}

	b.WriteString(m.footer())
	return b.String()
}

func (m model) resultView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.say(i18n.TuiDoneTitle)))
	b.WriteString("\n\n")

	if m.applied != nil {
		b.WriteString(m.say(i18n.TuiDoneBackup, m.applied.BackupBranch))
		b.WriteString(m.say(i18n.TuiDoneRan))
		for _, command := range m.applied.Commands {
			b.WriteString("    " + command + "\n")
		}
		b.WriteString(m.say(i18n.TuiDoneRerun))
	}

	b.WriteString(m.footer())
	return b.String()
}
