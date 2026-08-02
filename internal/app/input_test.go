package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModeTransitions(t *testing.T) {
	m := newTestApp(t)
	m.StartEdit("description")
	if m.mode != ModeInsert {
		t.Fatalf("mode = %v, want INSERT", m.mode)
	}
	m.ConfirmEdit()
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want NORMAL after confirm", m.mode)
	}
}

func TestEditDueParsesDate(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(1) // todo pane
	m.StartEdit("due")
	if m.mode != ModeInsert {
		t.Fatalf("mode = %v, want INSERT", m.mode)
	}
	m.input.SetValue("not a real date at all")
	m.ConfirmEdit() // 评审修正：解析失败留在输入态是"有意的改进"（原版是退出编辑并保留旧值）
	if m.mode != ModeInsert {
		t.Fatalf("invalid due should stay editing, mode = %v", m.mode)
	}
}

func TestEditDueAppliesValid(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(1)
	m.StartEdit("due")
	m.input.SetValue("tomorrow")
	m.ConfirmEdit()
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want NORMAL after valid due", m.mode)
	}
	if td := m.selectedTodo(); td == nil || td.Due == nil {
		t.Fatalf("due not applied: %+v", m.selectedTodo())
	}
}

func TestEditDescriptionApplies(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(1)
	m.StartEdit("description")
	m.input.SetValue("new desc")
	m.ConfirmEdit()
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want NORMAL", m.mode)
	}
	todo := m.selectedTodo()
	if todo == nil {
		t.Fatal("no todo selected")
	}
	if todo.Description != "new desc" {
		t.Fatalf("description = %q, want %q", todo.Description, "new desc")
	}
	// persisted?
	root, _ := m.store.LoadAll()
	ws := root.Children[0]
	if len(ws.Todos) == 0 || ws.Todos[0].Description != "new desc" {
		t.Fatalf("description not persisted: %+v", ws.Todos)
	}
}

func TestSearchCancels(t *testing.T) {
	m := newTestApp(t)
	m.StartSearch()
	if m.mode != ModeSearch {
		t.Fatalf("mode = %v, want SEARCH", m.mode)
	}
	m.cancelMode()
	if m.mode != ModeNormal {
		t.Fatalf("mode after cancel = %v, want NORMAL", m.mode)
	}
}

func TestConfirmYThenNo(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(0)
	before := len(m.root.Children)
	m.StartConfirm()
	if m.mode != ModeConfirm {
		t.Fatalf("mode = %v, want CONFIRM", m.mode)
	}
	// N cancels — nothing deleted.
	m.confirmNo()
	if m.mode != ModeNormal {
		t.Fatalf("mode after no = %v, want NORMAL", m.mode)
	}
	if len(m.root.Children) != before {
		t.Fatalf("children changed on no: %d → %d", before, len(m.root.Children))
	}
	// Y confirms — deletes the highlighted workspace.
	m.StartConfirm()
	m.confirmYes()
	if m.mode != ModeNormal {
		t.Fatalf("mode after yes = %v, want NORMAL", m.mode)
	}
	if len(m.root.Children) != before-1 {
		t.Fatalf("children after yes = %d, want %d", len(m.root.Children), before-1)
	}
}

func TestNotify(t *testing.T) {
	m := newTestApp(t)
	m.Notify("hello", "info")
	if m.notice != "hello" {
		t.Fatalf("notice = %q, want %q", m.notice, "hello")
	}
}

func TestHelpViewNonEmpty(t *testing.T) {
	m := newTestApp(t)
	if got := m.HelpView(); got == "" {
		t.Fatal("HelpView() returned empty")
	}
}

// TestUpdateRoutesEscapeFromInsert ensures escape cancels insert mode via Update.
func TestUpdateRoutesEscapeFromInsert(t *testing.T) {
	m := newTestApp(t)
	m.StartEdit("description")
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != ModeNormal {
		t.Fatalf("mode after escape = %v, want NORMAL", m.mode)
	}
}
