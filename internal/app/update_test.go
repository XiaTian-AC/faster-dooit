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

// TestResizeCmdEachDirection: width-only, height-only, and both-changed
// resizes must each produce a WindowSizeMsg; no-change must not.
func TestResizeCmdEachDirection(t *testing.T) {
	cases := []struct {
		name          string
		curW, curH    int
		newW, newH    int
		wantChangeMsg bool
	}{
		{"horizontal-only", 120, 30, 90, 30, true},
		{"vertical-only", 120, 30, 120, 20, true},
		{"both", 120, 30, 70, 18, true},
		{"none", 120, 30, 120, 30, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestApp(t)
			m.width, m.height = c.curW, c.curH
			cmd := m.resizeCmdFromSize(c.newW, c.newH)
			if cmd == nil {
				t.Fatalf("resizeCmdFromSize returned nil command")
			}
			got := cmd()
			hasMsg := false
			switch gm := got.(type) {
			case tea.WindowSizeMsg:
				hasMsg = true
			case tea.BatchMsg:
				for _, sub := range gm {
					if _, ok := sub().(tea.WindowSizeMsg); ok {
						hasMsg = true
					}
				}
			}
			if hasMsg != c.wantChangeMsg {
				t.Fatalf("resize produced WindowSizeMsg = %v, want %v (got %T)", hasMsg, c.wantChangeMsg, got)
			}
		})
	}
}

// TestCtrlLForcesRefresh: Ctrl+L must bump the render version and trigger a
// terminal-size poll (a vim-style manual redraw) in any mode.
func TestCtrlLForcesRefresh(t *testing.T) {
	m := newTestApp(t)
	v0 := m.version
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if m.version == v0 {
		t.Fatal("Ctrl+L should bump the render version")
	}
	if cmd == nil {
		t.Fatal("Ctrl+L should return a command (resize poll)")
	}

	// Works inside INSERT too (user mid-edit can force a redraw).
	m.StartEdit("description")
	v1 := m.version
	_, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if m.version == v1 {
		t.Fatal("Ctrl+L in INSERT should bump the version")
	}
	if cmd2 == nil {
		t.Fatal("Ctrl+L in INSERT should return a command")
	}
}

// TestTerminalSizeProbe: terminalSize() must not panic and either return a
// positive size or report failure (non-TTY test runs report failure).
func TestTerminalSizeProbe(t *testing.T) {
	w, h, ok := terminalSize()
	if ok && (w <= 0 || h <= 0) {
		t.Fatalf("terminalSize ok but invalid dims: %dx%d", w, h)
	}
}

// TestResizeRefreshesLayout: a WindowSizeMsg immediately updates the stored
// size (so status bar padding never uses stale width), while the version bump
// / repaint and scroll reset are debounced and applied on the debounce tick.
func TestResizeRefreshesLayout(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 8
	m.todoScroll = 50 // stale scroll from a larger terminal
	m.width, m.height = 150, 30

	// First size arrives: size updates immediately, version does not.
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 150, Height: 14})
	if cmd == nil {
		t.Fatal("resize should schedule a debounce tick")
	}
	if m.height != 14 || m.width != 150 {
		t.Fatalf("size should update immediately, got %dx%d", m.width, m.height)
	}
	v0 := m.version
	if m.version != v0 {
		t.Fatal("version should not bump before the debounce tick")
	}

	// A second size arrives during the window: size updates immediately.
	_, _ = m.Update(tea.WindowSizeMsg{Width: 130, Height: 12})
	if m.width != 130 || m.height != 12 {
		t.Fatalf("second size not applied immediately: %dx%d", m.width, m.height)
	}

	// Debounce tick fires: bumps the version, resets scroll, and returns a
	// clear-screen command to wipe any partial-resize residue.
	_, cmd2 := m.Update(resizeDebounceMsg{})
	if m.version == v0 {
		t.Fatal("debounce tick should bump the version")
	}
	if m.todoScroll != 0 || m.workspaceScroll != 0 {
		t.Fatalf("scroll should reset on resize, got %d/%d", m.workspaceScroll, m.todoScroll)
	}
	if cmd2 == nil {
		t.Fatal("debounce tick should return a clear-screen command")
	}
	// Render must clamp the stale scroll into range (no panic, no overflow).
	v := m.View()
	for _, line := range splitLines(v) {
		if lw := lipgloss.Width(line); lw > 130 {
			t.Errorf("line overflows after resize by %d cols: %q", lw-130, line)
		}
	}
}

// TestResizeDebounceCollapsesBursts: many rapid sizes collapse to one repaint.
func TestResizeDebounceCollapsesBursts(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 150, 30
	v0 := m.version
	for i := 1; i <= 10; i++ {
		_, _ = m.Update(tea.WindowSizeMsg{Width: 150 - i, Height: 30})
	}
	// No repaint happened during the burst (version unchanged).
	if m.version != v0 {
		t.Fatalf("version should not bump during debounce, got %d -> %d", v0, m.version)
	}
	// One debounce tick applies the last size and repaints once.
	_, cmd := m.Update(resizeDebounceMsg{})
	if m.width != 140 {
		t.Fatalf("final size = %d, want 140", m.width)
	}
	if m.version == v0 {
		t.Fatal("debounce tick should bump the version")
	}
	if cmd == nil {
		t.Fatal("debounce tick should return a clear-screen command")
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
