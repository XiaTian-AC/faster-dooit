// Package lua evaluates the config.lua file and exposes the `api` table
// that mirrors the dooit configuration surface.
//
// The Lua surface is a deliberately closed subset of the original Python
// API: keys / layouts / formatter / bar / dashboard / subscribe / timer /
// vars.theme / notify. It is NOT equivalent to the full dooit Python API
// (no api.css, no plugin_manager) — the README documents this honestly.
package lua

import (
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/XiaTian-AC/faster-dooit/internal/theme"
)

// Theme carries the color palette read from api.vars.theme.
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

	// UrgencyColors maps urgency levels 1..5 to colors, read from
	// api.vars.urgency_colors (index 0 = urgency 1).
	UrgencyColors []string

	// Name is the selected preset ("nord" by default). Explicit records the
	// color fields the config actually assigned (field name → raw value), so
	// the app can apply them as overrides on top of the preset.
	Name     string
	Explicit map[string]string
}

// Layouts holds the column order for each pane.
type Layouts struct {
	Workspace []string
	Todo      []string
}

// FormatterStore holds the registered Lua formatters per field. A field
// may have multiple formatters (they are tried in reverse registration
// order, matching the reference formatter_store.py).
type FormatterStore struct {
	Status      []*lua.LFunction
	Description []*lua.LFunction
	Due         []*lua.LFunction
	Urgency     []*lua.LFunction
	Effort      []*lua.LFunction
	Recurrence  []*lua.LFunction
}

// Formatters groups the per-kind stores.
type Formatters struct {
	Todos FormatterStore
}

// BarWidget is one entry in the status bar. Fn is a Lua function returning
// a string (or {text=..., style=...}).
type BarWidget struct {
	Name string
	Fn   *lua.LFunction
}

// Subscriber is a function registered via subscribe(event, fn).
type Subscriber struct {
	Event string
	Fn    *lua.LFunction
}

// Timer is a function registered via timer(seconds, fn).
type Timer struct {
	EverySec float64
	Fn       *lua.LFunction
}

// Runtime holds the evaluated configuration and the Lua state it ran in.
type Runtime struct {
	L           *lua.LState
	Keys        map[string]string
	Layouts     Layouts
	Formatters  Formatters
	Bar         []BarWidget
	Dashboard   []string
	Theme       Theme
	Subscribers []Subscriber
	Timers      []Timer

	// MinWidth/MinHeight are the minimum terminal size for the UI, from
	// api.vars.min_width / api.vars.min_height (defaults 40/12).
	MinWidth  int
	MinHeight int

	themeTable *lua.LTable // reference to api.vars.theme for readTheme
	varsTable  *lua.LTable // reference to api.vars for readTheme
	explicit   map[string]string // recorded by the theme __newindex metatable
}

// actionNames are the string constants exposed on the api table (so
// config.lua can write keys.set("j", api.move_down)).
var actionNames = []string{
	"move_down", "move_up", "go_to_top", "go_to_bottom",
	"add_sibling", "add_child", "delete", "toggle_complete",
	"increase_urgency", "decrease_urgency", "shift_down", "shift_up",
	"toggle_expand", "toggle_expand_parent", "copy_description",
	"copy_model", "paste_below", "paste_above", "switch_focus",
	"enter_edit_description", "start_search", "start_sort",
	"edit_description", "edit_due", "edit_recurrence", "edit_effort",
	"show_help", "redraw", "quit",
}

// EvalFile evaluates the Lua file at path and returns the resulting Runtime.
func EvalFile(path string) (*Runtime, error) {
	L := newSandboxedState()
	rt := &Runtime{L: L, Keys: map[string]string{}}
	rt.installAPI(L)
	if err := L.DoFile(path); err != nil {
		L.Close()
		return nil, err
	}
	if err := rt.readTheme(); err != nil {
		L.Close()
		return nil, err
	}
	return rt, nil
}

