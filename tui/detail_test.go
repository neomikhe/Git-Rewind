package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/timeline"
)

func TestDetailViewCarriesNoRepositoryControlSequences(t *testing.T) {
	events := timeline.FromReflog([]gitexec.ReflogEntry{{
		Ref:       "HEAD",
		Time:      time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		Hash:      strings.Repeat("c", 40),
		ActorName: "Mallory\x1b]52;c;ZXZpbA==\x07",
		Subject:   "commit: subject \x1b]52;c;ZXZpbA==\x07 \u202e",
		Operation: "commit",
	}})
	out := update(newModel(Session{Events: events}), key("enter")).View()
	for _, injected := range []string{"\x1b]52", "\x07", "\u202e"} {
		if strings.Contains(out, injected) {
			t.Errorf("detail view carries %q from the repository:\n%q", injected, out)
		}
	}
}
