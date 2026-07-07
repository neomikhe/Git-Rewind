// Package recipes defines the Recipe extension point. Each rescue scenario
// implements the same flow — Detect(repo) -> Plan{commands, warnings} ->
// Execute — so new rescues are added here, each with its own integration test.
// The Recipe interface is kept small and stable to keep contributions simple.
package recipes
