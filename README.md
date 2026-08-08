# faster-dooit

**English** | [简体中文](README.zh-CN.md)

> _Not affiliated with the official [dooit](https://github.com/dooit-org/dooit) project — this is an independent from-scratch port._

Your todo app takes two seconds to open and stutters on every keystroke. **faster-dooit** is a vim-style terminal todo manager that cold-loads 10,000 todos in **~34 ms** and never polls the database — so it feels instant, even with your whole life in it.

```
scoop bucket add faster-dooit https://github.com/XiaTian-AC/XiaTian-AC-bucket
scoop install faster-dooit
fdooit
```

```
┌─ Workspaces ─────────────────────┐  ┌─ Todos ──────────────────────────────┐
│  Work                            │  │  o  finish release notes      @today │
│  Personal                        │  │  o  write the spec                   │
└──────────────────────────────────┘  └──────────────────────────────────────┘
```

`j`/`k` to move, `a` to add, `d` to set a due date — `d 3d` or `d next monday` — `c` to complete, `?` for help. It's vim, for your todos.

## Why not the thing you already use?

| Instead of | Why faster-dooit |
|---|---|
| **dooit** (Python) | 1.9 s cold start vs **34 ms**; it rebuilds the whole tree on every frame, hits N+1 queries, and polls the DB every second |
| **A web app / mobile list** | No browser, no sync, no cloud — your tasks live in a local SQLite file you own |
| **taskwarrior / plain txt** | A two-pane tree with nesting, completion cascades, and a status bar — without leaving your keyboard |

## Features

| Pain | faster-dooit |
|---|---|
| Waiting seconds for your todo tool to open | 10k todos load in **~34 ms** (original dooit: ~1.9 s on this machine) |
| Every keystroke re-renders the whole tree | Viewport-only rendering — a row cache invalidated by a narrow dirty set |
| A 1-second DB poller churning in the background | DB read into memory once at startup; edits write through to SQLite immediately |
| Python runtime + a dependency tree to install | Pure Go, no CGO, a single static binary |
| Config that fights you | Lua config + 7 built-in themes (`nord`, `dracula`, `catppuccin_mocha`…) with per-color overrides |
| Inline editing that kicks you out on a typo | Validation errors keep you in the input until you fix them |
| No way to add a task to an empty app | `a` in an empty state creates the workspace and first task for you |

## Install

### One-line install (recommended)

```bash
# macOS / Linux (prefers Homebrew, falls back to a direct binary download)
curl -fsSL https://raw.githubusercontent.com/XiaTian-AC/faster-dooit/main/install.sh | bash
```

```powershell
# Windows / PowerShell (prefers Scoop, falls back to a direct zip download)
iwr -useb https://raw.githubusercontent.com/XiaTian-AC/faster-dooit/main/install.ps1 | iex
```

The command is `fdooit` (not `faster-dooit`).

### Homebrew (macOS / Linux)

```bash
brew tap XiaTian-AC/XiaTian-AC-bucket
brew install faster-dooit
```

The tap manifest is auto-updated by the release CI.

### Scoop (Windows)

```powershell
# One-time: add my personal bucket
scoop bucket add faster-dooit https://github.com/XiaTian-AC/XiaTian-AC-bucket
scoop install faster-dooit
```

The personal-bucket manifest is auto-updated by the release CI.

### Build from source

Requires Go ≥ 1.22 (no CGO, no gcc).

```powershell
git clone https://github.com/XiaTian-AC/faster-dooit.git
cd faster-dooit
go build -o fdooit.exe .
.\fdooit.exe
```

## Quick Start

1. Launch — the left pane is **Workspaces**, the right is **Todos**
2. Press `a` to add a todo, type a name, press Enter
3. Press `c` to complete, `d` to set a due date (`tomorrow`, `3d`, `next monday`)
4. Press `?` for help, `/` to force a full redraw

Natural-language dates, recurrence, search, sort, clipboard copy/paste, and a Lua extension surface all ship by default — no plugins to install.

## Reference

### Data & config

- DB (created on first run): `%APPDATA%\faster-dooit\todo.db` (Windows) / `$XDG_CONFIG_HOME/faster-dooit/todo.db` (Linux/macOS)
- Config: `~/.config/faster-dooit/config.lua` (all platforms). Override with `--db <path>` / `-c, --config <path>`
- An invalid config prints a `file:line` error and exits

### Keybindings (defaults)

| Key | Action | Key | Action |
|---|---|---|---|
| `j` / `k` | move down / up | `i` | edit description |
| `d` / `r` / `e` | edit due / recurrence / effort | `a` / `A` | add sibling / child |
| `z` / `Z` | expand / expand parent | `gg` / `G` | go to top / bottom |
| `J` / `K` | shift down / up | `xx` | delete (confirm) |
| `y` / `Y` | copy description / model | `p` / `P` | paste below / above |
| `c` | toggle complete | `=` / `+` / `-` / `_` | increase / decrease urgency |
| `/` | force redraw | `S` / `?` | search / help |
| `tab` / `h` / `l` | switch pane | `enter` | edit description |
| `ctrl+s` | sort | `ctrl+q` / `ctrl+c` | quit |

Remap anything from `config.lua`:

```lua
api.keys.set("i", api.add_sibling)     -- make i add instead of edit
api.keys.set("x", api.toggle_complete) -- single-key complete
```

### Search mode

`/` filters as you type; Enter lands the cursor on the first match. Operate normally on results (`c` to complete, `A` to add a child). Esc clears the filter — the cursor stays on the same item by ID.

### Themes

Pick a built-in theme, then override any color on top of it:

```lua
api.vars.theme.name = "dracula"
api.vars.theme.primary = "#FF79C6"      -- optional per-color override
-- api.vars.theme.background = "transparent"  -- show the terminal's own bg
```

Built-ins: `nord`, `catppuccin_mocha`, `catppuccin_latte`, `dracula`, `gruvbox_dark`, `solarized_light`, `tokyo_night`. Overridable: the 8 base colors plus `dim`, `selection`, `border_focused`, `border_unfocused`, `urgency_colors`. Unknown theme name = config error.

### Benchmarks

Machine: AMD Ryzen 7 H 260 / Windows 11. `go test ./internal/app/ -bench . -benchmem`.

| Benchmark | Result | Target | Status |
|---|---|---|---|
| `BenchmarkStartup10k` (cold load 10k todos) | ~34 ms | < 200 ms | ✅ |
| `BenchmarkVisibleTodos10k` (flatten 10k visible) | ~204 µs | < 10 ms | ✅ |
| `BenchmarkUpdate` (keypress dispatch) | ~157 µs | < 1 ms | ✅ |
| `BenchmarkRenderRow10k` (render one row) | ~233 µs | — | ✅ |

Baseline (this machine): `import dooit.ui.tui` before any render ≈ **1.9 s**. faster-dooit's entire cold-start path is ~34 ms.

## Design notes

- **Completion cascade** — completing a todo completes its subtree; a parent completes only when all children do; reopening any child reopens the parent; recurring todos never complete (due advances one period)
- **Differences from dooit** — old data is **not migrated** (fresh schema); editing errors keep you in the input; `poll_dooit_db` external-edit hot reload is deliberately not implemented

## Contributing

```powershell
go test ./...        # unit + parity + e2e
go vet ./...
go test ./internal/app/ -bench . -benchmem   # perf gates
```

Design spec and implementation plan live under [`docs/superpowers/`](docs/superpowers/); architecture and internals under [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Credits

- [dooit](https://github.com/dooit-org/dooit) — inspiration for the interaction design
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) / [lipgloss](https://github.com/charmbracelet/lipgloss) — Go TUI rendering stack
- Built with AI assistance; feedback welcome
