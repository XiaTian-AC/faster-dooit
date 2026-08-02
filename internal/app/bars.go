package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/XiaTian-AC/faster-dooit/internal/theme"
)

// bars.go renders the status / notification / confirm areas.
//
// The status bar is built from the config.lua bar widgets when present; the
// 1s clock tick is decoupled from the row cache (it never bumps the version,
// so row caches are not invalidated every second).

// tickMsg carries the current time to the bar widgets once per second.
type tickMsg time.Time

// startBarTick schedules the 1s tick that drives clock bar widgets.
func (m *Model) startBarTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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
	case ModeInsert, ModeSearch, ModeSort:
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
