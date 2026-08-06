package lua

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestEvalDefaultConfig(t *testing.T) {
	rt, err := EvalFile("../../config.lua")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Keys["j"] != "move_down" {
		t.Errorf("j key = %q, want move_down", rt.Keys["j"])
	}
	if len(rt.Bar) == 0 {
		t.Error("expected bar widgets")
	}
	if len(rt.Layouts.Todo) != 6 {
		t.Errorf("todo layout = %v, want 6 columns", rt.Layouts.Todo)
	}
	if len(rt.Dashboard) == 0 {
		t.Error("expected dashboard lines")
	}
}

func TestThemeLoaded(t *testing.T) {
	rt, _ := EvalFile("../../config.lua")
	if rt.Theme.Name == "" {
		t.Error("theme.name not loaded")
	}
}

func TestSubscribeAndTimerRegistered(t *testing.T) {
	rt, _ := EvalFile("../../config.lua")
	if len(rt.Subscribers) == 0 {
		t.Error("expected subscribers from subscribe() calls")
	}
	if len(rt.Timers) == 0 {
		t.Error("expected timers from timer() calls")
	}
}

func TestCallFormatterReturnsText(t *testing.T) {
	rt, _ := EvalFile("../../config.lua")
	// Use the registered status formatter (first in the store).
	if len(rt.Formatters.Todos.Status) == 0 {
		t.Skip("no status formatter registered")
	}
	out, err := rt.CallFormatter(rt.Formatters.Todos.Status[0], "pending", nil, rt.Theme)
	if err != nil {
		t.Fatalf("CallFormatter err = %v", err)
	}
	if out == "" {
		t.Error("CallFormatter returned empty text")
	}
}

func TestMinSizeDefaultsAndOverride(t *testing.T) {
	rt, err := EvalFileWithCode(`return`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.MinWidth != 40 || rt.MinHeight != 12 {
		t.Fatalf("defaults = %d/%d, want 40/12", rt.MinWidth, rt.MinHeight)
	}

	rt2, err := EvalFileWithCode(`api.vars.min_width = 60`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt2.Close()
	if rt2.MinWidth != 60 {
		t.Fatalf("min_width = %d, want 60", rt2.MinWidth)
	}
	if rt2.MinHeight != 12 {
		t.Fatalf("min_height = %d, want default 12", rt2.MinHeight)
	}
}

func TestSandboxNoOS(t *testing.T) {
	// os library must not be available to config scripts.
	_, err := EvalFileWithCode(`api.keys.set("j", "move_down"); return os.execute("echo hi")`)
	if err == nil {
		t.Error("expected os.execute to fail in sandboxed config")
	}
}

func TestLayoutsReadFromConfig(t *testing.T) {
	rt, err := EvalFile("../../config.lua")
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.Layouts.Workspace) == 0 || rt.Layouts.Workspace[0] != "description" {
		t.Errorf("workspace layout = %v, want [description]", rt.Layouts.Workspace)
	}
	want := []string{"status", "description", "due", "effort", "recurrence", "urgency"}
	if len(rt.Layouts.Todo) != len(want) {
		t.Fatalf("todo layout = %v, want %v", rt.Layouts.Todo, want)
	}
	for i, c := range want {
		if rt.Layouts.Todo[i] != c {
			t.Errorf("todo layout[%d] = %q, want %q", i, rt.Layouts.Todo[i], c)
		}
	}
}

func TestFormattersRegisteredPerField(t *testing.T) {
	rt, _ := EvalFile("../../config.lua")
	// Default config registers formatters for status, due, urgency, recurrence.
	fields := map[string]int{
		"status":     1,
		"due":        1,
		"urgency":    1,
		"recurrence": 1,
	}
	check := func(name string, fns []*lua.LFunction) {
		if len(fns) < fields[name] {
			t.Errorf("formatter %s registered %d fns, want >= %d", name, len(fns), fields[name])
		}
	}
	check("status", rt.Formatters.Todos.Status)
	check("due", rt.Formatters.Todos.Due)
	check("urgency", rt.Formatters.Todos.Urgency)
	check("recurrence", rt.Formatters.Todos.Recurrence)
}

func TestApiNotifyRegistered(t *testing.T) {
	// api.notify must exist and be callable without crashing.
	rt, err := EvalFileWithCode(`api.notify("hello", "info")`)
	if err != nil {
		t.Fatalf("api.notify errored: %v", err)
	}
	rt.Close()
}

func TestKeysSetArray(t *testing.T) {
	rt, err := EvalFileWithCode(`api.keys.set({"j","k"}, api.move_down)`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Keys["j"] != "move_down" || rt.Keys["k"] != "move_down" {
		t.Errorf("array keys = %v, want both move_down", rt.Keys)
	}
}

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
