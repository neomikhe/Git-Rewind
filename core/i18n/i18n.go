package i18n

import (
	"fmt"
	"strings"
)

// Lang is a language git-rewind can speak.
type Lang string

// The supported languages. English is the fallback for anything unrecognised.
const (
	EN Lang = "en"
	ES Lang = "es"
)

// EnvVar overrides both the configuration file and the operating system locale.
const EnvVar = "GIT_REWIND_LANG"

var catalogs = map[Lang]map[Key]string{
	EN: english,
	ES: spanish,
}

// Printer renders messages in one language.
type Printer struct {
	lang     Lang
	messages map[Key]string
}

// New returns a Printer for lang, falling back to English when lang is unknown.
func New(lang Lang) *Printer {
	messages, ok := catalogs[lang]
	if !ok {
		lang, messages = EN, english
	}
	return &Printer{lang: lang, messages: messages}
}

// Lang reports which language the Printer renders.
func (p *Printer) Lang() Lang { return p.lang }

// T renders a message, substituting args into it as fmt.Sprintf would.
func (p *Printer) T(key Key, args ...any) string {
	format, ok := p.messages[key]
	if !ok {
		if format, ok = english[key]; !ok {
			return fmt.Sprintf("!missing message %d!", int(key))
		}
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// Resolve picks a language from the override, then the configured value, then the locale
// environment, falling back to English.
func Resolve(override, configured string, env func(string) string) Lang {
	for _, candidate := range []string{override, env(EnvVar), configured} {
		if lang, ok := parse(candidate); ok {
			return lang
		}
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if lang, ok := parse(env(name)); ok {
			return lang
		}
	}
	return EN
}

func parse(value string) (Lang, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "auto" {
		return "", false
	}
	switch {
	case strings.HasPrefix(value, string(ES)):
		return ES, true
	case strings.HasPrefix(value, string(EN)):
		return EN, true
	default:
		return "", false
	}
}