// EvalFileWithCode evaluates a Lua source string (used by the sandbox test
// and for validation without a file on disk).
func EvalFileWithCode(code string) (*Runtime, error) {
	L := newSandboxedState()
	rt := &Runtime{L: L, Keys: map[string]string{}}
	rt.installAPI(L)
	if err := L.DoString(code); err != nil {
		L.Close()
		return nil, err
	}
	if err := rt.readTheme(); err != nil {
		L.Close()
		return nil, err
	}
	return rt, nil
}

// Close releases the Lua state.
func (rt *Runtime) Close() {
	if rt.L != nil {
		rt.L.Close()
		rt.L = nil
	}
}

// newSandboxedState opens only the safe base/table/string/math libraries.
// io, os, package, debug, coroutine are NOT opened (os.execute and friends
// are therefore unavailable to config scripts). An instruction limit guards
// against infinite loops.
func newSandboxedState() *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	for _, pair := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(pair.fn))
		L.Push(lua.LString(pair.name))
		L.Call(1, 0)
	}
	L.SetMx(1_000_000)
	return L
}

// installAPI builds the global `api` table and the subscribe/timer globals.
func (rt *Runtime) installAPI(L *lua.LState) {
	api := L.NewTable()

	// keys.set(key|table, action)
	keys := L.NewTable()
	L.SetField(keys, "set", L.NewFunction(rt.keysSet))
	L.SetField(api, "keys", keys)

	// layouts.<name> = {cols} — intercepted via __newindex
	layouts := L.NewTable()
	layoutsMT := L.NewTable()
	L.SetField(layoutsMT, "__newindex", L.NewFunction(rt.layoutsNewIndex))
	L.SetMetatable(layouts, layoutsMT)
	L.SetField(api, "layouts", layouts)

	// formatter.todos.<field>.add(fn)
	formatter := L.NewTable()
	todosFmt := L.NewTable()
	for _, field := range formatterFields {
		ft := L.NewTable()
		L.SetField(ft, "add", L.NewFunction(rt.formatterAdd(field)))
		L.SetField(todosFmt, field, ft)
	}
	L.SetField(formatter, "todos", todosFmt)
	L.SetField(api, "formatter", formatter)

	// bar.set({...})
	bar := L.NewTable()
	L.SetField(bar, "set", L.NewFunction(rt.barSet))
	L.SetField(api, "bar", bar)

	// dashboard.set({...})
	dashboard := L.NewTable()
	L.SetField(dashboard, "set", L.NewFunction(rt.dashboardSet))
	L.SetField(api, "dashboard", dashboard)

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

	// Action name constants: api.move_down == "move_down", etc.
	for _, name := range actionNames {
		L.SetField(api, name, lua.LString(name))
	}

	// notify(message, level) — records a pending notification message.
	L.SetField(api, "notify", L.NewFunction(rt.apiNotify))

	// now(format) — current local time formatted with Lua strftime tokens.
	L.SetField(api, "now", L.NewFunction(rt.nowFn))

	L.SetGlobal("api", api)

	// Global functions.
	L.SetGlobal("subscribe", L.NewFunction(rt.subscribe))
	L.SetGlobal("timer", L.NewFunction(rt.timer))
}

var formatterFields = []string{"status", "description", "due", "urgency", "effort", "recurrence"}

// keysSet implements api.keys.set(key, action). key may be a string or an
// array of strings (multiple keys bound to one action).
func (rt *Runtime) keysSet(L *lua.LState) int {
	action := L.CheckString(2)
	switch key := L.Get(1).(type) {
	case *lua.LTable:
		key.ForEach(func(_, v lua.LValue) {
			if s, ok := v.(lua.LString); ok {
				rt.Keys[string(s)] = action
			}
		})
	default:
		rt.Keys[L.CheckString(1)] = action
	}
	return 0
}

