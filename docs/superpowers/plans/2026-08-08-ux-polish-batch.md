# UX Polish Batch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship 6 UX fixes: config ergonomics, dynamic columns, urgency floor, scroll threshold, edit contrast, multi-line descriptions.

**Architecture:** Three independent batches (by file-conflict surface): (A) Lua config surface rewrite, (B) urgency + scroll threshold, (C) dynamic columns + input contrast + multi-line description. Each batch is independently testable and committed separately.

**Tech Stack:** Go, Bubble Tea, gopher-lua (existing).

## Global Constraints

- Spec: `docs/superpowers/specs/2026-08-08-ux-polish-batch-design.md` (approved).
- No backward compat for the old `api.` config prefix — old configs are rejected.
- Urgency minimum is 1 (cannot decrease below 1).
- Scroll floor defaults: height 6, width 30.
- New key `o` = toggle_description_expand.
- `max_description_lines` config default 3; `0` = no ellipsis.
- All tests green, `go vet` clean after each batch.

---

### Batch A: Config surface simplification (requiremt 1)

**Files:**
- Modify: `internal/lua/lua.go`
- Modify: `config.lua`
- Test: `internal/lua/lua_test.go`

**Interfaces:**
- Produces: `Runtime` fields unchanged; `installAPI` exposes bare globals
  (`theme`, `keys`, `layouts`, `formatter`, `bar`, `dashboard`, `notify`,
  `now`, `move_down`, ...) instead of under an `api` table.

- [ ] **Step 1: Read `internal/lua/lua.go` `installAPI` and `readTheme`**

Read `internal/lua/lua.go` fully first (the API-table construction in
`installAPI`, `readTheme`, `actionNames`, `themeFields`). Understand what is
currently mounted under `api`.

- [ ] **Step 2: Rewrite `installAPI` to expose bare globals**

Replace the `api := L.NewTable()` construction so each member is a global:

```go
func (rt *Runtime) installAPI(L *lua.LState) {
	// keys.set(key|table, action)
	keys := L.NewTable()
	L.SetField(keys, "set", L.NewFunction(rt.keysSet))
	L.SetGlobal("keys", keys)

	// layouts.<name> = {cols} — intercepted via __newindex
	layouts := L.NewTable()
	layoutsMT := L.NewTable()
	L.SetField(layoutsMT, "__newindex", L.NewFunction(rt.layoutsNewIndex))
	L.SetMetatable(layouts, layoutsMT)
	L.SetGlobal("layouts", layouts)

	// formatter.todos.<field>.add(fn)
	formatter := L.NewTable()
	todosFmt := L.NewTable()
	for _, field := range formatterFields {
		ft := L.NewTable()
		L.SetField(ft, "add", L.NewFunction(rt.formatterAdd(field)))
		L.SetField(todosFmt, field, ft)
	}
	L.SetField(formatter, "todos", todosFmt)
	L.SetGlobal("formatter", formatter)

	// bar.set({...}) / dashboard.set({...})
	bar := L.NewTable()
	L.SetField(bar, "set", L.NewFunction(rt.barSet))
	L.SetGlobal("bar", bar)
	dashboard := L.NewTable()
	L.SetField(dashboard, "set", L.NewFunction(rt.dashboardSet))
	L.SetGlobal("dashboard", dashboard)

	// theme + urgency_colors + collapse_depth + min_* as globals
	theme := L.NewTable()
	L.SetGlobal("theme", theme)
	rt.themeTable = theme
	// vars is gone; store the values table for readTheme via a new field
	// or read them straight from the theme table + a vars table for the
	// non-theme numbers. Keep rt.varsTable pointing at a new table used
	// only to capture urgency_colors / collapse_depth / min_width / min_height.
	vars := L.NewTable()
	L.SetGlobal("vars", vars)
	rt.varsTable = vars

	// Action name constants as bare globals: move_down == "move_down"
	for _, name := range actionNames {
		L.SetGlobal(name, lua.LString(name))
	}

	// notify(message, level)
	L.SetGlobal("notify", L.NewFunction(rt.apiNotify))
	// now(format)
	L.SetGlobal("now", L.NewFunction(rt.nowFn))
}
```

