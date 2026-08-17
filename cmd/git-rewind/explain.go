package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

type diagnosis struct {
	head      gitexec.Head
	status    safety.Status
	events    []timeline.Event
	orphans   int
	rescue    recipes.Recipe
	now       time.Time
	unhealthy bool
}

func runExplain(args []string, dir string, stdout io.Writer, p *i18n.Printer) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stdout)
	asJSON := fs.Bool("json", false, "print the diagnosis as JSON instead of text")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	git := gitexec.New(dir)

	events, err := timeline.Load(ctx, git, allEvents)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		if *asJSON {
			return writeJSON(stdout, jsonExplain{Schema: jsonSchema, Command: "explain"})
		}
		return flush(stdout, p.T(i18n.ExplainNoHistory))
	}

	d := diagnosis{events: events, now: time.Now()}
	if d.head, err = git.HeadState(ctx); err != nil {
		return err
	}
	if d.status, err = safety.WorkingTreeStatus(ctx, git); err != nil {
		return err
	}
	orphans, err := git.Orphans(ctx)
	if err != nil {
		return err
	}
	d.orphans = len(orphans)

	rescue, plan, err := chooseRescue(ctx, &recipes.Repo{Git: git, Events: events, Printer: p})
	if err != nil {
		return err
	}
	if plan != nil {
		d.rescue = rescue
	}
	d.unhealthy = d.orphans > 0 || d.head.Detached

	if *asJSON {
		last := toJSONEvent(d.events[0], p)
		return writeJSON(stdout, jsonExplain{
			Schema:      jsonSchema,
			Command:     "explain",
			Head:        toJSONHead(d.head),
			WorkingTree: toJSONWorkingTree(d.status),
			LastEvent:   &last,
			Unreachable: d.orphans,
			Rescue:      toJSONRescue(d.rescue, p),
		})
	}
	return flush(stdout, describeState(p, d))
}

func describeState(p *i18n.Printer, d diagnosis) string {
	fields := [][2]string{
		{p.T(i18n.ExplainFieldHead), headSummary(p, d.head)},
		{p.T(i18n.ExplainFieldWorkingTree), treeSummary(p, d.status)},
		{p.T(i18n.ExplainFieldLastEvent), eventSummary(p, d)},
		{p.T(i18n.ExplainFieldUnreachable), orphanSummary(p, d.orphans)},
	}
	width := 0
	for _, f := range fields {
		if n := utf8.RuneCountInString(f[0]); n > width {
			width = n
		}
	}

	var b strings.Builder
	b.WriteString(p.T(i18n.ExplainHeading))
	for _, f := range fields {
		b.WriteString(stateField(f[0], f[1], width))
	}

	b.WriteString("\n")
	if d.rescue != nil {
		b.WriteString(p.T(i18n.ExplainCanUndo, d.rescue.Title(p)))
		b.WriteString(p.T(i18n.ExplainReviewHint))
	}
	if d.orphans > 0 {
		b.WriteString(p.T(i18n.ExplainFindHint))
	}
	if d.rescue == nil && !d.unhealthy {
		b.WriteString(p.T(i18n.ExplainNothingWrong))
	}
	return b.String()
}

func stateField(label, value string, width int) string {
	padding := strings.Repeat(" ", width-utf8.RuneCountInString(label))
	return "  " + label + padding + "  " + value + "\n"
}

func headSummary(p *i18n.Printer, h gitexec.Head) string {
	if h.Detached {
		return p.T(i18n.ExplainHeadDetached, shortHash(h.Hash))
	}
	return p.T(i18n.ExplainHeadOnBranch, h.Branch, shortHash(h.Hash))
}

func treeSummary(p *i18n.Printer, s safety.Status) string {
	if s.Clean {
		return p.T(i18n.ExplainTreeClean)
	}
	return p.T(i18n.ExplainTreeDirty, changeCount(p, len(s.Changes)))
}

func changeCount(p *i18n.Printer, n int) string {
	if n == 1 {
		return p.T(i18n.ExplainChangeSingular, n)
	}
	return p.T(i18n.ExplainChangePlural, n)
}

func eventSummary(p *i18n.Printer, d diagnosis) string {
	last := d.events[0]
	return fmt.Sprintf("%s  %s", timeline.AgoPhrase(p, last.Entry.Time, d.now), last.Describe(p))
}

func orphanSummary(p *i18n.Printer, n int) string {
	switch n {
	case 0:
		return p.T(i18n.ExplainUnreachableNone)
	case 1:
		return p.T(i18n.ExplainUnreachableOne, n)
	default:
		return p.T(i18n.ExplainUnreachableMany, n)
	}
}