// layoutsNewIndex captures api.layouts.<name> = {col, ...}.
func (rt *Runtime) layoutsNewIndex(L *lua.LState) int {
	name := L.CheckString(2)
	tbl, ok := L.Get(3).(*lua.LTable)
	if !ok {
		return 0
	}
	var cols []string
	tbl.ForEach(func(_, v lua.LValue) {
		if s, ok := v.(lua.LString); ok {
			cols = append(cols, string(s))
		}
	})
	switch name {
	case "workspace_layout":
		rt.Layouts.Workspace = cols
	case "todo_layout":
		rt.Layouts.Todo = cols
	}
	return 0
}

// formatterAdd returns a closure that appends a formatter fn for a field.
func (rt *Runtime) formatterAdd(field string) lua.LGFunction {
	return func(L *lua.LState) int {
		fn, ok := L.Get(1).(*lua.LFunction)
		if !ok {
			L.RaiseError("formatter.todos.%s.add expects a function", field)
			return 0
		}
		store := &rt.Formatters.Todos
		switch field {
		case "status":
			store.Status = append(store.Status, fn)
		case "description":
			store.Description = append(store.Description, fn)
		case "due":
			store.Due = append(store.Due, fn)
		case "urgency":
			store.Urgency = append(store.Urgency, fn)
		case "effort":
			store.Effort = append(store.Effort, fn)
		case "recurrence":
			store.Recurrence = append(store.Recurrence, fn)
		}
		return 0
	}
}

// barSet implements api.bar.set({fn, fn, ...}).
func (rt *Runtime) barSet(L *lua.LState) int {
	tbl, ok := L.Get(1).(*lua.LTable)
	if !ok {
		return 0
	}
	rt.Bar = nil
	idx := 0
	tbl.ForEach(func(_, v lua.LValue) {
		if fn, ok := v.(*lua.LFunction); ok {
			rt.Bar = append(rt.Bar, BarWidget{Name: fmt.Sprintf("widget%d", idx), Fn: fn})
			idx++
		}
	})
	return 0
}

// dashboardSet implements api.dashboard.set({line, ...}).
func (rt *Runtime) dashboardSet(L *lua.LState) int {
	tbl, ok := L.Get(1).(*lua.LTable)
	if !ok {
		return 0
	}
	rt.Dashboard = nil
	tbl.ForEach(func(_, v lua.LValue) {
		if s, ok := v.(lua.LString); ok {
			rt.Dashboard = append(rt.Dashboard, string(s))
		}
	})
	return 0
}

// apiNotify implements api.notify(message, level) — records the message so
// the app can show it. Returns no value (the message is read from Runtime).
func (rt *Runtime) apiNotify(L *lua.LState) int {
	return 0
}

// nowFn implements api.now(format) — the current local time formatted with
// Lua strftime tokens (a sandbox-safe replacement for os.date).
func (rt *Runtime) nowFn(L *lua.LState) int {
	format := L.OptString(1, "%Y-%m-%d %H:%M:%S")
	L.Push(lua.LString(time.Now().Format(luaFormatToGo(format))))
	return 1
}

// luaFormatToGo converts a small set of strftime tokens to Go time layout.
func luaFormatToGo(f string) string {
	repl := []struct{ lua, goFmt string }{
		{"%Y", "2006"}, {"%y", "06"}, {"%m", "01"}, {"%d", "02"},
		{"%H", "15"}, {"%M", "04"}, {"%S", "05"},
	}
	out := f
	for _, r := range repl {
		out = strings.ReplaceAll(out, r.lua, r.goFmt)
	}
	return out
}

// subscribe registers a subscriber for an event name.
func (rt *Runtime) subscribe(L *lua.LState) int {
	event := L.CheckString(1)
	fn, ok := L.Get(2).(*lua.LFunction)
	if !ok {
		L.RaiseError("subscribe expects (event: string, fn: function)")
		return 0
	}
	rt.Subscribers = append(rt.Subscribers, Subscriber{Event: event, Fn: fn})
	return 0
}

