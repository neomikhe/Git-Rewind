package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/search"
)

const (
	maxMatchLineLen = 100
	rescuePrefix    = "rescued/"
)

func runFind(args []string, dir string, stdout io.Writer) error {
	fs := flag.NewFlagSet("find", flag.ContinueOnError)
	fs.SetOutput(stdout)
	messagesOnly := fs.Bool("messages", false, "search commit messages only, not file contents (faster on large repositories)")
	limit := fs.Int("limit", search.DefaultMaxCommits, "how many unreachable commits to inspect")
	if err := fs.Parse(args); err != nil {
		return err
	}

	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return errors.New("nothing to look for (try: git rewind find \"the text you lost\")")
	}

	result, err := search.Find(context.Background(), gitexec.New(dir), query, search.Options{
		MessagesOnly: *messagesOnly,
		MaxCommits:   *limit,
	})
	if err != nil {
		return err
	}
	return flush(stdout, describeSearch(result))
}

func describeSearch(r search.Result) string {
	if len(r.Matches) == 0 {
		return nothingFound(r)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %s matching %q, out of %s no branch or tag reaches.\n",
		plural(len(r.Matches), "commit", "commits"),
		r.Query,
		plural(r.Inspected, "commit", "commits"))

	for _, m := range r.Matches {
		b.WriteString("\n" + matchHeading(m) + "\n")
		if m.InMessage {
			b.WriteString("      the commit message matches\n")
		}
		for _, hit := range m.InFiles {
			fmt.Fprintf(&b, "      %s:%d  %s\n", hit.Path, hit.Line, clip(strings.TrimSpace(hit.Text)))
		}
		fmt.Fprintf(&b, "      keep it with: git branch %s%s %s\n",
			rescuePrefix, shortHash(m.Commit.Hash), m.Commit.Hash)
	}

	b.WriteString("\nThat command only adds a branch pointing at the commit; nothing else changes.\n")
	if r.Truncated() {
		fmt.Fprintf(&b, "Only the %d most recent of %d unreachable commits were searched; raise --limit to widen it.\n",
			r.Inspected, r.Orphans)
	}
	return b.String()
}

func nothingFound(r search.Result) string {
	if r.Orphans == 0 {
		return "git-rewind: no unreachable commits in this repository, so there is nothing lost to search.\n"
	}
	return fmt.Sprintf("git-rewind: nothing matching %q in the %s searched.\n\n"+
		"Git expires unreachable objects after 90 days by default, so work lost long ago may\n"+
		"already be gone. Searching file contents as well as messages is the default; --messages\n"+
		"restricts it to messages.\n",
		r.Query, plural(r.Inspected, "unreachable commit", "unreachable commits"))
}

func matchHeading(m search.Match) string {
	return fmt.Sprintf("  %s  %s  %s  %q",
		shortHash(m.Commit.Hash),
		m.Commit.When.Format("2006-01-02 15:04"),
		m.Commit.Author,
		m.Commit.Subject)
}

func clip(s string) string {
	if len(s) <= maxMatchLineLen {
		return s
	}
	return s[:maxMatchLineLen] + "..."
}

func shortHash(hash string) string {
	if len(hash) > shortHashLen {
		return hash[:shortHashLen]
	}
	return hash
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
