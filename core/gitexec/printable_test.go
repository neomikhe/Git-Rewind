package gitexec

import "testing"

func TestPrintableReplacesWhatATerminalWouldInterpret(t *testing.T) {
	cases := map[string]string{
		"plain subject": "plain subject",
		"\u00c1rea \u00f1 \u6f22\u5b57 \U0001f600": "\u00c1rea \u00f1 \u6f22\u5b57 \U0001f600",
		"tab\tstays":                        "tab\tstays",
		"osc52 \x1b]52;c;ZXZpbA==\x07 done": "osc52 \ufffd]52;c;ZXZpbA==\ufffd done",
		"colour \x1b[31mred":                "colour \ufffd[31mred",
		"line\nbreak\rreturn":               "line\ufffdbreak\ufffdreturn",
		"c1 \u009b31m":                      "c1 \ufffd31m",
		"delete \x7f":                       "delete \ufffd",
		"evil\u202egnp.exe":                 "evil\ufffdgnp.exe",
		"isolate \u2066x\u2069":             "isolate \ufffdx\ufffd",
		"marks \u200e\u200f\u061c":          "marks \ufffd\ufffd\ufffd",
		"broken \xff byte":                  "broken \ufffd byte",
	}
	for in, want := range cases {
		if got := Printable(in); got != want {
			t.Errorf("Printable(%q) = %q, want %q", in, got, want)
		}
	}
}
