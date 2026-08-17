package i18n

import (
	"regexp"
	"strings"
	"testing"
)

var verbPattern = regexp.MustCompile(`%[a-zA-Z]`)

func TestEveryCatalogueCoversEveryKey(t *testing.T) {
	for lang, messages := range catalogs {
		for key := Key(0); key < numKeys; key++ {
			if _, ok := messages[key]; !ok {
				t.Errorf("catalogue %q is missing key %d", lang, int(key))
			}
		}
		if len(messages) != int(numKeys) {
			t.Errorf("catalogue %q has %d messages, want %d — an unknown key would go unnoticed",
				lang, len(messages), int(numKeys))
		}
	}
}

func TestTranslationsUseTheSameFormatVerbs(t *testing.T) {
	for lang, messages := range catalogs {
		if lang == EN {
			continue
		}
		for key, translated := range messages {
			want := verbPattern.FindAllString(english[key], -1)
			got := verbPattern.FindAllString(translated, -1)
			if strings.Join(want, "") != strings.Join(got, "") {
				t.Errorf("key %d in %q uses verbs %v, but English uses %v — Sprintf would break",
					int(key), lang, got, want)
			}
		}
	}
}

func TestNoMessageIsEmpty(t *testing.T) {
	for lang, messages := range catalogs {
		for key, message := range messages {
			if strings.TrimSpace(message) == "" {
				t.Errorf("key %d in %q is empty", int(key), lang)
			}
		}
	}
}

func TestPrinterFallsBackToEnglish(t *testing.T) {
	p := New("klingon")
	if p.Lang() != EN {
		t.Errorf("Lang = %q, want English for an unknown language", p.Lang())
	}
	if got := p.T(ExplainTreeClean); got != english[ExplainTreeClean] {
		t.Errorf("T = %q, want the English message", got)
	}
}

func TestPrinterFormats(t *testing.T) {
	if got := New(ES).T(ExplainHeadOnBranch, "main", "945a801"); !strings.Contains(got, "main") {
		t.Errorf("T = %q, want the branch substituted", got)
	}
	if got := New(EN).T(ExplainTreeClean, "unused"); got == "" {
		t.Error("a message with no verbs must survive extra arguments")
	}
}

func TestResolvePrefersTheOverrideThenTheEnvThenTheConfig(t *testing.T) {
	empty := func(string) string { return "" }
	spanishEnv := func(name string) string {
		if name == EnvVar {
			return "es"
		}
		return ""
	}

	cases := []struct {
		name       string
		override   string
		configured string
		env        func(string) string
		want       Lang
	}{
		{"override wins", "es", "en", empty, ES},
		{"env beats config", "", "en", spanishEnv, ES},
		{"config used when no override", "", "es", empty, ES},
		{"auto is not a language", "", "auto", empty, EN},
		{"locale fallback", "", "", func(n string) string {
			if n == "LANG" {
				return "es_ES.UTF-8"
			}
			return ""
		}, ES},
		{"unknown locale falls back to English", "", "", func(n string) string {
			if n == "LANG" {
				return "fr_FR.UTF-8"
			}
			return ""
		}, EN},
		{"nothing set", "", "", empty, EN},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Resolve(c.override, c.configured, c.env); got != c.want {
				t.Errorf("Resolve = %q, want %q", got, c.want)
			}
		})
	}
}
