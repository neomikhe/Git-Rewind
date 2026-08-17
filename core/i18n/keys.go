package i18n

// Key identifies one message. Every Key must appear in every catalogue; a test enforces it.
type Key int

// The messages git-rewind can show, grouped by the surface they belong to.
const (
	EventFirstCommit Key = iota
	EventCommit
	EventAmend
	EventReset
	EventCheckout
	EventMerged
	EventMergedInto
	EventRebase
	EventPull
	EventBranch
	EventClone
	EventCherryPick
	EventRevert
	EventOrphanSuffixOne
	EventOrphanSuffixMany
	CommitSingular
	CommitPlural
	TimeJustNow
	TimeAgo

	LastNothingToUndo
	LastRescueHeading
	LastWillRun
	LastDirtyWarning
	LastDryRun
	LastAbortedDirty
	LastDone

	FindNoOrphans
	FindNoMatch
	FindHeading
	FindMessageMatches
	FindKeepWith
	FindOnlyAddsBranch
	FindTruncated
	FindNoQuery
	FindUnreachableSingular
	FindUnreachablePlural

	ExplainNoHistory
	ExplainHeading
	ExplainFieldHead
	ExplainFieldWorkingTree
	ExplainFieldLastEvent
	ExplainFieldUnreachable
	ExplainHeadOnBranch
	ExplainHeadDetached
	ExplainTreeClean
	ExplainTreeDirty
	ExplainUnreachableNone
	ExplainUnreachableOne
	ExplainUnreachableMany
	ExplainCanUndo
	ExplainReviewHint
	ExplainFindHint
	ExplainNothingWrong
	ExplainChangeSingular
	ExplainChangePlural

	RootNoHistory
	RootUnknownCommand

	numKeys
)
