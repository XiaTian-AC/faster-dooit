# Responsive Layout & UX Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 5 UX bugs (swallowed validation errors, invisible effort input, selected-row overflow + highlight wrap, messy narrow-terminal rendering) by adding a progressive layout engine, per-pane viewport scrolling, stacked layout, and dynamic column hiding.

**Architecture:** Introduce a `layoutMode` decision in `View()` driven by terminal width/height, with three progressive states (normal dual-pane → stacked single-column → too-small notice). Add per-pane scroll offsets for viewport rendering. Fix `renderSelectedRow` to pad to the content area (not the box width), split the INSERT status bar into edit-context + error halves, and render inline editing as a full-width input row.

**Tech Stack:** Go, Bubble Tea, lipgloss (bordered boxes), `charmbracelet/x/ansi` (ANSI-aware width).

## Global Constraints

- Go ≥ 1.22, no CGO, no new external dependencies.
- All rendered rows must never exceed the pane's content-area width (borders excluded) — no terminal wrap.
- Column truncation keeps using `ansi.Truncate` / `…` ellipsis (already ANSI-aware).
- Config-driven minimums: `api.vars.min_width` (default 40) / `api.vars.min_height` (default 12).
- Layout thresholds: `W_stack` = 100, `W_hide` = 72, `H_ok` = 24.
- All tests run with `go test ./...`; lint via `go vet ./...`.

---

### Task 1: Min-size Lua config

**Files:**
- Modify: `internal/lua/lua.go` (Runtime struct + readTheme)
- Test: `internal/lua/lua_test.go`

**Interfaces:**
- Produces: `Runtime.MinWidth int`, `Runtime.MinHeight int` (defaults 40/12 when config omits them)

- [ ] **Step 1: Write the failing test**

Add to `internal/lua/lua_test.go`:

```go
func TestMinSizeDefaultsAndOverride(t *testing.T) {
	rt, err := EvalFileWithCode(`return`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.MinWidth != 40 || rt.MinHeight != 12 {
		t.Fatalf("defaults = %d/%d, want 40/12", rt.MinWidth, rt.MinHeight)
	}

	rt2, err := EvalFileWithCode(`api.vars.min_width = 60`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt2.Close()
	if rt2.MinWidth != 60 {
		t.Fatalf("min_width = %d, want 60", rt2.MinWidth)
	}
	if rt2.MinHeight != 12 {
		t.Fatalf("min_height = %d, want default 12", rt2.MinHeight)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lua/ -run TestMinSizeDefaultsAndOverride -v`
Expected: FAIL (`rt.MinWidth` is 0, zero-value struct field).

- [ ] **Step 3: Implement**

Add fields to `Runtime`:

```go
type Runtime struct {
	...
	MinWidth  int
	MinHeight int
	...
}
```

In `readTheme()`, after the urgency-colors block:

```go
if rt.varsTable != nil {
	if n, ok := L.GetField(rt.varsTable, "min_width").(lua.LNumber); ok {
		rt.MinWidth = int(n)
	}
	if n, ok := L.GetField(rt.varsTable, "min_height").(lua.LNumber); ok {
		rt.MinHeight = int(n)
	}
}
if rt.MinWidth == 0 {
	rt.MinWidth = 40
}
if rt.MinHeight == 0 {
	rt.MinHeight = 12
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/lua/ -run TestMinSizeDefaultsAndOverride -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/lua/lua.go internal/lua/lua_test.go
git commit -m "feat(lua): configurable min_width/min_height"
```

---

### Task 2: Scroll offsets on the Model

**Files:**
- Modify: `internal/app/app.go` (Model struct)
- Test: `internal/app/app_test.go`

**Interfaces:**
- Produces: `Model.workspaceScroll int`, `Model.todoScroll int` fields

- [ ] **Step 1: Write the failing test**

Add to `internal/app/app_test.go`:

```go
func TestScrollOffsetsDefault(t *testing.T) {
	m := newTestApp(t)
	if m.workspaceScroll != 0 || m.todoScroll != 0 {
		t.Fatalf("scroll offsets should start at 0, got %d/%d", m.workspaceScroll, m.todoScroll)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestScrollOffsetsDefault -v`
Expected: FAIL (`m.workspaceScroll` undefined).

- [ ] **Step 3: Implement**

