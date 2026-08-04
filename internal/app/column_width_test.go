package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
)

// TestTruncateByWidthCJK verifies truncation is by display width, so a
// full-width (CJK) character counts as 2 columns and is never split in half.
func TestTruncateByWidthCJK(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"中文测试", 5, "中文"}, // 中(2)文(2)=4, 测 would make 6 > 5
		{"中文测试", 4, "中文"},
		{"中文测试", 3, "中"},
		{"abc中文", 4, "abc"},
		{"abc中文", 5, "abc中"},
		{"中文测试", 1, ""},
	}
	for _, c := range cases {
		got := truncateByWidth(c.in, c.n)
		if got != c.want {
			t.Errorf("truncateByWidth(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
		if w := lipgloss.Width(got); w > c.n {
			t.Errorf("truncateByWidth(%q, %d) result %q has display width %d > %d", c.in, c.n, got, w, c.n)
		}
	}
}

// TestFitColumnCJK verifies a too-wide CJK cell is clipped with an ellipsis
// and then padded exactly back to the column width.
func TestFitColumnCJK(t *testing.T) {
	got := fitColumn("这是一段很长的中文描述", 10)
	if lipgloss.Width(got) != 10 {
		t.Fatalf("fitColumn width = %d, want 10; got %q", lipgloss.Width(got), got)
	}
}

// TestTruncateByWidthANSI verifies truncation never splits an ANSI escape
// sequence: styled cells (status/urgency/Lua formatters) must keep their
// escape codes intact after clipping.
func TestTruncateByWidthANSI(t *testing.T) {
	styled := "\x1b[1;38;2;255;85;85m!1000\x1b[0m"
	got := truncateByWidth(styled, 4)
	if w := lipgloss.Width(got); w > 4 {
		t.Fatalf("truncateByWidth width = %d, want <= 4; got %q", w, got)
	}
	if !strings.HasPrefix(got, "\x1b[1;38;2;255;85;85m") {
		t.Fatalf("escape prefix dropped: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("escape suffix dropped: %q", got)
	}
}

// TestFitColumnANSI verifies fitColumn preserves escape sequences on styled
// cells (urgency ≥ 10 renders "!NN" with a color).
func TestFitColumnANSI(t *testing.T) {
	styled := "\x1b[1;38;2;255;85;85m!1000\x1b[0m"
	got := fitColumn(styled, 4)
	if w := lipgloss.Width(got); w != 4 {
		t.Fatalf("fitColumn width = %d, want 4; got %q", w, got)
	}
	if !strings.HasPrefix(got, "\x1b[1;38;2;255;85;85m") {
		t.Fatalf("escape prefix dropped: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("escape suffix dropped: %q", got)
	}
}

// TestVisibleColumnsDropsOnNarrowBudget verifies that as the column budget
// shrinks, less-important fixed columns are dropped (urgency → effort → due)
// so the elastic description keeps at least a minimum width. description is
// never dropped.
func TestVisibleColumnsDropsOnNarrowBudget(t *testing.T) {
	m := newTestApp(t)
	// Column hiding only engages in the stacked layout; set a stacked size so
	// the drop logic runs against the given budget.
	m.width, m.height = 80, 30
	cases := []struct {
		budget int
		want   int
	}{
		{100, 4}, // all columns
		{45, 4},  // description has >= minDescWidth
		{30, 3},  // urgency dropped
		{22, 2},  // due dropped, only status+description
	}
	for _, c := range cases {
		cols := m.visibleColumns(PaneTodo, c.budget)
		if len(cols) != c.want {
			t.Errorf("budget %d: got %d columns %v, want %d", c.budget, len(cols), cols, c.want)
		}
		found := false
		for _, col := range cols {
			if col == "description" {
				found = true
			}
		}
		if !found {
			t.Errorf("budget %d: description dropped from %v", c.budget, cols)
		}
	}
}

// TestHideColumnsGate: column hiding only engages in the stacked layout; in
// dual-pane (or when the pane is rendered directly in tests with width 0) the
// full layout is kept.
func TestHideColumnsGate(t *testing.T) {
	m := newTestApp(t)
	// Direct renderTodoPane call (width 0) must keep the full layout.
	if m.hideColumns() {
		t.Fatal("hideColumns should be false when width is 0")
	}
	// Dual-pane width keeps all columns.
	m.width, m.height = 120, 30
	if m.hideColumns() {
		t.Fatal("hideColumns should be false in dual-pane layout")
	}
	// Stacked width engages hiding.
	m.width, m.height = 80, 30
	if !m.hideColumns() {
		t.Fatal("hideColumns should be true in stacked layout")
	}
}

// TestTodoRowsFitPaneWidthCJK renders a todo pane with a CJK description and
// asserts every row's display width is at most the pane width (rows must not
// overflow the bordered pane and wrap to the next terminal line).
func TestTodoRowsFitPaneWidthCJK(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.selectedTodo().Description = "这是一段足够长的中文描述用来触发列宽截断正确性检查"

	for _, w := range []int{40, 60, 100} {
		v := m.renderTodoPane(w)
		for _, line := range strings.Split(v, "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("pane width %d: row overflows by %d cols: %q", w, lw-w, line)
			}
		}
	}
}

// TestDeepNestedRowsFitPaneWidth renders deeply nested todos (wide indent) with
// a long CJK description and asserts rows still fit the pane width. The column
// budget must shrink by the indent+marker width rather than falling back to a
// fixed elastic width that overflows.
func TestDeepNestedRowsFitPaneWidth(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0

	// Build a 5-level nesting chain under the selected workspace.
	cur := m.selectedTodo()
	cur.Description = "root level todo"
	for depth := 1; depth <= 5; depth++ {
		child := &model.Todo{
			Description:  "子任务",
			ParentTodoID: &cur.ID,
			Pending:      true,
		}
		cur.Todos = append(cur.Todos, child)
		if err := m.store.SaveTodo(child); err != nil {
			t.Fatal(err)
		}
		cur = child
	}
	// Deepest todo carries a long CJK description to force truncation at depth.
	cur.Description = "这是最深一层足够长的中文描述用于触发列宽截断与缩进预算扣除"

	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	if got := m.visibleTodos()[len(m.visibleTodos())-1].NestLevel(); got != 5 {
		t.Fatalf("deepest nest level = %d, want 5", got)
	}

	for _, w := range []int{40, 60} {
		v := m.renderTodoPane(w)
		for _, line := range strings.Split(v, "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("pane width %d: row overflows by %d cols: %q", w, lw-w, line)
			}
		}
	}
}