Notes:
- `readTheme` still reads `api.vars.theme` — it must read the `theme` global
  and `vars` global instead. Update `readTheme` accordingly:
  - `rt.themeTable` = the `theme` global (same name, still `api.vars.theme`
    semantics for the table itself, but the config writes `theme.name = ...`).
  - `urgency_colors`, `collapse_depth`, `min_width`, `min_height` read from
    the `vars` global (config writes `vars.urgency_colors = {...}` etc.).
- The theme table's `__newindex` metatable (explicit-override tracking) stays;
  it is installed on the `theme` global.

- [ ] **Step 3: Update `readTheme` for the new globals**

`readTheme` currently does `L.GetField(rt.themeTable, k)` and
`L.GetField(rt.varsTable, ...)`. Since the tables are now mounted as `theme`
and `vars` globals, keep `rt.themeTable` / `rt.varsTable` pointing at those
tables (set in `installAPI`). Verify `readTheme` needs no logic change beyond
the table references — the field names (`primary`, `urgency_colors`,
`collapse_depth`, `min_width`) are unchanged.

- [ ] **Step 4: Update `config.lua` to the new syntax**

Replace every `api.` prefix:

- `api.vars.theme.primary` → `theme.primary`
- `api.vars.urgency_colors` → `vars.urgency_colors`
- `api.vars.collapse_depth` → `vars.collapse_depth`
- `api.keys.set("j", api.move_down)` → `keys.set("j", move_down)`
- `api.layouts.todo_layout = {...}` → `layouts.todo = {...}`
- `api.formatter.todos.status.add(...)` → `formatter.todos.status.add(...)`
- `api.bar.set({...})` → `bar.set({...})`
- `api.dashboard.set({...})` → `dashboard.set({...})`
- Inside formatters/bar functions, the `api` param is gone; they now receive
  `(value, model, theme)` where `theme` is the resolved theme table — keep the
  formatter signatures (`function(status, model, theme)`).
- `api.now` → `now`, `api.vars.theme` → `theme` inside bar widgets.
- `subscribe` / `timer` stay global (unchanged).

- [ ] **Step 5: Update Lua tests**

`internal/lua/lua_test.go` uses `EvalFile("../../config.lua")` and
`EvalFileWithCode(...)`. Update any test that references `api.` to the new
syntax. Add a negative test asserting the old `api.` form fails to evaluate:

```go
func TestOldAPISyntaxRejected(t *testing.T) {
	if _, err := EvalFileWithCode(`api.vars.theme.name = "nord"`); err == nil {
		t.Fatal("old api.* syntax must be rejected (no backward compat)")
	}
}
```

- [ ] **Step 6: Run tests + vet**

Run: `go test ./internal/lua/` then `go test ./internal/app/`
Expected: PASS (update app tests that embed `api.` config strings if any).

- [ ] **Step 7: Commit**

```bash
git add internal/lua/lua.go internal/lua/lua_test.go config.lua
git commit -m "feat: drop api. prefix from config surface"
```

---

### Batch B: Urgency floor + scroll threshold (requirements 3, 4)

