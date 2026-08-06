# Mouse Support — Design Spec

Date: 2026-08-06
Status: Approved (design)
Scope: `internal/app` (new `mouse.go`, `main.go`), docs, tests. **Design only — implementation deferred.**

## Goal

Add mouse support to the TUI: scroll wheel, click-to-select, a right-click
context menu for editing todo fields, completion toggle, and priority
adjustment — without breaking the existing keyboard-first interaction.

## Technical foundation

- Bubble Tea `WithMouseCellMotion()` enables click + wheel events in SGR
  extended mode (good terminal support, no hover needed). Add to
  `tea.NewProgram(model, tea.WithAltScreen())` in `main.go`.
- `MouseMsg` carries `X`, `Y`, `Button` (left/right/wheel-up/wheel-down),
  `Action` (press/release).
- Dispatched in `Model.Update` alongside existing `KeyMsg` handling.

## Architecture

New file `internal/app/mouse.go` with pure, unit-testable helpers:

### Coordinate mapping (`internal/app/mouse.go`)

```
screen (X,Y)
  → layoutMode() + pane rectangles (dual-pane or stacked)
  → (pane, pane-local row)   [border + padding subtracted]
  → (pane-local row) - title row + scroll offset
  → global item index
```

- `paneRectAt(x, y) (pane, ok)` — which pane (or neither) contains a point,
  for both `layoutDual` and `layoutStacked`, accounting for the border box.
- `itemIndexAt(y, pane, scroll)` — pane-local content row from a screen Y:
  subtract the pane's top, the title row, add scroll.
- `visibleRowRange(pane)` — the scroll window `[lo, hi)` for hit testing.

This is pure arithmetic and must be tested without any rendering.

### Wheel

- `MouseWheelUp` / `MouseWheelDown` scroll only the pane the pointer is over
  (confirmed decision), clamped by the existing `clampScroll`.
- Wheel does NOT change focus (scroll without focus jump).

### Left click

- Clicking a todo/workspace row selects it: sets the pane's cursor to the
  mapped index and `SetFocus` to that pane.
- Clicking a pane's title / border / empty area focuses that pane without
  moving the cursor.
- Clicking the status bar or outside any pane is ignored.

### Right-click context menu

Trigger: right-click on a todo row (selects the row first, then opens the
menu below the cursor row).

Menu interaction: **keyboard navigation AND mouse click both supported**
(confirmed decision):
- Keyboard: `j`/`k` move selection, `Enter` confirm, `Esc` close.
- Mouse: clicking a menu item executes it; clicking outside closes.

Menu items (map onto the existing action system):

| Item | Action |
|---|---|
| 编辑描述 | `edit_description` |
| 编辑截止日期 | `edit_due` |
| 编辑循环 | `edit_recurrence` |
| 编辑耗时 | `edit_effort` |
| 完成 / 重开 | `toggle_complete` |
| 优先级 ▲/▼ | submenu: increase / decrease urgency |
| 删除 | `delete` (with confirm) |

Priority submenu: choosing "优先级" swaps the menu items to
`↑ 提升` / `↓ 降低` / `返回`, navigable by keyboard or mouse.

## State management

- New mode `ModeContextMenu` + a menu state struct (items, selected index,
  target item reference).
- `Update` routes `MouseMsg` to: wheel scroll / row click / menu click /
  menu dismissal.
- Opening the menu while another mode is active: only from NORMAL (or
  SEARCH with results); other modes ignore right-click.
- Menu open while the user scrolls or clicks outside: close the menu and
  apply the scroll/click.

## Rendering

- Menu drawn below the triggered row, width fitting the longest item, with a
  border.
- Selected item highlighted (`Selection` background + `primary` foreground).
- Menu overlays the content rows beneath it (normal TUI behavior).

## Testing

- `internal/app/mouse_test.go` — pure coordinate mapping (both layouts,
  scroll offset, title/border offsets), wheel clamp, click→select+focus,
  menu open/navigate/click/close, priority submenu up/down.
- Existing keyboard tests must stay green (no regression to keyboard path).

## Error handling

- Clicks on invalid regions (outside panes, empty panes, borders) are
  silently ignored — no crash, no state change.
- Deletion from the menu still goes through the existing confirm dialog.

## Out of scope (deliberately deferred)

- Hover highlighting (`WithMouseAllMotion`) — needs more terminal support;
  `WithMouseCellMotion` is enough for the requested interactions.
- Drag / click-to-scrollbar.
- Clicking menu via double-click.
