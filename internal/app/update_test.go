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

// TestResizeCmdDetectsChange: the size-change branch must record a pending
// size and schedule a debounced apply (a tick command); same-size reschedules.
func TestResizeCmdDetectsChange(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 120, 30

	// Same size → pending stays nil, just a reschedule command.
	cmd := m.resizeCmdFromSize(120, 30)
	if cmd == nil {
		t.Fatal("same-size poll should still reschedule")
	}
	if m.pendingResize != nil {
		t.Fatal("same-size poll must not set pending resize")
	}

	// Changed size → pending recorded, command returns a BatchMsg of ticks.
	cmd = m.resizeCmdFromSize(80, 14)
	if cmd == nil {
		t.Fatal("changed-size poll should return a command")
	}
	if m.pendingResize == nil || m.pendingResize.w != 80 || m.pendingResize.h != 14 {
		t.Fatalf("pending resize not recorded: %+v", m.pendingResize)
	}
}

// TestResizeCmdEachDirection: width-only, height-only, and both-changed
// resizes must each set a pending resize; no-change must not.
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
			hasPending := m.pendingResize != nil
			if hasPending != c.wantChangeMsg {
				t.Fatalf("resize set pending = %v, want %v", hasPending, c.wantChangeMsg)
			}
		})
	}
}

// TestSlashForcesRefresh: the "/" key (bound to api.redraw by default) must
// bump the render version and trigger a terminal-size poll — a manual redraw
// for terminals that don't report resizes.
func TestSlashForcesRefresh(t *testing.T) {
	m := newTestAppLua(t)
	v0 := m.version
	_, cmd := m.Update(keyMsg('/'))
	if m.version == v0 {
		t.Fatal(`"/" should bump the render version`)
	}
	if cmd == nil {
		t.Fatal(`"/" should return a command (resize poll)`)
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

// TestResizeRefreshesLayout: an event-driven WindowSizeMsg applies immediately
// (size, scroll reset, version bump). The poll path (resizeCmdFromSize) only
// records a pending size; the debounce tick applies it and emits one
// WindowSizeMsg so the renderer repaints a single frame.
func TestResizeRefreshesLayout(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 8
	m.todoScroll = 50 // stale scroll from a larger terminal
	m.width, m.height = 150, 30

	// Event-driven resize: applies immediately.
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 150, Height: 14})
	if cmd != nil {
		t.Fatalf("event-driven resize should not return a command, got %v", cmd)
	}
	if m.height != 14 || m.width != 150 {
		t.Fatalf("size not applied: %dx%d", m.width, m.height)
	}
	if m.todoScroll != 0 || m.workspaceScroll != 0 {
		t.Fatalf("scroll should reset on resize, got %d/%d", m.workspaceScroll, m.todoScroll)
	}

	// Poll path: records pending, does not apply yet.
	m.todoScroll = 5
	_ = m.resizeCmdFromSize(130, 12)
	if m.width != 150 || m.height != 14 {
		t.Fatalf("poll path must not apply before debounce: %dx%d", m.width, m.height)
	}
	if m.pendingResize == nil {
		t.Fatal("poll path should record a pending size")
	}
	// Debounce tick applies it and emits one WindowSizeMsg.
	_, cmd2 := m.Update(resizeDebounceMsg{})
	if m.width != 130 || m.height != 12 {
		t.Fatalf("debounce did not apply final size: %dx%d", m.width, m.height)
	}
	if m.todoScroll != 0 {
		t.Fatalf("scroll should reset on debounce, got %d", m.todoScroll)
	}
	if cmd2 == nil {
		t.Fatal("debounce tick should return a command (WindowSizeMsg)")
	}

	// Render must clamp the stale scroll into range (no panic, no overflow).
	v := m.View()
	for _, line := range splitLines(v) {
		if lw := lipgloss.Width(line); lw > 130 {
			t.Errorf("line overflows after resize by %d cols: %q", lw-130, line)
		}
	}
}

// TestResizeDebounceCollapsesBursts: repeated poll detections collapse into a
// single pending size; the version does not bump per frame.
func TestResizeDebounceCollapsesBursts(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 150, 30
	v0 := m.version
	for i := 1; i <= 10; i++ {
		_ = m.resizeCmdFromSize(150-i, 30)
	}
	// No repaint happened during the burst (version unchanged, only pending set).
	if m.version != v0 {
		t.Fatalf("version should not bump during debounce, got %d -> %d", v0, m.version)
	}
	if m.pendingResize == nil || m.pendingResize.w != 140 {
		t.Fatalf("pending should hold the last size, got %+v", m.pendingResize)
	}
	// One debounce tick applies the final size and repaints once.
	_, cmd := m.Update(resizeDebounceMsg{})
	if m.width != 140 {
		t.Fatalf("final size = %d, want 140", m.width)
	}
	if m.version == v0 {
		t.Fatal("debounce tick should bump the version")
	}
	if cmd == nil {
		t.Fatal("debounce tick should return a command (WindowSizeMsg)")
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
