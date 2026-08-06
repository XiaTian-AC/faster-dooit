# faster-dooit Architecture

This document explains how faster-dooit is built and why. It is the
**Explanation** + **Reference** half of the project documentation; for
usage, keybindings, and installation see the [README](../README.md).

- **Tutorial / How-to:** README (Quick Start), this file's per-module sections
- **Reference:** package APIs, schema, Lua surface, benchmarks
- **Explanation:** the performance model, rendering pipeline, completion
  cascade, theme resolution

## Overview

faster-dooit is a vim-style terminal todo manager written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea). It is a
**from-scratch, independent reimplementation** of the [dooit](https://github.com/dooit-org/dooit)
interaction design — the code, schema, and data format are incompatible and
unrelated.

The core design goal is **performance**: the original dooit is
Python/Textual/SQLAlchemy and pays a 1.9 s cold-start and per-frame tree
rebuilds. faster-dooit replaces those with a Go + SQLite stack that loads
10,000 todos in ~34 ms and renders only the viewport.

```
main.go ──► internal/app ──► internal/{model,store,theme}
                │
                ├── internal/lua ──► (config.lua evaluation + runtime API)
                └── internal/dateparse
```

## Package map

| Package | Responsibility |
|---|---|
| `main` | Flag parsing, DB/config path resolution, TUI bootstrap |
| `internal/app` | Bubble Tea model: input modes, key dispatch, actions, rendering, resize handling |
| `internal/model` | In-memory tree: `Workspace` / `Todo` structs, sort, completion cascade rules |
| `internal/store` | SQLite persistence: schema, load-all, save/delete, reorder |
| `internal/lua` | Sandboxed `config.lua` evaluation; exposes the `api` surface |
| `internal/theme` | Resolved color theme + built-in presets |
| `internal/dateparse` | Natural-language date parsing (`tomorrow`, `3d`, `next monday`) |

## The performance model (Explanation)

Three deliberate choices keep the app fast regardless of list size:

1. **Load once, write-through.** `store.LoadAll()` reads the entire tree into
   memory at startup. Edits write straight back to SQLite and re-render only
   the affected rows. There is **no database poller** — the original dooit
   polled the DB's mtime every second.
2. **Viewport-only rendering.** Rows are cached keyed by
   `(pane, model id, version)`. The 1-second clock tick is decoupled from
   this cache (it never bumps the version), so the clock doesn't thrash the
   row cache.
3. **Direct SQLite.** `modernc.org/sqlite` is pure Go (no CGO). A single
   connection with `SetMaxOpenConns(1)` + `_pragma=foreign_keys(1)` avoids
   pool-coherence and FK-lock pitfalls.

Result (AMD Ryzen 7 H 260 / Windows 11, `go test ./internal/app/ -bench .`):

| Benchmark | Result | Target |
|---|---|---|
| `BenchmarkStartup10k` (cold load 10k todos) | ~34 ms | < 200 ms |
| `BenchmarkVisibleTodos10k` (flatten 10k visible) | ~204 µs | < 10 ms |
| `BenchmarkUpdate` (keypress dispatch) | ~157 µs | < 1 ms |
| `BenchmarkRenderRow10k` (render one row) | ~233 µs | — |

## Input modes & dispatch (Reference)

Modes are a string enum in `internal/app/mode.go`:

- `NORMAL` — key dispatch through the key manager
- `INSERT` — inline editing (`description` / `due` / `effort` / `recurrence`)
- `SEARCH` — `/` filter overlay
- `SORT` — sort-field overlay
- `CONFIRM` — destructive-action confirmation (default no)

Key dispatch: `update.go` routes `KeyMsg` through the key manager
(`keymap.go`), which resolves single keys and chords (`gg`, `xx`) against a
binding table. Bindings come from `defaultKeyBindings()` or `config.lua`
(`api.keys.set`), whichever is configured.

Todo-only edits (`d`/`r`/`e`) are **no-ops** unless the todo pane is focused
and a todo is under the cursor; `enter`/`i` are no-ops on an empty pane.

## Rendering pipeline (Explanation)

`Model.View()` (in `internal/app/view.go`):

1. Choose layout: `layoutNormal` (side-by-side ≥100 cols) or `layoutStacked`.
2. Render each pane's visible rows into bordered boxes, viewport-scrolled so
   the cursor row stays visible.
3. Join panes + status bar; vertically center if the content is shorter than
   the terminal.
4. `fillBackground` pads every line to the terminal width and extends to the
   full terminal height, applying the theme's `Background` (skipped entirely
   when background is `transparent`).

