package tui

import "strings"

func (m model) confirmView() string {
	if len(m.rescues) == 0 {
		return m.rescuesView()
	}
	selected := m.rescues[m.choice]

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
	if m.discardsUncommitted() {
		b.WriteString("\n" + warnStyle.Render("  ! You have uncommitted changes. They are NOT saved to the backup and would be lost.") + "\n")
	}

	b.WriteString(m.footer())
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

	b.WriteString(m.footer())
	return b.String()
}
