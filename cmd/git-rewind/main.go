// Command git-rewind reads a Git repository's reflog, fsck, and working-tree
// state, translates it into a human-readable timeline of recent events, and
// offers safe, reversible "rescue" actions to undo Git mistakes.
//
// When installed on PATH as "git-rewind", Git invokes it as the native
// subcommand "git rewind".
package main

import (
	"fmt"
	"os"
)

// version is the build version. It is overridden at release time via
// -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "git-rewind:", err)
		os.Exit(1)
	}
}

// run holds the real entrypoint logic, kept separate from main so it can be
// tested and so the process exits from a single place.
func run(_ []string) error {
	// TODO(phase 3): parse flags and subcommands, then launch the TUI.
	fmt.Printf("git-rewind %s\n", version)
	fmt.Println("scaffold ready — timeline and rescue recipes coming soon.")
	return nil
}