A 200 ms `redrawTickMsg` forces a full repaint; Bubble Tea's renderer diffs
the output so only changed rows are written.

## Data model (Reference)

### SQLite schema

`workspace`: `id`, `order_index`, `description`, `is_root`, `parent_id`
(cascades delete). `todo`: `id`, `order_index`, `description`, `due` (RFC3339
text), `effort`, `recurrence` (duration in ns), `urgency`, `pending`,
`parent_workspace_id`, `parent_todo_id`. Foreign keys cascade deletes.

### In-memory tree

`model.Workspace` and `model.Todo` are loaded with `Parent` pointers wired up
by `LoadAll`. Ordering is `order_index` ascending, tie-broken by `id`.

### Completion cascade (Explanation)

The rules live in `internal/model/model.go`:

- **R1** — completing a todo completes its whole subtree
- **R2** — a parent auto-completes only when all its children complete
- **R3** — reopening any child reopens the parent
- **R4** — a recurring todo never completes: its `due` advances one period
  and it stays pending

### Sort semantics (Reference)

The composite key `pending → due → order_index` applies **only** to the
`pending` sort option. Other fields sort ascending with NULL `due` last;
`reverse` inverts the result.

## Lua configuration (Reference)

`internal/lua` evaluates `config.lua` in a sandbox (no `io`/`os`/`package`/
`debug`/`coroutine`; 1M-instruction limit). It exposes a closed subset of the
original Python API:

- `api.keys.set(key|{keys}, action)`
- `api.layouts.<name> = {col, ...}`
- `api.formatter.todos.<field>.add(fn)`
- `api.bar.set({fn, ...})`, `api.dashboard.set({line, ...})`
- `api.vars.theme`, `api.vars.urgency_colors`, `api.vars.min_width/height`
- `api.notify`, `api.now`, globals `subscribe` / `timer`

An invalid file prints a `file:line` error and exits.

## Theme resolution (Explanation)

Colors are resolved by `internal/theme`:

1. `config.lua` sets `api.vars.theme.name` (default `nord`); a Lua
   `__newindex` metatable records which color fields were explicitly assigned.
2. `theme.Resolve(name, explicit)` starts from the named preset and applies
   each explicitly-assigned color as an override — order-independent.
3. Unknown names are config errors listing the available presets.

Built-in presets: `nord`, `catppuccin_mocha`, `catppuccin_latte`, `dracula`,
`gruvbox_dark`, `solarized_light`, `tokyo_night`. Twelve semantic colors:
8 base (`primary`, `secondary`, `background`, `background1`, `green`,
`yellow`, `orange`, `red`) plus `dim`, `selection`, `border_focused`,
`border_unfocused`, and `urgency_colors`. Setting
`api.vars.theme.background = "transparent"` disables the full-screen fill.

## Default paths

| Item | Windows | Linux/macOS |
|---|---|---|
| Database | `%APPDATA%\faster-dooit\todo.db` | `$XDG_CONFIG_HOME/faster-dooit/todo.db` |
| Config | `~/.config/faster-dooit/config.lua` | `~/.config/faster-dooit/config.lua` |

Both are overridable with `--db <path>` and `-c, --config <path>`.

## Testing

- `go test ./...` — unit + parity + e2e
- `go vet ./...`
- `go test ./internal/app/ -bench . -benchmem` — performance gates

Design specs and implementation plans live under
[`docs/superpowers/`](superpowers/).
