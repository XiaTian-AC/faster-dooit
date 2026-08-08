# Collapse / Expand — Design Spec

Date: 2026-08-06
Status: Approved (design)
Scope: `internal/app` (view/renderers/action), `internal/lua` (config), `internal/theme`, tests, docs

## Goal

Make tree folding actually work. The expand/collapse skeleton already exists
(`expanded map[int64]bool`, `z`/`Z` keybindings, `toggle_expand` /
`toggle_expand_parent` actions) but is disabled by `|| true` in the tree
traversals. Add a configurable default collapse depth, visual expand arrows,
and cursor repositioning on collapse.

## Confirmed decisions

1. **Default depth collapse** via `api.vars.collapse_depth` (default **0** =
   everything expanded, matching current behavior). Setting 0 always expands
   unless manually collapsed.
2. **Two trees count independently**: workspaces (root = 0, first level = 1)
   and todos (workspace's top-level todos = 0, children = 1).
3. **Default depth affects only the initial state**; manual toggles are not
   persisted (in-memory only, reset on restart to the default depth).
4. **Visual indicator**: collapsed nodes show `▸`, expanded show `▾`
   (theme primary color), only for nodes that have children.
5. **Cursor**: after a collapse, if the cursor was on a now-hidden child, it
   jumps up to the collapsing node itself.
6. **Keybindings stay** `z` (toggle self) / `Z` (toggle parent) — already bound.

## Mechanism

### `isExpanded(id, depth)` helper

Replace the `|| true` in `VisibleWorkspaces` and `visibleTodos` with a helper
that consults manual overrides first, then the default depth:

```go
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

`VisibleWorkspaces` / `visibleTodos` use `m.isExpanded(ws.ID, ws.NestLevel())`
/ `m.isExpanded(t.ID, t.NestLevel())` instead of `|| true`.

### Config: `collapse_depth`

- Lua: read `api.vars.collapse_depth` (integer, default 0) into `Runtime`.
- App: `collapseDepth()` returns the configured value (0 without config).

### Visual arrows

```go
// renderExpandArrow appends ▸ (collapsed) or ▾ (expanded) to a row when the
// node has children. Arrows take 2 columns (space + glyph); pane width
// budgeting accounts for them.
func (m *Model) renderExpandArrow(row string, hasChildren, expanded bool) string
```

- Only nodes with children get an arrow.
- Arrow appended at the row tail, before the scrollbar column.
- Theme `primary` color.

### Cursor repositioning

After a collapse (toggle to collapsed), if the cursor pointed at a child of
the collapsed node, move the cursor to the collapsed node itself:

```go
func (m *Model) clampAfterCollapse(pane int, collapsedID int64)
```

Finds `collapsedID` in the visible list; if it is no longer visible (its
parent collapsed it), walk up to the nearest visible ancestor and place the
cursor there. Works for both panes.

## Files touched

- `internal/app/app.go` — `isExpanded`, `collapseDepth`, remove `|| true`
- `internal/app/action.go` — call `clampAfterCollapse` after a collapse
- `internal/app/renderers.go` — `renderExpandArrow` + pane width budgeting
- `internal/app/view.go` — wire arrows into row rendering
- `internal/lua/lua.go` — read `collapse_depth`
- tests: `internal/app/*_test.go`, `internal/lua/lua_test.go`

## Testing

- `isExpanded`: manual override wins over depth; default-depth logic (depth 0
  expanded with collapse_depth 0; depth 1 collapsed with collapse_depth 0)
- Collapsed nodes absent from `VisibleWorkspaces` / `visibleTodos`
- Cursor on a collapsed node's child jumps to the node after collapse
- `collapse_depth` config read (default 0, `=1` collapses depth 1)
- Arrow rendering: collapsed `▸`, expanded `▾`, leaf no arrow
- Existing keyboard tests stay green
