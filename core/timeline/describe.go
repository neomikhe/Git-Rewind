package timeline

import (
	"fmt"
	"strings"
)

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
