package app

import (
	"testing"

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
