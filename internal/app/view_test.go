package app

import (
	"strings"
	"testing"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/charmbracelet/lipgloss"
)

func TestCursorAlignsWithIndent(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	// Build a 2-level tree: root todo with one child.
	root := m.selectedTodo()
	child := &model.Todo{Description: "child", Pending: true, ParentTodoID: &root.ID}
	root.Todos = append(root.Todos, child)
	if err := m.store.SaveTodo(child); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Cursor on the nested child row (index 1); its marker must sit after the
	// indent (2 spaces for level 1).
	m.TodoCursor = 1
	v := m.renderTodoPane(80)
	if !strings.Contains(v, "  > ") {
		t.Fatalf("cursor should align with nested indent, got:\n%s", v)
	}
}

func TestViewVerticallyCenters(t *testing.T) {
	m := newTestApp(t)
	m.width = 80
	m.height = 30
	v := m.View()
	// With 30 rows of room and ~10 content lines, content must be padded down
	// from the top (leading newline(s)), not stuck at the first row.
	if !strings.HasPrefix(v, "\n\n") {
		t.Fatalf("content should be vertically centered (leading blank rows missing), got:\n%q", v)
	}
}

func TestInlineEditRendersOnCursorRow(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("description")
	v := m.renderTodoPane(80)
	if !strings.Contains(v, m.input.View()) {
		t.Fatalf("input should render on the cursor row, got:\n%s", v)
	}
}

// TestViewFitsTerminalWidth renders the full view (two bordered panes) at
// several terminal widths and asserts every line fits within the terminal —
// the pane borders were previously under-budgeted, overflowing even with
// zero nesting.
func TestViewFitsTerminalWidth(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.selectedTodo().Description = "zero nesting long description to probe overflow"

	for _, tw := range []int{40, 60, 80, 100} {
		m.width = tw
		m.height = 30
		v := m.View()
		for _, line := range strings.Split(v, "\n") {
			if lw := lipgloss.Width(line); lw > tw {
				t.Errorf("terminal %d: line overflows by %d cols: %q", tw, lw-tw, line)
			}
		}
	}
}
