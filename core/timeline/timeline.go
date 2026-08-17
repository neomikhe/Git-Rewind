package timeline

import (
	"strings"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// Risk is how dangerous an event is to the user's committed work.
type Risk int

// Risk levels, from safe forward progress to operations that can discard committed work.
const (
	RiskGreen Risk = iota
	RiskYellow
	RiskRed
)

func (r Risk) String() string {
	switch r {
	case RiskGreen:
		return "green"
	case RiskYellow:
		return "yellow"
	case RiskRed:
		return "red"
	default:
		return "unknown"
	}
}

// Kind is the classified operation behind a reflog entry.
type Kind int

// Kind values name the operation behind a reflog entry; KindOther covers the unrecognized.
const (
	KindOther Kind = iota
	KindCommit
	KindInitialCommit
	KindAmend
	KindReset
	KindCheckout
	KindMerge
	KindRebase
	KindPull
	KindBranch
	KindClone
	KindCherryPick
	KindRevert
)

func (k Kind) String() string {
	switch k {
	case KindCommit:
		return "commit"
	case KindInitialCommit:
		return "initial-commit"
	case KindAmend:
		return "amend"
	case KindReset:
		return "reset"
	case KindCheckout:
		return "checkout"
	case KindMerge:
		return "merge"
	case KindRebase:
		return "rebase"
	case KindPull:
		return "pull"
	case KindBranch:
		return "branch"
	case KindClone:
		return "clone"
	case KindCherryPick:
		return "cherry-pick"
	case KindRevert:
		return "revert"
	default:
		return "other"
	}
}

// Event is a single reflog entry classified by kind and risk, with any commits it stranded.
type Event struct {
	Entry    gitexec.ReflogEntry
	Kind     Kind
	Risk     Risk
	Orphaned []string
}

// FromReflog classifies reflog entries into events, preserving their most-recent-first order.
func FromReflog(entries []gitexec.ReflogEntry) []Event {
	events := make([]Event, len(entries))
	for i, entry := range entries {
		kind := classify(entry.Operation)
		events[i] = Event{Entry: entry, Kind: kind, Risk: riskOf(kind)}
	}
	return events
}

// AttachOrphans records, on each history-rewriting event, the recoverable tip it made unreachable.
func AttachOrphans(events []Event, orphans map[string]struct{}) {
	for i := range events {
		if !canOrphan(events[i].Kind) {
			continue
		}
		prev, ok := previousValue(events, i)
		if !ok || prev == events[i].Entry.Hash {
			continue
		}
		if _, isOrphan := orphans[prev]; isOrphan {
			events[i].Orphaned = append(events[i].Orphaned, prev)
		}
	}
}

func canOrphan(kind Kind) bool {
	switch kind {
	case KindReset, KindAmend, KindRebase:
		return true
	default:
		return false
	}
}

func previousValue(events []Event, i int) (string, bool) {
	if i+1 >= len(events) {
		return "", false
	}
	return events[i+1].Entry.Hash, true
}

func classify(operation string) Kind {
	op := strings.TrimSpace(operation)
	switch {
	case strings.HasPrefix(op, "commit (amend)"):
		return KindAmend
	case strings.HasPrefix(op, "commit (initial)"):
		return KindInitialCommit
	case strings.HasPrefix(op, "commit"):
		return KindCommit
	case strings.HasPrefix(op, "reset"):
		return KindReset
	case strings.HasPrefix(op, "checkout"):
		return KindCheckout
	case strings.HasPrefix(op, "merge"):
		return KindMerge
	case strings.HasPrefix(op, "rebase"):
		return KindRebase
	case strings.HasPrefix(op, "pull"):
		return KindPull
	case strings.HasPrefix(op, "clone"):
		return KindClone
	case strings.HasPrefix(op, "branch"):
		return KindBranch
	case strings.HasPrefix(op, "cherry-pick"):
		return KindCherryPick
	case strings.HasPrefix(op, "revert"):
		return KindRevert
	default:
		return KindOther
	}
}

func riskOf(kind Kind) Risk {
	switch kind {
	case KindReset, KindRebase, KindAmend:
		return RiskRed
	case KindCheckout, KindMerge, KindPull:
		return RiskYellow
	default:
		return RiskGreen
	}
}
