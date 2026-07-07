// Command git-rewind reads a Git repository's reflog, fsck, and working-tree
// state, translates it into a human-readable timeline of recent events, and
// offers safe, reversible "rescue" actions to undo Git mistakes.
//
// When installed on PATH as "git-rewind", Git invokes it as the native
// subcommand "git rewind".
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/tui"
)

func main() {
	if err := run(".", os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "git-rewind:", err)
		os.Exit(1)
	}
}

// run loads the reflog for the repository at dir and shows it in the timeline
// TUI. When there is no history yet, it prints a short notice to stdout instead
// of launching the interactive view.
func run(dir string, stdout io.Writer) error {
	entries, err := gitexec.New(dir).Reflog(context.Background())
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(stdout, "git-rewind: no repository history to show yet.")
		return err
	}
	return tui.Run(entries)
}
