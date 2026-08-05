package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
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

// TestEditDuePersists verifies an edited due survives a reload from the store.
func TestEditDuePersists(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("due")
	m.input.SetValue("tomorrow")
	m.ConfirmEdit()

	root, err := m.store.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	todo := root.Children[0].Todos[0]
	if todo.Due == nil {
		t.Fatal("due not persisted to store")
	}
	tomorrow := time.Now().AddDate(0, 0, 1)
	if !sameDayApp(*todo.Due, tomorrow) {
		t.Fatalf("persisted due = %v, want tomorrow %v", todo.Due, tomorrow)
	}
}

func sameDayApp(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
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

// TestSearchEnterMovesCursorToList: pressing Enter applies the filter and
// moves the cursor to the list so the user can operate on the results; the
// filter stays active (shown in the status bar) until Esc.
func TestSearchEnterMovesCursorToList(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	// Two todos: only "Buy milk" matches "milk".
	first := m.selectedTodo()
	first.Description = "Buy milk"
	if err := m.store.SaveTodo(first); err != nil {
		t.Fatal(err)
	}
	other := &model.Todo{Description: "Write report", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
	if err := m.store.SaveTodo(other); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}

	m.StartSearch()
	m.input.SetValue("milk")
	m.confirmMode()

	if m.mode != ModeNormal {
		t.Fatalf("Enter should return to NORMAL (cursor usable), got %v", m.mode)
	}
	if m.filter != "milk" {
		t.Fatalf("filter should stay active, got %q", m.filter)
	}
	// Cursor must land on the first matching item.
	sel := m.selectedTodo()
	if sel == nil || sel.Description != "Buy milk" {
		t.Fatalf("cursor should be on the matching item, got %+v", sel)
	}
}

// TestSearchEscapeClearsAndShowsAll: pressing Esc in SEARCH clears the filter
// and returns to NORMAL so the full list is shown again.
func TestSearchEscapeClearsAndShowsAll(t *testing.T) {
	m := newTestApp(t)
	m.StartSearch()
	m.input.SetValue("milk")
	m.filter = "milk"
	m.handleModeKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != ModeNormal {
		t.Fatalf("Esc should exit SEARCH, got %v", m.mode)
	}
	if m.filter != "" {
		t.Fatalf("filter should be cleared on Esc, got %q", m.filter)
	}
}

// TestNormalEscapeClearsFilter: after applying a search filter, Esc in NORMAL
// mode clears it so the full list shows again.
func TestNormalEscapeClearsFilter(t *testing.T) {
	m := newTestApp(t)
	m.filter = "milk"
	m.mode = ModeNormal
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.filter != "" {
		t.Fatalf("NORMAL Esc should clear the filter, got %q", m.filter)
	}
}

// TestEscPreservesCursorItem: after a filtered search lands the cursor on a
// match, Esc must clear the filter while keeping the cursor on that SAME item
// (by ID) in the full list — not jump to whatever index happens to match.
func TestEscPreservesCursorItem(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	first := m.selectedTodo()
	first.Description = "plain one"
	if err := m.store.SaveTodo(first); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"milk B", "plain two", "plain three"} {
		td := &model.Todo{Description: d, Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(td); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}

	// Filter "milk" → cursor lands on "milk B".
	m.StartSearch()
	m.input.SetValue("milk")
	m.confirmMode()
	if m.mode != ModeNormal {
		t.Fatalf("expected NORMAL, got %v", m.mode)
	}
	if sel := m.selectedTodo(); sel == nil || sel.Description != "milk B" {
		t.Fatalf("cursor should be on milk B, got %+v", sel)
	}
	milkBID := m.selectedTodo().ID

	// Esc clears the filter; cursor must still point at milk B.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.filter != "" {
		t.Fatalf("filter should be cleared, got %q", m.filter)
	}
	if sel := m.selectedTodo(); sel == nil || sel.ID != milkBID {
		t.Fatalf("cursor should stay on milk B after Esc, got %+v", sel)
	}
}

// TestStatusBarShowsActiveFilter: with a filter applied in NORMAL mode, the
// status bar shows the search term so the user knows the list is filtered.
func TestStatusBarShowsActiveFilter(t *testing.T) {
	m := newTestApp(t)
	m.mode = ModeNormal
	m.filter = "milk"
	v := m.renderStatusBar()
	if !strings.Contains(v, "milk") {
		t.Fatalf("status bar should show the active filter, got %q", v)
	}
}

