package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/neomikhe/git-rewind/core/i18n"
)

func TestSpanishExplainOutput(t *testing.T) {
	t.Setenv(i18n.EnvVar, "es")
	dir, _ := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"explain"}, dir, &buf); err != nil {
		t.Fatalf("run explain: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Estado del repositorio",
		"en la rama main",
		"Área de trabajo",
		"limpia",
		"Moviste la rama a HEAD~1",
		"1 commit quedó sin rama",
		"Hay algo que se puede deshacer:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Spanish explain is missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "Repository state") {
		t.Errorf("English leaked into the Spanish output\n---\n%s", out)
	}
}

func TestSpanishFindOutput(t *testing.T) {
	t.Setenv(i18n.EnvVar, "es")
	dir, _ := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"find", "two"}, dir, &buf); err != nil {
		t.Fatalf("run find: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Coincidencias con", "consérvalo con: git branch", "no cambia nada más"} {
		if !strings.Contains(out, want) {
			t.Errorf("Spanish find is missing %q\n---\n%s", want, out)
		}
	}
}

func TestSpanishLastDryRun(t *testing.T) {
	t.Setenv(i18n.EnvVar, "es")
	dir, _ := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"last"}, dir, &buf); err != nil {
		t.Fatalf("run last: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Rescate:", "Va a ejecutar:", "Simulación."} {
		if !strings.Contains(out, want) {
			t.Errorf("Spanish last is missing %q\n---\n%s", want, out)
		}
	}
	if !strings.Contains(out, "git reset --hard") {
		t.Error("the git commands themselves must never be translated")
	}
}

func TestUnknownLanguageFallsBackToEnglish(t *testing.T) {
	t.Setenv(i18n.EnvVar, "klingon")
	dir, _ := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"explain"}, dir, &buf); err != nil {
		t.Fatalf("run explain: %v", err)
	}
	if !strings.Contains(buf.String(), "Repository state") {
		t.Errorf("an unknown language must fall back to English\n---\n%s", buf.String())
	}
}

func TestJSONFieldNamesStayEnglishInEveryLanguage(t *testing.T) {
	t.Setenv(i18n.EnvVar, "es")
	dir, _ := resetHardRepo(t)

	got := decode[jsonExplain](t, []string{"explain", "--json"}, dir)
	if got.Command != "explain" {
		t.Errorf("command = %q, want the untranslated key", got.Command)
	}
	if got.LastEvent == nil || got.LastEvent.Kind != "reset" || got.LastEvent.Risk != "red" {
		t.Errorf("machine-readable enums must not be translated: %+v", got.LastEvent)
	}
	if !strings.Contains(got.LastEvent.Description, "Moviste") {
		t.Errorf("the human description should follow the language: %q", got.LastEvent.Description)
	}
}
