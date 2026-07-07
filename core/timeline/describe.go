package timeline

import (
	"fmt"
	"strings"
)

// This file holds every user-facing sentence the timeline produces. Keeping the
// wording in one place is deliberate: the EN/ES internationalization planned for
// v1.0 only needs to translate what lives here, not touch any logic. Strings are
// English for now, per the repo's default language convention.

// Describe returns a short, human-readable description of the event, including
// whether it left any commit recoverable.
func (e Event) Describe() string {
	base := describeOperation(e)
	if n := len(e.Orphaned); n > 0 {
		base += fmt.Sprintf(" (%s left unreachable, recoverable)", plural(n, "commit", "commits"))
	}
	return base
}

// describeOperation phrases what the event's operation did, drawing on the
// reflog subject for specifics (the target of a reset, the branches of a
// checkout, the commit message, and so on).
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
		// detail is git's "moving to <target>"; drop the "moving" filler.
		return "Reset the branch " + strings.TrimPrefix(detail, "moving ")
	case KindCheckout:
		// detail is git's "moving from <a> to <b>".
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
		// Unrecognized operation: show git's own words rather than guess.
		return e.Entry.Subject
	}
}

// mergeSentence names the merged ref, which git puts in the operation label
// (for example "merge feature").
func mergeSentence(operation string) string {
	target := strings.TrimPrefix(operation, "merge ")
	if target == "" || target == "merge" {
		return "Merged"
	}
	return "Merged " + target
}

// detailOf returns the part of a reflog subject after the first ": " separator,
// or an empty string when there is none.
func detailOf(subject string) string {
	_, detail, ok := strings.Cut(subject, ": ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(detail)
}

// withMessage appends a quoted message to a prefix, or returns the prefix alone
// when there is no message.
func withMessage(prefix, message string) string {
	if message == "" {
		return prefix
	}
	return fmt.Sprintf("%s %q", prefix, message)
}

// plural formats a count with the right noun form.
func plural(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
