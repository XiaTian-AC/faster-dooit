package app

import (
	"strings"
	"testing"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// newTestAppTwoWorkspaces builds an app with two sibling workspaces so pane
// sync can be observed. SaveWorkspace auto-attaches a root-less workspace to
// the root, giving a second top-level sibling.
func newTestAppTwoWorkspaces(t *testing.T) *Model {
	t.Helper()
	m := newTestApp(t)
	second := &model.Workspace{Description: "Second", OrderIndex: 1}
	if err := m.store.SaveWorkspace(second); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	return m
}

// Workspace cursor movement must re-sync the todo pane's selected workspace.
func TestWorkspaceMoveSyncsTodoPane(t *testing.T) {
	m := newTestAppTwoWorkspaces(t)
	m.SetFocus(PaneWorkspace)
	m.WorkspaceCursor = 0
	first := m.selectedWorkspaceByCursor()

	m.actionMoveDown(m)
	second := m.selectedWorkspaceByCursor()
	if second == nil || second.ID == first.ID {
		t.Fatalf("cursor should have moved to a different workspace")
	}
	if got := m.selectedWorkspaceID; got != second.ID {
		t.Fatalf("todo pane should follow cursor: selectedWorkspaceID=%d, want %d", got, second.ID)
	}
}

// A (add_child) in the todo pane must create a nested child (parent todo),
// not a sibling — so the new row renders indented.
func TestAddChildCreatesNestedTodo(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	parent := m.selectedTodo()
	parentID := parent.ID

	m.actionAddChild(m)
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// The cursor now sits on the new child; re-find the original parent by id.
	parent = findTodoInWorkspace(m.root, parentID)
	if parent == nil {
		t.Fatal("original parent should still exist")
	}
	if len(parent.Todos) != 1 {
		t.Fatalf("add_child should nest under the selected todo, got %d children", len(parent.Todos))
	}
	child := parent.Todos[0]
	if child.ParentTodoID == nil || *child.ParentTodoID != parent.ID {
		t.Fatalf("child's ParentTodoID should point at the selected todo")
	}
	// And it should render indented (nest level 1).
	if child.NestLevel() != 1 {
		t.Fatalf("child nest level = %d, want 1", child.NestLevel())
	}
}

// New todos default to urgency 1.
func TestNewTodoDefaultsUrgency1(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.actionAddSibling(m)
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	todos := m.visibleTodos()
	if len(todos) == 0 {
		t.Fatal("expected a todo")
	}
	last := todos[len(todos)-1]
	if last.Urgency != 1 {
		t.Fatalf("new todo urgency = %d, want 1", last.Urgency)
	}
}

// Selected row renders as a full-width highlighted row (padded to the pane
// width) with the cursor arrow, while an unselected row is not padded.
func TestSelectedRowHighlighted(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0

	// The selected row carries a background ANSI code and is padded to the
	// pane width. Selection is shown by the highlight, not a cursor arrow.
	sel := m.renderSelectedRow("abc", 20)
	if !strings.Contains(sel, "\x1b[48;2;") {
		t.Fatalf("selected row should carry a background highlight, got %q", sel)
	}

	v := m.renderTodoPane(20)
	found := false
	for _, line := range strings.Split(v, "\n") {
		if strings.Contains(line, "\x1b[48;2;") {
			found = true
			if lipgloss.Width(line) < 20 {
				t.Fatalf("highlighted row should span the pane width, got %d cols", lipgloss.Width(line))
			}
		}
	}
	if !found {
		t.Fatalf("a highlighted row should be present:\n%s", v)
	}
}
