package app

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/XiaTian-AC/faster-dooit/internal/lua"
	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/XiaTian-AC/faster-dooit/internal/store"
)

// newTestAppLua builds the app with the real config.lua loaded.
func newTestAppLua(t *testing.T) *Model {
	t.Helper()
	rt, err := lua.EvalFile("../../config.lua")
	if err != nil {
		t.Fatalf("eval config.lua: %v", err)
	}
	t.Cleanup(rt.Close)

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

	m := New(st, rt)
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	m.SetFocus(1) // todo pane
	return m
}

func TestRenderRowCaches(t *testing.T) {
	m := newTestAppLua(t)
	v0 := m.RenderRow(1, 0)
	v1 := m.RenderRow(1, 0)
	if v0 != v1 {
		t.Fatal("cache miss on unchanged state")
	}
	if v0 == "" {
		t.Fatal("expected non-empty row")
	}
	m.BumpVersion()
	v2 := m.RenderRow(1, 0)
	if v2 == "" {
		t.Fatal("expected rendered row after bump")
	}
}

func TestColumnLayoutFromConfig(t *testing.T) {
	m := newTestAppLua(t)
	cols := m.ColumnLayout(1)
	if len(cols) == 0 {
		t.Fatal("no todo layout columns")
	}
	if cols[0] != "status" {
		t.Fatalf("first todo column = %q, want status", cols[0])
	}
}

func TestDashboardFromConfig(t *testing.T) {
	m := newTestAppLua(t)
	lines := m.DashboardLines()
	if len(lines) == 0 {
		t.Fatal("no dashboard lines from config.lua")
	}
}

func TestAppThemeResolvesPresetAndOverride(t *testing.T) {
	m := newTestAppLua(t)
	// newTestAppLua evaluates config.lua (theme.name defaults to nord).
	th := m.appTheme()
	if th.Primary == "" || th.Background == "" || th.Dim == "" || th.Selection == "" {
		t.Fatalf("appTheme incomplete: %+v", th)
	}
}

