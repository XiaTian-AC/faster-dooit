package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model. Routes messages through the key manager
// (when in NORMAL mode and a KeyMsg arrives), updates the model, and
// returns the bubbletea.Cmd produced by the resolved action.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.bumpVersion()
		return m, nil

	case noticeMsg:
		m.notice = string(msg)
		m.bumpVersion()
		return m, nil

	case tea.KeyMsg:
		// Ctrl-key shortcuts handled here (the keyManager is rune-based).
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlQ:
			if a, ok := m.actions["quit"]; ok {
				return m, a(m)
			}
			return m, tea.Quit
		case tea.KeyCtrlS:
			if a, ok := m.actions["start_sort"]; ok {
				return m, a(m)
			}
		}
		if m.mode != ModeNormal {
			// input overlays (Task 4) handle non-NORMAL modes; for now
			// the skeleton just falls back to escape-to-normal.
			if msg.Type == tea.KeyEsc {
				m.mode = ModeNormal
			}
			return m, nil
		}

		action := m.keys.feed(msg.Runes[0])
		if action == "" {
			return m, nil
		}
		if a, ok := m.actions[action]; ok {
			return m, a(m)
		}
	}
	return m, nil
}
