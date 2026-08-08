# UX Polish Batch — Design Spec

Date: 2026-08-08
Status: Approved (design)
Scope: `internal/app` (view/renderers/action/input/keymap), `internal/lua`, `config.lua`, docs, tests

## Overview

Six UX improvements across config ergonomics, dynamic columns, an urgency
bug, scroll threshold, edit contrast, and multi-line descriptions.

## Confirmed decisions

1. **Config simplification**: drop the `api.` prefix entirely — no backward
   compat. New forms: `theme.name`, `keys.set(...)`, `layouts.todo`, etc.
2. **Dynamic per-row columns**: a row with no due/recurrence/effort omits that
   column entirely (no width, no gap); freed width goes to description.
3. **Urgency floor**: cannot decrease below 1. `-` on urgency 1 is a no-op.
4. **Scroll threshold**: the "too small" threshold drops (default height well
   below 12); short terminals keep rendering with per-pane scrolling instead
   of a full-stop notice.
5. **Edit contrast**: textinput gets the theme's `primary` (or `secondary`)
   foreground so light themes stay readable.
6. **Multi-line description**: `max_description_lines` config (default 3).
   Collapsed: column-truncated with `…`. Pressing `o` toggles the full
   multi-line description (up to the config max); beyond it, `…`. `0` =
   never trigger the ellipsis (always show full description).

---

## 1. Config simplification

### Current

`config.lua` writes `api.vars.theme.name`, `api.keys.set("j", api.move_down)`,
`api.layouts.todo_layout = {...}`, `api.formatter.todos.status.add(fn)`,
`api.bar.set({...})`, `api.dashboard.set({...})`, `api.notify`, `api.now`,
`subscribe(...)`, `timer(...)`.

### Target (no backward compat)

- `api.vars.theme.name` → `theme.name`
- `api.vars.urgency_colors` → `urgency_colors`
- `api.vars.collapse_depth` → `collapse_depth`
- `api.vars.min_width/min_height` → `min_width/min_height`
- `api.keys.set(...)` → `keys.set(...)` (actions stay `api.move_down`? see
  note)
- `api.layouts.todo_layout/workspace_layout` → `layouts.todo/workspace`
- `api.formatter.todos.<field>.add(...)` → `formatter.todos.<field>.add(...)`
- `api.bar.set(...)` → `bar.set(...)`
- `api.dashboard.set(...)` → `dashboard.set(...)`
- `api.notify(...)` / `api.now(...)` → `notify(...)` / `now(...)`
- `subscribe` / `timer` unchanged (already global)

**Action names in `keys.set`**: keep them as `"move_down"` strings — the
`api.move_down` constants are convenient but the whole `api` table is going
away. `keys.set("j", "move_down")`.

**Implementation**: rewrite `installAPI` in `internal/lua/lua.go` to expose
bare globals (`theme`, `keys`, `layouts`, `formatter`, `bar`, `dashboard`,
`notify`, `now`) instead of under an `api` table. `actionNames` constants
currently hang off `api`; expose them as globals too (`move_down` etc.) so
`keys.set("j", move_down)` still reads well. Update `config.lua` and docs.

---

## 2. Dynamic per-row columns

### Current

`formatTodoAligned` iterates a fixed `cols` list; `fitColumn` pads empty cells
to the fixed column width. Empty due/effort/recurrence still consume their
column width + a gap.

### Target

Per row, drop columns whose formatted cell is empty, then lay out only the
non-empty columns. Freed width (from dropped columns and gaps) goes to the
elastic description column.

**Implementation**:
- In `formatTodoAligned`, build `activeCols` = cols whose `formatTodoColumn`
  returns a non-empty cell (before padding).
- Compute widths only for `activeCols`; description gets the elastic budget
  (all remaining width).
- `columnWidths` and `visibleColumns` operate on `activeCols` (description
  still elastic; the min-desc-width / drop logic still applies to fixed cols).
