# faster-dooit

**English** | [简体中文](README.zh-CN.md)

> **Disclaimer: This project is NOT affiliated with the official [dooit](https://github.com/dooit-org/dooit) project.**
> faster-dooit is a **from-scratch**, independent vim-style terminal todo manager inspired by dooit's interaction design, but its code, architecture, and data format are incompatible and unrelated.
> This project is **AI-assisted** and created by a **middle-school student hobbyist developer**. Features may be rough; Issues/PRs are welcome, but please don't expect enterprise-grade quality.

A from-scratch, high-performance vim-style TUI todo manager in **Go + Bubble Tea** — inspired by [dooit](https://github.com/dooit-org/dooit)'s two-pane tree, six modes, vim keybindings, natural-language dates, recurrence, search/sort/clipboard, status bar and Lua extension surface — without the Python startup and per-frame cost.

## Install

### Scoop (recommended — personal bucket)

```powershell
# One-time: add my personal bucket
scoop bucket add faster-dooit https://github.com/XiaTian-AC/scoop-faster-dooit
scoop install faster-dooit
```

The command is `fdooit` (not `faster-dooit`).
The personal bucket manifest is auto-updated by the release CI (tag a release and it's fully automatic).

### Winget

```powershell
winget install faster-dooit
```

> **Status**: official winget-pkgs PR submitted (pending Microsoft review). Until it's approved, use the Scoop personal bucket or build from source.

### Official Scoop Extras

PR submitted (ScoopInstaller/Extras). Note: Extras requires **100 stars / 50 forks** for GitHub-hosted packages as a community gate; new projects usually don't meet it, so the personal bucket may remain the primary channel. See [RELEASING.md](docs/RELEASING.md).

### Build from source

Requirements: Go ≥ 1.22 (no CGO, no gcc).

```powershell
git clone https://github.com/XiaTian-AC/faster-dooit.git
cd faster-dooit
go build -o fdooit.exe .
.\fdooit.exe
```

## Quick Start (Tutorial)

1. On launch, the left pane is **Workspaces**, the right pane is **Todos**
2. Press `a` to add a todo at the cursor, type a name, press Enter
3. Press `c` to toggle completion, `d` to set a due date (supports `tomorrow`, `3d`, `next monday`, etc.)
4. Press `?` for help, `/` to force a terminal redraw

## Features

### Why a rewrite

The original dooit is Python/Textual/SQLAlchemy. Its felt slowness comes from rebuilding the whole tree on every refresh, N+1 queries, a 1-second DB-mtime poller, and reactive-framework overhead.

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

Original baseline (this machine): `import dooit.ui.tui` (Textual + SQLAlchemy, before any render) ≈ **1.9 s**. faster-dooit's entire cold-start data path is ~34 ms.

## Data & Config (Reference)

The database is created on first run at:

- **Windows**: `%APPDATA%\faster-dooit\todo.db`
- **Linux/macOS**: `$XDG_CONFIG_HOME/faster-dooit/todo.db` (falls back to `~/.config`)

The config file `config.lua` lives in the same directory. Override either with `--db <path>` / `-c, --config <path>`.

### Keybindings (defaults)

| Key | Action | Key | Action |
|---|---|---|---|
| `j` / `k` | move down / up | `i` | edit description |
| `d` / `r` / `e` | edit due / recurrence / effort | `a` / `A` | add sibling / child |
| `z` / `Z` | expand / expand parent | `gg` / `G` | go to top / bottom |
| `J` / `K` | shift down / up | `xx` | delete (confirm) |
| `y` / `Y` | copy description / model | `p` / `P` | paste below / above |
| `c` | toggle complete | `=` / `+` / `-` / `_` | increase / decrease urgency |
| `/` | redraw (force refresh) | `S` / `?` | search / help |
| `tab` / `h` / `l` | switch pane | `enter` | edit description |
| `ctrl+s` | sort | `ctrl+q` / `ctrl+c` | quit |

All keybindings can be remapped from `config.lua` via `api.keys.set(key, api.<action>)`:

```lua
api.keys.set("i", api.add_sibling)     -- make i add instead of edit
api.keys.set("x", api.toggle_complete) -- single-key complete
```

### Search mode (How-to)

- `/` opens search; type a term and press **Enter** to apply the filter and move the cursor to the results (the status bar shows `search: <term>`)
- Operate normally on the results (`j`/`k` to move, `c` to complete, `A` to add a child)
- **Esc** clears the filter and restores the full list
- Pressing `a`/`A` while searching applies the filter and starts adding directly (new items won't be hidden by the filter)

### Lua config

The config surface is a deliberately closed subset of the original Python API: **keys / layouts / formatter / bar / dashboard / subscribe / timer / `vars.theme` / notify**. `api.css` and the plugin manager are intentionally absent. If no `config.lua` is found, built-in defaults are used; an invalid file prints a `file:line` Lua error and exits.

**Theme example:**

```lua
api.vars.theme.primary   = "#FFB86C"
api.vars.theme.secondary = "#6272A4"
api.vars.theme.background = "#282A36"
```

**Formatter example:**

```lua
api.formatter.todos.description.add(function(description, model, theme)
  if description and #description > 20 then
    return { text = description:sub(1, 19) .. "…", style = theme.yellow }
  end
  return { text = description, style = "" }
end)
```

## Design Notes (Explanation)

### Completion cascade

- R1: completing a todo completes its whole subtree
- R2: a parent auto-completes only when all its children complete
- R3: reopening any child reopens the parent
- R4: a recurring todo never completes — due advances by one period and it stays pending

### Sort semantics

- The `pending` composite key `pending→due→order_index` applies only to the `pending` option
- Other fields sort ascending with NULL `due` last; `reverse` inverts

### Differences from dooit

- Old dooit data is **not migrated** (fresh schema)
- Editing errors keep you in the input field (the original exits and keeps the old value)
- **Not implemented**: `poll_dooit_db` external-edit hot reload (a single-process tradeoff, deliberately omitted)

## Migrating from dooit

Data is not migrated; start from an empty DB. The Python API → Lua correspondence is mostly same-named (see the Lua config section).

## Development (Contribution)

```powershell
go test ./...        # unit + parity + e2e
go vet ./...
go test ./internal/app/ -bench . -benchmem   # perf gates
```

Project docs: design spec and implementation plan live under [`docs/superpowers/`](docs/superpowers/).

## Credits

- [dooit](https://github.com/dooit-org/dooit) — inspiration for the interaction design
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) / [lipgloss](https://github.com/charmbracelet/lipgloss) — Go TUI rendering stack
- This project's code is AI-assisted; feedback welcome
