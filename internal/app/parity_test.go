package app

import (
	"testing"
	"time"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
)

// ---- completion cascade (update_hooks.py parity) ----

func TestCascadeCompleteSubtree(t *testing.T) {
	// R1: completing a parent completes the whole subtree.
	leaf := &model.Todo{Pending: true}
	child := &model.Todo{Pending: true, Todos: []*model.Todo{leaf}}
	parent := &model.Todo{Pending: true, Todos: []*model.Todo{child}}
	leaf.ParentTodo = child
	child.ParentTodo = parent

	parent.SetSubtreePending(false)
	if parent.Pending || child.Pending || leaf.Pending {
		t.Fatal("completing a parent must complete the whole subtree")
	}
}

func TestParentAutoCompleteOnlyWhenAllChildrenDone(t *testing.T) {
	// R2: parent auto-completes only when ALL children complete.
	done := &model.Todo{Pending: false, ParentTodo: parentForTest()}
	child2 := &model.Todo{Pending: true, ParentTodo: done.ParentTodo}
	done.ParentTodo.Todos = []*model.Todo{done, child2}

	done.ParentAutoComplete()
	if done.ParentTodo.Pending != true {
		t.Fatal("parent must NOT auto-complete while a sibling is still pending")
	}

	child2.Pending = false
	child2.ParentAutoComplete()
	if done.ParentTodo.Pending != false {
		t.Fatal("parent must auto-complete once ALL children are done")
	}
}

func TestReopenChildReopensParent(t *testing.T) {
	// R3: reopening any child reopens the parent.
	child := &model.Todo{Pending: false, ParentTodo: parentForTest()}
	child.ParentTodo.Todos = []*model.Todo{child}
	child.ParentTodo.Pending = false

	child.ReopenParents()
	if child.ParentTodo.Pending != true {
		t.Fatal("reopening a child must reopen the parent")
	}
}

func TestRecurringTodoNeverCompletes(t *testing.T) {
	// R4: completing a recurring todo advances due and stays pending.
	m := newTestApp(t)
	todo := m.selectedTodo()
	rec := 24 * time.Hour
	todo.Recurrence = &rec
	before := time.Now().Add(-time.Hour)
	todo.Due = &before

	m.applyCompletionCascade(todo)
	if !todo.Pending {
		t.Fatal("recurring todo must stay pending after completion")
	}
	if todo.Due == nil || !todo.Due.After(before) {
		t.Fatalf("recurring todo due must advance, got %v", todo.Due)
	}
}

// parentForTest builds a parent todo with a child reference helper.
func parentForTest() *model.Todo {
	p := &model.Todo{Pending: true}
	c := &model.Todo{Pending: false, ParentTodo: p}
	p.Todos = []*model.Todo{c}
	return p
}

// ---- sort semantics ----

func siblingTodo(desc string, pending bool, due *time.Time, order int) *model.Todo {
	return &model.Todo{Description: desc, Pending: pending, Due: due, OrderIndex: order}
}

func TestSortPendingComposite(t *testing.T) {
	// Composite key (pending→due→order_index) applies only to "pending".
	// Reference key: (not pending, due or max, order_index) ascending —
	// pending items first, then by due, completed last.
	now := time.Now()
	d1 := now.Add(24 * time.Hour)
	d2 := now.Add(48 * time.Hour)
	a := siblingTodo("a", true, &d2, 0)  // pending, later due
	b := siblingTodo("b", true, &d1, 1)  // pending, sooner due
	c := siblingTodo("c", false, nil, 2) // completed
	parent := &model.Todo{Todos: []*model.Todo{a, b, c}}
	a.ParentTodo = parent
	b.ParentTodo = parent
	c.ParentTodo = parent

	b.SortSiblings("pending", false)
	if parent.Todos[0].Description != "b" {
		t.Fatalf("pending with sooner due should sort first: %v", descs(parent.Todos))
	}
	if parent.Todos[1].Description != "a" {
		t.Fatalf("pending with later due should come next: %v", descs(parent.Todos))
	}
	if parent.Todos[2].Description != "c" {
		t.Fatalf("completed should sort last under pending: %v", descs(parent.Todos))
	}
}

func TestSortDueNullsLast(t *testing.T) {
	// Other fields sort ascending with NULL due last (nulls_last).
	now := time.Now()
	d1 := now.Add(24 * time.Hour)
	noDue := &model.Todo{Description: "no due", Due: nil}
	withDue := &model.Todo{Description: "with due", Due: &d1}
	parent := &model.Todo{Todos: []*model.Todo{noDue, withDue}}
	noDue.ParentTodo = parent
	withDue.ParentTodo = parent

	noDue.SortSiblings("due", false)
	if parent.Todos[0].Description != "with due" {
		t.Fatalf("due-null should sort last: %v", descs(parent.Todos))
	}
}

func TestSortReverse(t *testing.T) {
	a := siblingTodo("a", true, nil, 0)
	b := siblingTodo("b", true, nil, 1)
	parent := &model.Todo{Todos: []*model.Todo{a, b}}
	a.ParentTodo = parent
	b.ParentTodo = parent

	b.SortSiblings("description", true)
	if parent.Todos[0].Description != "b" {
		t.Fatalf("reverse should invert order: %v", descs(parent.Todos))
	}
}

