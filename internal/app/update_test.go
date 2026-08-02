package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHLSwitchFocus(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneWorkspace)
	_, _ = m.Update(keyMsg('l'))
	if m.focus != PaneTodo {
		t.Fatalf("l should switch to todo pane, got %d", m.focus)
	}
	_, _ = m.Update(keyMsg('h'))
	if m.focus != PaneWorkspace {
		t.Fatalf("h should switch back to workspace pane, got %d", m.focus)
	}
}

func TestHelpAnyKeyReturns(t *testing.T) {
	m := newTestApp(t)
	m.actionShowHelp(m)
	if !m.helpVisible {
		t.Fatal("help should be visible")
	}
	_, _ = m.Update(keyMsg('j')) // any non-quit key
	if m.helpVisible {
		t.Fatal("any key should dismiss help")
	}
	if m.mode != ModeNormal {
		t.Fatalf("mode after help dismiss = %v", m.mode)
	}
	// The dismiss key must be swallowed — it must NOT have moved the cursor.
	if m.TodoCursor != 0 {
		t.Fatalf("help-dismiss key must be swallowed, cursor moved to %d", m.TodoCursor)
	}
}

func TestHelpQuitStillWorks(t *testing.T) {
	m := newTestApp(t)
	m.actionShowHelp(m)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if cmd == nil {
		t.Fatal("ctrl+q while help is shown should still quit")
	}
}