Add fields to `Model` struct in `internal/app/app.go`:

```go
	// Per-pane viewport scroll offsets for short terminals.
	workspaceScroll int
	todoScroll      int
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run TestScrollOffsetsDefault -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go internal/app/app_test.go
git commit -m "feat(app): per-pane viewport scroll offsets"
```

---

### Task 3: Layout mode decision + too-small notice

**Files:**
- Modify: `internal/app/view.go` (View entry + new helpers)
- Test: `internal/app/view_test.go`

**Interfaces:**
- Consumes: `m.width`, `m.height`, `m.luaCfg.MinWidth/MinHeight`
- Produces: `type layoutMode int` with constants `layoutNormal`, `layoutStacked`, `layoutTooSmall`; `func (m *Model) layoutMode() layoutMode`; `func (m *Model) renderTooSmall() string`

- [ ] **Step 1: Write the failing tests**

Add to `internal/app/view_test.go`:

```go
func TestLayoutModeDecision(t *testing.T) {
	m := newTestApp(t)
	cases := []struct {
		w, h int
		want layoutMode
	}{
		{120, 30, layoutNormal},
		{80, 30, layoutStacked},
		{100, 30, layoutStacked}, // < W_stack is stacked (100 not stacked)
		{150, 30, layoutNormal},
		{80, 10, layoutTooSmall},  // too short
		{30, 30, layoutTooSmall},  // too narrow
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run "TestLayoutModeDecision|TestTooSmallNotice" -v`
Expected: FAIL (`layoutMode` undefined).

- [ ] **Step 3: Implement**

Add to `internal/app/view.go` before `View()`:

```go
// layoutMode selects the responsive layout for the current terminal size.
// Width and height are evaluated independently; either being too small
// wins over everything.
type layoutMode int

const (
	layoutNormal  layoutMode = iota // dual pane side-by-side
	layoutStacked                   // stacked, single column
	layoutTooSmall                  // stop rendering, show notice
)

const (
	layoutWStack = 100 // width >= this: dual pane
	layoutHOk    = 24  // height >= this: no viewport scroll
)

func (m *Model) layoutMode() layoutMode {
	mw, mh := 40, 12
	if m.luaCfg != nil {
		if m.luaCfg.MinWidth > 0 {
			mw = m.luaCfg.MinWidth
		}
		if m.luaCfg.MinHeight > 0 {
			mh = m.luaCfg.MinHeight
		}
	}
	if m.width < mw || m.height < mh {
		return layoutTooSmall
	}
	if m.width < layoutWStack {
		return layoutStacked
	}
	return layoutNormal
}

func (m *Model) renderTooSmall() string {
	mw, mh := 40, 12
	if m.luaCfg != nil {
		if m.luaCfg.MinWidth > 0 {
			mw = m.luaCfg.MinWidth
		}
		if m.luaCfg.MinHeight > 0 {
			mh = m.luaCfg.MinHeight
		}
	}
	return fmt.Sprintf("Terminal size too small: Width = %d Height = %d\nNeeded for current config: Width = %d Height = %d",
		m.width, m.height, mw, mh)
}
```

In `View()`, right after the `m.width == 0 || m.height == 0` guard:

