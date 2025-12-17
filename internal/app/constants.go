package app

// AppType represents the type of the application.
type AppType int

const (
	// AppTypeMstl represents the 'mstl' application.
	AppTypeMstl AppType = iota
	// AppTypeMstlGh represents the 'mstl-gh' application.
	AppTypeMstlGh
)

const (
	// AppNameMstl is the display name for mstl.
	AppNameMstl = "mstl"
	// AppNameMstlGh is the display name for mstl-gh.
	AppNameMstlGh = "Mistletoe-gh"
)

// Subcommand constants
const (
	CmdInit     = "init"
	CmdSnapshot = "snapshot"
	CmdSwitch   = "switch"
	CmdStatus   = "status"
	CmdSync     = "sync"
	CmdPush     = "push"
	CmdPr       = "pr"
	CmdHelp     = "help"
	CmdVersion  = "version"
	CmdCreate   = "create"
)

// Status symbols
const (
	StatusSymbolPullable = "<"
	StatusSymbolUnpushed = ">"
	StatusSymbolConflict = "!"
)

// Status colors (internal logic identifiers)
const (
	ColorNone   = 0
	ColorYellow = 1
	ColorGreen  = 2
)