// TestSearchAddExitsAndCreates: pressing 'a' while searching exits SEARCH
// (clearing the filter) and runs the normal add-sibling flow (enters the
// inline edit for the new item).
func TestSearchAddExitsAndCreates(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartSearch()
	m.input.SetValue("milk")
	m.filter = "milk"

	m.handleModeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.mode == ModeSearch {
		t.Fatalf("pressing a in SEARCH should exit search, mode = %v", m.mode)
	}
	if m.filter != "" {
		t.Fatalf("filter should be cleared when exiting search via a, got %q", m.filter)
	}
	// The add flow should have started the inline edit (INSERT mode).
	if m.mode != ModeInsert {
		t.Fatalf("a should start inline edit (INSERT), got %v", m.mode)
	}
}

// TestSearchModeAddChildUnderSelected: pressing A directly while in SEARCH
// (without pressing Enter first) must still add a child under the matching
// item — the search input is applied as a filter first so the cursor lands on
// a real match before add_child runs.
func TestSearchModeAddChildUnderSelected(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	first := m.selectedTodo()
	first.Description = "plain one"
	if err := m.store.SaveTodo(first); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"milk B", "plain two"} {
		td := &model.Todo{Description: d, Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(td); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}

	// Type "milk" into the search box, then press A (no Enter first).
	m.StartSearch()
	m.input.SetValue("milk")
	m.handleModeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	if m.mode == ModeSearch {
		t.Fatalf("A should exit SEARCH, got %v", m.mode)
	}
	// The child must be under "milk B", not "plain one".
	root, err := m.store.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	ws := root.Children[0]
	var milkB *model.Todo
	for _, td := range ws.Todos {
		if td.Description == "milk B" {
			milkB = td
		}
	}
	if milkB == nil {
		t.Fatal("milk B not found")
	}
	if len(milkB.Todos) != 1 {
		t.Fatalf("milk B should have exactly 1 child, got %d", len(milkB.Todos))
	}
}

// TestSearchAddChildAddsUnderSelected: pressing A while searching must add a
// child under the SELECTED (filtered) item — not under whatever the cursor
// index happens to point to once the filter is cleared.
func TestSearchAddChildAddsUnderSelected(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	// Three todos; the second one ("milk B") matches "milk" and is the one
	// the cursor selects after filtering.
	todos := []*model.Todo{
		{Description: "plain one", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID},
		{Description: "milk B", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID},
		{Description: "plain two", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID},
	}
	for _, td := range todos {
		if err := m.store.SaveTodo(td); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}

	// Filter "milk" → only "milk B" is visible; cursor lands on it.
	m.StartSearch()
	m.input.SetValue("milk")
	m.confirmMode()
	if m.mode != ModeNormal {
		t.Fatalf("expected NORMAL after search confirm, got %v", m.mode)
	}
	sel := m.selectedTodo()
	if sel == nil || sel.Description != "milk B" {
		t.Fatalf("cursor should be on milk B, got %+v", sel)
	}
	selectedID := sel.ID

	// Press A while the filter is active: adds a child under milk B.
	// Go through Update so the NORMAL-mode keymap dispatches add_child.
	m.Update(keyMsg('A'))
	if m.mode == ModeSearch {
		t.Fatalf("A should exit search, got %v", m.mode)
	}
	if m.filter != "" {
		t.Fatalf("A should clear the filter, got %q", m.filter)
	}

	// The child must be under milk B, not a sibling elsewhere.
	root, err := m.store.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	ws := root.Children[0]
	for _, td := range ws.Todos {
		if td.ID == selectedID {
			if len(td.Todos) != 1 {
				t.Fatalf("milk B should have exactly 1 child, got %d", len(td.Todos))
			}
			return
		}
	}
	t.Fatalf("selected milk B (%d) not found after reload", selectedID)
}

