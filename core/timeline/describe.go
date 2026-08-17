package timeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/neomikhe/git-rewind/core/i18n"
)

const hoursPerDay = 24

// RelativeTime renders how long before now something happened, compactly: now, 5m, 3h, 2d.
func RelativeTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < hoursPerDay*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/hoursPerDay))
	}
}

// AgoPhrase renders RelativeTime as a phrase: "just now", "3h ago".
func AgoPhrase(p *i18n.Printer, t, now time.Time) string {
	if rel := RelativeTime(t, now); rel != "now" {
		return p.T(i18n.TimeAgo, rel)
	}
	return p.T(i18n.TimeJustNow)
}

// Describe returns a plain-language description of the event and what it left recoverable.
func (e Event) Describe(p *i18n.Printer) string {
	base := describeOperation(e, p)
	switch n := len(e.Orphaned); {
	case n == 1:
		base += p.T(i18n.EventOrphanSuffixOne, n)
	case n > 1:
		base += p.T(i18n.EventOrphanSuffixMany, n)
	}
	return base
}

// CommitCount renders a commit total with the right singular or plural form.
func CommitCount(p *i18n.Printer, n int) string {
	if n == 1 {
		return p.T(i18n.CommitSingular, n)
	}
	return p.T(i18n.CommitPlural, n)
}

func describeOperation(e Event, p *i18n.Printer) string {
	detail := detailOf(e.Entry.Subject)
	switch e.Kind {
	case KindInitialCommit:
		return withMessage(p.T(i18n.EventFirstCommit), detail)
	case KindCommit:
		return withMessage(p.T(i18n.EventCommit), detail)
	case KindAmend:
		return withMessage(p.T(i18n.EventAmend), detail)
	case KindReset:
		return resetSentence(p, detail, e.Entry.Subject)
	case KindCheckout:
		return checkoutSentence(p, detail, e.Entry.Subject)
	case KindMerge:
		return mergeSentence(p, e.Entry.Operation)
	case KindRebase:
		return p.T(i18n.EventRebase)
	case KindPull:
		return p.T(i18n.EventPull)
	case KindBranch:
		return p.T(i18n.EventBranch)
	case KindClone:
		return p.T(i18n.EventClone)
	case KindCherryPick:
		return withMessage(p.T(i18n.EventCherryPick), detail)
	case KindRevert:
		return withMessage(p.T(i18n.EventRevert), detail)
	default:
		return e.Entry.Subject
	}
}

func resetSentence(p *i18n.Printer, detail, subject string) string {
	target, ok := strings.CutPrefix(detail, "moving to ")
	if !ok {
		return subject
	}
	return p.T(i18n.EventReset, target)
}

func checkoutSentence(p *i18n.Printer, detail, subject string) string {
	rest, ok := strings.CutPrefix(detail, "moving from ")
	if !ok {
		return subject
	}
	from, to, ok := strings.Cut(rest, " to ")
	if !ok {
		return subject
	}
	return p.T(i18n.EventCheckout, from, to)
}

func mergeSentence(p *i18n.Printer, operation string) string {
	target := strings.TrimPrefix(operation, "merge ")
	if target == "" || target == "merge" {
		return p.T(i18n.EventMerged)
	}
	return p.T(i18n.EventMergedInto, target)
}

func detailOf(subject string) string {
	_, detail, ok := strings.Cut(subject, ": ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(detail)
}

func withMessage(prefix, message string) string {
	if message == "" {
		return prefix
	}
	return fmt.Sprintf("%s %q", prefix, message)
}