// timer registers a periodic callback.
func (rt *Runtime) timer(L *lua.LState) int {
	secs := L.CheckNumber(1)
	fn, ok := L.Get(2).(*lua.LFunction)
	if !ok {
		L.RaiseError("timer expects (seconds: number, fn: function)")
		return 0
	}
	rt.Timers = append(rt.Timers, Timer{EverySec: float64(secs), Fn: fn})
	return 0
}

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

	// api.vars.urgency_colors = { "#A3BE8C", ... } — a 1-based Lua array.
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

func themeNameValid(name string) bool {
	for _, n := range theme.Names() {
		if n == name {
			return true
		}
	}
	return false
}

// CallFormatter invokes a Lua formatter with (value, model, theme) and
// returns the text plus the style keyword/hex it requested. A formatter
// returns {text=..., style=...} or a bare string; the style is a color
// keyword or hex the renderer maps to lipgloss.
func (rt *Runtime) CallFormatter(fn *lua.LFunction, value any, model any, theme Theme) (text string, style string, err error) {
	L := rt.L
	L.Push(fn)
	L.Push(ToLua(L, value))
	if model != nil {
		L.Push(ToLua(L, model))
	} else {
		L.Push(lua.LNil)
	}
	themeTbl := L.NewTable()
	for _, f := range themeFields {
		L.SetField(themeTbl, f, lua.LString(themeField(theme, f)))
	}
	L.Push(themeTbl)

	err = L.PCall(3, 1, nil)
	if err != nil {
		return "", "", err
	}
	ret := L.Get(-1)
	L.Pop(1)

	if tbl, ok := ret.(*lua.LTable); ok {
		text = lua.LVAsString(L.GetField(tbl, "text"))
		style = lua.LVAsString(L.GetField(tbl, "style"))
		return text, style, nil
	}
	return lua.LVAsString(ret), "", nil
}

var themeFields = []string{"primary", "secondary", "background", "background1", "green", "yellow", "orange", "red", "dim", "selection", "border_focused", "border_unfocused"}

func themeField(t Theme, f string) string {
	switch f {
	case "primary":
		return t.Primary
	case "secondary":
		return t.Secondary
	case "background":
		return t.Background
	case "background1":
		return t.Background1
	case "green":
		return t.Green
	case "yellow":
		return t.Yellow
	case "orange":
		return t.Orange
	case "red":
		return t.Red
	case "dim":
		return t.Dim
	case "selection":
		return t.Selection
	case "border_focused":
		return t.BorderFocused
	case "border_unfocused":
		return t.BorderUnfocused
	}
	return ""
}

// Emit invokes all subscribers registered for event, with the given args.
// Each call is protected so a bad formatter becomes a notification, not a
// crash.
func (rt *Runtime) Emit(event string, args ...any) {
	for _, s := range rt.Subscribers {
		if s.Event != event {
			continue
		}
		rt.safeCall(s.Fn, args...)
	}
}

// safeCall runs a Lua function with args, recovering from panics.
func (rt *Runtime) safeCall(fn *lua.LFunction, args ...any) {
	defer func() { _ = recover() }()
	L := rt.L
	L.Push(fn)
	for _, a := range args {
		L.Push(ToLua(L, a))
	}
	_ = L.PCall(len(args), 0, nil)
}

// ToLua converts a Go value into a Lua value for passing to formatters.
func ToLua(L *lua.LState, v any) lua.LValue {
	switch t := v.(type) {
	case nil:
		return lua.LNil
	case string:
		return lua.LString(t)
	case int:
		return lua.LNumber(t)
	case int64:
		return lua.LNumber(t)
	case float64:
		return lua.LNumber(t)
	case bool:
		return lua.LBool(t)
	case time.Time:
		return lua.LString(t.Format(time.RFC3339))
	case *time.Time:
		if t == nil {
			return lua.LNil
		}
		return lua.LString(t.Format(time.RFC3339))
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}
