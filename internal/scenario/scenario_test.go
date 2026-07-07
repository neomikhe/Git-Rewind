package scenario

import (
	"os/exec"
	"testing"
)

// TestScenariosProduceBrokenState builds every scenario in a fresh repository
// and checks that it ended up in the expected broken (but recoverable) state.
// This is the trust foundation the rescue recipes build on.
func TestScenariosProduceBrokenState(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	for _, s := range All() {
		t.Run(s.Name, func(t *testing.T) {
			b, err := s.Build(t.TempDir())
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if err := s.Verify(b); err != nil {
				t.Fatalf("scenario %q is not in the expected broken state: %v", s.Name, err)
			}
		})
	}
}

// TestAllScenariosAreWellFormed guards the registry itself: names must be unique
// and non-empty, and every scenario must supply both a builder and a verifier.
func TestAllScenariosAreWellFormed(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range All() {
		if s.Name == "" {
			t.Error("found a scenario with an empty name")
		}
		if seen[s.Name] {
			t.Errorf("duplicate scenario name %q", s.Name)
		}
		seen[s.Name] = true

		if s.Description == "" {
			t.Errorf("scenario %q has no description", s.Name)
		}
		if s.Build == nil || s.Verify == nil {
			t.Errorf("scenario %q is missing Build or Verify", s.Name)
		}
	}
}
