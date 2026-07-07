package timeline

import (
	"strings"

	"github.com/neomikhe/git-rewind/core/gitexec"
)

// Risk is how dangerous an event is to the user's committed work. It is the
// green/yellow/red signal the timeline highlights so a user can find the moment
// something went wrong.
type Risk int

const (
	// RiskGreen is normal, safe forward progress (a plain commit, a new branch).
	RiskGreen Risk = iota
	// RiskYellow changes what HEAD or a branch points at, or pulls in outside
	// changes; recoverable and usually intentional, but worth noticing.
	RiskYellow
	// RiskRed can discard or rewrite committed history, so work may have become
	// unreachable from its branch.
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

// Kind values name the operation behind a reflog entry; KindOther covers
// anything the timeline does not specifically recognize.
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

// Event is a single reflog entry classified by kind and risk.
type Event struct {
	// Entry is the underlying reflog entry the event was derived from.
	Entry gitexec.ReflogEntry
	// Kind is the operation the entry represents.
	Kind Kind
	// Risk is how dangerous that operation is to committed work.
	Risk Risk
	// Orphaned holds commit hashes this event made unreachable from any branch
	// or tag but that still exist and can be recovered. It is populated by
	// AttachOrphans; nil until then.
	Orphaned []string
}

// FromReflog classifies reflog entries into timeline events, preserving the
// reflog's most-recent-first order.
func FromReflog(entries []gitexec.ReflogEntry) []Event {
	events := make([]Event, len(entries))
	for i, entry := range entries {
		kind := classify(entry.Operation)
		events[i] = Event{Entry: entry, Kind: kind, Risk: riskOf(kind)}
	}
	return events
}

// AttachOrphans links each history-rewriting event to the commit tip it made
// unreachable, when that tip appears in the orphan set (as returned by
// gitexec.Orphans). The orphaned tip is the ref's previous value — the hash
// recorded by the next-older reflog entry — because consecutive HEAD reflog
// entries chain. Events keep their reflog order; the input slice is modified in
// place.
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

// canOrphan reports whether a kind can leave a previously referenced commit
// unreachable.
func canOrphan(kind Kind) bool {
	switch kind {
	case KindReset, KindAmend, KindRebase:
		return true
	default:
		return false
	}
}

// previousValue returns the ref value in effect before event i — the hash of the
// next-older reflog entry — or false when i is the oldest entry.
func previousValue(events []Event, i int) (string, bool) {
	if i+1 >= len(events) {
		return "", false
	}
	return events[i+1].Entry.Hash, true
}

// classify maps a reflog operation label (the text before the first colon, such
// as "commit (amend)" or "merge feature") to a Kind. Order matters: the more
// specific commit variants are checked before the plain "commit" prefix.
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

// riskOf assigns a risk level to a kind. Operations that can rewrite or discard
// committed history are red; operations that only move refs or pull in outside
// changes are yellow; additive, forward progress is green.
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
