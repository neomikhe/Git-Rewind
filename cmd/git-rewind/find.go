package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/search"
	"github.com/neomikhe/git-rewind/core/timeline"
)

const (
	maxMatchLineLen = 100
	rescuePrefix    = "rescued/"
)

func runFind(args []string, dir string, stdout io.Writer, p *i18n.Printer) error {
	fs := flag.NewFlagSet("find", flag.ContinueOnError)
	fs.SetOutput(stdout)
	messagesOnly := fs.Bool("messages", false, "search commit messages only, not file contents (faster on large repositories)")
	limit := fs.Int("limit", search.DefaultMaxCommits, "how many unreachable commits to inspect")
	asJSON := fs.Bool("json", false, "print the matches as JSON instead of text")

	leading, rest := splitLeadingText(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}

	query := strings.TrimSpace(strings.Join(append(leading, fs.Args()...), " "))
	if query == "" {
		return errors.New(p.T(i18n.FindNoQuery))
	}

	result, err := search.Find(context.Background(), gitexec.New(dir), query, search.Options{
		MessagesOnly: *messagesOnly,
		MaxCommits:   *limit,
	})
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(stdout, jsonFind{
			Schema:    jsonSchema,
			Command:   "find",
			Query:     result.Query,
			Orphans:   result.Orphans,
			Inspected: result.Inspected,
			Truncated: result.Truncated(),
			Matches:   toJSONMatches(result.Matches),
		})
	}
	return flush(stdout, describeSearch(p, result))
}

func splitLeadingText(args []string) (text, rest []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			return args[:i], args[i:]
		}
	}
	return args, nil
}

func describeSearch(p *i18n.Printer, r search.Result) string {
	if len(r.Matches) == 0 {
		return nothingFound(p, r)
	}

	var b strings.Builder
	b.WriteString(p.T(i18n.FindHeading,
		r.Query,
		timeline.CommitCount(p, len(r.Matches)),
		timeline.CommitCount(p, r.Inspected)))

	for _, m := range r.Matches {
		b.WriteString("\n" + matchHeading(m) + "\n")
		if m.InMessage {
			b.WriteString(p.T(i18n.FindMessageMatches))
		}
		for _, hit := range m.InFiles {
			fmt.Fprintf(&b, "      %s:%d  %s\n", hit.Path, hit.Line, clip(strings.TrimSpace(hit.Text)))
		}
		b.WriteString(p.T(i18n.FindKeepWith, rescuePrefix+shortHash(m.Commit.Hash), m.Commit.Hash))
	}

	b.WriteString(p.T(i18n.FindOnlyAddsBranch))
	if r.Truncated() {
		b.WriteString(p.T(i18n.FindTruncated, r.Inspected, r.Orphans))
	}
	return b.String()
}

func nothingFound(p *i18n.Printer, r search.Result) string {
	if r.Orphans == 0 {
		return p.T(i18n.FindNoOrphans)
	}
	return p.T(i18n.FindNoMatch, r.Query, unreachableCount(p, r.Inspected))
}

func unreachableCount(p *i18n.Printer, n int) string {
	if n == 1 {
		return p.T(i18n.FindUnreachableSingular, n)
	}
	return p.T(i18n.FindUnreachablePlural, n)
}

func matchHeading(m search.Match) string {
	return fmt.Sprintf("  %s  %s  %s  %q",
		shortHash(m.Commit.Hash),
		m.Commit.When.Format("2006-01-02 15:04"),
		m.Commit.Author,
		m.Commit.Subject)
}

// clip shortens a matched line by runes, not bytes: a matched file can contain any UTF-8,
// and slicing it by byte would cut a character in half and emit invalid output.
func clip(s string) string {
	if utf8.RuneCountInString(s) <= maxMatchLineLen {
		return s
	}
	return string([]rune(s)[:maxMatchLineLen]) + "..."
}

func shortHash(hash string) string {
	if len(hash) > shortHashLen {
		return hash[:shortHashLen]
	}
	return hash
}
