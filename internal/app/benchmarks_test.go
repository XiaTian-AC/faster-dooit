package app

import (
	"path/filepath"
	"testing"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/XiaTian-AC/faster-dooit/internal/store"
)

// seedDB builds a file-backed store with n todos under one workspace, the
// same shape main.go loads on cold start.
func seedDB(b *testing.B, dir string, n int) *store.Store {
	b.Helper()
	st, err := store.New(filepath.Join(dir, "startup.db"))
	if err != nil {
		b.Fatal(err)
	}
	root, err := st.LoadAll()
	if err != nil {
		b.Fatal(err)
	}
	ws := &model.Workspace{Description: "W"}
	root.Children = append(root.Children, ws)
	if err := st.SaveWorkspace(ws); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < n; i++ {
		t := &model.Todo{Description: "todo item", Pending: true, ParentWorkspaceID: &ws.ID, OrderIndex: i}
		if err := st.SaveTodo(t); err != nil {
			b.Fatal(err)
		}
	}
	return st
}

// BenchmarkStartup10k measures the cold-start path that dominates TUI launch:
// open the file DB, load the whole tree, and construct the model — exactly
// what main.go runs before tea.NewProgram. Target: < 200ms for 10k todos.
func BenchmarkStartup10k(b *testing.B) {
	dir := b.TempDir()
	st := seedDB(b, dir, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		root, err := st.LoadAll()
		if err != nil {
			b.Fatal(err)
		}
		m := New(st, nil)
		m.root = root
		m.RefreshFromStore()
	}
	st.Close() // release the file so b.TempDir cleanup can remove it
}

// newBenchApp builds an app with n todos under one workspace.
func newBenchApp(b *testing.B, n int) *Model {
	b.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	root, err := st.LoadAll()
	if err != nil {
		b.Fatal(err)
	}
	ws := &model.Workspace{Description: "W"}
	root.Children = append(root.Children, ws)
	if err := st.SaveWorkspace(ws); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < n; i++ {
		t := &model.Todo{Description: "todo item", Pending: true, ParentWorkspaceID: &ws.ID, OrderIndex: i}
		ws.Todos = append(ws.Todos, t)
		if err := st.SaveTodo(t); err != nil {
			b.Fatal(err)
		}
	}
	m := New(st, nil)
	if err := m.RefreshFromStore(); err != nil {
		b.Fatal(err)
	}
	m.SetFocus(1) // todo pane
	return m
}

// Target: Update (keypress) < 1ms, VisibleRows over 10k todos < 10ms.
func BenchmarkVisibleTodos10k(b *testing.B) {
	m := newBenchApp(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.visibleTodos()
	}
}

func BenchmarkUpdate(b *testing.B) {
	m := newBenchApp(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.actionMoveDown(m)
	}
}

func BenchmarkRenderRow10k(b *testing.B) {
	m := newBenchApp(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RenderRow(1, 0)
	}
}

// BenchmarkRenderViewport10k measures one full View() render for a 30-row
// viewport over 10k todos — the resize/drag hot path. Viewport rendering
// must stay bounded regardless of list size.
func BenchmarkRenderViewport10k(b *testing.B) {
	m := newBenchApp(b, 10000)
	m.width, m.height = 120, 30
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}
