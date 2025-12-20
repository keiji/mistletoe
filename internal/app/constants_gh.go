package app

// GH-specific subcommand constants
const (
	CmdPr            = "pr"
	CmdCreate        = "create"
	CmdCheckout      = "checkout"
	PrTitleMaxLength = 256
)

// GitHub PR State Constants
const (
	GitHubPrStateOpen   = "OPEN"
	GitHubPrStateMerged = "MERGED"
	GitHubPrStateClosed = "CLOSED"
	// DRAFT is not a valid state in GitHub API (it's Open + IsDraft)

	DisplayPrStateOpen   = "Open"
	DisplayPrStateMerged = "Merged"
	DisplayPrStateClosed = "Closed"
	DisplayPrStateDraft  = "Draft"
)
