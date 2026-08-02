package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpKeyRows is the keybinding reference shown by the help screen.
var helpKeyRows = [][2]string{
	{"j / k", "move down / up"},
	{"gg / G", "go to top / bottom"},
	{"i / d / r / e", "edit description / due / recurrence / effort"},
	{"a / A", "add sibling / child"},
	{"z / Z", "toggle expand / expand parent"},
	{"J / K", "shift down / up"},
	{"xx", "delete (with confirm)"},
	{"y / Y", "copy description / copy item"},
	{"p / P", "paste below / above"},
	{"c", "toggle complete"},
	{"= / +  - / _", "increase / decrease urgency"},
	{"/", "search"},
	{"ctrl+s", "sort"},
	{"tab", "switch pane"},
	{"enter", "edit description"},
	{"?", "show this help"},
	{"ctrl+c / ctrl+q", "quit"},
}

// HelpView renders the keybinding reference.
func (m *Model) HelpView() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Keybindings") + "\n\n")
	for _, row := range helpKeyRows {
		b.WriteString("  " + row[0] + pad(max(0, 20-len(row[0]))) + row[1] + "\n")
	}
	b.WriteString("\n  Press any key to return.\n")
	return b.String()
}
