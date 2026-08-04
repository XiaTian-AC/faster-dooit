package app

import (
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"github.com/XiaTian-AC/faster-dooit/internal/theme"
)

// bars.go renders the status / notification / confirm areas.
//
// The status bar is built from the config.lua bar widgets when present; the
// 1s clock tick is decoupled from the row cache (it never bumps the version,
// so row caches are not invalidated every second).

// tickMsg carries the current time to the bar widgets once per second.
type tickMsg time.Time

// resizeTickMsg triggers a terminal-size poll. Windows has no SIGWINCH, so
// Bubble Tea never learns about resizes on its own; polling is the standard
// cross-platform fallback.
type resizeTickMsg struct{}

// startBarTick schedules the 1s tick that drives clock bar widgets.
func (m *Model) startBarTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// startResizeTick schedules a periodic terminal-size poll. When the detected
// size differs from the model's current width/height, it produces a
// tea.WindowSizeMsg so the normal resize path (scroll reset + repaint) runs.
func (m *Model) startResizeTick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
		return resizeTickMsg{}
	})
}

// pollTerminalSize checks the actual terminal size and returns a
// tea.WindowSizeMsg if it changed, or nil to keep the current frame.
func (m *Model) pollTerminalSize() tea.Cmd {
	w, h, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		// Can't query the size (e.g. piped output) — do nothing.
		return m.startResizeTick()
	}
	return m.resizeCmdFromSize(w, h)
}

// resizeCmdFromSize builds the resize command for a newly-detected size:
// a WindowSizeMsg when it differs from the model's current size, otherwise a
// bare reschedule. Extracted for testing.
func (m *Model) resizeCmdFromSize(w, h int) tea.Cmd {
	if w == m.width && h == m.height {
		return m.startResizeTick()
	}
	return tea.Batch(
		func() tea.Msg { return tea.WindowSizeMsg{Width: w, Height: h} },
		m.startResizeTick(),
	)
}

// renderStatusBar returns the single-line status area: Lua bar widgets (or
// the mode chip + notice fallback).
func (m *Model) renderStatusBar() string {
	th := m.appTheme()

	// Config bar widgets take precedence.
	if m.luaCfg != nil && len(m.luaCfg.Bar) > 0 {
		return m.renderLuaBar(th)
	}

	mode := " " + string(m.mode) + " "
	right := ""
	switch m.mode {
	case ModeConfirm:
		right = " " + m.input.Placeholder + " "
	case ModeInsert:
		// Split: left shows what's being edited; right shows a validation
		// error from the last confirm (if any), else the field notice.
		left := " editing " + m.editField + " "
		right := ""
		if m.notice != "" {
			right = " " + m.notice + " "
		}
		return th.Style("primary").Render(mode) + pad(max(0, m.width-len(mode)-len(left)-len(right))) +
			th.Style("secondary").Render(left+right)
	case ModeSearch, ModeSort:
		right = " " + m.input.View() + " "
	default:
		if m.notice != "" {
			right = " " + m.notice + " "
		}
	}
	return th.Style("primary").Render(mode) + pad(max(0, m.width-len(mode)-len(right))) + th.Style("secondary").Render(right)
}

// renderLuaBar invokes each config bar widget and joins the results.
func (m *Model) renderLuaBar(th theme.Theme) string {
	var parts []string
	for _, w := range m.luaCfg.Bar {
		text, err := m.luaCfg.CallFormatter(w.Fn, nil, nil, m.luaCfg.Theme)
		if err == nil && text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return " " + string(m.mode) + " "
	}
	return strings.Join(parts, "")
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