```go
	if m.layoutMode() == layoutTooSmall {
		return m.renderTooSmall()
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run "TestLayoutModeDecision|TestTooSmallNotice" -v`
Expected: PASS. Then run `go test ./internal/app/` — the existing `TestViewFitsTerminalWidth` (widths 40+) and `TestViewVerticallyCenters` (80×30, now stacked) must still pass; fix if the stacked path regresses `TestViewVerticallyCenters` (it only checks leading `\n\n`, so it should hold).

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go internal/app/view_test.go
git commit -m "feat(view): responsive layout mode + too-small notice"
```

---

### Task 4: Stacked layout (workspace above, focus-weighted)

**Files:**
- Modify: `internal/app/view.go` (`View`, `renderWorkspacePane`, `renderTodoPane` signatures + new stacked renderer)
- Test: `internal/app/view_test.go`

**Interfaces:**
- Consumes: `layoutMode()`, `layoutStacked`
- Produces: `func (m *Model) renderStacked() string` (both panes stacked, focus pane ~70%)

- [ ] **Step 1: Write the failing test**

Add to `internal/app/view_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestStackedLayoutRendersBothPanes -v`
Expected: FAIL (both titles on the same line in dual-pane mode).

- [ ] **Step 3: Implement**

Restructure `View()`:

```go
func (m *Model) View() string {
	if m.quitting {
		return "bye.\n"
	}
	if m.width == 0 || m.height == 0 {
		return "loading…\n"
	}
	if m.layoutMode() == layoutTooSmall {
		return m.renderTooSmall()
	}
	if m.helpVisible {
		return lipgloss.JoinVertical(lipgloss.Left, m.HelpView(), m.renderStatusBar())
	}

	var combined string
	if m.layoutMode() == layoutStacked {
		combined = m.renderStacked()
	} else {
		combined = m.renderDualPane()
	}

	status := m.renderStatusBar()
	content := lipgloss.JoinVertical(lipgloss.Left, combined, status)

	if m.height > 0 {
		lines := strings.Count(content, "\n") + 1
		if top := (m.height - lines) / 2; top > 0 {
			content = strings.Repeat("\n", top) + content
		}
	}
	return content
}
```

Move the existing dual-pane body into a helper and add the stacked helper:

```go
func (m *Model) renderDualPane() string {
	paneW := m.width / 4
	if paneW < 16 {
		paneW = 16
	}
	rightW := m.width - paneW - 4
	left := m.renderWorkspacePane(paneW - 2)
	right := m.renderTodoPane(rightW - 2)
	var combined string
	if m.focus == PaneWorkspace {
		combined = lipgloss.JoinHorizontal(lipgloss.Top,
			focusedBorder.Width(paneW).Render(left),
			dimBorder.Width(rightW).Render(right),
		)
	} else {
		combined = lipgloss.JoinHorizontal(lipgloss.Top,
			dimBorder.Width(paneW).Render(left),
			focusedBorder.Width(rightW).Render(right),
		)
	}
	return combined
}

// renderStacked lays the two panes vertically, giving ~70% of the height to
// the focused pane and the rest to the other. Each pane's rows are viewport
// clipped by its own scroll offset.
func (m *Model) renderStacked() string {
	statusH := 1
	avail := m.height - statusH
	focusH := avail * 7 / 10
	otherH := avail - focusH
	if otherH < 3 {
		otherH = 3
		focusH = avail - otherH
	}
	// Content width inside the bordered box (border 1 each side + padding 1
	// each side = 4 chrome columns).
	contentW := m.width - 4

	var top, bottom string
	if m.focus == PaneWorkspace {
		top = focusedBorder.Width(m.width).Render(m.renderWorkspacePaneClipped(contentW, focusH, m.workspaceScroll))
		bottom = dimBorder.Width(m.width).Render(m.renderTodoPaneClipped(contentW, otherH, m.todoScroll))
	} else {
		top = dimBorder.Width(m.width).Render(m.renderWorkspacePaneClipped(contentW, otherH, m.workspaceScroll))
		bottom = focusedBorder.Width(m.width).Render(m.renderTodoPaneClipped(contentW, focusH, m.todoScroll))
	}
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}
```

**Note:** `renderWorkspacePane`/`renderTodoPane` currently take a content width and render all rows. For this task, introduce thin wrappers that add the scroll/viewport slicing (kept minimal here; the full viewport logic lands in Task 5). Add:

```go
func (m *Model) renderWorkspacePaneClipped(contentW, maxLines, scroll int) string {
	ws := m.VisibleWorkspaces()
	m.clampWorkspaceScroll(ws, maxLines)
	return sliceRenderedLines(m.renderWorkspacePane(contentW), m.workspaceScroll, maxLines)
}

func (m *Model) renderTodoPaneClipped(contentW, maxLines, scroll int) string {
	todos := m.visibleTodos()
	m.clampTodoScroll(todos, maxLines)
	return sliceRenderedLines(m.renderTodoPane(contentW), m.todoScroll, maxLines)
}

