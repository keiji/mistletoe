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

// ANSI Color Codes
const (
	AnsiReset    = "\033[0m"
	AnsiFgRed    = "\033[31m"
	AnsiFgGreen  = "\033[32m"
	AnsiFgYellow = "\033[33m"
	AnsiFgGray   = "\033[90m"
)