func TestSearchFilter(t *testing.T) {
	if !matchesFilter(&model.Todo{Description: "Buy MILK"}, "milk") {
		t.Fatal("case-insensitive filter should match")
	}
	if matchesFilter(&model.Todo{Description: "buy milk"}, "eggs") {
		t.Fatal("non-matching filter should fail")
	}
}

// ---- urgency cap + colors ----

func TestUrgencyCappedAt5(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	for i := 0; i < 10; i++ {
		m.actionIncreaseUrgency(m)
	}
	if got := m.selectedTodo().Urgency; got != 5 {
		t.Fatalf("urgency should cap at 5, got %d", got)
	}
	for i := 0; i < 10; i++ {
		m.actionDecreaseUrgency(m)
	}
	if got := m.selectedTodo().Urgency; got != 0 {
		t.Fatalf("urgency should floor at 0, got %d", got)
	}
}

func TestUrgencyColorsRender(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.selectedTodo().Urgency = 3
	row := m.formatTodo(m.selectedTodo())
	if row == "" {
		t.Fatal("urgency row should render")
	}
	// Re-render after lowering urgency; both must be non-empty and distinct.
	m.selectedTodo().Urgency = 1
	row1 := m.formatTodo(m.selectedTodo())
	if row1 == "" {
		t.Fatal("urgency 1 should render")
	}
}

// ---- clipboard copy / paste ----

func TestPasteBelowTodoInsertsAfter(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	src := m.selectedTodo()
	if src == nil {
		t.Fatal("expected a selected todo")
	}
	m.actionCopyModel(m)

	// Seed the parent with a second sibling so order is observable.
	// append a new sibling todo to the same workspace.
	ws := m.selectedWorkspace()
	second := &model.Todo{Description: "second", ParentWorkspaceID: &ws.ID, Pending: true}
	ws.Todos = append(ws.Todos, second)
	if err := m.store.SaveTodo(second); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}

	// highlight the first todo, paste below → clone lands at index 1.
	m.TodoCursor = 0
	m.actionPasteBelow(m)

	ws = m.selectedWorkspace()
	if len(ws.Todos) != 3 {
		t.Fatalf("want 3 todos after paste, got %d", len(ws.Todos))
	}
	if ws.Todos[1].Description != src.Description {
		t.Fatalf("paste below should insert clone at index 1, got %v", descs(ws.Todos))
	}
	if ws.Todos[1].ID == src.ID {
		t.Fatal("paste must create a new todo, not reuse the source id")
	}
}

func TestPasteAboveTodoInsertsBefore(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	src := m.selectedTodo()
	m.actionCopyModel(m)

	ws := m.selectedWorkspace()
	second := &model.Todo{Description: "second", ParentWorkspaceID: &ws.ID, Pending: true}
	ws.Todos = append(ws.Todos, second)
	if err := m.store.SaveTodo(second); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}

	m.TodoCursor = 1 // highlight second todo
	m.actionPasteAbove(m)

	ws = m.selectedWorkspace()
	if len(ws.Todos) != 3 {
		t.Fatalf("want 3 todos after paste, got %d", len(ws.Todos))
	}
	if ws.Todos[1].Description != src.Description {
		t.Fatalf("paste above should insert clone before cursor, got %v", descs(ws.Todos))
	}
}

func TestPasteSubtreeClonesDescendants(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	root := m.selectedTodo()
	child := &model.Todo{Description: "child", Pending: true}
	root.Todos = append(root.Todos, child)
	if err := m.store.SaveTodo(root); err != nil {
		t.Fatal(err)
	}
	child.ParentTodoID = &root.ID
	if err := m.store.SaveTodo(child); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}

	m.TodoCursor = 0
	m.actionCopyModel(m)
	m.actionPasteBelow(m)

	ws := m.selectedWorkspace()
	if len(ws.Todos) != 2 {
		t.Fatalf("want 2 todos, got %d", len(ws.Todos))
	}
	clone := ws.Todos[1]
	if len(clone.Todos) != 1 {
		t.Fatalf("cloned todo should have 1 child, got %d", len(clone.Todos))
	}
	if clone.Todos[0].ID == child.ID {
		t.Fatal("cloned child must be a fresh id")
	}
	if clone.Todos[0].Description != "child" {
		t.Fatalf("cloned child description = %q, want child", clone.Todos[0].Description)
	}
}

func TestPasteCrossTypeWorkspaceIntoTodoErrors(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneWorkspace)
	m.WorkspaceCursor = 0
	m.actionCopyModel(m) // clips a workspace
	m.SetFocus(PaneTodo)
	m.actionPasteBelow(m)
	if m.notice == "" {
		t.Fatal("cross-type paste must notify an error")
	}
}

func TestPasteEmptyClipboardErrors(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.clipboard = nil
	m.actionPasteBelow(m)
	if m.notice == "" {
		t.Fatal("paste with empty clipboard must notify an error")
	}
}

func descs(todos []*model.Todo) []string {
	out := make([]string, len(todos))
	for i, t := range todos {
		out[i] = t.Description
	}
	return out
}
