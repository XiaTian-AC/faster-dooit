package app

// Mode is the current input mode. Reserved literal `date` is kept for
// parity with the original dooit ModeType literal but is never activated
// in this port (due editing goes through INSERT + dateparse).
type Mode string

const (
	ModeNormal  Mode = "NORMAL"
	ModeInsert  Mode = "INSERT"
	ModeDate    Mode = "DATE" // reserved, never activated
	ModeSearch  Mode = "SEARCH"
	ModeSort    Mode = "SORT"
	ModeConfirm Mode = "CONFIRM"
)

// Pane identifies which of the two trees has focus.
const (
	PaneWorkspace = 0
	PaneTodo      = 1
)
