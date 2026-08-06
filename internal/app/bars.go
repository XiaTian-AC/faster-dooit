package app

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// resizeDebounceMsg fires after the debounce window to apply the final
// pending size (dragging reports many sizes; we only repaint on the last).
type resizeDebounceMsg struct{}

// redrawTickMsg fires every 200ms to force a full repaint. Experimental:
// cheap (View() is diffed by the renderer) and keeps the terminal in sync on
// terminals that drop or miss repaints.
type redrawTickMsg struct{}

// resizeDebounce is the window in which consecutive WindowSizeMsgs are
// collapsed into a single repaint. 80ms smooths a fast drag.
const resizeDebounce = 80 * time.Millisecond

// redrawInterval is how often the UI forces a full repaint.
const redrawInterval = 200 * time.Millisecond

// startBarTick schedules the 1s tick that drives clock bar widgets.
func (m *Model) startBarTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// startRedrawTick schedules the periodic forced repaint. It keeps the whole
// screen (including the global background fill) in sync even when the
// renderer's diff skips unchanged rows.
func (m *Model) startRedrawTick() tea.Cmd {
	return tea.Tick(redrawInterval, func(time.Time) tea.Msg {
		return redrawTickMsg{}
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

// pollTerminalSize checks the actual terminal size and returns a command.
// On a detected change it records the pending size and schedules a debounced
// apply (one repaint); identical sizes just reschedule the poll. The poll
// deliberately does NOT emit a WindowSizeMsg directly — that would force a
// renderer repaint on every polled frame during a drag.
func (m *Model) pollTerminalSize() tea.Cmd {
	if w, h, ok := terminalSize(); ok {
		return m.resizeCmdFromSize(w, h)
	}
	// Can't query the size (e.g. piped output) — do nothing.
	return m.startResizeTick()
}

// resizeCmdFromSize returns the command for a newly-detected size: a
// debounced apply when it differs from the current size, otherwise a bare
// reschedule. Extracted for testing.
func (m *Model) resizeCmdFromSize(w, h int) tea.Cmd {
	if w == m.width && h == m.height {
		return m.startResizeTick()
	}
	m.pendingResize = &pendingResizeState{w: w, h: h}
	return tea.Batch(
		tea.Tick(resizeDebounce, func(time.Time) tea.Msg {
			return resizeDebounceMsg{}
		}),
		m.startResizeTick(),
	)
}// renderStatusBar returns the single-line status area: Lua bar widgets (or
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
		// Show both the active search filter and any notice (e.g. a
		// recurrence due-advance) — they must not overwrite each other.
		var parts []string
		if m.filter != "" {
			parts = append(parts, " search: "+m.filter+" ")
		}
		if m.notice != "" {
			parts = append(parts, " "+m.notice+" ")
		}
		right = strings.Join(parts, "")
	}
	return th.Style("primary").Render(mode) + pad(max(0, m.width-len(mode)-len(right))) + th.Style("secondary").Render(right)
}

// renderLuaBar invokes each config bar widget and joins the results.
func (m *Model) renderLuaBar(th theme.Theme) string {
	var parts []string
	for _, w := range m.luaCfg.Bar {
		text, style, err := m.luaCfg.CallFormatter(w.Fn, nil, nil, m.luaCfg.Theme)
		if err == nil && text != "" {
			// The widget's style is a hex color; fall back to primary.
			if style == "" {
				style = th.Primary
			}
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color(style)).Render(text))
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
