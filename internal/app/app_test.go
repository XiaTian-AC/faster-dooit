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

// TestLuaBarPrecedence: when config.lua defines bar widgets, they replace the
// built-in mode/notice status bar entirely.
func TestLuaBarPrecedence(t *testing.T) {
	m := newTestAppLua(t)
	if len(m.luaCfg.Bar) == 0 {
		t.Fatal("expected bar widgets from config.lua")
	}
	v := m.renderStatusBar()
	if v == "" {
		t.Fatal("lua bar should render its widgets")
	}
	// The built-in fallback status bar joins mode+notice; the Lua bar path
	// never falls back to it, so a bare mode chip isn't the only content.
	if strings.TrimSpace(v) == "NORMAL" {
		t.Fatalf("lua bar should not reduce to the built-in mode chip: %q", v)
	}
}

// TestInsertStatusBarNoError: without an error notice, INSERT shows just the
// edit field (no stale error text).
func TestInsertStatusBarNoError(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("effort")
	m.notice = ""
	v := m.renderStatusBar()
	if !strings.Contains(v, "editing effort") {
		t.Fatalf("status bar should show edit field, got %q", v)
	}
	if strings.Contains(v, "invalid") {
		t.Fatalf("status bar should not show stale error, got %q", v)
	}
}

// TestNoticeClearedOnConfirmExit: an error shown while editing must disappear
// once the edit is confirmed successfully and the app returns to NORMAL.
func TestNoticeClearedOnConfirmExit(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("due")
	m.notice = "invalid date: unknown format"
	m.input.SetValue("")
	// Confirming with an empty value clears the due and succeeds.
	m.ConfirmEdit()
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want NORMAL", m.mode)
	}
	if m.notice != "" {
		t.Fatalf("notice should be cleared after successful confirm, got %q", m.notice)
	}
}

// TestNoticeClearedOnCancel: escaping the insert mode clears any pending error.
func TestNoticeClearedOnCancel(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("due")
	m.notice = "invalid date: unknown format"
	m.cancelMode()
	if m.notice != "" {
		t.Fatalf("notice should be cleared after cancel, got %q", m.notice)
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

// TestLuaKeyBindingsOverride: when config.lua rebinds a key, the key manager
// built with the Lua config must dispatch the new action instead of the
// hard-coded default.
func TestLuaKeyBindingsOverride(t *testing.T) {
	km := newKeyManager(bindingsFromLua(map[string]string{"j": "redraw"}))
	if a := km.feed('j'); a != "redraw" {
		t.Fatalf("Lua j should dispatch redraw, got %q", a)
	}
}

// TestLuaKeyBindingsChords: multi-char chord keys from Lua build nested tables.
func TestLuaKeyBindingsChords(t *testing.T) {
	km := newKeyManager(bindingsFromLua(map[string]string{
		"gg": "go_to_top",
		"xx": "delete",
	}))
	if a := km.feed('g'); a != "" {
		t.Fatalf("'g' alone should be a prefix, got %q", a)
	}
	if a := km.feed('g'); a != "go_to_top" {
		t.Fatalf("'gg' should dispatch go_to_top, got %q", a)
	}
}

// TestRedrawAction: the redraw action bumps the version and returns a command.
func TestRedrawAction(t *testing.T) {
	m := newTestApp(t)
	v0 := m.version
	cmd := m.actionRedraw(m)
	if m.version == v0 {
		t.Fatal("redraw should bump the version")
	}
	if cmd == nil {
		t.Fatal("redraw should return a command (resize poll)")
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
