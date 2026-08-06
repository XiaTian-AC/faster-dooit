# Theme System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-UI background color, 7 built-in themes selectable via `api.vars.theme.name`, and per-color overrides on top of a theme.

**Architecture:** Preset themes live in Go (`internal/theme/presets.go`). `config.lua` selects a theme by name and records explicit color overrides via a Lua `__newindex` metatable. `appTheme()` resolves preset + overrides into a complete `theme.Theme`. The renderer builds styles from the resolved theme and applies a global background fill in `View()`.

**Tech Stack:** Go, gopher-lua, lipgloss, existing `internal/theme`, `internal/lua`, `internal/app`.

## Global Constraints

- No new dependencies.
- Existing configs that hand-write all 8 base colors (no `theme.name`) must render identically.
- Unknown `theme.name` is a config error: surface as a Lua eval error (main.go already prints `file:line` and exits).
- `theme.name` and per-color overrides are order-independent: preset is the base, every explicitly-assigned color field overrides it.
- Existing test files: `internal/theme` currently has no test file (create `presets_test.go`), `internal/lua/lua_test.go`, `internal/app/*_test.go`.
- Go version floor 1.22.

---

### Task 1: Extend `theme.Theme` struct with semantic colors

**Files:**
- Modify: `internal/theme/theme.go`
- Test: `internal/theme/theme_test.go` (create)

**Interfaces:**
- Consumes: nothing new.
- Produces: `theme.Theme` gains fields `Dim`, `Selection`, `BorderFocused`, `BorderUnfocused`. `Color(name)` and `Style(name)`/`Bg(name)` accept the new names: `"dim"`, `"selection"`, `"border_focused"`, `"border_unfocused"`. `UrgencyColor` behavior unchanged.

- [ ] **Step 1: Write the failing test**

```go
package theme

import "testing"

func TestThemeNewSemanticColors(t *testing.T) {
	th := Theme{
		Primary: "#111111", Secondary: "#222222",
		Background: "#333333", Background1: "#444444",
		Green: "#555555", Yellow: "#666666", Orange: "#777777", Red: "#888888",
		Dim: "#999999", Selection: "#AAAAAA",
		BorderFocused: "#BBBBBB", BorderUnfocused: "#CCCCCC",
	}
	if th.Color("dim") != "#999999" {
		t.Fatalf("dim = %q", th.Color("dim"))
	}
	if th.Color("selection") != "#AAAAAA" {
		t.Fatalf("selection = %q", th.Color("selection"))
	}
	if th.Color("border_focused") != "#BBBBBB" {
		t.Fatalf("border_focused = %q", th.Color("border_focused"))
	}
	if th.Color("border_unfocused") != "#CCCCCC" {
		t.Fatalf("border_unfocused = %q", th.Color("border_unfocused"))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/theme/ -run TestThemeNewSemanticColors`
Expected: FAIL — `Color("dim")` returns Primary because the fields don't exist yet.

- [ ] **Step 3: Extend the struct and Color()**

In `internal/theme/theme.go`, add the four fields and the four cases to `Color()`:

```go
type Theme struct {
	Primary     string
	Secondary   string
	Background  string
	Background1 string
	Green       string
	Yellow      string
	Orange      string
	Red         string

	// Semantic colors for UI chrome: dimmed text / unfocused border,
	// selected-row background, and focused border.
	Dim             string
	Selection       string
	BorderFocused   string
	BorderUnfocused string

	UrgencyColors []string
}
```

In `Color()` add cases:

```go
	case "dim":
		return t.Dim
	case "selection":
		return t.Selection
	case "border_focused":
		return t.BorderFocused
	case "border_unfocused":
		return t.BorderUnfocused
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/theme/ -run TestThemeNewSemanticColors`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/theme/theme.go internal/theme/theme_test.go
git commit -m "theme: add semantic color fields (dim/selection/border)"
```

---

### Task 2: Add preset table and `Resolve()`

**Files:**
- Modify: `internal/theme/theme.go` (add `Resolve`, `Names`)
- Create: `internal/theme/presets.go`
- Test: `internal/theme/presets_test.go` (create)

**Interfaces:**
- Consumes: `theme.Theme` with the new fields (Task 1).
- Produces:
  - `func Names() []string` — sorted list of preset theme names.
  - `func Resolve(name string, explicit map[string]string) (Theme, error)` — returns the complete theme for `name` (empty name → `"nord"`) with every entry of `explicit` applied as an override; unknown name returns an error.

- [ ] **Step 1: Write the failing test**

```go
package theme