func sliceRenderedLines(s string, scroll, maxLines int) string {
	lines := strings.Split(s, "\n")
	if scroll >= len(lines) {
		scroll = max(0, len(lines)-1)
	}
	end := scroll + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	if scroll > end {
		scroll = end
	}
	return strings.Join(lines[scroll:end], "\n")
}
```

Add scroll-clamping stubs (full logic in Task 5):

```go
func (m *Model) clampWorkspaceScroll(ws []*model.Workspace, maxLines int) {
	titleLines := 1 // "Workspaces" title
	max := len(ws) + titleLines
	if m.WorkspaceCursor >= maxLines {
		m.workspaceScroll = m.WorkspaceCursor - maxLines + titleLines + 1
	} else if m.workspaceScroll > 0 && m.workspaceScroll > m.WorkspaceCursor {
		m.workspaceScroll = m.WorkspaceCursor
	}
	if m.workspaceScroll < 0 {
		m.workspaceScroll = 0
	}
	if m.workspaceScroll > max-titleLines {
		m.workspaceScroll = max - titleLines
	}
}

func (m *Model) clampTodoScroll(todos []*model.Todo, maxLines int) {
	titleLines := 1
	max := len(todos) + titleLines
	if m.TodoCursor >= maxLines {
		m.todoScroll = m.TodoCursor - maxLines + titleLines + 1
	} else if m.todoScroll > 0 && m.todoScroll > m.TodoCursor {
		m.todoScroll = m.TodoCursor
	}
	if m.todoScroll < 0 {
		m.todoScroll = 0
	}
	if m.todoScroll > max-titleLines {
		m.todoScroll = max - titleLines
	}
}
```

**Note on width accounting:** `renderWorkspacePane(w)` and `renderTodoPane(w)` are called with the *content width* `contentW = m.width - 4`. In dual-pane mode this task keeps the existing calls (already content-width after the border fix). The `View()` refactor must keep dual-pane output identical to before this task.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestStackedLayoutRendersBothPanes -v`
Expected: PASS
Run: `go test ./internal/app/ -run TestViewFitsTerminalWidth -v`
Expected: PASS (dual-pane unchanged)
Run: `go test ./internal/app/`
Expected: PASS (all)

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go internal/app/view_test.go
git commit -m "feat(view): stacked layout with focus-weighted heights"
```

---

### Task 5: Viewport scrolling (short terminals)

**Files:**
- Modify: `internal/app/view.go` (scroll clamping), `internal/app/action.go` (scroll sync on cursor move)
- Test: `internal/app/view_test.go`, `internal/app/app_test.go`

**Interfaces:**
- Consumes: `workspaceScroll`, `todoScroll`, `clampWorkspaceScroll`, `clampTodoScroll`
- Produces: scroll keeps cursor visible; moving down at the bottom shifts the window

- [ ] **Step 1: Write the failing test**

Add to `internal/app/view_test.go`:

```go
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
	// 4 visible rows (title + 3 items).
	m.width, m.height = 80, 6
	v := m.View()
	lines := strings.Split(v, "\n")
	// "Todos" title must appear.
	found := false
	for _, ln := range lines {
		if strings.Contains(ln, "Todos") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Todos title missing:\n%s", v)
	}
	// Cursor must be at the bottom row: the last item row (starting with ">")
	// must be the last non-empty content line before the status bar.
	last := ""
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			last = ln
		}
	}
	if !strings.Contains(last, "> ") {
		t.Fatalf("cursor row should be visible at bottom, last content line = %q", last)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestViewportScrollKeepsCursorVisible -v`
Expected: FAIL (no viewport — cursor row renders past the pane bottom / wraps).

- [ ] **Step 3: Implement**

In `View()` (or the stacked renderer), ensure scroll clamping runs whenever rendering. The `clamp*Scroll` stubs from Task 4 already handle the "cursor below the window" case. Verify they do: when `maxLines = 4` and `TodoCursor = 8`, `todoScroll = 8 - 4 + 1 + 1 = 6`, so the window shows lines 6..9 (title at 0 is scrolled off, showing items 6,7,8). Cursor at row index 8 → rendered at line 8-6 = 2 (0-based within the 4-line window) — visible. `sliceRenderedLines` returns lines[6:10] capped at len.

Fix the off-by-one so the window is `[scroll, scroll+maxLines)` and the cursor line is within it. Adjust `clampTodoScroll`/`clampWorkspaceScroll`:

```go
func (m *Model) clampTodoScroll(todos []*model.Todo, maxLines int) {
	// +1 for the "Todos" title line.
	total := len(todos) + 1
	// Keep the cursor row visible: index is offset by the title line.
	cursorLine := m.TodoCursor + 1
	if cursorLine < m.todoScroll {
		m.todoScroll = cursorLine
	}
	if cursorLine >= m.todoScroll+maxLines {
		m.todoScroll = cursorLine - maxLines + 1
	}
	if m.todoScroll < 0 {
		m.todoScroll = 0
	}
	if m.todoScroll > total-maxLines {
		m.todoScroll = total - maxLines
	}
	if m.todoScroll < 0 {
		m.todoScroll = 0
	}
}
```

(analogous for workspace with `WorkspaceCursor`)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestViewportScrollKeepsCursorVisible -v`
Expected: PASS
Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go internal/app/view_test.go
git commit -m "feat(view): viewport scrolling keeps cursor visible"
```

---

### Task 6: Fix selected-row highlight overflow (bugs #3 + #5)

**Files:**
- Modify: `internal/app/view.go` (`renderSelectedRow`)
- Test: `internal/app/view_test.go`

**Interfaces:**
- Consumes: content width `w`
- Produces: `renderSelectedRow(row, w)` pads to `w` (content area), never beyond

- [ ] **Step 1: Write the failing test**

Add to `internal/app/view_test.go`:

```go
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
	// In stacked mode the todo pane content width is m.width-4.
	v := m.View()
	for _, line := range strings.Split(v, "\n") {
		if lw := lipgloss.Width(line); lw > m.width {
			t.Errorf("selected row overflows terminal by %d cols: %q", lw-m.width, line)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestSelectedRowNeverExceedsPane -v`
Expected: FAIL (selected row padded to `w` = box width, 2 cols wider than content).

- [ ] **Step 3: Implement**

`renderSelectedRow(row, w)` is called from `renderTodoPane`/`renderWorkspacePane` with the content width `w`. Verify every call site passes the *content* width (after the Task 4 refactor, the panes receive content width, and `w` inside the pane funcs is already content width). The padding uses `w`; since `w` is now the content width, the fix is already applied — but **confirm no call site still passes the box width**. Audit `view.go`: in the stacked path `contentW = m.width - 4`; in dual-pane, `rightW-2`/`paneW-2`. Also ensure `formatTodoAligned` columns sum to ≤ `w` (they do: `columnWidths` budgets to `w` minus gaps).

If any call site passes box width, change it to content width. Also remove the leftover `w` arithmetic inside `renderSelectedRow` if it double-counts:

```go
func (m *Model) renderSelectedRow(row string, w int) string {
	visible := lipgloss.Width(row)
	if pad := w - visible; pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	th := m.appTheme()
	bg := ansiBackground(th.Background1)
	if bg == "" {
		return row
	}
	reset := "\x1b[0m"
	return bg + strings.ReplaceAll(row, reset, reset+bg) + reset
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestSelectedRowNeverExceedsPane -v`
Expected: PASS
Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go internal/app/view_test.go
git commit -m "fix(view): selected-row highlight fits content area (no wrap)"
```

---

### Task 7: Dynamic column hiding in stacked mode

**Files:**
- Modify: `internal/app/renderers.go` (`ColumnLayout`/`columnWidths`), `internal/app/view.go` (stacked renderer passes content width)
- Test: `internal/app/column_width_test.go`

**Interfaces:**
- Consumes: `layoutStacked`, content width
- Produces: `func (m *Model) visibleColumns(pane int, contentW int) []string` (drops urgency → effort → due until fixed columns fit)

- [ ] **Step 1: Write the failing test**

Add to `internal/app/column_width_test.go`:

```go
// TestStackedHidesColumnsOnNarrowWidth: as the stacked pane narrows, less
// important columns are dropped (urgency first, then effort, then due),
// leaving at least status+description.
func TestStackedHidesColumnsOnNarrowWidth(t *testing.T) {
	m := newTestApp(t)
	full := []string{"status", "description", "due", "urgency"}
	cases := []struct {
		w    int
		keep int // minimum expected number of columns kept
	}{
		{120, 4},
		{72, 4},
		{60, 3},
		{45, 2},
	}
	for _, c := range cases {
		cols := m.visibleColumns(PaneTodo, c.w-4)
		if len(cols) > c.keep {
			t.Errorf("width %d: kept %d columns %v, want <= %d", c.w, len(cols), cols, c.keep)
		}
		// description must always survive.
		found := false
		for _, col := range cols {
			if col == "description" {
				found = true
			}
		}
		if !found {
			t.Errorf("width %d: description dropped from %v", c.w, cols)
		}
	}
	// Sanity: full layout at wide width.
	cols := m.visibleColumns(PaneTodo, 200)
	if len(cols) != len(full) {
		t.Fatalf("wide width columns = %v, want %v", cols, full)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestStackedHidesColumnsOnNarrowWidth -v`
Expected: FAIL (`visibleColumns` undefined).

- [ ] **Step 3: Implement**

Add to `internal/app/renderers.go`:

```go
// dropOrder lists columns from least to most important; when the pane is too
// narrow to fit all fixed columns, they are removed in this order until the
// remaining fixed widths fit. description is elastic and always survives.
var dropOrder = []string{"urgency", "effort", "due"}

func fixedWidth(col string) int {
	switch col {
	case "status":
		return 1
	case "due":
		return 16
	case "effort":
		return 4
	case "recurrence":
		return 6
	case "urgency":
		return 4
	}
	return 0
}

// visibleColumns returns the columns to render for pane given the available
// content width. It starts from the configured layout and drops the least
// important fixed columns (in dropOrder) until the fixed columns + gaps fit
// in contentW; description (elastic) is never dropped.
func (m *Model) visibleColumns(pane int, contentW int) []string {
	cols := append([]string{}, m.ColumnLayout(pane)...)
	drop := append([]string{}, dropOrder...)
	for {
		fixed := 0
		elastic := 0
		for _, c := range cols {
			if w := fixedWidth(c); w > 0 {
				fixed += w
			} else {
				elastic++
			}
		}
		gaps := len(cols) - 1
		if fixed+gaps <= contentW || elastic == 0 {
			break
		}
		// drop the least important fixed column that's still present
		dropped := false
		for _, d := range drop {
			for i, c := range cols {
				if c == d {
					cols = append(cols[:i], cols[i+1:]...)
					dropped = true
					break
				}
			}
			if dropped {
				break
			}
		}
		if !dropped {
			break // nothing left to drop
		}
	}
	return cols
}
```

Wire `visibleColumns` into `columnWidths` (replace the `cols := m.ColumnLayout(pane)` line):

```go
func (m *Model) columnWidths(pane int, paneW int) map[string]int {
	cols := m.visibleColumns(pane, paneW)
	...
}
```

**Note:** `columnWidths`'s `paneW` is the total column budget (including gaps). The test passes `c.w-4` as content width; `visibleColumns` compares `fixed+gaps <= contentW` which is consistent with how `columnWidths` budgets `paneW = fixed + gaps + elastic`. To keep semantics aligned, in `columnWidths` call `m.visibleColumns(pane, paneW)`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestStackedHidesColumnsOnNarrowWidth -v`
Expected: PASS
Run: `go test ./internal/app/`
Expected: PASS (dual-pane widths stay full-columned since paneW ≥ 100-ish; existing tests use 40/60/100 which in *direct* `renderTodoPane` calls keep current behavior — verify `TestTodoRowsFitPaneWidthCJK` still passes)

- [ ] **Step 5: Commit**

```bash
git add internal/app/renderers.go internal/app/column_width_test.go
git commit -m "feat(render): drop less-important columns on narrow stacked panes"
```

---

### Task 8: Split INSERT status bar (bug #1)

**Files:**
- Modify: `internal/app/bars.go` (`renderStatusBar`)
- Test: `internal/app/bars_test.go` (new file, or `app_test.go`)

**Interfaces:**
- Consumes: `m.mode`, `m.editField`, `m.notice`
- Produces: INSERT status bar shows both edit context and error

- [ ] **Step 1: Write the failing test**

Add to `internal/app/app_test.go`:

```go
func TestInsertStatusBarShowsErrorAndField(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("due")
	m.notice = "invalid date: unknown format"
	v := m.renderStatusBar()
	if !strings.Contains(v, "editing due") {
		t.Fatalf("status bar should show edit field, got %q", v)
	}
	if !strings.Contains(v, "invalid date") {
		t.Fatalf("status bar should show the error, got %q", v)
	}
}
```

(need `"strings"` import in `app_test.go`)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestInsertStatusBarShowsErrorAndField -v`
Expected: FAIL (INSERT branch shows only `editing due`, no error).

- [ ] **Step 3: Implement**

Rewrite the `ModeInsert` branch in `renderStatusBar` (`bars.go:44-50`):

```go
	case ModeInsert:
		// Split: left shows what's being edited; right shows a validation
		// error from the last confirm (if any), else the field notice.
		left := " editing " + m.editField + " "
		right := ""
		if m.notice != "" {
			right = " " + m.notice + " "
		}
		return th.Style("primary").Render(mode) + pad(max(0, m.width-len(mode)-len(left)-len(right))) +
			th.Style("secondary").Render(left+right)
```

Remove the `else if m.notice` duplicate inside that branch. (The `default` branch still handles NORMAL-mode notices.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestInsertStatusBarShowsErrorAndField -v`
Expected: PASS
Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/bars.go internal/app/app_test.go
git commit -m "fix(bars): show validation error beside edit field in INSERT"
```

---

### Task 9: Full-width inline edit row (bug #2)

**Files:**
- Modify: `internal/app/view.go` (`renderTodoPane`, `renderWorkspacePane`), `internal/app/renderers.go` (`formatTodoAligned` editField branch)
- Test: `internal/app/view_test.go`

**Interfaces:**
- Consumes: `m.mode == ModeInsert`, `m.input.View()`
- Produces: INSERT renders `indent + marker + input` full-width; `formatTodoAligned` no longer special-cases `editField`

- [ ] **Step 1: Write the failing test**

Add to `internal/app/view_test.go`:

```go
// TestInlineEditFullWidthInput: while editing effort (a 4-col column), the
// input must render at full available width — not clipped to the column.
func TestInlineEditFullWidthInput(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.StartEdit("effort")
	m.input.SetValue("1234567890")
	v := m.renderTodoPane(60)
	// The input text must be fully visible somewhere in the row.
	if !strings.Contains(v, "1234567890") {
		t.Fatalf("effort input should show full value, got:\n%s", v)
	}
	// And the input row must not exceed the pane content width.
	for _, line := range strings.Split(v, "\n") {
		if lw := lipgloss.Width(line); lw > 60 {
			t.Errorf("inline edit row overflows pane by %d cols: %q", lw-60, line)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestInlineEditFullWidthInput -v`
Expected: FAIL (value clipped to 4 cols; "1234567890" absent).

- [ ] **Step 3: Implement**

In `renderTodoPane`, replace the insert-mode branch:

```go
		if m.mode == ModeInsert && selected {
			row := indent + marker + m.input.View()
			lines = append(lines, row)
			continue
		}
```

(Remove the `formatTodoAligned(todos[i], widths, editField, editInput)` call.) Same change in `renderWorkspacePane`:

```go
		if m.mode == ModeInsert && selected {
			lines = append(lines, indent+marker+m.input.View())
			continue
		}
```

In `renderers.go` `formatTodoAligned`, remove the `editField`/`input` parameters and the special-case:

```go
func (m *Model) formatTodoAligned(t *model.Todo, widths map[string]int) string {
	cols := m.visibleColumns(PaneTodo, 0) // width unused here
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		cell := m.formatTodoColumn(col, t)
		if w := widths[col]; w > 0 {
			cell = fitColumn(cell, w)
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, " ")
}
```

Update both call sites in `view.go` to `m.formatTodoAligned(todos[i], widths)`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestInlineEditFullWidthInput -v`
Expected: PASS
Run: `go test ./internal/app/`
Expected: PASS (existing inline-edit tests `TestInlineEditRendersOnCursorRow` must still pass)

- [ ] **Step 5: Commit**

```bash
git add internal/app/view.go internal/app/renderers.go internal/app/view_test.go
git commit -m "feat(view): full-width inline edit row for all fields"
```

---

### Task 10: Final verification

- [ ] **Step 1: Run the full suite + vet**

Run: `go test ./...`
Expected: all ok
Run: `go vet ./...`
Expected: no output

- [ ] **Step 2: Run the perf gates**

Run: `go test ./internal/app/ -bench . -benchmem`
Expected: no regressions vs README targets (Startup10k <200ms, Update <1ms).

- [ ] **Step 3: Manual smoke (optional, user-driven)**

Run the app in a narrow (60×20) and wide (150×30) terminal; verify stacked/dual modes, too-small notice, INSERT error display, effort editing visibility.
