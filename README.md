# faster-dooit

A from-scratch, high-performance reimplementation of [dooit](https://github.com/dooit-org/dooit) — the vim-style TUI todo manager — in **Go + Bubble Tea**. Same two-pane tree, six modes, vim keybindings, natural-language dates, recurrence, search/sort/clipboard, status bar and Lua extension surface, without the Python startup and per-frame cost.

> **Status**: core parity complete. See [Parity matrix](#parity-matrix) for what is and isn't implemented.

## Why a rewrite

The original is Python/Textual/SQLAlchemy. Its felt slowness comes from the whole tree being rebuilt on every refresh, N+1 queries, a 1-second DB-mtime poller, and the reactive framework overhead.

faster-dooit removes those root causes:

- **No DB poller.** The whole database is read into memory once at startup (`LoadAll`); edits write through to SQLite immediately and re-render only the affected rows.
- **Viewport-only rendering.** The `Model` keeps a row cache invalidated by a narrow dirty set; the 1-second clock tick is decoupled from it so the clock doesn't thrash the cache.
- **Direct SQLite.** `modernc.org/sqlite` (pure Go, no CGO), single connection, `SetMaxOpenConns(1)` + `_pragma=foreign_keys(1)`.

### Benchmarks

Machine: AMD Ryzen 7 H 260 / Windows 11. `go test ./internal/app/ -bench . -benchmem`.

| Benchmark | Result | Target | Status |
|---|---|---|---|
| `BenchmarkStartup10k` (cold load 10k todos) | ~34 ms | < 200 ms | ✅ |
| `BenchmarkVisibleTodos10k` (flatten 10k visible) | ~204 µs | < 10 ms | ✅ |
| `BenchmarkUpdate` (keypress dispatch) | ~157 µs | < 1 ms | ✅ |
| `BenchmarkRenderRow10k` (render one row) | ~233 µs | — | ✅ |

Original baseline (this machine): importing `dooit.ui.tui` (Textual + SQLAlchemy, before any render) ≈ **1.9 s**. faster-dooit's entire cold-start data path is ~34 ms.

## Install & build

Requirements: Go ≥ 1.22 (no CGO, no gcc). Then:

```powershell
cd faster-dooit
go build -o faster-dooit.exe .
.\faster-dooit.exe
```

The database is created on first run at:

- **Windows**: `%APPDATA%\faster-dooit\todo.db`
- **Linux/macOS**: `$XDG_CONFIG_HOME/faster-dooit/todo.db` (falls back to `~/.config`)

Config file (see below) lives next to it at `config.lua`. Override either with `--db <path>` / `-c, --config <path>`.

## Config (`config.lua`)

`faster-dooit` is configured in Lua. The file mirrors the surface of the original `default_config.py` through a deliberately closed API subset — **keys / layouts / formatter / bar / dashboard / subscribe / timer / `vars.theme` / notify**. `api.css` and the Python plugin manager are intentionally absent.

If no `config.lua` is found, built-in defaults are used. An invalid file prints a `file:line` Lua error and exits.

### Example 1 — remap keys

```lua
api.keys.set("i", api.add_sibling)     -- make `i` add instead of edit
api.keys.set("x", api.toggle_complete) -- single-key complete
```

### Example 2 — add a formatter

```lua
api.formatter.todos.description.add(function(description, model, theme)
  if description and #description > 20 then
    return { text = description:sub(1, 19) .. "…", style = theme.yellow }
  end
  return { text = description, style = "" }
end)
```

### Example 3 — custom theme

```lua
api.vars.theme.primary   = "#FFB86C"
api.vars.theme.secondary = "#6272A4"
api.vars.theme.background = "#282A36"
api.vars.theme.green     = "#50FA7B"
api.vars.theme.red       = "#FF5555"
```

### Keybindings (defaults)

| Key | Action | Key | Action |
|---|---|---|---|
| `j` / `k` | move down / up | `i` | edit description |
| `d` / `r` / `e` | edit due / recurrence / effort | `a` / `A` | add sibling / child |
| `z` / `Z` | expand / expand parent | `gg` / `G` | go to top / bottom |
| `J` / `K` | shift down / up | `xx` | delete (confirm) |
| `y` / `Y` | copy description / model | `p` / `P` | paste below / above |
| `c` | toggle complete | `=` / `+` / `-` / `_` | increase / decrease urgency |
| `/` | search | `?` | help |
| `tab` / `h` / `l` | switch pane | `enter` | edit description |
| `ctrl+s` | sort | `ctrl+q` / `ctrl+c` | quit |

## Parity matrix

### Date parsing

`internal/dateparse` mirrors the original `test_date_parse.py` cases and more.

**Supported:**

| Form | Example |
|---|---|
| ISO date | `2020-01-01` |
| ISO date + time | `2020-01-01 14:30`, `2020/01/01 14:30` |
| Slash date | `2020/01/01` |
| English month | `july 1 2034`, `jan 1` (assumes current year) |
| Relative | `today`, `tomorrow` |
| Shorthand | `3d`, `2w`, `1h` |
| Next weekday | `next monday`, `next fri` (full + 3-letter names) |
| In-phrase | `in 2 days`, `in 3 weeks`, `in 4 hours` |

**Not supported** (by design — the original's ambiguous forms are out of scope): bare weekday without `next` (`monday` alone), natural-language ranges, and locale-specific month names.

### Feature checklist

| Feature | Status | Notes |
|---|---|---|
| Two-pane tree (workspace + todo) | ✅ | custom flattening, expand/collapse |
| Six modes (normal/insert/search/sort/confirm/help) | ✅ | |
| Vim keybindings + chords (`gg`, `xx`) | ✅ | prefix-matching state machine |
| CRUD (add sibling/child, edit 4 fields, delete w/ confirm) | ✅ | new items get a default name + inline edit on the cursor row |
| Urgency (1–5) with per-level colors | ✅ | cap 5; colors from `api.vars.urgency_colors` (built-in default when absent) |
| Vertical centering | ✅ | panes center in the terminal when content fits |
| Natural-language dates | ✅ | see table above |
| Recurrence + completion cascade | ✅ | R1–R4 rules from `update_hooks.py`; recurring todos never complete |
| Search filter | ✅ | case-insensitive, matches description |
| Sort (`ctrl+s`) | ✅ | `pending` composite key `pending→due→order_index`; other fields ascending + `nulls_last`; `reverse` inverts |
| Clipboard copy / paste | ✅ | copy desc or model; paste below/above clones the subtree; cross-type paste notifies an error |
| Status bar (mode / clock / user) | ✅ | Lua `api.bar` |
| Dashboard welcome pane | ✅ | Lua `api.dashboard` |
| Help screen | ✅ | |
| Lua extensions (subscribe/timer/formatter/bar/theme/layout/keys) | ✅ | closed API subset |
| **`poll_dooit_db`** (external-edit hot reload) | ❌ | **Single-process tradeoff, deliberately not implemented.** The DB is loaded once at startup and written through directly; edits from another process are not picked up. Use one faster-dooit process. |

## Moving from dooit

### Data

**Old dooit data is not migrated.** faster-dooit uses a fresh, intentionally simpler schema (`workspace` + `todo` with `order_index`); there is no import from the SQLAlchemy database. Start a new DB, or re-enter your todos.

### Python API → Lua

| dooit Python API | faster-dooit Lua |
|---|---|
| `api.keys.set(...)` | `api.keys.set(...)` |
| `api.vars.theme.*` | `api.vars.theme.*` |
| `api.layouts.*` | `api.layouts.*` |
| `api.formatter.todos.<field>.add(fn)` | `api.formatter.todos.<field>.add(fn)` |
| `api.bar.set(...)` | `api.bar.set(...)` |
| `api.dashboard.set(...)` | `api.dashboard.set(...)` |
| `subscribe(event, fn)` | `subscribe(event, fn)` |
| `timer(seconds, fn)` | `timer(seconds, fn)` |
| `api.css` / plugin manager | ❌ absent |
| Python classes / arbitrary imports | ❌ closed sandbox (gopher-lua) |

### Key differences

- **Completion cascade**: completing a todo completes its whole subtree; a parent auto-completes only when all its children complete; reopening any child reopens the parent; a recurring todo advances `due` and stays pending (never completes).
- **Sort semantics**: the `pending` composite key applies only to the `pending` option; other fields sort ascending with NULL `due` last.
- **Editing errors**: an invalid due/effort/urgency/recurrence keeps you in the input field (the original exits editing and keeps the old value).

## Development

```powershell
go test ./...          # unit + parity + e2e
go vet ./...
go test ./internal/app/ -bench . -benchmem   # perf gates
```

Project docs: design spec and implementation plan live under [`docs/superpowers/`](docs/superpowers/).
