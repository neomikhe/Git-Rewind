package gitexec

import (
	"strings"
	"unicode/utf8"
)

const (
	firstC1Control = 0x80
	lastC1Control  = 0x9f
	deleteControl  = 0x7f
)

// Printable returns text read from a repository with every character a terminal would interpret replaced.
func Printable(s string) string {
	return strings.Map(func(r rune) rune {
		if isTerminalControl(r) {
			return utf8.RuneError
		}
		return r
	}, s)
}

func isTerminalControl(r rune) bool {
	switch {
	case r == '\t':
		return false
	case r < ' ', r == deleteControl:
		return true
	case r >= firstC1Control && r <= lastC1Control:
		return true
	default:
		return isBidiControl(r)
	}
}

func isBidiControl(r rune) bool {
	switch {
	case r == '\u061c', r == '\u200e', r == '\u200f':
		return true
	case r >= '\u202a' && r <= '\u202e':
		return true
	default:
		return r >= '\u2066' && r <= '\u2069'
	}
}
