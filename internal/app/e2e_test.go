package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// keyMsg builds a rune KeyMsg (matches what the terminal delivers for a
// single printable key).
func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// cast reinterprets the tea.Model returned by Update back into our *Model.
func cast(m tea.Model) *Model {
	return m.(*Model)
}

// TestE2EKeypressToPersistence drives real KeyMsg messages through Update
// (the same path bubbletea's tea.TestModel exercises) and verifies the edit
// survives a fresh load from the store: tab → edit description → type →
// enter. Mirrors the DX review amendment "端到端首次运行冒烟".
func TestE2EKeypressToPersistence(t *testing.T) {
	m := newTestApp(t)

	// tab → focus the todo pane (starts on the workspace pane).
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = cast(next)
	if cmd != nil {
		t.Fatalf("tab should not return a command, got %v", cmd)
	}
	if m.focus != PaneTodo {
		t.Fatalf("tab should focus the todo pane, got %d", m.focus)
	}

	// enter → enter_edit_description, pre-filling the current "a".
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = cast(next)
	if m.mode != ModeInsert {
		t.Fatalf("enter should enter INSERT mode, got %v", m.mode)
	}
	if got := m.input.Value(); got != "a" {
		t.Fatalf("edit should pre-fill current description %q, got %q", "a", got)
	}

	// type "bc" → description becomes "abc".
	for _, r := range "bc" {
		m.Update(keyMsg(r))
	}
	if got := m.input.Value(); got != "abc" {
		t.Fatalf("typed description = %q, want %q", got, "abc")
	}

	// enter → confirm edit, back to NORMAL.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = cast(next)
	if m.mode != ModeNormal {
		t.Fatalf("confirm should return to NORMAL, got %v", m.mode)
	}

	// Persistence: reload a fresh tree from the same store and check the
	// edited description is there.
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	todo := m.selectedTodo()
	if todo == nil {
		t.Fatal("expected a selected todo after reload")
	}
	if todo.Description != "abc" {
		t.Fatalf("persisted description = %q, want %q", todo.Description, "abc")
	}
}

// TestE2EAddSiblingAndEdit walks add_sibling (a) → type → enter and confirms
// the new todo is inserted, starts an inline edit, and its description
// persists.
func TestE2EAddSiblingAndEdit(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0

	// a → add_sibling creates a new todo and opens an inline edit on it.
	next, _ := m.Update(keyMsg('a'))
	m = cast(next)
	if m.mode != ModeInsert {
		t.Fatalf("add_sibling should enter INSERT for inline edit, got %v", m.mode)
	}
	if m.input.Placeholder != "New task" {
		t.Fatalf("new todo placeholder should be %q, got %q", "New task", m.input.Placeholder)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	todos := m.visibleTodos()
	if len(todos) != 2 {
		t.Fatalf("want 2 todos after add_sibling, got %d", len(todos))
	}

	// Type a real name, replacing the placeholder default.
	for _, r := range "new task" {
		m.Update(keyMsg(r))
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = cast(next)
	if m.mode != ModeNormal {
		t.Fatalf("confirm should return to NORMAL, got %v", m.mode)
	}

	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	todos = m.visibleTodos()
	if len(todos) != 2 {
		t.Fatalf("want 2 todos after edit, got %d", len(todos))
	}
	if !strings.Contains(todos[1].Description, "new task") {
		t.Fatalf("edited sibling description = %q, want it to contain %q", todos[1].Description, "new task")
	}
}
