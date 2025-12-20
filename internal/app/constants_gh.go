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
	GitHubPrStateDraft  = "DRAFT"

	DisplayPrStateOpen   = "Open"
	DisplayPrStateMerged = "Merged"
	DisplayPrStateClosed = "Closed"
	DisplayPrStateDraft  = "Draft"
)
