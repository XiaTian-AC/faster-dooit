package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// TestResizePollReschedules: a resizeTickMsg must be handled without
// crashing and must reschedule the poll command (in non-TTY test runs the
// size query fails and we simply reschedule, keeping the loop alive).
func TestResizePollReschedules(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 120, 30
	m.todoScroll = 5
	_, cmd := m.Update(resizeTickMsg{})
	if cmd == nil {
		t.Fatal("resizeTickMsg should reschedule the poll command")
	}
}

// TestResizeCmdDetectsChange: the size-change branch must emit a
// WindowSizeMsg (driving scroll reset + repaint) only when the size differs.
func TestResizeCmdDetectsChange(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 120, 30

	// Same size → no WindowSizeMsg, just a reschedule.
	msg := m.resizeCmdFromSize(120, 30)
	if msg == nil {
		t.Fatal("same-size poll should still reschedule")
	}

	// Changed size → the command stream must carry a WindowSizeMsg.
	cmd := m.resizeCmdFromSize(80, 14)
	if cmd == nil {
		t.Fatal("changed-size poll should return a command")
	}
	got := cmd()
	switch gm := got.(type) {
	case tea.WindowSizeMsg:
		// directly a window size
	case tea.BatchMsg:
		found := false
		for _, sub := range gm {
			if _, ok := sub().(tea.WindowSizeMsg); ok {
				found = true
			}
		}
		if !found {
			t.Fatalf("BatchMsg should contain a WindowSizeMsg, got %#v", gm)
		}
	default:
		t.Fatalf("changed-size poll should emit WindowSizeMsg or BatchMsg, got %T", got)
	}
}

// TestResizeRefreshesLayout: a WindowSizeMsg must update the dimensions,
// bump the version, and reset scroll offsets so a shrink from tall to short
// doesn't leave the old scroll sticking out of the new viewport.
func TestResizeRefreshesLayout(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 8
	m.todoScroll = 50 // stale scroll from a larger terminal
	m.width, m.height = 150, 30

	// Shrink to a short terminal.
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 150, Height: 14})
	if cmd != nil {
		t.Fatalf("resize should not produce a command, got %v", cmd)
	}
	if m.width != 150 || m.height != 14 {
		t.Fatalf("size not updated: %dx%d", m.width, m.height)
	}
	if m.todoScroll != 0 || m.workspaceScroll != 0 {
		t.Fatalf("scroll should reset on resize, got %d/%d", m.workspaceScroll, m.todoScroll)
	}
	// Render must clamp the stale scroll into range (no panic, no overflow).
	v := m.View()
	for _, line := range splitLines(v) {
		if lw := lipgloss.Width(line); lw > 150 {
			t.Errorf("line overflows after resize by %d cols: %q", lw-150, line)
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
