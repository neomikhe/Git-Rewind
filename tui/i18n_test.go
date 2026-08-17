package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/neomikhe/git-rewind/core/i18n"
)

func spanishSession() Session {
	s := sampleSession()
	s.Printer = i18n.New(i18n.ES)
	return s
}

func TestSpanishTimelineAndDetail(t *testing.T) {
	m := newModel(spanishSession())

	for _, want := range []string{"línea de tiempo de git-rewind", "Moviste la rama a HEAD~1", "enter: detalle", "q, esc: salir"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("Spanish timeline is missing %q\n---\n%s", want, m.View())
		}
	}

	detail := update(m, key("enter"))
	for _, want := range []string{"Evento HEAD@{0}", "Cuándo", "Quién", "Qué", "Commits que quedaron sin rama"} {
		if !strings.Contains(detail.View(), want) {
			t.Errorf("Spanish detail is missing %q\n---\n%s", want, detail.View())
		}
	}
	if strings.Contains(detail.View(), "Working tree") || strings.Contains(m.View(), "timeline (") {
		t.Error("English leaked into the Spanish views")
	}
}

func TestSpanishDetailLabelsStayAligned(t *testing.T) {
	detail := update(newModel(spanishSession()), key("enter"))

	var columns []int
	for _, line := range strings.Split(detail.View(), "\n") {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "    ") {
			continue
		}
		label, rest, found := strings.Cut(strings.TrimPrefix(line, "  "), "  ")
		if !found || strings.TrimSpace(rest) == "" {
			continue
		}
		padding := len(rest) - len(strings.TrimLeft(rest, " "))
		columns = append(columns, 2+utf8.RuneCountInString(label)+2+padding)
	}
	if len(columns) < 2 {
		t.Fatalf("expected several label/value rows, found %d", len(columns))
	}
	for _, c := range columns[1:] {
		if c != columns[0] {
			t.Errorf("values start at differing columns %v — accented labels must be padded by runes, not bytes", columns)
			break
		}
	}
}

func TestSpanishRescueAndConfirm(t *testing.T) {
	m := atRescues(t, []rescue{discardingRescue()}, true)
	m.session.Printer = i18n.New(i18n.ES)

	if !strings.Contains(m.View(), "Rescates disponibles") {
		t.Errorf("Spanish rescue list is missing its title\n---\n%s", m.View())
	}
	if !strings.Contains(m.View(), "Recuperar commits descartados") {
		t.Errorf("the recipe title should be translated\n---\n%s", m.View())
	}

	confirm := update(m, key("enter"))
	for _, want := range []string{"Rescate:", "Va a ejecutar:", "f: aplicar, descartando cambios", "NO se guardan"} {
		if !strings.Contains(confirm.View(), want) {
			t.Errorf("Spanish confirm is missing %q\n---\n%s", want, confirm.View())
		}
	}
	if !strings.Contains(confirm.View(), "git reset --hard") {
		t.Error("the git commands themselves must never be translated")
	}
}

func TestSpanishHelpScreen(t *testing.T) {
	help := update(newModel(spanishSession()), key("?"))

	for _, want := range []string{"Teclas — línea de tiempo", "moverte por la línea de tiempo", "salir de git-rewind", "backup/rewind-"} {
		if !strings.Contains(help.View(), want) {
			t.Errorf("Spanish help is missing %q\n---\n%s", want, help.View())
		}
	}
}

func TestSpanishDirtyTreeErrorIsTranslated(t *testing.T) {
	m := atRescues(t, []rescue{discardingRescue()}, true)
	m.session.Printer = i18n.New(i18n.ES)
	m = update(m, key("enter"))

	refused := update(m, key("y"))
	if !strings.Contains(refused.View(), "pulsa f para aplicarlo") {
		t.Errorf("the dirty-tree refusal should be translated\n---\n%s", refused.View())
	}
}

func TestSpanishFooterStillFitsEightyColumns(t *testing.T) {
	screens := []model{
		newModel(spanishSession()),
		update(newModel(spanishSession()), key("enter")),
	}
	dirty := atRescues(t, []rescue{discardingRescue()}, true)
	dirty.session.Printer = i18n.New(i18n.ES)
	screens = append(screens, dirty, update(dirty, key("enter")))

	for _, m := range screens {
		if width := lipgloss.Width(m.footerHint()); width > 80 {
			t.Errorf("screen %v footer is %d columns wide in Spanish: %q", m.screen, width, m.footerHint())
		}
	}
}
