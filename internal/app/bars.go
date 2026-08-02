package app

// bars.go renders the status / notification / confirm areas.
//
// Task 6 wires these to Lua-configured bar widgets; for now the skeleton
// renders the mode, the notice, and the active confirm prompt.

// renderStatusBar returns the single-line status area: mode chip on the
// left, notice or confirm prompt on the right.
func (m *Model) renderStatusBar() string {
	mode := " " + string(m.mode) + " "
	right := ""
	switch m.mode {
	case ModeConfirm:
		right = " " + m.input.Placeholder + " "
	case ModeInsert, ModeSearch, ModeSort:
		right = " " + m.input.View() + " "
	default:
		if m.notice != "" {
			right = " " + m.notice + " "
		}
	}
	return " " + mode + pad(m.width-len(mode)-len(right)) + right
}

func pad(n int) string {
	if n <= 0 {
		return " "
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
