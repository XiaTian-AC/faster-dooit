package store

import (
	"testing"
	"time"
)

func TestStoreCRUD(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	root, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if root == nil || !root.IsRoot {
		t.Fatal("expected a root workspace")
	}

	ws := &Workspace{Description: "Work"}
	root.Children = append(root.Children, ws)
	if err := st.SaveWorkspace(ws); err != nil {
		t.Fatal(err)
	}

	due := time.Now().Add(24 * time.Hour)
	todo := &Todo{Description: "ship", Due: &due, ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	if err := st.SaveTodo(todo); err != nil {
		t.Fatal(err)
	}

	got, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Children) != 1 || len(got.Children[0].Todos) != 1 {
		t.Fatalf("reload mismatch: children=%d todos=%d", len(got.Children), len(got.Children[0].Todos))
	}
	if got.Children[0].Todos[0].Description != "ship" {
		t.Fatalf("desc = %q", got.Children[0].Todos[0].Description)
	}
}

func TestStoreDelete(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	ws := &Workspace{Description: "x"}
	root.Children = append(root.Children, ws)
	st.SaveWorkspace(ws)
	todo := &Todo{Description: "t", ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	st.SaveTodo(todo)

	if err := st.DeleteWorkspace(ws.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadAll()
	if len(got.Children) != 0 {
		t.Fatalf("expected 0 children, got %d", len(got.Children))
	}
}

// 评审修正：FK 确实生效——删 workspace 后直查 todo 表无孤儿行。
func TestDeleteCascadesToTodos(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	ws := &Workspace{Description: "x"}
	root.Children = append(root.Children, ws)
	st.SaveWorkspace(ws)
	todo := &Todo{Description: "t", ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	st.SaveTodo(todo)

	if err := st.DeleteWorkspace(ws.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM todo`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("FK cascade failed: %d orphan todos remain after workspace delete", n)
	}
}

// 评审修正：两个新节点 id 不同，且子节点父键匹配父节点 id。
func TestNewNodeIDsUniqueAndParentMatches(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()

	a := &Workspace{Description: "A"}
	root.Children = append(root.Children, a)
	st.SaveWorkspace(a)
	b := &Workspace{Description: "B"}
	root.Children = append(root.Children, b)
	st.SaveWorkspace(b)

	if a.ID == 0 || b.ID == 0 {
		t.Fatalf("expected non-zero ids, got a=%d b=%d", a.ID, b.ID)
	}
	if a.ID == b.ID {
		t.Fatalf("expected distinct ids, got both %d", a.ID)
	}
	if a.ParentID == nil || *a.ParentID != root.ID {
		t.Fatalf("child a.ParentID = %v, want %d", a.ParentID, root.ID)
	}

	// todo 的父键同样匹配
	ta := &Todo{Description: "ta", ParentWorkspaceID: &a.ID}
	a.Todos = append(a.Todos, ta)
	st.SaveTodo(ta)
	if ta.ID == 0 {
		t.Fatalf("expected non-zero todo id")
	}
	if ta.ParentWorkspaceID == nil || *ta.ParentWorkspaceID != a.ID {
		t.Fatalf("todo parent = %v, want %d", ta.ParentWorkspaceID, a.ID)
	}
}

// 评审修正：已有 ID 的 upsert 更新，不回写新 ID、不产生重复行。
func TestSaveExistingUpdatesNotDuplicates(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	ws := &Workspace{Description: "v1"}
	root.Children = append(root.Children, ws)
	st.SaveWorkspace(ws)
	id := ws.ID

	ws.Description = "v2"
	if err := st.SaveWorkspace(ws); err != nil {
		t.Fatal(err)
	}
	if ws.ID != id {
		t.Fatalf("upsert changed id: %d -> %d", id, ws.ID)
	}

	got, _ := st.LoadAll()
	if len(got.Children) != 1 {
		t.Fatalf("expected 1 child after upsert, got %d", len(got.Children))
	}
	if got.Children[0].Description != "v2" {
		t.Fatalf("desc = %q, want v2", got.Children[0].Description)
	}
}

// 评审修正：新建非根 workspace 无父时自动挂到根。
func TestNewNonRootAutoAttachesToRoot(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	ws := &Workspace{Description: "orphan-no-parent"}
	if err := st.SaveWorkspace(ws); err != nil {
		t.Fatal(err)
	}
	if ws.ID == 0 {
		t.Fatalf("expected non-zero id")
	}
	if ws.ParentID == nil || *ws.ParentID != root.ID {
		t.Fatalf("parent = %v, want root %d", ws.ParentID, root.ID)
	}
}

// 评审修正：due 时间可往返存储（RFC3339 编码），NULL 与空指针对等。
func TestDueRoundTrip(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	ws := &Workspace{Description: "w"}
	root.Children = append(root.Children, ws)
	st.SaveWorkspace(ws)

	due := time.Date(2026, 8, 5, 15, 30, 0, 0, time.Local)
	todo := &Todo{Description: "d", Due: &due, ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	st.SaveTodo(todo)

	got, _ := st.LoadAll()
	gotDue := got.Children[0].Todos[0].Due
	if gotDue == nil {
		t.Fatal("expected non-nil due after reload")
	}
	if !gotDue.Equal(due) {
		t.Fatalf("due = %v, want %v", gotDue, due)
	}
}

// 评审修正：单连接内存库——New(:memory:) 必须 SetMaxOpenConns(1)，否则每次查询
// 拿到的是空库。本测试通过反复 Open/Save/LoadAll（这是生产路径）验证同一 Store
// 实例上写过的数据能在后续读取中复现。
func TestSingleConnectionMemoryDB(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	root, _ := st.LoadAll()
	ws := &Workspace{Description: "w"}
	root.Children = append(root.Children, ws)
	if err := st.SaveWorkspace(ws); err != nil {
		t.Fatal(err)
	}

	// 重新加载，验证同一 Store 上的数据没丢
	root2, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range root2.Children {
		if c.Description == "w" && c.ID == ws.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("memory DB lost data within same Store: want ws id=%d desc=w, got %+v", ws.ID, root2.Children)
	}
}

// 确保外键约束本身被启用（数据库层校验，LoadAll 无法测到）。
func TestForeignKeyEnforcement(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	// PRAGMA foreign_keys 返回值非 0 才代表启用
	var fk int
	if err := st.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk == 0 {
		t.Fatal("foreign_keys pragma not enabled")
	}
}

// 保证 database/sql 连接池仅一个连接（评审 critical #1）。
func TestSingleOpenConnection(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if n := st.db.Stats().MaxOpenConnections; n != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", n)
	}
}

// 保证 New 建表后 root 行存在。
func TestRootExistsInDB(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM workspace WHERE is_root = 1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 root row, got %d", n)
	}
}

func TestDeleteTodo(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	ws := &Workspace{Description: "w"}
	root.Children = append(root.Children, ws)
	st.SaveWorkspace(ws)
	todo := &Todo{Description: "t", ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	st.SaveTodo(todo)

	if err := st.DeleteTodo(todo.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadAll()
	if len(got.Children[0].Todos) != 0 {
		t.Fatalf("expected 0 todos, got %d", len(got.Children[0].Todos))
	}
}

func TestBatchOrder(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	a := &Workspace{Description: "a"}
	root.Children = append(root.Children, a)
	st.SaveWorkspace(a)
	b := &Workspace{Description: "b"}
	root.Children = append(root.Children, b)
	st.SaveWorkspace(b)

	rootID := root.ID
	items := []orderItem{
		{ID: a.ID, Order: 2},
		{ID: b.ID, Order: 1},
	}
	if err := st.BatchOrder("workspace", &rootID, items); err != nil {
		t.Fatal(err)
	}

	got, _ := st.LoadAll()
	if got.Children[0].ID != b.ID {
		t.Fatalf("children[0].ID = %d, want %d", got.Children[0].ID, b.ID)
	}
	if got.Children[1].ID != a.ID {
		t.Fatalf("children[1].ID = %d, want %d", got.Children[1].ID, a.ID)
	}
}

// 无 parentID（todo 按 workspace 分组时不用；这里测 nil parent 路径不崩）。
func TestBatchOrderNilParent(t *testing.T) {
	st, _ := New(":memory:")
	if err := st.BatchOrder("todo", nil, nil); err != nil {
		t.Fatalf("nil items should be a no-op, got err: %v", err)
	}
}
