package app

import (
	"strings"
	"testing"

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
api.vars.theme.name = "dracula"
api.vars.theme.primary = "#123456"
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
	rt, err := lua.EvalFileWithCode(`api.vars.theme.name = "catppuccin_latte"`)
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
	// Expanded with children: "▾".
	if got := stripANSI(m.renderRowMarker(true, true)); got != "▾ " {
		t.Fatalf("expanded marker = %q, want \"▾ \"", got)
	}
}
