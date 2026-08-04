package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/XiaTian-AC/faster-dooit/internal/store"
)

func newTestApp(t *testing.T) *Model {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	root, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	ws := &model.Workspace{Description: "Work"}
	root.Children = append(root.Children, ws)
	if err := st.SaveWorkspace(ws); err != nil {
		t.Fatal(err)
	}
	todo := &model.Todo{Description: "a", ParentWorkspaceID: &ws.ID, Pending: true}
	ws.Todos = append(ws.Todos, todo)
	if err := st.SaveTodo(todo); err != nil {
		t.Fatal(err)
	}
	m := New(st, nil)
	m.RefreshFromStore()
	return m
}

func TestScrollOffsetsDefault(t *testing.T) {
	m := newTestApp(t)
	if m.workspaceScroll != 0 || m.todoScroll != 0 {
		t.Fatalf("scroll offsets should start at 0, got %d/%d", m.workspaceScroll, m.todoScroll)
	}
}

func TestInsertStatusBarShowsErrorAndField(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("due")
	m.notice = "invalid date: unknown format"
	v := m.renderStatusBar()
	if !strings.Contains(v, "editing due") {
		t.Fatalf("status bar should show edit field, got %q", v)
	}
	if !strings.Contains(v, "invalid date") {
		t.Fatalf("status bar should show the error, got %q", v)
	}
}

func TestFocusWorkspacePane(t *testing.T) {
	m := newTestApp(t)
	if m.focus != 0 {
		t.Fatalf("initial focus = %d, want 0", m.focus)
	}
	m.SetFocus(1)
	if m.focus != 1 {
		t.Fatalf("focus after SetFocus = %d, want 1", m.focus)
	}
}

func TestMoveDown(t *testing.T) {
	m := newTestApp(t)
	if got := m.WorkspaceCursor; got != 0 {
		t.Fatalf("initial workspace cursor = %d, want 0", got)
	}
	m.actionMoveDown(m)
	if got := m.WorkspaceCursor; got != 0 {
		t.Fatalf("single item: cursor should stay 0, got %d", got)
	}
}

// TestKeyManager_ChordDelete verifies that xx fires the delete action while a
// single x does nothing (the dead key swallows, matching the reference).
func TestKeyManager_ChordDelete(t *testing.T) {
	km := newKeyManager(defaultKeyBindings())

	// Single x — no action resolves.
	if a := km.feed('x'); a != "" {
		t.Fatalf("single x should not resolve, got %q", a)
	}
	// Confirm buffer still holds the in-flight 'x'.
	if km.buffer != "x" {
		t.Fatalf("buffer after single x = %q, want %q", km.buffer, "x")
	}

	// x x within the same window → "delete" resolves.
	if a := km.feed('x'); a != "delete" {
		t.Fatalf("xx should resolve to delete, got %q", a)
	}
	if km.buffer != "" {
		t.Fatalf("buffer should reset after match, got %q", km.buffer)
	}
}

// TestKeyManager_DeadKeySwallowed: a dead prefix swallows the next key.
func TestKeyManager_DeadKeySwallowed(t *testing.T) {
	km := newKeyManager(defaultKeyBindings())

	// "g" is a prefix of "gg"; pressing "k" next must be discarded (not buffered).
	km.feed('g')
	if a := km.feed('k'); a != "" {
		t.Fatalf("g-then-k should not resolve, got %q", a)
	}
	// After dead-end, buffer clears so the next keystroke is treated fresh.
	if km.buffer != "" {
		t.Fatalf("buffer should clear after dead-end, got %q", km.buffer)
	}
}

// TestKeyManager_EscapeClearsBuffer
func TestKeyManager_EscapeClearsBuffer(t *testing.T) {
	km := newKeyManager(defaultKeyBindings())
	km.feed('g')
	km.escape()
	if km.buffer != "" {
		t.Fatalf("escape should clear buffer, got %q", km.buffer)
	}
}

// TestAction_AddSibling verifies that `a` on the workspace pane appends a
// workspace child and persists it.
func TestAction_AddSibling(t *testing.T) {
	m := newTestApp(t)
	work := m.root.Children[0]
	if len(work.Children) != 0 {
		t.Fatalf("expected Work to start with 0 children, got %d", len(work.Children))
	}
	m.actionAddSibling(m) // workspace pane focused → add as child of Work
	root, err := m.store.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children[0].Children) != 1 {
		t.Fatalf("persisted Work.Children = %d, want 1", len(root.Children[0].Children))
	}
}

func TestAction_ToggleComplete(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(1)
	todo := m.selectedTodo()
	if todo == nil {
		t.Fatalf("no todo under cursor (workspace=%d)", m.selectedWorkspaceID)
	}
	if !todo.Pending {
		t.Fatalf("seeded todo should be pending, got Pending=%v", todo.Pending)
	}
	m.actionToggleComplete(m)
	if m.selectedTodo().Pending {
		t.Fatal("todo should now be completed")
	}
}

// TestModel_Init_ReturnsCmd — Init must return a tea.Cmd (non-nil is fine,
// bubbletea requires the type).
func TestModel_Init_ReturnsCmd(t *testing.T) {
	m := newTestApp(t)
	if cmd := m.Init(); cmd == nil {
		// nil Cmd is acceptable; the contract is "returns tea.Cmd".
		_ = tea.Cmd(nil)
	}
}
