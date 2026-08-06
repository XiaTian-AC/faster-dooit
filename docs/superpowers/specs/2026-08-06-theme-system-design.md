# Theme System — Design Spec

Date: 2026-08-06
Status: Approved (design)
Scope: `internal/theme`, `internal/lua`, `internal/app` (view/renderers/bars), `config.lua`, docs, tests

## Problem

The app currently has a partial color system: `api.vars.theme.xxx` exposes 8
named colors (primary/secondary/background/background1/green/yellow/orange/red)
read from `config.lua`, plus `api.vars.urgency_colors`. But:

1. Only the selected row uses a background color (`renderSelectedRow` →
   `Background1`); there is no unified full-UI background.
2. Several UI styles are hardcoded in `view.go` (title, focused border,
   dim border, cursor, dim text, status bar) and do not follow the theme.
3. There is no way to select a whole ready-made color scheme; the user must
   hand-write every color.

Goal: support (a) a global full-UI background color, (b) built-in color
themes selectable from config, and (c) per-color override on top of a theme.

## Config surface (`config.lua`)

```lua
-- Select a built-in theme (optional, defaults to "nord").
api.vars.theme.name = "dracula"

-- Per-color overrides (optional; override the chosen theme's colors).
api.vars.theme.primary = "#FF79C6"
api.vars.theme.dim = "#6272A4"
api.vars.theme.selection = "#44475A"
api.vars.theme.border_focused = "#FF79C6"
api.vars.theme.border_unfocused = "#44475A"
```

Semantics:

- Order of `theme.name` vs per-color writes does NOT matter. Result = preset
  for `name` as the base, then every explicitly-assigned color field
  overwrites it.
- Fields not explicitly assigned take the preset value.
- Omitting `theme.name` entirely uses the `nord` preset.
- An unknown `theme.name` is a config error: print `file:line` and exit
  (consistent with existing invalid-config handling).
- `urgency_colors` may also be overridden on top of the preset (when
  explicitly assigned).
- Backward compatibility: an existing config that hand-writes all 8 base
  colors and no `theme.name` still renders the same (the explicit values
  override the nord preset).

## Built-in themes

Seven presets in the Go binary (not in Lua):

| name | style |
|---|---|
| `nord` | current default, cool dark (#2E3440 bg) |
| `catppuccin_mocha` | dark purple-blue (#1e1e2e bg) |
| `catppuccin_latte` | classic light variant of catppuccin |
| `dracula` | dark (#282A36 bg) |
| `gruvbox_dark` | warm retro (#282828 bg) |
| `solarized_light` | classic light (#fdf6e3 bg) |
| `tokyo_night` | dark (#1a1b26 bg) |

## Go-side structure

### `internal/theme/theme.go`

`Theme` struct gains four semantic-color fields:

```go
type Theme struct {
    Primary, Secondary, Background, Background1 string
    Green, Yellow, Orange, Red string
    Dim, Selection, BorderFocused, BorderUnfocused string
    UrgencyColors []string
}
```

### `internal/theme/presets.go` (new)

```go
var presets = map[string]Theme{
    "nord":               {...},
    "catppuccin_mocha":   {...},
    "catppuccin_latte":   {...},
    "dracula":            {...},
    "gruvbox_dark":       {...},
    "solarized_light":    {...},
    "tokyo_night":        {...},
}

// Resolve returns the full theme for name with explicit overrides applied.
// name defaults to "nord" when empty. explicit is a map of color field →
// configured hex (only fields the user actually assigned). Unknown names
// return an error.
func Resolve(name string, explicit map[string]string) (Theme, error)
```

`Resolve` walks the preset fields, replacing any field present in `explicit`.

### `internal/lua/lua.go`

- `lua.Theme` gains `Name string`, the 4 semantic colors, and
  `Explicit map[string]string` (color field name → the raw assigned value,
  only for fields the user actually assigned).
- `readTheme()` reads `api.vars.theme.name` (default `"nord"`).
- The `api.vars.theme` table gets a `__newindex` metatable recording which
  fields were explicitly assigned (so an override whose value happens to
  equal the preset is still treated as an explicit override). Only string
  color fields count; non-string values are ignored.
- Invalid `theme.name` raises a Lua error with file:line info (surfaces
  through the existing `main.go` config-error exit).

### `internal/app/renderers.go` — `appTheme()`

Replace the hand-rolled theme merge with:

```go
func (m *Model) appTheme() theme.Theme {
    // m.luaCfg.Theme carries Name + Explicit + raw values.
    // theme.Resolve(name, explicit) → complete theme.
    // Fall back to Resolve("nord", nil) when luaCfg is nil.
}
```

Default urgency colors fallback stays.

## Rendering changes

- **Global background fill**: `View()` applies the full-UI `Background` when
  assembling the final output (each line padded to `width` and background
  ANSI applied once at final assembly — not per cached row). The selected row
  keeps its `Selection` highlight.
- **Hardcoded styles → theme-driven**: `view.go` package-level style
  constants (`titleStyle`, `focusedBorder`, `dimBorder`, `cursorStyle`,
  `dimStyle`, `statusStyle`) become functions/methods that build the style
  from the active theme.

Semantic color mapping:

| UI element | semantic color |
|---|---|
| pane title / focused border | `Primary` |
| unfocused border / dim text | `Dim` |
| cursor arrow / selected text | `Green` |
| selected row background | `Selection` |
| status bar / dashboard subtitle | `Secondary` |
| global background | `Background` |

Note: `renderSelectedRow` previously used `Background1`; it switches to
`Selection`. `Background1` remains available (e.g. for subtle pane shading
if added later) but the selected row highlight uses `Selection`.

## Default `config.lua` + docs

- Default `config.lua` switches to `api.vars.theme.name = "nord"` and shows
  the override usage in comments (default appearance unchanged = Nord).
- `README.md` + `README.zh-CN.md`: new "Themes" subsection listing the 7
  theme names and the override syntax.
- `docs/RELEASING.md`: mention the theme mechanism if relevant.

## Testing

- `internal/theme`: preset table completeness (each theme has all 12 colors +
  urgency_colors non-empty); `Resolve` override semantics (preset base +
  explicit override + default nord + unknown-name error).
- `internal/lua`: `readTheme` reads `name` + explicit key set; invalid name
  raises an error.
- `internal/app`: `appTheme()` returns a complete theme; rendering no longer
  relies on the old hardcoded styles (spot-check title/border/selected row).