// TestAddClearsFilterShowsNewItem: adding a new item (a/A) with a filter
// active clears the filter so the freshly created item is visible.
func TestAddClearsFilterShowsNewItem(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.filter = "milk"
	m.mode = ModeNormal

	m.actionAddSibling(m)
	if m.filter != "" {
		t.Fatalf("adding should clear the filter, got %q", m.filter)
	}
	if m.mode != ModeInsert {
		t.Fatalf("add should start inline edit (INSERT), got %v", m.mode)
	}
}

// TestEditDueEmptyClears: confirming an empty due input removes the due date.
func TestEditDueEmptyClears(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	now := time.Now()
	todo := m.selectedTodo()
	todo.Due = &now
	m.store.SaveTodo(todo)

	m.StartEdit("due")
	m.input.SetValue("")
	m.ConfirmEdit()
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want NORMAL", m.mode)
	}
	if td := m.selectedTodo(); td.Due != nil {
		t.Fatalf("due should be cleared, got %v", td.Due)
	}
}

// TestEditEffortBoundaries: effort accepts non-negative integers and rejects
// negatives / non-numeric input (staying in INSERT).
func TestEditEffortBoundaries(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		m := newTestApp(t)
		m.SetFocus(PaneTodo)
		m.StartEdit("effort")
		m.input.SetValue("30")
		m.ConfirmEdit()
		if m.mode != ModeNormal {
			t.Fatalf("mode = %v, want NORMAL", m.mode)
		}
		if td := m.selectedTodo(); td.Effort != 30 {
			t.Fatalf("effort = %d, want 30", td.Effort)
		}
	})
	t.Run("negative-rejected", func(t *testing.T) {
		m := newTestApp(t)
		m.SetFocus(PaneTodo)
		m.StartEdit("effort")
		m.input.SetValue("-5")
		m.ConfirmEdit()
		if m.mode != ModeInsert {
			t.Fatalf("negative effort should stay editing, mode = %v", m.mode)
		}
	})
	t.Run("non-numeric-rejected", func(t *testing.T) {
		m := newTestApp(t)
		m.SetFocus(PaneTodo)
		m.StartEdit("effort")
		m.input.SetValue("abc")
		m.ConfirmEdit()
		if m.mode != ModeInsert {
			t.Fatalf("non-numeric effort should stay editing, mode = %v", m.mode)
		}
	})
	t.Run("empty-keeps", func(t *testing.T) {
		m := newTestApp(t)
		m.SetFocus(PaneTodo)
		m.StartEdit("effort")
		m.input.SetValue("")
		m.ConfirmEdit()
		if m.mode != ModeNormal {
			t.Fatalf("empty effort should confirm, mode = %v", m.mode)
		}
	})
}

// TestEditRecurrenceBoundaries: recurrence accepts duration tokens and rejects
// malformed input (staying in INSERT). Setting one forces the todo pending.
func TestEditRecurrenceBoundaries(t *testing.T) {
	t.Run("valid-sets-pending", func(t *testing.T) {
		m := newTestApp(t)
		m.SetFocus(PaneTodo)
		m.StartEdit("recurrence")
		m.input.SetValue("2d")
		m.ConfirmEdit()
		if m.mode != ModeNormal {
			t.Fatalf("mode = %v, want NORMAL", m.mode)
		}
		td := m.selectedTodo()
		if td.Recurrence == nil {
			t.Fatal("recurrence not set")
		}
		if !td.Pending {
			t.Fatal("setting recurrence must force pending")
		}
	})
	t.Run("invalid-rejected", func(t *testing.T) {
		m := newTestApp(t)
		m.SetFocus(PaneTodo)
		m.StartEdit("recurrence")
		m.input.SetValue("2x")
		m.ConfirmEdit()
		if m.mode != ModeInsert {
			t.Fatalf("invalid recurrence should stay editing, mode = %v", m.mode)
		}
	})
	t.Run("empty-clears", func(t *testing.T) {
		m := newTestApp(t)
		m.SetFocus(PaneTodo)
		m.StartEdit("recurrence")
		m.input.SetValue("")
		m.ConfirmEdit()
		if m.mode != ModeNormal {
			t.Fatalf("empty recurrence should confirm, mode = %v", m.mode)
		}
		if td := m.selectedTodo(); td.Recurrence != nil {
			t.Fatalf("recurrence should be cleared, got %v", td.Recurrence)
		}
	})
}
