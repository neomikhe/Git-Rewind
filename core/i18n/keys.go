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
	LastFailedWithBackup

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

	RecipeUndoCommitTitle
	RecipeUndoCommitWarning
	RecipeUndoCommitStep
	RecipeUndoCommitHardTitle
	RecipeUndoCommitHardWarning
	RecipeUndoCommitHardStep
	RecipeUndoAmendTitle
	RecipeUndoAmendWarning
	RecipeUndoAmendStep
	RecipeRecoverResetTitle
	RecipeRecoverResetWarning
	RecipeRecoverResetStep
	RecipeRestoreBranchTitle
	RecipeRestoreBranchWarning
	RecipeRestoreBranchStep
	RecipeUndoMergeTitle
	RecipeUndoMergeWarning
	RecipeUndoMergeStep
	RecipeUndoRebaseTitle
	RecipeUndoRebaseWarning
	RecipeUndoRebaseStep
	RecipeRestoreStashTitle
	RecipeRestoreStashWarning
	RecipeRestoreStashStep

	TuiTimelineTitle
	TuiTimelineTitleMore
	TuiEventTitle
	TuiFieldWhen
	TuiFieldKind
	TuiFieldRisk
	TuiFieldCommit
	TuiFieldWho
	TuiFieldWhat
	TuiFieldReflog
	TuiRecoverableHeading
	TuiRescuesTitle
	TuiNoRescues
	TuiDirtyNotice
	TuiConfirmTitle
	TuiWillRun
	TuiBackupPromise
	TuiConfirmDirty
	TuiDoneTitle
	TuiDoneBackup
	TuiDoneRan
	TuiDoneRerun
	TuiWorking
	TuiError
	TuiErrDirtyTree
	TuiErrApplyFailed
	TuiHelpTitle
	TuiHelpSafety
	TuiHelpFooter
	TuiScreenTimeline
	TuiScreenDetail
	TuiScreenRescues
	TuiScreenConfirm
	TuiScreenResult
	KeyMove
	KeyMoveTimeline
	KeyMoveRescues
	KeyDetails
	KeyOpenDetail
	KeyRescues
	KeyListRescues
	KeyMore
	KeyLoadOlder
	KeyBack
	KeyBackTimeline
	KeyBackDetail
	KeyBackRescues
	KeyReview
	KeyReviewCommands
	KeyApply
	KeyApplyRescue
	KeyApplyDiscard
	KeyApplyDiscardLong
	KeyQuit
	KeyQuitLong
	KeyHelp
	KeyHelpLong

	numKeys
)
