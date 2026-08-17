package timeline

import (
	"fmt"
	"strings"
	"time"
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
func AgoPhrase(t, now time.Time) string {
	if rel := RelativeTime(t, now); rel != "now" {
		return rel + " ago"
	}
	return "just now"
}

// Describe returns a plain-language description of the event and what it left recoverable.
func (e Event) Describe() string {
	base := describeOperation(e)
	if n := len(e.Orphaned); n > 0 {
		base += fmt.Sprintf(" (%s left unreachable, recoverable)", plural(n, "commit", "commits"))
	}
	return base
}

func describeOperation(e Event) string {
	detail := detailOf(e.Entry.Subject)
	switch e.Kind {
	case KindInitialCommit:
		return withMessage("Made the first commit", detail)
	case KindCommit:
		return withMessage("Committed", detail)
	case KindAmend:
		return withMessage("Amended the last commit", detail)
	case KindReset:
		return "Reset the branch " + strings.TrimPrefix(detail, "moving ")
	case KindCheckout:
		return "Switched " + strings.TrimPrefix(detail, "moving ")
	case KindMerge:
		return mergeSentence(e.Entry.Operation)
	case KindRebase:
		return "Rebased the branch"
	case KindPull:
		return "Pulled from the remote"
	case KindBranch:
		return "Created a branch"
	case KindClone:
		return "Cloned the repository"
	case KindCherryPick:
		return withMessage("Cherry-picked", detail)
	case KindRevert:
		return withMessage("Reverted", detail)
	default:
		return e.Entry.Subject
	}
}

func mergeSentence(operation string) string {
	target := strings.TrimPrefix(operation, "merge ")
	if target == "" || target == "merge" {
		return "Merged"
	}
	return "Merged " + target
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

func plural(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
