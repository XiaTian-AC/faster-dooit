package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model. Routes messages through the key manager
// (in NORMAL mode) or the active input overlay (in INSERT/SEARCH/SORT/
// CONFIRM modes), updates the model, and returns the tea.Cmd produced by
// the resolved action.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reset per-pane scroll offsets so a shrink to a shorter terminal
		// never leaves the old scroll sticking out of the new viewport.
		m.workspaceScroll = 0
		m.todoScroll = 0
		m.BumpVersion()
		return m, nil

	case noticeMsg:
		m.notice = string(msg)
		m.BumpVersion()
		return m, nil

	case tickMsg:
		// Drive the clock bar widget. Deliberately does NOT bump the version
		// so row caches stay warm (decoupled from the 1s tick).
		return m, m.startBarTick()

	case resizeTickMsg:
		// Poll the terminal size (Windows has no SIGWINCH); only repaint on
		// an actual change via a WindowSizeMsg.
		return m, m.pollTerminalSize()

	case tea.KeyMsg:
		// Global quit shortcuts work in every mode.
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlQ {
			return m, m.actions["quit"](m)
		}

		// While help is shown, swallow every other key and return to NORMAL.
		if m.helpVisible {
			m.helpVisible = false
			m.BumpVersion()
			return m, nil
		}

		// Non-NORMAL modes route keys into the input overlay / confirm.
		if m.mode != ModeNormal {
			return m, m.handleModeKey(msg)
		}

		// NORMAL-mode special keys.
		switch msg.Type {
		case tea.KeyTab:
			return m, m.actions["switch_focus"](m)
		case tea.KeyEnter:
			return m, m.actions["enter_edit_description"](m)
		case tea.KeyCtrlS:
			return m, m.actions["start_sort"](m)
		case tea.KeyEsc:
			m.keys.escape()
			return m, nil
		}

		// Rune dispatch through the key manager (handles single keys and
		// chords such as gg / xx).
		if len(msg.Runes) > 0 {
			action := m.keys.feed(msg.Runes[0])
			if action == "" {
				return m, nil
			}
			if a, ok := m.actions[action]; ok {
				return m, a(m)
			}
		}
	}
	return m, nil
}