func TestAppThemeOverrideApplied(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	rt, err := lua.EvalFileWithCode(`
theme.name = "dracula"
theme.primary = "#123456"
`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	m := New(st, rt)
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	th := m.appTheme()
	if th.Primary != "#123456" {
		t.Fatalf("override primary = %q, want #123456", th.Primary)
	}
	if th.Background == "" {
		t.Fatal("dracula background should be populated")
	}
}

// TestDescriptionUsesThemeColor: the todo description column must be styled
// with the active theme's primary color, not the terminal default foreground.
// Without this, light themes (e.g. catppuccin_latte) render pale-on-pale and
// the theme has no control over the main text.
func TestDescriptionUsesThemeColor(t *testing.T) {
	// Force 24-bit color output (no TTY in tests) so ANSI assertions work.
	lipgloss.SetColorProfile(termenv.TrueColor)
	rt, err := lua.EvalFileWithCode(`theme.name = "catppuccin_latte"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, rt)
	todo := &model.Todo{Description: "buy milk", Pending: true}
	cell := m.formatTodoColumn("description", todo)
	// A 24-bit foreground ANSI sequence must style the description.
	if !strings.Contains(cell, "\x1b[38;2;") {
		t.Fatalf("description cell %q should be styled with a theme foreground color", cell)
	}
}


func TestRenderRowMarker(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	// Leaf node: blank marker.
	if got := m.renderRowMarker(false, false); got != "  " {
		t.Fatalf("leaf marker = %q, want blank", got)
	}
	// Collapsed with children: ">".
	if got := stripANSI(m.renderRowMarker(true, false)); got != "> " {
		t.Fatalf("collapsed marker = %q, want \"> \"", got)
	}
	// Expanded with children: "⌄".
	if got := stripANSI(m.renderRowMarker(true, true)); got != "⌄ " {
		t.Fatalf("expanded marker = %q, want \"⌄ \"", got)
	}
}

// TestDynamicColumnsDropEmpty: a todo with only a description keeps just the
// status + description columns — empty due/effort/recurrence/urgency cells
// must not consume width or gaps.
func TestDynamicColumnsDropEmpty(t *testing.T) {
	m := newTestApp(t)
	todo := &model.Todo{Description: "just a task", Pending: true}
	cols := []string{"status", "description", "due", "effort", "recurrence", "urgency"}
	active := m.activeColumns(todo, cols)
	if len(active) != 2 {
		t.Fatalf("active columns = %v, want [status description]", active)
	}
	if active[0] != "status" || active[1] != "description" {
		t.Fatalf("active columns = %v, want [status description]", active)
	}
}

// TestDynamicColumnsDropsOnlyEmpty: a todo with a due date keeps the due
// column while still dropping the empty effort/recurrence/urgency columns.
func TestDynamicColumnsDropsOnlyEmpty(t *testing.T) {
	m := newTestApp(t)
	due := time.Now().Add(24 * time.Hour)
	todo := &model.Todo{Description: "with a due", Pending: true, Due: &due}
	cols := []string{"status", "description", "due", "effort", "recurrence", "urgency"}
	active := m.activeColumns(todo, cols)
	if len(active) != 3 {
		t.Fatalf("active columns = %v, want [status description due]", active)
	}
	if active[2] != "due" {
		t.Fatalf("active columns = %v, want due third", active)
	}
}

// TestToggleDescriptionExpand: `o` toggles a todo's expanded description
// (session-only) on and off.
func TestToggleDescriptionExpand(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	todo := m.selectedTodo()
	m.toggleDescriptionExpand(m)
	if !m.expandedDesc[todo.ID] {
		t.Fatal("o should expand the description")
	}
	m.toggleDescriptionExpand(m)
	if m.expandedDesc[todo.ID] {
		t.Fatal("o again should collapse it")
	}
}

// TestKeyODispatchesToggleDescriptionExpand: the default keymap binds `o` to
// the toggle_description_expand action.
func TestKeyODispatchesToggleDescriptionExpand(t *testing.T) {
	km := newKeyManager(defaultKeyBindings())
	if a := km.feed('o'); a != "toggle_description_expand" {
		t.Fatalf("o should dispatch toggle_description_expand, got %q", a)
	}
}

// TestExpandedDescriptionRendersMultipleLines: expanding a long description
// renders the description across multiple terminal lines; continuation lines
// align under the first line's description column.
func TestExpandedDescriptionRendersMultipleLines(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	todo := m.selectedTodo()
	todo.Description = "This is a very long description that wraps across multiple terminal lines when expanded with the o key"
	m.maxDescLines = 3
	m.expandedDesc[todo.ID] = true

	v := m.renderTodoPane(60)
	lines := strings.Split(v, "\n")
	// The description should occupy at least two lines after expansion.
	count := 0
	for _, ln := range lines {
		s := stripANSI(ln)
		if strings.Contains(s, "This is a very long description") ||
			strings.Contains(s, "wraps across multiple terminal") ||
			strings.Contains(s, "lines when expanded") {
			count++
		}
	}
	if count < 2 {
		t.Fatalf("expanded description should span multiple lines, found %d:\n%s", count, v)
	}
}

// TestExpandedDescriptionRespectsMaxLines: with maxDescLines = 2 the expanded
// render is capped at 2 lines and the last line carries an ellipsis.
func TestExpandedDescriptionRespectsMaxLines(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	todo := m.selectedTodo()
	todo.Description = "This is a very long description that wraps across multiple terminal lines when expanded with the o key and continues well beyond the configured limit"
	m.maxDescLines = 2
	m.expandedDesc[todo.ID] = true

	v := m.renderTodoPane(60)
	lines := strings.Split(v, "\n")
	descLines := 0
	for _, ln := range lines {
		s := stripANSI(ln)
		if strings.Contains(s, "This is a very long") ||
			strings.Contains(s, "wraps across multiple") ||
			strings.Contains(s, "when expanded with") ||
			strings.Contains(s, "beyond the configured") {
			descLines++
		}
	}
	if descLines != 2 {
		t.Fatalf("maxDescLines=2 should cap the description at 2 lines, got %d:\n%s", descLines, v)
	}
	if !strings.Contains(stripANSI(v), "…") {
		t.Fatalf("truncated expanded description should end with an ellipsis:\n%s", v)
	}
}

// TestExpandedDescriptionZeroNeverEllipsizes: maxDescLines = 0 renders the
// full description (more than the default 3 lines) without any ellipsis.
func TestExpandedDescriptionZeroNeverEllipsizes(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	todo := m.selectedTodo()
	todo.Description = "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty"
	m.maxDescLines = 0
	m.expandedDesc[todo.ID] = true

	v := m.renderTodoPane(40)
	lines := strings.Split(v, "\n")
	descLines := 0
	for _, ln := range lines {
		s := stripANSI(ln)
		if strings.Contains(s, "one two three") ||
			strings.Contains(s, "four five six") ||
			strings.Contains(s, "ten eleven twelve") ||
			strings.Contains(s, "thirteen fourteen") ||
			strings.Contains(s, "seventeen eighteen nineteen twenty") {
			descLines++
		}
	}
	if descLines < 4 {
		t.Fatalf("maxDescLines=0 should render more than the default 3 lines, got %d:\n%s", descLines, v)
	}
	if strings.Contains(stripANSI(v), "…") {
		t.Fatalf("maxDescLines=0 must never ellipsize:\n%s", v)
	}
}
