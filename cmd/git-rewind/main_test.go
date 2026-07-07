package main

import "testing"

// TestRunSucceeds is a smoke test that establishes the test harness and proves
// the entrypoint wiring compiles and runs without error.
func TestRunSucceeds(t *testing.T) {
	if err := run(nil); err != nil {
		t.Fatalf("run returned an unexpected error: %v", err)
	}
}
