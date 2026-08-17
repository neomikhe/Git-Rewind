package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
)

const fieldWidth = 13

type diagnosis struct {
	head      gitexec.Head
	status    safety.Status
	events    []timeline.Event
	orphans   int
	rescue    recipes.Recipe
	now       time.Time
	unhealthy bool
}

func runExplain(args []string, dir string, stdout io.Writer) error {
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
		return flush(stdout, "git-rewind: no repository history yet, so there is nothing to explain.\n")
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

	rescue, plan, err := chooseRescue(ctx, &recipes.Repo{Git: git, Events: events})
	if err != nil {
		return err
	}
	if plan != nil {
		d.rescue = rescue
	}
	d.unhealthy = d.orphans > 0 || d.head.Detached

	if *asJSON {
		last := toJSONEvent(d.events[0])
		return writeJSON(stdout, jsonExplain{
			Schema:      jsonSchema,
			Command:     "explain",
			Head:        toJSONHead(d.head),
			WorkingTree: toJSONWorkingTree(d.status),
			LastEvent:   &last,
			Unreachable: d.orphans,
			Rescue:      toJSONRescue(d.rescue),
		})
	}
	return flush(stdout, describeState(d))
}

func describeState(d diagnosis) string {
	var b strings.Builder
	b.WriteString("Repository state\n\n")
	b.WriteString(stateField("HEAD", headSummary(d.head)))
	b.WriteString(stateField("Working tree", treeSummary(d.status)))
	b.WriteString(stateField("Last event", eventSummary(d)))
	b.WriteString(stateField("Unreachable", orphanSummary(d.orphans)))

	b.WriteString("\n")
	if d.rescue != nil {
		b.WriteString("Something can be undone: " + d.rescue.Title() + "\n")
		b.WriteString("  \"git rewind\" reviews it interactively; \"git rewind last\" prints the exact commands.\n")
	}
	if d.orphans > 0 {
		b.WriteString("  \"git rewind find <text>\" searches the unreachable commits for lost work.\n")
	}
	if d.rescue == nil && !d.unhealthy {
		b.WriteString("Nothing looks wrong.\n")
	}
	return b.String()
}

func stateField(label, value string) string {
	return fmt.Sprintf("  %-*s %s\n", fieldWidth, label, value)
}

func headSummary(h gitexec.Head) string {
	if h.Detached {
		return "detached at " + shortHash(h.Hash) + " — you are not on a branch, so new commits are easy to lose"
	}
	return "on branch " + h.Branch + " at " + shortHash(h.Hash)
}

func treeSummary(s safety.Status) string {
	if s.Clean {
		return "clean"
	}
	return plural(len(s.Changes), "uncommitted change", "uncommitted changes") +
		" — a backup branch does not preserve these"
}

func eventSummary(d diagnosis) string {
	last := d.events[0]
	return fmt.Sprintf("%s  %s", timeline.AgoPhrase(last.Entry.Time, d.now), last.Describe())
}

func orphanSummary(n int) string {
	if n == 0 {
		return "nothing — no commit is missing from your branches"
	}
	return plural(n, "commit", "commits") + " no branch or tag reaches, still recoverable"
}
