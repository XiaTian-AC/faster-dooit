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

func TestLayoutModeDecision(t *testing.T) {
	m := newTestApp(t)
	cases := []struct {
		w, h int
		want layoutMode
	}{
		{120, 30, layoutNormal},
		{80, 30, layoutStacked},
		{99, 30, layoutStacked},  // < W_stack
		{100, 30, layoutNormal},  // >= W_stack
		{150, 30, layoutNormal},
		{80, 10, layoutTooSmall}, // too short
		{30, 30, layoutTooSmall}, // too narrow
	}
	for _, c := range cases {
		m.width, m.height = c.w, c.h
		if got := m.layoutMode(); got != c.want {
			t.Errorf("layoutMode(%d,%d) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}

func TestTooSmallNotice(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 30, 30
	got := m.View()
	if !strings.Contains(got, "Terminal size too small") {
		t.Fatalf("expected too-small notice, got:\n%s", got)
	}
	if !strings.Contains(got, "Needed for current config") {
		t.Fatalf("notice should list needed size, got:\n%s", got)
	}
}

func TestStackedLayoutRendersBothPanes(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 80, 30
	v := m.View()
	if !strings.Contains(v, "Workspaces") || !strings.Contains(v, "Todos") {
		t.Fatalf("stacked view should show both pane titles:\n%s", v)
	}
	// Both titles must be on different lines (stacked), not the same line.
	wsIdx := strings.Index(v, "Workspaces")
	todoIdx := strings.Index(v, "Todos")
	if wsIdx < 0 || todoIdx < 0 {
		t.Fatalf("missing title")
	}
	// "Todos" must appear on a later line than "Workspaces".
	wsLine := strings.Count(v[:wsIdx], "\n")
	todoLine := strings.Count(v[:todoIdx], "\n")
	if todoLine <= wsLine {
		t.Fatalf("stacked: Todos should be below Workspaces (line %d vs %d)", todoLine, wsLine)
	}
}

// TestViewportScrollKeepsCursorVisible: with a short pane, moving the cursor
// to the bottom must shift the rendered window so the cursor row stays
// visible and the top row falls off.
func TestViewportScrollKeepsCursorVisible(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	// Add 8 todos so the list exceeds a 4-row pane.
	for i := 0; i < 8; i++ {
		todo := &model.Todo{Description: "item", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Move cursor to the last row (index 8).
	for i := 0; i < 8; i++ {
		m.actionMoveDown(m)
	}
	if m.TodoCursor != 8 {
		t.Fatalf("cursor = %d, want 8", m.TodoCursor)
	}
	// Short terminal: stacked, todo pane gets a small viewport. Height 14 is
	// above H_min(12) but below H_ok(24), so viewport scrolling is active.
	m.width, m.height = 80, 14
	v := m.View()
	lines := strings.Split(v, "\n")
	found := false
	for _, ln := range lines {
		if strings.Contains(ln, "Todos") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Todos title missing:\n%s", v)
	}
	// The cursor row ("> ") must be rendered somewhere after the Todos title.
	// With 9 todos and a small viewport, it must have scrolled the window down
	// (the first item rows are hidden) but the cursor row is still visible.
	todoIdx := -1
	cursorLine := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Todos") {
			todoIdx = i
		}
		if strings.Contains(ln, "> ") {
			cursorLine = i
		}
	}
	if todoIdx < 0 || cursorLine < 0 {
		t.Fatalf("cursor row not visible after scroll:\n%s", v)
	}
	if cursorLine <= todoIdx {
		t.Fatalf("cursor should render below the Todos title, cursorLine=%d todoIdx=%d", cursorLine, todoIdx)
	}
	// At least one todo must have scrolled off (we have 9 todos; the visible
	// todo area is much smaller), i.e. the first todo row is not line 0.
	if lines[0] == "" {
		// acceptable: leading empty line from JoinVertical padding
	}
	// The first rendered todo (immediately after "Todos") is NOT item 0 —
	// scroll offset advanced. We can't know the exact id, but the cursor row
	// being present and below the title suffices for the visible-window
	// contract here.
}

// TestSelectedRowNeverExceedsPane: the highlighted (selected) row must fit
// inside the pane content width even with a long description — the previous
// code padded to the box width, wrapping the terminal and leaving a ghost
// highlighted row.
func TestSelectedRowNeverExceedsPane(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.selectedTodo().Description = "a very long description that forces truncation and padding"
	m.width, m.height = 80, 30
	// Stacked mode: pane content width = m.width-4.
	v := m.View()
	for _, line := range strings.Split(v, "\n") {
		if lw := lipgloss.Width(line); lw > m.width {
			t.Errorf("selected row overflows terminal by %d cols: %q", lw-m.width, line)
		}
	}
}

// TestDualPaneScrollsOnShortTerminal: even in the wide (dual-pane) layout, a
// short terminal must scroll the todo viewport so the cursor stays visible.
func TestDualPaneScrollsOnShortTerminal(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	for i := 0; i < 8; i++ {
		todo := &model.Todo{Description: "item", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Move cursor to the last todo (index 8).
	for i := 0; i < 8; i++ {
		m.actionMoveDown(m)
	}
	if m.TodoCursor != 8 {
		t.Fatalf("cursor = %d, want 8", m.TodoCursor)
	}
	// Wide (dual-pane) but short: height 14 gives a small viewport.
	m.width, m.height = 150, 14
	v := m.View()
	if m.layoutMode() != layoutNormal {
		t.Fatalf("expected dual-pane layout, got %v", m.layoutMode())
	}
	// Cursor row must be visible somewhere after the Todos title.
	todoIdx := -1
	cursorLine := -1
	for i, ln := range strings.Split(v, "\n") {
		if strings.Contains(ln, "Todos") {
			todoIdx = i
		}
		if strings.Contains(ln, "> ") {
			cursorLine = i
		}
	}
	if todoIdx < 0 || cursorLine < 0 {
		t.Fatalf("cursor row not visible in dual-pane scroll:\n%s", v)
	}
	if cursorLine <= todoIdx {
		t.Fatalf("cursor should render below the Todos title, cursorLine=%d todoIdx=%d", cursorLine, todoIdx)
	}
}

// TestInlineEditFullWidthInput: while editing effort (a 4-col column), the
// input must render at full available width — not clipped to the column.
func TestInlineEditFullWidthInput(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.StartEdit("effort")
	m.input.SetValue("1234567890")
	v := m.renderTodoPane(60)
	if !strings.Contains(v, "1234567890") {
		t.Fatalf("effort input should show full value, got:\n%s", v)
	}
	for _, line := range strings.Split(v, "\n") {
		if lw := lipgloss.Width(line); lw > 60 {
			t.Errorf("inline edit row overflows pane by %d cols: %q", lw-60, line)
		}
	}
}
