package tui

import "strings"

func (m model) confirmView() string {
	if len(m.rescues) == 0 {
		return m.rescuesView()
	}
	selected := m.rescues[m.choice]
	discardsUncommitted := selected.plan.DiscardsChanges && m.dirty

	var b strings.Builder
	b.WriteString(titleStyle.Render("Rescue: " + selected.recipe.Title()))
	b.WriteString("\n\nWill run:\n")

	for i, command := range selected.plan.Preview() {
		b.WriteString("  " + command + "\n")
		if explain := selected.plan.Commands[i].Explain; explain != "" {
			b.WriteString("      " + labelStyle.Render(explain) + "\n")
		}
	}

	b.WriteString("\n  Your current state is saved to a backup branch before anything runs.\n")
	for _, warning := range selected.plan.Warnings {
		b.WriteString("\n  " + warning + "\n")
	}

	help := "y: apply  |  esc: back  |  q: quit"
	if discardsUncommitted {
		b.WriteString("\n  ! You have uncommitted changes. They are NOT saved to the backup and would be lost.\n")
		help = "f: apply and discard uncommitted changes  |  esc: back  |  q: quit"
	}

	b.WriteString(m.footer(help))
	return b.String()
}

func (m model) resultView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Done"))
	b.WriteString("\n\n")

	if m.applied != nil {
		b.WriteString("  Your previous state is saved on branch " + m.applied.BackupBranch + "\n")
		b.WriteString("\n  Ran:\n")
		for _, command := range m.applied.Commands {
			b.WriteString("    " + command + "\n")
		}
		b.WriteString("\n  Re-run git rewind to see the updated timeline.\n")
	}

	b.WriteString(m.footer("q: quit"))
	return b.String()
}