**Files:**
- Modify: `internal/app/action.go` (urgency floor)
- Modify: `internal/app/view.go` (scroll floor)
- Test: `internal/app/app_test.go`, `internal/app/view_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `actionDecreaseUrgency` no-ops at urgency 1; layout floor is
  `(30, 6)`.

- [ ] **Step 1: Write failing urgency test**

Append to `internal/app/app_test.go`:

```go
func TestUrgencyFloorsAtOne(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	todo := m.selectedTodo()
	todo.Urgency = 1
	m.actionDecreaseUrgency(m)
	if todo.Urgency != 1 {
		t.Fatalf("urgency should floor at 1, got %d", todo.Urgency)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestUrgencyFloorsAtOne`
Expected: FAIL — currently decreases to 0.

- [ ] **Step 3: Fix `actionDecreaseUrgency`**

In `internal/app/action.go`:

```go
func (m *Model) actionDecreaseUrgency(_ *Model) tea.Cmd {
	t := m.selectedTodo()
	if t == nil {
		return nil
	}
	if t.Urgency > 1 {
		t.Urgency--
	}
	...unchanged save/bump...
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run TestUrgencyFloorsAtOne`
Expected: PASS

- [ ] **Step 5: Write failing scroll-threshold test**

Append to `internal/app/view_test.go`:

```go
func TestRendersBelowOldFloor(t *testing.T) {
	m := newTestApp(t)
	// Old floor was height 12; new floor is 6. Height 8 must render panes,
	// not the "too small" notice.
	m.width, m.height = 80, 8
	v := m.View()
	if strings.Contains(v, "Terminal size too small") {
		t.Fatalf("height 8 should render panes, got the too-small notice:\n%s", v)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestRendersBelowOldFloor`
Expected: FAIL — height 8 currently returns `layoutTooSmall`.

- [ ] **Step 7: Lower the floor**

In `internal/app/view.go`, `layoutMode` and `minSize`, change defaults:

```go
mw, mh := 30, 6
```

(Keep the config override path unchanged.)

- [ ] **Step 8: Verify no overflow at tiny heights**

Run `go test ./internal/app/ -run 'TestRendersBelowOldFloor|TestViewportScrollKeepsCursorVisible|TestDualPaneScrollsOnShortTerminal'`
Expected: PASS (scrollbar/clamp logic must hold at height 6-8).

- [ ] **Step 9: Run full app tests + vet**

Run: `go test ./...` and `go vet ./...`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/app/action.go internal/app/view.go internal/app/app_test.go internal/app/view_test.go
git commit -m "feat: urgency floor at 1, lower scroll threshold"
```

---

### Batch C: Dynamic columns + input contrast + multi-line description (requirements 2, 5, 6)

**Files:**
- Modify: `internal/app/renderers.go` (dynamic columns, multi-line desc)
- Modify: `internal/app/view.go` (continuation lines)
- Modify: `internal/app/input.go` (theme-colored textinput)
- Modify: `internal/app/app.go` (`expandedDesc` map)
- Modify: `internal/app/action.go` (`toggle_description_expand`)
- Modify: `internal/app/keymap.go` (bind `o`)
- Modify: `internal/lua/lua.go` (read `max_description_lines`)
- Test: `internal/app/*_test.go`, `internal/lua/lua_test.go`

**Interfaces:**
- Consumes: `appTheme()`, `formatTodoColumn`, `fitColumn`.
- Produces:
  - `Model.expandedDesc map[int64]bool`
  - `func (m *Model) toggleDescriptionExpand(_ *Model) tea.Cmd`
  - `formatTodoAligned` returns multi-line string when expanded
  - `lua.Runtime.MaxDescriptionLines int`
  - `newInput` → `m.newInput(placeholder)` (Model method)

- [ ] **Step 1: Read the current rendering path**

Read `internal/app/renderers.go` (`formatTodoColumn`, `formatTodoAligned`,
`columnWidths`, `fitColumn`) and `internal/app/view.go`
(`renderTodoPaneViewport` lines 460-500). Confirm the row pipeline:
`indent + marker + formatTodoAligned(...)` then selection + scrollbar.

- [ ] **Step 2: Write failing tests (multi-line + dynamic columns)**

Append to `internal/app/renderers_test.go`:

```go
func TestDynamicColumnsDropEmpty(t *testing.T) {
	// A todo with only a description should render just the description
	// (no padding for empty due/effort/recurrence columns).
	m := newTestApp(t)
	todo := &model.Todo{Description: "just a task", Pending: true}
	cols := []string{"status", "description", "due", "effort", "recurrence", "urgency"}
	// Build active columns: status (o) + description only.
	active := m.activeColumns(todo, cols)
	if len(active) != 2 {
		t.Fatalf("active columns = %v, want [status description]", active)
	}
}

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
```

- [ ] **Step 3: Run to verify they fail**

Run: `go test ./internal/app/ -run 'TestDynamicColumnsDropEmpty|TestToggleDescriptionExpand'`
Expected: FAIL — `activeColumns` undefined, `expandedDesc` undefined,
`toggleDescriptionExpand` undefined.

- [ ] **Step 4: Add `expandedDesc` + `toggleDescriptionExpand`**

In `internal/app/app.go`, add to `Model`:

```go
	// expandedDesc[id] = true when a todo's long description is expanded to
	// multiple lines (session-only).
	expandedDesc map[int64]bool
```

In `New()`, init `expandedDesc: map[int64]bool{}`.

In `internal/app/action.go`, add:

```go
func (m *Model) toggleDescriptionExpand(_ *Model) tea.Cmd {
	t := m.selectedTodo()
	if t == nil {
		return nil
	}
	m.expandedDesc[t.ID] = !m.expandedDesc[t.ID]
	m.BumpVersion()
	return nil
}
```

Register it in `defaultActions`:

```go
		"toggle_description_expand": wrap(m.toggleDescriptionExpand),
```

Bind in `internal/app/keymap.go`:

```go
		"o":   "toggle_description_expand",
```

- [ ] **Step 5: Implement `activeColumns`**

In `internal/app/renderers.go`:

```go
// activeColumns returns the columns whose cell is non-empty for this todo,
// so empty due/effort/recurrence columns don't consume width. status and
// description are always kept.
func (m *Model) activeColumns(t *model.Todo, cols []string) []string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if c == "status" || c == "description" {
			out = append(out, c)
			continue
		}
		if m.formatTodoColumn(c, t) != "" {
			out = append(out, c)
		}
	}
	return out
}
```

- [ ] **Step 6: Rework `formatTodoAligned` for dynamic columns**

`formatTodoAligned` must operate on `activeColumns` instead of the full `cols`.
`columnWidths`/`visibleColumns` must be called with the active list so widths
budget matches. The description column stays elastic and absorbs freed space.

```go
func (m *Model) formatTodoAligned(t *model.Todo, cols []string, paneW int) []string {
	active := m.activeColumns(t, cols)
	widths := m.columnWidths(active, paneW)
	parts := make([]string, 0, len(active))
	for _, col := range active {
		cell := m.formatTodoColumn(col, t)
		if w := widths[col]; w > 0 {
			cell = fitColumn(cell, w)
		}
		parts = append(parts, cell)
	}
	row := strings.Join(parts, " ")
	if m.expandedDesc[t.ID] {
		return m.expandDescription(row, t, widths["description"], active)
	}
	return []string{row}
}
```

Note: `columnWidths` currently takes `(pane int, paneW int)`. Change its
signature to `columnWidths(cols []string, paneW int)` and update callers in
`view.go` (`renderTodoPaneViewport`) to pass the active list. `visibleColumns`
also needs the active list (it can reuse `activeColumns`).

- [ ] **Step 7: Implement `expandDescription`**

In `internal/app/renderers.go`:

```go
// expandDescription splits an expanded long description across multiple
// terminal lines. The first line keeps the full row (other columns); each
// continuation line shows only the description, indented to its column, with
// other columns blank. maxLines bounds the total; 0 = no ellipsis.
func (m *Model) expandDescription(firstLine string, t *model.Todo, descW, maxLines int) []string {
	// word-wrap t.Description to descW columns
	wrapped := wrapDescription(t.Description, descW)
	out := []string{firstLine}
	rest := wrapped[1:]
	if maxLines > 0 && len(out)+len(rest) > maxLines {
		// keep firstLine + maxLines-1 continuation lines, last gets ellipsis
		keep := maxLines - 1
		rest = rest[:keep]
		out = append(out, rest...)
		out[maxLines-1] = truncateWithEllipsis(out[maxLines-1], descW)
		return out
	}
	out = append(out, rest...)
	return out
}

func wrapDescription(s string, w int) []string {
	// use ansi.Wordwrap or a simple width-aware wrap; ansi package is imported
	return ansi.Wordwrap(s, w)
}

func truncateWithEllipsis(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}
```

- [ ] **Step 8: Update `view.go` row rendering to handle multi-line rows**

In `renderTodoPaneViewport`, the row loop currently appends one line per todo.
Change it: `formatTodoAligned` now returns `[]string`; loop over them,
applying `renderSelectedRow` + scrollbar per line, and indent continuation
lines to the description column:

```go
	rows := m.formatTodoAligned(todos[i], cols, budget)
	for li, row := range rows {
		if li > 0 {
			// continuation: only the description column, indented past the
			// marker + other-column width offset.
			row = m.continuationRow(row, widths, cols)
		}
		if selected {
			row = m.renderSelectedRow(row, contentW)
		}
		row = m.renderExpandArrow(row, hasChildren, expanded) // arrows only on line 0
		if li == 0 && hasScrollbar {
			row = m.appendScrollbar(row, contentW, contentRows, thumb, i-lo)
		}
		lines = append(lines, row)
	}
```

`continuationRow` pads the row to the description column start (skip marker +
sum of leading fixed widths) so continuation text aligns under the first-line
description:

```go
func (m *Model) continuationRow(row string, widths map[string]int, cols []string) string {
	// description is the first non-fixed column after status; compute its
	// x-offset and pad the row to it.
	offset := 2 // marker
	for _, c := range cols {
		if c == "description" {
			break
		}
		offset += widths[c] + 1 // +1 gap
	}
	return strings.Repeat(" ", offset) + row
}
```

- [ ] **Step 9: Theme-colored textinput (requirement 5)**

In `internal/app/input.go`, change `newInput(placeholder string)` (free
function) to a `Model` method so it can style with the theme:

```go
// newInput returns a theme-colored textinput.
func (m *Model) newInput(placeholder string) textinput.Model {
	t := textinput.New()
	t.Placeholder = placeholder
	t.Prompt = ""
	t.CharLimit = 0
	th := m.appTheme()
	t.TextStyle = th.Style("secondary")
	t.PlaceholderStyle = th.Style("dim")
	t.Focus()
	return t
}
```

Update every caller (`StartEdit`, `StartSearch`, `StartSort`,
`StartConfirmPrompt`) from `newInput(...)` to `m.newInput(...)`.

- [ ] **Step 10: Lua `max_description_lines` (requirement 6 config)**

In `internal/lua/lua.go`, add to `Runtime`:

```go
	// MaxDescriptionLines bounds an expanded long description (default 3;
	// 0 = no ellipsis).
	MaxDescriptionLines int
```

In `readTheme` (after `collapse_depth` read):

```go
	if rt.varsTable != nil {
		if n, ok := L.GetField(rt.varsTable, "max_description_lines").(lua.LNumber); ok {
			rt.MaxDescriptionLines = int(n)
		}
	}
```

In `internal/app/app.go` `New()`, load it:

```go
	if luaCfg != nil {
		m.collapseDepth = luaCfg.CollapseDepth
		m.maxDescLines = luaCfg.MaxDescriptionLines
	}
```

Add `maxDescLines int` to `Model`; default 3 in `New` when 0.

- [ ] **Step 11: Update `config.lua` + docs for `max_description_lines`**

Add to `config.lua` (in the `vars.` section):

```lua
-- Max lines for an expanded long description (0 = never ellipsize). Press o
-- to toggle a todo's full description.
vars.max_description_lines = 3
```

- [ ] **Step 12: Update callers of the changed `columnWidths`/`visibleColumns`**

`internal/app/view.go` `renderTodoPaneViewport` currently calls
`m.columnWidths(PaneTodo, budget)` and `m.visibleColumns(PaneTodo, budget)`.
Update them to pass the active column list:

```go
	cols := m.ColumnLayout(PaneTodo)
	active := m.activeColumnsForPane(todos, cols)
	widths := m.columnWidths(active, budget)
	visible := m.visibleColumns(active, budget)
```

(Add `activeColumnsForPane` if needed to build the per-row list — but since
each row differs, compute `activeColumns` per row inside `formatTodoAligned`
and keep `visibleColumns` operating on the full config layout for the
width-budget drop logic. Decide the exact seam while implementing; the goal is
empty columns don't consume width per row.)

- [ ] **Step 13: Run full tests + vet**

Run: `go test ./...` and `go vet ./...`
Expected: PASS. Fix any tests that assumed fixed-width empty columns or
single-line rows.

- [ ] **Step 14: Commit**

```bash
git add internal/app internal/lua/lua.go internal/lua/lua_test.go config.lua
git commit -m "feat: dynamic columns, theme-colored input, multi-line descriptions"
```

---

### Batch D: Docs + final verification

**Files:**
- Modify: `docs/ARCHITECTURE.md`, `README.md` (config example + features)

- [ ] **Step 1: Update docs**

Update `docs/ARCHITECTURE.md` Lua section for the new config syntax and
`max_description_lines`. Update `README.md` config example (`api.` → new
forms) and add `o` to the usage/features.

- [ ] **Step 2: Full test + vet + build**

Run: `go test ./...`, `go vet ./...`, `go build ./...`
Expected: all pass.

- [ ] **Step 3: Smoke-check**

Build and run: confirm `o` expands a long description, empty columns collapse
per row, urgency floors at 1, height-8 terminal renders with scrolling, and
the input is readable on catppuccin_latte.

- [ ] **Step 4: Commit**

```bash
git add docs/ARCHITECTURE.md README.md
git commit -m "docs: new config syntax, o key, multi-line descriptions"
```
