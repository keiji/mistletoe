package app

// Type represents the type of the application.
type Type int

const (
	// TypeMstl represents the 'mstl' application.
	TypeMstl Type = iota
	// TypeMstlGh represents the 'mstl-gh' application.
	TypeMstlGh
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
	CmdHelp     = "help"
	CmdVersion  = "version"
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