- Workspaces: only `description` column, so unaffected.

---

## 3. Urgency floor of 1

### Current

`actionDecreaseUrgency` allows `Urgency > 0` → 0. Formatter shows nothing at
0.

### Target

`Urgency` cannot go below 1. `actionDecreaseUrgency` becomes a no-op when
`Urgency <= 1`. Seed default urgency stays 1.

---

## 4. Scroll threshold

### Current

`layoutMode` returns `layoutTooSmall` when `height < mh` (default 12) or
`width < mw` (default 40). `View()` then renders only a "Terminal size too
small" notice — the panes never render, so short terminals can't scroll.

### Target

Lower the effective floor so short terminals keep rendering with the existing
per-pane viewport scrolling. Default floor: `height >= 6` and `width >= 30`
(pane + border still fit). Below that, keep the notice. The floor stays
configurable via `min_width/min_height`.

**Implementation**:
- Change defaults in `layoutMode`/`minSize` from `(40,12)` to `(30,6)`.
- Ensure `renderStacked` and `renderDualPane` already clamp scroll for tiny
  heights (they do via `clampScroll`); verify no overflow at height 6-7.

---

## 5. Edit input contrast

### Current

`newInput` uses bubbles `textinput` defaults (terminal foreground). Light
themes (e.g. catppuccin_latte) render typed text at low contrast.

### Target

Bind the textinput's text/placeholder colors to the theme:
- Text: theme `secondary` (readable on both light and dark).
- Placeholder: theme `dim`.

**Implementation**: `newInput(placeholder)` becomes a `Model` method (needs
`appTheme()`) setting `t.TextStyle` / `t.PlaceholderStyle`. Update all callers.

---

## 6. Multi-line description

### Current

`fitColumn` truncates the description cell with `…` at the column width; a
row is always one terminal line.

### Target

- Config `max_description_lines` (default 3, `0` = no ellipsis).
- **Collapsed** (default): description truncated with `…` as today.
- **Expanded** (toggle `o`): description wraps across up to
  `max_description_lines` terminal lines (word-wrap at display width).
- Beyond the max: truncate with `…`.
- `max_description_lines = 0`: expanded description is never truncated
  (always full, may push the row beyond max lines).
- Toggle per-todo, session-only (like fold state).

**Layout**: first line = all columns (status/due/effort/recurrence +
description first line); continuation lines show only the description,
indented to the description column, other columns blank. Selection highlight
and scrollbar span all continuation lines.

**Implementation**:
- Add `expandedDesc map[int64]bool` to `Model` (session state).
- New action `toggle_description_expand` bound to `o`.
- `formatTodoAligned` returns a multi-line string for an expanded todo whose
  description exceeds the column width; `renderTodoPaneViewport` splits the
  returned string and appends each line (applying selection/scrollbar per
  line).
- Lua: read `api.vars.max_description_lines` (default 3) into `Runtime`.

---

## Files touched

- `internal/lua/lua.go` — config surface rewrite (bare globals), read
  `max_description_lines`
- `internal/app/app.go` — `expandedDesc` map
- `internal/app/action.go` — urgency floor, `toggle_description_expand`
- `internal/app/keymap.go` — bind `o`
- `internal/app/input.go` — theme-colored textinput
- `internal/app/renderers.go` — dynamic columns, multi-line description
- `internal/app/view.go` — per-line continuation rendering, scroll threshold
- `config.lua`, docs, tests

## Testing

- Config: bare-global forms load; old `api.` forms rejected (no compat)
- Dynamic columns: empty due row omits the column; description widens
- Urgency: `-` at urgency 1 is a no-op; never drops below 1
- Scroll: renders (not notice) at height 6; panes scroll; notice below floor
- Input: textinput carries theme secondary foreground
- Multi-line: collapsed truncates `…`; `o` expands to N lines; `0` disables
  ellipsis; continuation lines render with highlight/scrollbar