import "testing"

func TestNamesContainsBuiltins(t *testing.T) {
	got := Names()
	want := map[string]bool{
		"nord": true, "catppuccin_mocha": true, "catppuccin_latte": true,
		"dracula": true, "gruvbox_dark": true, "solarized_light": true,
		"tokyo_night": true,
	}
	for _, n := range got {
		if want[n] {
			delete(want, n)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing preset themes: %v", want)
	}
}

func TestResolveDefaultNord(t *testing.T) {
	th, err := Resolve("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if th.Primary == "" || th.Background == "" || th.Dim == "" || th.Selection == "" {
		t.Fatalf("nord theme incomplete: %+v", th)
	}
}

func TestResolveOverride(t *testing.T) {
	th, err := Resolve("dracula", map[string]string{"primary": "#FF0000"})
	if err != nil {
		t.Fatal(err)
	}
	if th.Primary != "#FF0000" {
		t.Fatalf("override not applied: primary=%q", th.Primary)
	}
	if th.Background == "" {
		t.Fatal("non-overridden field should keep the preset value")
	}
}

func TestResolveUnknownName(t *testing.T) {
	if _, err := Resolve("no_such_theme", nil); err == nil {
		t.Fatal("unknown theme name should error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/theme/ -run 'TestNamesContainsBuiltins|TestResolve'`
Expected: FAIL — `Names`/`Resolve` not defined.

- [ ] **Step 3: Create `internal/theme/presets.go`**

Nord uses the existing defaults. The other six are canonical community palettes. Full code:

```go
package theme

import (
	"fmt"
	"sort"
)

// defaultUrgencyColors mirrors the built-in urgency palette (level 1..5).
var defaultUrgencyColors = []string{"#A3BE8C", "#EBCB8B", "#D08770", "#BF616A", "#FF5C5C"}

// presets are the built-in color themes. Every theme must set all 12 named
// colors plus an urgency palette.
var presets = map[string]Theme{
	"nord": {
		Primary: "#8FBCBB", Secondary: "#81A1C1",
		Background: "#2E3440", Background1: "#3B4252",
		Green: "#A3BE8C", Yellow: "#EBCB8B", Orange: "#D08770", Red: "#BF616A",
		Dim: "#4C566A", Selection: "#3B4252",
		BorderFocused: "#8FBCBB", BorderUnfocused: "#4C566A",
		UrgencyColors: defaultUrgencyColors,
	},
	"catppuccin_mocha": {
		Primary: "#89B4FA", Secondary: "#A6ADC8",
		Background: "#1E1E2E", Background1: "#313244",
		Green: "#A6E3A1", Yellow: "#F9E2AF", Orange: "#FAB387", Red: "#F38BA8",
		Dim: "#6C7086", Selection: "#313244",
		BorderFocused: "#89B4FA", BorderUnfocused: "#585B70",
		UrgencyColors: []string{"#A6E3A1", "#F9E2AF", "#FAB387", "#F38BA8", "#F38BA8"},
	},
	"catppuccin_latte": {
		Primary: "#1E66F5", Secondary: "#4C4F69",
		Background: "#EFF1F5", Background1: "#CCD0DA",
		Green: "#40A02B", Yellow: "#DF8E1D", Orange: "#FE640B", Red: "#D20F39",
		Dim: "#9CA0B0", Selection: "#CCD0DA",
		BorderFocused: "#1E66F5", BorderUnfocused: "#BCC0CC",
		UrgencyColors: []string{"#40A02B", "#DF8E1D", "#FE640B", "#D20F39", "#D20F39"},
	},
	"dracula": {
		Primary: "#BD93F9", Secondary: "#F8F8F2",
		Background: "#282A36", Background1: "#343746",
		Green: "#50FA7B", Yellow: "#F1FA8C", Orange: "#FFB86C", Red: "#FF5555",
		Dim: "#6272A4", Selection: "#44475A",
		BorderFocused: "#BD93F9", BorderUnfocused: "#44475A",
		UrgencyColors: []string{"#50FA7B", "#F1FA8C", "#FFB86C", "#FF5555", "#FF5555"},
	},
	"gruvbox_dark": {
		Primary: "#83A598", Secondary: "#D5C4A1",
		Background: "#282828", Background1: "#3C3836",
		Green: "#B8BB26", Yellow: "#FABD2F", Orange: "#FE8019", Red: "#FB4934",
		Dim: "#928374", Selection: "#3C3836",
		BorderFocused: "#83A598", BorderUnfocused: "#504945",
		UrgencyColors: []string{"#B8BB26", "#FABD2F", "#FE8019", "#FB4934", "#FB4934"},
	},
	"solarized_light": {
		Primary: "#268BD2", Secondary: "#586E75",
		Background: "#FDF6E3", Background1: "#EEE8D5",
		Green: "#859900", Yellow: "#B58900", Orange: "#CB4B16", Red: "#DC322F",
		Dim: "#93A1A1", Selection: "#EEE8D5",
		BorderFocused: "#268BD2", BorderUnfocused: "#93A1A1",
		UrgencyColors: []string{"#859900", "#B58900", "#CB4B16", "#DC322F", "#DC322F"},
	},
	"tokyo_night": {
		Primary: "#7AA2F7", Secondary: "#A9B1D6",
		Background: "#1A1B26", Background1: "#16161E",
		Green: "#9ECE6A", Yellow: "#E0AF68", Orange: "#FF9E64", Red: "#F7768E",
		Dim: "#565F89", Selection: "#292E42",
		BorderFocused: "#7AA2F7", BorderUnfocused: "#3B4261",
		UrgencyColors: []string{"#9ECE6A", "#E0AF68", "#FF9E64", "#F7768E", "#F7768E"},
	},
}

// fieldNames lists the overrideable color fields in canonical order.
var fieldNames = []string{
	"primary", "secondary", "background", "background1",
	"green", "yellow", "orange", "red",
	"dim", "selection", "border_focused", "border_unfocused",
}

// setColor sets a Theme field by its config name. Returns false for an
// unknown name (callers ignore it so unknown override keys are harmless).
func (t *Theme) setColor(name, value string) bool {
	switch name {
	case "primary":
		t.Primary = value
	case "secondary":
		t.Secondary = value
	case "background":
		t.Background = value
	case "background1":
		t.Background1 = value
	case "green":
		t.Green = value
	case "yellow":
		t.Yellow = value
	case "orange":
		t.Orange = value
	case "red":
		t.Red = value
	case "dim":
		t.Dim = value
	case "selection":
		t.Selection = value
	case "border_focused":
		t.BorderFocused = value
	case "border_unfocused":
		t.BorderUnfocused = value
	default:
		return false
	}
	return true
}

// Names returns the sorted list of built-in theme names.
func Names() []string {
	out := make([]string, 0, len(presets))
	for n := range presets {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Resolve returns the complete theme for name ("" → "nord") with the given
// explicit overrides applied on top. Unknown names return an error.
func Resolve(name string, explicit map[string]string) (Theme, error) {
	if name == "" {
		name = "nord"
	}
	base, ok := presets[name]
	if !ok {
		return Theme{}, fmt.Errorf("theme: unknown theme %q (available: %v)", name, Names())
	}
	for k, v := range explicit {
		if v != "" {
			base.setColor(k, v)
		}
	}
	return base, nil
}
```

Note: this also fixes the pre-existing bug where `appTheme()` used `m.luaCfg.Theme.Primary` directly (empty when unset → all-black). `Resolve` guarantees non-empty values.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/theme/ -run 'TestNamesContainsBuiltins|TestResolve'`
Expected: PASS

- [ ] **Step 5: Run the whole theme package tests**

Run: `go test ./internal/theme/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/theme/presets.go internal/theme/presets_test.go
git commit -m "theme: add built-in theme presets and Resolve()"
```

---

### Task 3: Lua — read `theme.name`, track explicit overrides, validate

**Files:**
- Modify: `internal/lua/lua.go`
- Test: `internal/lua/lua_test.go`

**Interfaces:**
- Consumes: `theme.Names()` for validation.
- Produces: `lua.Theme` gains `Name string` and `Explicit map[string]string` (field name → raw value, only for fields the user assigned). `Runtime.readTheme()` returns an `error` (unknown name). `themeFields` list gains the 4 semantic names so formatters see them.

- [ ] **Step 1: Write the failing tests**

```go
func TestThemeNameLoaded(t *testing.T) {
	rt, err := EvalFileWithCode(`api.vars.theme.name = "dracula"`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Theme.Name != "dracula" {
		t.Fatalf("name = %q, want dracula", rt.Theme.Name)
	}
}

func TestThemeNameDefaultsNord(t *testing.T) {
	rt, err := EvalFileWithCode(``)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Theme.Name != "nord" {
		t.Fatalf("name = %q, want nord", rt.Theme.Name)
	}
}

func TestThemeExplicitTracksOverrides(t *testing.T) {
	rt, err := EvalFileWithCode(`
api.vars.theme.name = "dracula"
api.vars.theme.primary = "#FF0000"
api.vars.theme.dim = "#555555"
`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Theme.Explicit["primary"] != "#FF0000" {
		t.Fatalf("explicit primary = %q", rt.Theme.Explicit["primary"])
	}
	if rt.Theme.Explicit["dim"] != "#555555" {
		t.Fatalf("explicit dim = %q", rt.Theme.Explicit["dim"])
	}
	if _, ok := rt.Theme.Explicit["secondary"]; ok {
		t.Fatal("secondary was not explicitly assigned and must not appear")
	}
}

func TestThemeUnknownNameErrors(t *testing.T) {
	if _, err := EvalFileWithCode(`api.vars.theme.name = "bogus"`); err == nil {
		t.Fatal("unknown theme name must error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lua/ -run 'TestThemeName|TestThemeExplicit|TestThemeUnknown'`
Expected: FAIL — `lua.Theme` has no `Name`/`Explicit`; `readTheme` returns nothing.

- [ ] **Step 3: Modify `internal/lua/lua.go`**

Add to the `lua.Theme` struct:

```go
type Theme struct {
	Primary     string
	Secondary   string
	Background  string
	Background1 string
	Green       string
	Yellow      string
	Orange      string
	Red         string

	Dim             string
	Selection       string
	BorderFocused   string
	BorderUnfocused string

	UrgencyColors []string

	// Name is the selected preset ("nord" by default). Explicit records the
	// color fields the config actually assigned (field name → raw value), so
	// the app can apply them as overrides on top of the preset.
	Name     string
	Explicit map[string]string
}
```

In `Runtime`, add a field to hold the raw theme table's override tracking:

```go
	themeTable *lua.LTable // reference to api.vars.theme for readTheme
	varsTable  *lua.LTable // reference to api.vars for readTheme
	explicit   map[string]string // recorded by the theme __newindex metatable
```

In `EvalFile` and `EvalFileWithCode`, replace `rt.readTheme()` with error handling:

```go
	if err := rt.readTheme(); err != nil {
		L.Close()
		return nil, err
	}
```

Change `installAPI` to install a `__newindex` metatable on the theme table:

```go
	// vars.theme + vars.urgency_colors
	vars := L.NewTable()
	theme := L.NewTable()
	themeMT := L.NewTable()
	L.SetField(themeMT, "__newindex", L.NewFunction(rt.themeNewIndex))
	L.SetMetatable(theme, themeMT)
	L.SetField(vars, "theme", theme)
	L.SetField(api, "vars", vars)
	rt.themeTable = theme
	rt.varsTable = vars
	rt.explicit = map[string]string{}
```

Add the `themeNewIndex` handler (records string assignments into `rt.explicit` and does the raw set so the value still lands in the table):

```go
// themeNewIndex intercepts api.vars.theme.<field> = value. It records string
// color assignments (so readTheme knows which fields were explicitly set) and
// stores the value in the table. Non-string values are ignored for overrides.
func (rt *Runtime) themeNewIndex(L *lua.LState) int {
	key := L.ToString(2)
	val := L.Get(3)
	if s, ok := val.(lua.LString); ok {
		rt.explicit[key] = string(s)
	}
	L.RawSet(rt.themeTable, lua.LString(key), val)
	return 0
}
```

Change `readTheme` to read `name`, apply defaults, populate `Explicit`, and return an error for unknown names. Full replacement:

```go
// readTheme copies color values out of api.vars.theme and
// api.vars.urgency_colors after eval. name defaults to "nord"; an unknown
// name is a config error.
func (rt *Runtime) readTheme() error {
	L := rt.L
	get := func(k string) string {
		if rt.themeTable == nil {
			return ""
		}
		v := L.GetField(rt.themeTable, k)
		if s, ok := v.(lua.LString); ok {
			return string(s)
		}
		return ""
	}
	rt.Theme = Theme{
		Primary:     get("primary"),
		Secondary:   get("secondary"),
		Background:  get("background"),
		Background1: get("background1"),
		Green:       get("green"),
		Yellow:      get("yellow"),
		Orange:      get("orange"),
		Red:         get("red"),
		Dim:         get("dim"),
		Selection:   get("selection"),
		BorderFocused:   get("border_focused"),
		BorderUnfocused: get("border_unfocused"),
		Name:        get("name"),
		Explicit:    rt.explicit,
	}
	if rt.Theme.Name == "" {
		rt.Theme.Name = "nord"
	}

	// api.vars.urgency_colors = { ... } — a 1-based Lua array.
	if rt.varsTable != nil {
		if uc, ok := L.GetField(rt.varsTable, "urgency_colors").(*lua.LTable); ok {
			var colors []string
			uc.ForEach(func(_, v lua.LValue) {
				if s, ok := v.(lua.LString); ok && s != lua.LString("") {
					colors = append(colors, string(s))
				}
			})
			if len(colors) > 0 {
				rt.Theme.UrgencyColors = colors
			}
		}
	}

	// api.vars.min_width / api.vars.min_height (numeric).
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

	if !themeNameValid(rt.Theme.Name) {
		return fmt.Errorf("unknown theme %q (available: %v)", rt.Theme.Name, theme.Names())
	}
	return nil
}
```

Add a helper + import `internal/theme`:

```go
func themeNameValid(name string) bool {
	for _, n := range theme.Names() {
		if n == name {
			return true
		}
	}
	return false
}
```

Import line: add `"github.com/XiaTian-AC/faster-dooit/internal/theme"` to `internal/lua/lua.go`.

Extend `themeFields` used by `CallFormatter` so Lua formatters can reference the new colors:

```go
var themeFields = []string{"primary", "secondary", "background", "background1", "green", "yellow", "orange", "red", "dim", "selection", "border_focused", "border_unfocused"}
```

And extend `themeField()` with the four new cases:

```go
	case "dim":
		return t.Dim
	case "selection":
		return t.Selection
	case "border_focused":
		return t.BorderFocused
	case "border_unfocused":
		return t.BorderUnfocused
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/lua/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/lua/lua.go internal/lua/lua_test.go
git commit -m "lua: read theme.name with explicit-override tracking"
```

---

### Task 4: `appTheme()` resolves preset + overrides

**Files:**
- Modify: `internal/app/renderers.go`
- Test: `internal/app/renderers_test.go`

**Interfaces:**
- Consumes: `theme.Resolve(name, explicit)`, `lua.Theme.Name`/`Explicit`.
- Produces: `appTheme()` returns a fully-populated `theme.Theme` (all 12 colors + urgency colors). The old `defaultUrgencyColors` var moves into the theme package (keep a local alias if referenced by tests).

- [ ] **Step 1: Write the failing tests**

```go
func TestAppThemeResolvesPresetAndOverride(t *testing.T) {
	m := newTestAppLua(t)
	// newTestAppLua evaluates config.lua (theme.name defaults to nord).
	th := m.appTheme()
	if th.Primary == "" || th.Background == "" || th.Dim == "" || th.Selection == "" {
		t.Fatalf("appTheme incomplete: %+v", th)
	}
}

func TestAppThemeOverrideApplied(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	rt, err := lua.EvalFileWithCode(`
api.vars.theme.name = "dracula"
api.vars.theme.primary = "#123456"
`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	m := New(st, rt)
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	th := m.appTheme()
	if th.Primary != "#123456" {
		t.Fatalf("override primary = %q, want #123456", th.Primary)
	}
	if th.Background == "" {
		t.Fatal("dracula background should be populated")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestAppTheme'`
Expected: FAIL — `appTheme` still uses the raw (empty) lua fields.

- [ ] **Step 3: Rewrite `appTheme()` in `internal/app/renderers.go`**

```go
// appTheme resolves the active theme: the named preset as a base with any
// explicitly-configured color fields applied as overrides. Without config it
// returns the nord preset.
func (m *Model) appTheme() theme.Theme {
	if m.luaCfg == nil {
		th, _ := theme.Resolve("", nil)
		return th
	}
	th, err := theme.Resolve(m.luaCfg.Theme.Name, m.luaCfg.Theme.Explicit)
	if err != nil {
		// Unknown name is caught at config eval; fall back defensively.
		th, _ = theme.Resolve("", nil)
	}
	if len(m.luaCfg.Theme.UrgencyColors) > 0 {
		th.UrgencyColors = m.luaCfg.Theme.UrgencyColors
	}
	return th
}
```

Remove the old package-level `defaultUrgencyColors` var from `renderers.go` (now owned by the theme package). Check whether any test references it and update if so.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestAppTheme'`
Expected: PASS

- [ ] **Step 5: Run full app tests**

Run: `go test ./internal/app/`
Expected: PASS (if `defaultUrgencyColors` was referenced by a test, move it to a local in the test file).

- [ ] **Step 6: Commit**

```bash
git add internal/app/renderers.go
git commit -m "app: resolve theme from preset + overrides in appTheme()"
```

---

### Task 5: Theme-driven hardcoded styles + global background

**Files:**
- Modify: `internal/app/view.go`
- Modify: `internal/app/bars.go` (bar fallback status bar already theme-driven; no change needed unless a hardcoded color is found — none)
- Test: `internal/app/view_test.go`, `internal/app/ux_fix_test.go`

**Interfaces:**
- Consumes: `appTheme()` returning a complete theme.
- Produces: `View()` applies a full-UI background fill. The `view.go` package-level style vars (`titleStyle`, `focusedBorder`, `dimBorder`, `cursorStyle`, `dimStyle`, `statusStyle`) become theme-driven (built from `appTheme()`); `renderSelectedRow` uses `Selection`.

- [ ] **Step 1: Write the failing tests**

```go
func TestViewFillsBackground(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 100, 30
	v := m.View()
	if !strings.Contains(v, "\x1b[48;2;") {
		t.Fatalf("View should apply a global background ANSI fill")
	}
}

func TestSelectedRowUsesSelectionColor(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	th := m.appTheme()
	sel := m.renderSelectedRow("> abc", 20)
	// renderSelectedRow injects the Selection background as 24-bit ANSI.
	if !strings.Contains(sel, "\x1b[48;2;") {
		t.Fatalf("selected row should carry a background")
	}
	// The rendered background must match the theme's Selection hex.
	expect := "\x1b[48;2;" + ansiRGB(th.Selection)
	if !strings.Contains(sel, expect) {
		t.Fatalf("selected row should use Selection color %q, got %q", expect, sel)
	}
}
```

Add helper to view_test.go (or reuse `ansiBackground` since it's in package app):

```go
// ansiRGB converts #RRGGBB to the "r;g;b" digits of the 24-bit ANSI sequence.
func ansiRGB(hex string) string {
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 6 {
		return ""
	}
	r, _ := strconv.ParseInt(s[0:2], 16, 32)
	g, _ := strconv.ParseInt(s[2:4], 16, 32)
	b, _ := strconv.ParseInt(s[4:6], 16, 32)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestViewFillsBackground|TestSelectedRowUsesSelectionColor'`
Expected: FAIL — no global fill; `renderSelectedRow` uses `Background1`, not `Selection`.

- [ ] **Step 3: Make `renderSelectedRow` use `Selection`**

In `internal/app/view.go`, change:

```go
	bg := ansiBackground(th.Background1)
```
to:
```go
	bg := ansiBackground(th.Selection)
```

- [ ] **Step 4: Convert hardcoded styles to theme-driven methods**

In `internal/app/view.go`, replace the package-level style block with methods:

```go
// The palette is built from the resolved theme at render time; there are no
// hardcoded UI colors.
func (m *Model) titleStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Primary))
}

func (m *Model) focusedBorder() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(th.BorderFocused)).Padding(0, 1)
}

func (m *Model) dimBorder() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(th.BorderUnfocused)).Padding(0, 1)
}

func (m *Model) cursorStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Green))
}

func (m *Model) dimStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Foreground(lipgloss.Color(th.Dim))
}

func (m *Model) statusStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Foreground(lipgloss.Color(th.Secondary))
}
```

Update every usage site in `view.go`:
- `focusedBorder.Width(...)` → `m.focusedBorder().Width(...)` (lines ~136, 142, 168, 172)
- `dimBorder.Width(...)` → `m.dimBorder().Width(...)` (lines ~137, 141, 169, 171)
- `dimStyle.Render(...)` → `m.dimStyle().Render(...)` (lines ~262, 329)
- The pane titles already use `th.Style("primary")` — leave them; `titleStyle`/`cursorStyle`/`statusStyle` may remain unused if no site references them, but keep them for the theme-driven API (Go does not complain about unused methods).

- [ ] **Step 5: Add the global background fill to `View()`**

In `internal/app/view.go`, `View()` — after building `content` and vertical centering, wrap with background fill. The pane content and status bar already render inside bordered boxes that fill their width, so the fill mainly covers gaps (vertically centered whitespace and lines shorter than the terminal). Implementation:

```go
	// Vertically center the two panes when they fit inside the terminal;
	// fall back to top-aligned once the content overflows the height.
	if m.height > 0 {
		lines := strings.Count(content, "\n") + 1
		if top := (m.height - lines) / 2; top > 0 {
			content = strings.Repeat("\n", top) + content
		}
	}
	return m.fillBackground(content)
```

Add the method:

```go
// fillBackground applies the theme's Background color to the full rendered
// output, padding every line to the terminal width so the background spans
// the whole screen. Lines that already carry their own background (the
// selected row's Selection highlight) keep it because the fill only prepends
// the base background and re-applies it after each ANSI reset, never after a
// row's own explicit background.
func (m *Model) fillBackground(content string) string {
	th := m.appTheme()
	bg := ansiBackground(th.Background)
	if bg == "" || m.width <= 0 {
		return content
	}
	reset := "\x1b[0m"
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		// Pad the visible width to the terminal width so the fill covers gaps.
		if pad := m.width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		out = append(out, bg+strings.ReplaceAll(line, reset, reset+bg)+reset)
	}
	return strings.Join(out, "\n")
}
```

Note: this wraps every line including the status bar and selected row; the selected row's own `Selection` background is applied inside its line and the fill's re-application happens only after plain `\x1b[0m` resets, so the highlight survives (verify in Step 6).

- [ ] **Step 6: Run tests to verify**

Run: `go test ./internal/app/ -run 'TestViewFillsBackground|TestSelectedRowUsesSelectionColor'`
Expected: PASS

Run: `go test ./internal/app/`
Expected: PASS. If any width-related test breaks (e.g. `update_test.go:190` checks lines ≤ 130 after resize), it should still hold because fill pads TO the width, not beyond. If a test fails because `lipgloss.Width` now includes the padded spaces, inspect and adjust the test's assertion to `<=` width.

- [ ] **Step 7: Commit**

```bash
git add internal/app/view.go
git commit -m "app: theme-driven styles + global background fill"
```

---

### Task 6: Default config.lua + docs + README

**Files:**
- Modify: `config.lua`
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/RELEASING.md` (optional one-line mention)
- Test: `internal/lua/lua_test.go` (TestEvalDefaultConfig still passes), `internal/app/app_test.go`

**Interfaces:**
- Consumes: all prior tasks.
- Produces: documented theme mechanism.

- [ ] **Step 1: Update `config.lua`**

Replace the color block with the theme name + commented override examples:

```lua
-- Theme: pick a built-in preset, then optionally override individual colors.
-- Built-ins: nord, catppuccin_mocha, catppuccin_latte, dracula,
--            gruvbox_dark, solarized_light, tokyo_night
api.vars.theme.name = "nord"

-- Override a color on top of the selected theme (optional):
-- api.vars.theme.primary = "#8FBCBB"
-- api.vars.theme.secondary = "#81A1C1"
-- api.vars.theme.background = "#2E3440"
-- api.vars.theme.background1 = "#3B4252"
-- api.vars.theme.green = "#A3BE8C"
-- api.vars.theme.yellow = "#EBCB8B"
-- api.vars.theme.orange = "#D08770"
-- api.vars.theme.red = "#BF616A"
-- api.vars.theme.dim = "#4C566A"
-- api.vars.theme.selection = "#3B4252"
-- api.vars.theme.border_focused = "#8FBCBB"
-- api.vars.theme.border_unfocused = "#4C566A"
```

Keep the `urgency_colors` line and everything below unchanged.

- [ ] **Step 2: Verify config still evaluates**

Run: `go test ./internal/lua/`
Expected: PASS

- [ ] **Step 3: Update `README.md` (Themes subsection)**

After the "Lua config" intro paragraph, add:

```markdown
**Theme selection**

Pick a built-in theme with `api.vars.theme.name`, then optionally override
any color:

```lua
api.vars.theme.name = "dracula"

-- override one color on top of the theme
api.vars.theme.primary = "#FF79C6"
```

Built-in themes: `nord`, `catppuccin_mocha`, `catppuccin_latte`, `dracula`,
`gruvbox_dark`, `solarized_light`, `tokyo_night`. Overridable colors: the 8
base colors plus `dim`, `selection`, `border_focused`, `border_unfocused`,
and `urgency_colors`. An unknown theme name is a config error.
```

- [ ] **Step 4: Update `README.zh-CN.md` with the same content in Chinese**

```markdown
**主题选择**

用 `api.vars.theme.name` 选择内置主题，之后可单独覆盖任意颜色：

```lua
api.vars.theme.name = "dracula"

-- 在主题上覆盖单个颜色
api.vars.theme.primary = "#FF79C6"
```

内置主题：`nord`、`catppuccin_mocha`、`catppuccin_latte`、`dracula`、
`gruvbox_dark`、`solarized_light`、`tokyo_night`。可覆盖颜色：8 个基础色
加 `dim`、`selection`、`border_focused`、`border_unfocused`、
`urgency_colors`。未知主题名会报错退出。
```

- [ ] **Step 5: Update `docs/RELEASING.md`**

Add one line under a "Notes" or similar heading: "Theme system: `api.vars.theme.name` selects a built-in preset; per-color overrides apply on top. See README."

- [ ] **Step 6: Run full test suite**

Run: `go test ./...` and `go vet ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add config.lua README.md README.zh-CN.md docs/RELEASING.md
git commit -m "docs: theme system config, README, and release notes"
```

---

### Task 7: Final verification

**Files:** none.

- [ ] **Step 1: Full test + vet + build**

Run: `go test ./...`
Expected: PASS

Run: `go vet ./...`
Expected: no output

Run: `go build ./...`
Expected: builds

- [ ] **Step 2: Smoke-check the TUI against each theme**

Run the app with a config that sets `theme.name = "dracula"` and verify the
UI renders with the Dracula palette (background filled, selected row using
Selection). Repeat for `nord` and `solarized_light` (light background).
Note any crash or obviously broken colors.

- [ ] **Step 3: Confirm final git state**

Run: `git log --oneline -12`
Expected: the six feature commits + this plan's spec commit.
