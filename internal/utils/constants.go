package utils

// Default Actions - used in configuration validation
const (
	DefaultActionManualReview = "manual_review"
	DefaultActionAutoApprove  = "auto_approve"
)

// MR States - used in webhook processing
const (
	MRStateOpened = "opened"
	MRStateClosed = "closed"
	MRStateMerged = "merged"
)
