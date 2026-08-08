# Collapse / Expand Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make tree folding work — configurable default collapse depth, visual expand arrows, cursor repositioning on collapse.

**Architecture:** Remove the `|| true` that disables folding in `VisibleWorkspaces`/`visibleTodos`, add an `isExpanded(id, depth)` helper that honors manual overrides then a config-driven `collapse_depth`, render `▸`/`▾` arrows on collapsible rows, and reposition the cursor after a collapse so it never points at a hidden child.

**Tech Stack:** Go, Bubble Tea, gopher-lua (existing).

## Global Constraints

- `api.vars.collapse_depth` defaults to **0** = everything expanded (matches current behavior).
- Two trees count depth independently: workspace root = 0; todo at workspace top-level = 0.
- Manual toggles override `collapse_depth` for the session only; not persisted.
- Arrows render only on nodes with children: collapsed `▸`, expanded `▾`, theme `primary`.
- Cursor jumps to the collapsing node when its child was selected.
- Keybindings `z` / `Z` already exist — no keymap changes.
- Existing keyboard tests must stay green.

---

### Task 1: `collapseDepth()` config accessor + `isExpanded()` helper

**Files:**
- Modify: `internal/lua/lua.go` (Runtime struct + readTheme)
- Modify: `internal/app/app.go` (Model field + collapseDepth + isExpanded)
- Test: `internal/lua/lua_test.go`, `internal/app/app_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `lua.Runtime.CollapseDepth int`
  - `func (m *Model) collapseDepth() int`
  - `func (m *Model) isExpanded(id int64, depth int) bool`

- [ ] **Step 1: Write the failing Lua test**

Append to `internal/lua/lua_test.go`:

```go
func TestCollapseDepthLoaded(t *testing.T) {
	rt, err := EvalFileWithCode(`api.vars.collapse_depth = 2`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CollapseDepth != 2 {
		t.Fatalf("collapse_depth = %d, want 2", rt.CollapseDepth)
	}
}

func TestCollapseDepthDefaultsZero(t *testing.T) {
	rt, err := EvalFileWithCode(``)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CollapseDepth != 0 {
		t.Fatalf("collapse_depth default = %d, want 0", rt.CollapseDepth)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lua/ -run TestCollapseDepth`
Expected: FAIL — `Runtime` has no field `CollapseDepth`.

- [ ] **Step 3: Add `CollapseDepth` to the Lua Runtime**

In `internal/lua/lua.go`, add to the `Runtime` struct after `MinHeight`:

```go
	// MinWidth/MinHeight are the minimum terminal size for the UI, from
	// api.vars.min_width / api.vars.min_height (defaults 40/12).
	MinWidth  int
	MinHeight int

	// CollapseDepth is the default tree-collapse depth from
	// api.vars.collapse_depth (default 0 = everything expanded).
	CollapseDepth int
```

In `readTheme()` (after the `MinHeight` default block), read it:

```go
	if rt.MinHeight == 0 {
		rt.MinHeight = 12
	}

	if rt.varsTable != nil {
		if n, ok := L.GetField(rt.varsTable, "collapse_depth").(lua.LNumber); ok {
			rt.CollapseDepth = int(n)
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/lua/ -run TestCollapseDepth`
Expected: PASS

- [ ] **Step 5: Write the failing app test**

Append to `internal/app/app_test.go`:

```go
func TestIsExpandedManualOverrideWins(t *testing.T) {
	m := newTestApp(t)
	// Manual expansion beats default depth.
	m.expanded[99] = true
	if !m.isExpanded(99, 5) {
		t.Fatal("manual true should expand a deep node")
	}
	m.expanded[100] = false
	if m.isExpanded(100, 0) {
		t.Fatal("manual false should collapse a shallow node")
	}
}

func TestIsExpandedDefaultDepth(t *testing.T) {
	m := newTestApp(t)
	// collapse_depth 0: depth 0 expanded, depth >= 1 collapsed.
	if !m.isExpanded(1, 0) {
		t.Fatal("depth 0 should be expanded with collapse_depth 0")
	}
	if m.isExpanded(2, 1) {
		t.Fatal("depth 1 should be collapsed with collapse_depth 0")
	}
	// collapse_depth 1: depth 0 and 1 expanded, depth 2 collapsed.
	m.collapseDepth = 1
	if !m.isExpanded(2, 1) {
		t.Fatal("depth 1 should be expanded with collapse_depth 1")
	}
	if m.isExpanded(3, 2) {
		t.Fatal("depth 2 should be collapsed with collapse_depth 1")
	}
}
```

Note: this requires a `collapseDepth int` field on `Model` (Task 1 Step 6) and
a way to set it for tests — use the field directly.

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestIsExpanded'`
Expected: FAIL — `isExpanded` undefined; `collapseDepth` field undefined.

- [ ] **Step 7: Add `collapseDepth` field + `collapseDepth()` + `isExpanded()`**

In `internal/app/app.go`:
- Add field to `Model` struct after `expanded`:

```go
	// expanded[id] = true if the node is expanded in its tree view.
	expanded map[int64]bool

	// collapseDepth is the default collapse depth (0 = expand all) from
	// config api.vars.collapse_depth. Overridden by manual z/Z toggles.
	collapseDepth int
```

- In `New()` after `expanded: map[int64]bool{}`:

```go
		if luaCfg != nil {
			m.collapseDepth = luaCfg.CollapseDepth
		}
```

- Add methods:

```go
// collapseDepth returns the configured default collapse depth.
func (m *Model) collapseDepth() int {
	return m.collapseDepth
}

// isExpanded reports whether a node at the given tree depth renders expanded.
// A manual override in m.expanded wins; otherwise nodes at depth <=
// collapse_depth start expanded, deeper nodes start collapsed.
func (m *Model) isExpanded(id int64, depth int) bool {
	if v, ok := m.expanded[id]; ok {
		return v
	}
	return depth <= m.collapseDepth()
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestIsExpanded'`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/lua/lua.go internal/lua/lua_test.go internal/app/app.go internal/app/app_test.go
git commit -m "feat: collapse depth config + isExpanded helper"
```

---

### Task 2: Enable folding in tree traversal

**Files:**
- Modify: `internal/app/app.go` (VisibleWorkspaces, visibleTodos)
- Test: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `isExpanded(id, depth int) bool` from Task 1.
- Produces: nothing new (folded nodes simply disappear from the lists).

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/app_test.go`:

```go
func TestVisibleTodosHonorsCollapse(t *testing.T) {
	m := newTestApp(t)
	root := m.selectedTodo()
	child := &model.Todo{Description: "child", Pending: true, ParentTodoID: &root.ID}
	root.Todos = append(root.Todos, child)
	if err := m.store.SaveTodo(child); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// With collapse_depth 0, all visible.
	if got := len(m.visibleTodos()); got != 2 {
		t.Fatalf("expected 2 visible todos (root+child), got %d", got)
	}
	// Collapse the root: child disappears.
	m.expanded[root.ID] = false
	if got := len(m.visibleTodos()); got != 1 {
		t.Fatalf("expected 1 visible todo after collapse, got %d", got)
	}
	if m.visibleTodos()[0].ID != root.ID {
		t.Fatal("collapsed root should still be visible, child hidden")
	}
}

func TestVisibleWorkspacesHonorsCollapse(t *testing.T) {
	m := newTestApp(t)
	// newTestApp creates one top-level workspace; add a nested one.
	top := m.root.Children[0]
	nested := &model.Workspace{Description: "nested", OrderIndex: 0}
	if err := m.store.SaveWorkspace(nested); err != nil {
		t.Fatal(err)
	}
	// Attach nested under top via refresh (SaveWorkspace auto-attaches to root;
	// we rewire in-memory).
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Find the nested one (auto-attached to root); re-parent it under top.
	nested = findWorkspace(m.root, nested.ID)
	top = m.root.Children[0]
	// Remove from root children, add under top.
	for i, c := range m.root.Children {
		if c.ID == nested.ID {
			m.root.Children = append(m.root.Children[:i], m.root.Children[i+1:]...)
			break
		}
	}
	nested.ParentID = &top.ID
	nested.Parent = top
	top.Children = append(top.Children, nested)
	if err := m.store.ReorderAll(m.root); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	if got := len(m.VisibleWorkspaces()); got != 2 {
		t.Fatalf("expected 2 visible workspaces, got %d", got)
	}
	m.expanded[top.ID] = false
	if got := len(m.VisibleWorkspaces()); got != 1 {
		t.Fatalf("expected 1 visible workspace after collapse, got %d", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestVisibleTodosHonorsCollapse|TestVisibleWorkspacesHonorsCollapse'`
Expected: FAIL — `|| true` keeps children visible even when collapsed.

- [ ] **Step 3: Replace `|| true` with `isExpanded`**

In `internal/app/app.go` `VisibleWorkspaces`:

```go
	walk = func(ws *model.Workspace) {
		out = append(out, ws)
		if m.isExpanded(ws.ID, ws.NestLevel()) {
			for _, c := range ws.Children {
				walk(c)
			}
		}
	}
```

In `visibleTodos`:

```go
	walk = func(t *model.Todo) {
		if m.filter == "" || matchesFilter(t, m.filter) {
			out = append(out, t)
		}
		if m.isExpanded(t.ID, t.NestLevel()) {
			for _, c := range t.Todos {
				walk(c)
			}
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestVisibleTodosHonorsCollapse|TestVisibleWorkspacesHonorsCollapse'`
Expected: PASS

- [ ] **Step 5: Run the full app tests**

Run: `go test ./internal/app/`
Expected: PASS (any test that built deep trees and asserted full visibility may need a `m.expanded[id] = true` seed — fix inline if so, keeping the "collapse_depth 0 = expand all" invariant via `isExpanded`).

- [ ] **Step 6: Commit**

```bash
git add internal/app/app.go internal/app/app_test.go
git commit -m "feat: honor expand state in tree traversal"
```

---

### Task 3: Expand arrows in rendering

**Files:**
- Modify: `internal/app/renderers.go` (`renderExpandArrow`)
- Modify: `internal/app/view.go` (workspace + todo pane row rendering)
- Test: `internal/app/renderers_test.go`, `internal/app/view_test.go`

**Interfaces:**
- Consumes: `isExpanded(id, depth) bool`; `appTheme()`.
- Produces: `func (m *Model) renderExpandArrow(row string, hasChildren, expanded bool) string`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/renderers_test.go`:

```go
func TestRenderExpandArrow(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	// No children: no arrow.
	no := m.renderExpandArrow("abc", false, false)
	if strings.Contains(no, "▸") || strings.Contains(no, "▾") {
		t.Fatalf("leaf node must not get an arrow, got %q", no)
	}
	// Collapsed with children: ▸.
	collapsed := m.renderExpandArrow("abc", true, false)
	if !strings.Contains(collapsed, "▸") {
		t.Fatalf("collapsed node should show ▸, got %q", collapsed)
	}
	// Expanded with children: ▾.
	expanded := m.renderExpandArrow("abc", true, true)
	if !strings.Contains(expanded, "▾") {
		t.Fatalf("expanded node should show ▾, got %q", expanded)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestRenderExpandArrow`
Expected: FAIL — `renderExpandArrow` undefined.

- [ ] **Step 3: Implement `renderExpandArrow`**

In `internal/app/renderers.go` (after `formatTodoColumn`):

```go
// renderExpandArrow appends ▸ (collapsed) or ▾ (expanded) to a row when the
// node has children. Leaf nodes get no arrow.
func (m *Model) renderExpandArrow(row string, hasChildren, expanded bool) string {
	if !hasChildren {
		return row
	}
	arrow := "▸"
	if expanded {
		arrow = "▾"
	}
	return row + m.appTheme().Style("primary").Render(" " + arrow)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run TestRenderExpandArrow`
Expected: PASS

- [ ] **Step 5: Wire arrows into workspace pane rendering**

In `internal/app/view.go` `renderWorkspacePaneViewport`, after the row is built
(after the scrollbar block, before `lines = append`), add:

```go
		// Expand/collapse arrow for nodes with children.
		ws := ws[i]
		row = m.renderExpandArrow(row, len(ws.Children) > 0, m.isExpanded(ws.ID, ws.NestLevel()))
		lines = append(lines, row)
```

(Replace the existing `lines = append(lines, row)` at the end of that loop.)

- [ ] **Step 6: Wire arrows into todo pane rendering**

In `internal/app/view.go` `renderTodoPaneViewport`, after the row is built
(after the scrollbar block, before `lines = append`), add:

```go
		row = m.renderExpandArrow(row, len(todos[i].Todos) > 0, m.isExpanded(todos[i].ID, todos[i].NestLevel()))
		lines = append(lines, row)
```

(Replace the existing `lines = append(lines, row)` at the end of that loop.)

- [ ] **Step 7: Run the app tests**

Run: `go test ./internal/app/`
Expected: PASS. If width-boundary tests fail (rows now 2 cols wider with
arrows), adjust the assertion width by 2, or account for the arrow in the
budget. Prefer keeping arrows within the existing pane width by reducing the
content budget by 2 when the node has children (optional; if it causes
overflow, note it and keep arrows appended after content).

- [ ] **Step 8: Commit**

```bash
git add internal/app/renderers.go internal/app/renderers_test.go internal/app/view.go
git commit -m "feat: render expand/collapse arrows"
```

---

### Task 4: Cursor repositioning after collapse

**Files:**
- Modify: `internal/app/action.go` (`actionToggleExpand`, `actionToggleExpandParent`)
- Test: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `visibleTodos()`, `VisibleWorkspaces()`, `indexOfTodoByID`, `indexOfWorkspaceByID`.
- Produces: `func (m *Model) clampAfterCollapse(pane int, collapsedID int64)`

- [ ] **Step 1: Write the failing test**

Append to `internal/app/app_test.go`:

```go
func TestClampAfterCollapseMovesCursorUp(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	root := m.selectedTodo()
	child := &model.Todo{Description: "child", Pending: true, ParentTodoID: &root.ID}
	root.Todos = append(root.Todos, child)
	if err := m.store.SaveTodo(child); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Put the cursor on the child (index 1), then collapse the root.
	m.TodoCursor = 1
	m.expanded[root.ID] = false
	m.clampAfterCollapse(PaneTodo, root.ID)
	// Cursor must now point at the root (index 0).
	sel := m.selectedTodo()
	if sel == nil || sel.ID != root.ID {
		t.Fatalf("cursor should jump to the collapsed node, got %+v", sel)
	}
}

func TestClampAfterCollapseRootVisible(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	root := m.selectedTodo()
	// Collapse the root while the cursor is on it; stays put.
	m.TodoCursor = 0
	m.expanded[root.ID] = false
	m.clampAfterCollapse(PaneTodo, root.ID)
	if sel := m.selectedTodo(); sel == nil || sel.ID != root.ID {
		t.Fatalf("cursor should stay on the root, got %+v", sel)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestClampAfterCollapse'`
Expected: FAIL — `clampAfterCollapse` undefined.

- [ ] **Step 3: Implement `clampAfterCollapse`**

In `internal/app/action.go` (near `actionToggleExpand`):

```go
// clampAfterCollapse repositions the cursor after a node collapses so it
// never points at a now-hidden child. collapsedID is the node that collapsed;
// the cursor jumps to it if it was below it in the list.
func (m *Model) clampAfterCollapse(pane int, collapsedID int64) {
	if pane == PaneWorkspace {
		ws := m.VisibleWorkspaces()
		if idx := indexOfWorkspaceByID(ws, collapsedID); idx >= 0 {
			m.WorkspaceCursor = idx
			return
		}
		if len(ws) == 0 {
			m.WorkspaceCursor = 0
			return
		}
		if m.WorkspaceCursor >= len(ws) {
			m.WorkspaceCursor = len(ws) - 1
		}
		return
	}
	todos := m.visibleTodos()
	if idx := indexOfTodoByID(todos, collapsedID); idx >= 0 {
		m.TodoCursor = idx
		return
	}
	if len(todos) == 0 {
		m.TodoCursor = 0
		return
	}
	if m.TodoCursor >= len(todos) {
		m.TodoCursor = len(todos) - 1
	}
}
```

- [ ] **Step 4: Call it from the toggle actions**

In `actionToggleExpand`, after `m.expanded[id] = !m.expanded[id]`, detect a
collapse and call the clamp:

```go
	m.expanded[id] = !m.expanded[id]
	if !m.expanded[id] { // just collapsed
		m.clampAfterCollapse(m.focus, id)
	}
	m.BumpVersion()
	return nil
```

In `actionToggleExpandParent`, same pattern (collapse the parent's id):

```go
	m.expanded[id] = !m.expanded[id]
	if !m.expanded[id] { // just collapsed the parent
		m.clampAfterCollapse(m.focus, id)
	}
	m.BumpVersion()
	return nil
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestClampAfterCollapse'`
Expected: PASS

- [ ] **Step 6: Run the full app tests**

Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/app/action.go internal/app/app_test.go
git commit -m "feat: reposition cursor after collapse"
```

---

### Task 5: Default config.lua + docs

**Files:**
- Modify: `config.lua`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `README.md` (features/usage one-liner)

**Interfaces:**
- Consumes: `api.vars.collapse_depth` (Task 1).

- [ ] **Step 1: Add `collapse_depth` to `config.lua`**

Add near the other `api.vars` settings:

```lua
-- Default tree-collapse depth: nodes at depth > collapse_depth start
-- collapsed. 0 = expand everything (default). Toggle any node with z / Z.
api.vars.collapse_depth = 0
```

- [ ] **Step 2: Verify config still evaluates**

Run: `go test ./internal/lua/`
Expected: PASS

- [ ] **Step 3: Update `docs/ARCHITECTURE.md`**

In the Lua configuration section, add a line:

```
- `api.vars.collapse_depth` — default collapse depth (0 = expand all; nodes
  deeper than this start collapsed). `z` / `Z` toggle a node / its parent.
```

- [ ] **Step 4: Update `README.md` features + usage**

In the features list, change the tree bullet to mention folding:

```
- 🗂️ **Two-pane tree** — nested workspaces + todos, folding (`z`/`Z`), completion cascades, natural-language dates, recurrence
```

- [ ] **Step 5: Run full tests + vet + build**

Run: `go test ./...` and `go vet ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add config.lua docs/ARCHITECTURE.md README.md
git commit -m "docs: collapse_depth config + folding docs"
```

---

### Task 6: Final verification

**Files:** none.

- [ ] **Step 1: Full test + vet + build**

Run: `go test ./...`, `go vet ./...`, `go build ./...`
Expected: all pass

- [ ] **Step 2: Smoke-check folding**

Run the app (`fdooit --db <tmp>/t.db`), add a nested todo, press `z` on the
parent and confirm the child hides and the arrow flips to `▸`; press `Z` to
collapse the parent from a child. Confirm cursor jumps to the collapsed node.

- [ ] **Step 3: Confirm final git state**

Run: `git log --oneline -8`
Expected: the feature commits from Tasks 1–5.
