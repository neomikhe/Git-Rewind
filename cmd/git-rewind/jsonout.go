package main

import (
	"encoding/json"
	"io"
	"time"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/search"
	"github.com/neomikhe/git-rewind/core/timeline"
)

const jsonSchema = 1

type jsonHead struct {
	Hash     string `json:"hash"`
	Branch   string `json:"branch"`
	Detached bool   `json:"detached"`
}

type jsonWorkingTree struct {
	Clean   bool `json:"clean"`
	Changes int  `json:"changes"`
}

type jsonEvent struct {
	Hash        string    `json:"hash"`
	Time        time.Time `json:"time"`
	Kind        string    `json:"kind"`
	Risk        string    `json:"risk"`
	Description string    `json:"description"`
	Orphaned    []string  `json:"orphaned"`
}

type jsonRescue struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type jsonCommand struct {
	Args    []string `json:"args"`
	Preview string   `json:"preview"`
	Explain string   `json:"explain"`
}

type jsonFileMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type jsonMatch struct {
	Hash        string          `json:"hash"`
	ShortHash   string          `json:"shortHash"`
	Author      string          `json:"author"`
	Time        time.Time       `json:"time"`
	Subject     string          `json:"subject"`
	InMessage   bool            `json:"inMessage"`
	Files       []jsonFileMatch `json:"files"`
	KeepCommand []string        `json:"keepCommand"`
}

type jsonExplain struct {
	Schema      int             `json:"schema"`
	Command     string          `json:"command"`
	Head        jsonHead        `json:"head"`
	WorkingTree jsonWorkingTree `json:"workingTree"`
	LastEvent   *jsonEvent      `json:"lastEvent"`
	Unreachable int             `json:"unreachableCommits"`
	Rescue      *jsonRescue     `json:"rescue"`
}

type jsonFind struct {
	Schema    int         `json:"schema"`
	Command   string      `json:"command"`
	Query     string      `json:"query"`
	Orphans   int         `json:"orphanCommits"`
	Inspected int         `json:"inspected"`
	Truncated bool        `json:"truncated"`
	Matches   []jsonMatch `json:"matches"`
}

type jsonLast struct {
	Schema           int           `json:"schema"`
	Command          string        `json:"command"`
	Rescue           *jsonRescue   `json:"rescue"`
	DryRun           bool          `json:"dryRun"`
	Applied          bool          `json:"applied"`
	DiscardsChanges  bool          `json:"discardsChanges"`
	WorkingTreeClean bool          `json:"workingTreeClean"`
	Commands         []jsonCommand `json:"commands"`
	Warnings         []string      `json:"warnings"`
	BackupBranch     string        `json:"backupBranch"`
}

func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func toJSONHead(h gitexec.Head) jsonHead {
	return jsonHead{Hash: h.Hash, Branch: h.Branch, Detached: h.Detached}
}

func toJSONWorkingTree(s safety.Status) jsonWorkingTree {
	return jsonWorkingTree{Clean: s.Clean, Changes: len(s.Changes)}
}

func toJSONEvent(e timeline.Event, p *i18n.Printer) jsonEvent {
	orphaned := e.Orphaned
	if orphaned == nil {
		orphaned = []string{}
	}
	return jsonEvent{
		Hash:        e.Entry.Hash,
		Time:        e.Entry.Time,
		Kind:        e.Kind.String(),
		Risk:        e.Risk.String(),
		Description: e.Describe(p),
		Orphaned:    orphaned,
	}
}

func toJSONRescue(r recipes.Recipe, p *i18n.Printer) *jsonRescue {
	if r == nil {
		return nil
	}
	return &jsonRescue{Name: r.Name(), Title: r.Title(p)}
}

func toJSONCommands(plan safety.Plan) []jsonCommand {
	preview := plan.Preview()
	commands := make([]jsonCommand, len(plan.Commands))
	for i, c := range plan.Commands {
		commands[i] = jsonCommand{Args: c.Args, Preview: preview[i], Explain: c.Explain}
	}
	return commands
}

func lastResult(p *i18n.Printer, recipe recipes.Recipe, plan *safety.Plan, status safety.Status, dryRun bool, backup string) jsonLast {
	warnings := plan.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return jsonLast{
		Schema:           jsonSchema,
		Command:          "last",
		Rescue:           toJSONRescue(recipe, p),
		DryRun:           dryRun,
		Applied:          !dryRun,
		DiscardsChanges:  plan.DiscardsChanges,
		WorkingTreeClean: status.Clean,
		Commands:         toJSONCommands(*plan),
		Warnings:         warnings,
		BackupBranch:     backup,
	}
}

func toJSONMatches(matches []search.Match) []jsonMatch {
	out := make([]jsonMatch, len(matches))
	for i, m := range matches {
		files := make([]jsonFileMatch, len(m.InFiles))
		for j, f := range m.InFiles {
			files[j] = jsonFileMatch{Path: f.Path, Line: f.Line, Text: f.Text}
		}
		short := shortHash(m.Commit.Hash)
		out[i] = jsonMatch{
			Hash:        m.Commit.Hash,
			ShortHash:   short,
			Author:      m.Commit.Author,
			Time:        m.Commit.When,
			Subject:     m.Commit.Subject,
			InMessage:   m.InMessage,
			Files:       files,
			KeepCommand: []string{"git", "branch", rescuePrefix + short, m.Commit.Hash},
		}
	}
	return out
}
