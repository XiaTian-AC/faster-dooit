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
	out, _, err := rt.CallFormatter(rt.Formatters.Todos.Status[0], "pending", nil, rt.Theme)
	if err != nil {
		t.Fatalf("CallFormatter err = %v", err)
	}
	if out == "" {
		t.Error("CallFormatter returned empty text")
	}
}

func TestCallFormatterReturnsStyle(t *testing.T) {
	rt, err := EvalFileWithCode(`
theme.red = "#FF0000"
formatter.todos.description.add(function(desc, model, theme)
  return { text = desc, style = theme.red }
end)
`)
	if err != nil {
		t.Fatal(err)
	}
	fns := rt.Formatters.Todos.Description
	if len(fns) == 0 {
		t.Skip("no description formatter registered")
	}
	// config.lua formatters return theme.<color> which evaluates to a hex
	// string (e.g. "#FF0000"); CallFormatter must pass that through as style.
	text, style, err := rt.CallFormatter(fns[len(fns)-1], "buy milk", nil, rt.Theme)
	if err != nil {
		t.Fatalf("CallFormatter err = %v", err)
	}
	if text != "buy milk" {
		t.Fatalf("text = %q, want buy milk", text)
	}
	if style != "#FF0000" {
		t.Fatalf("style = %q, want #FF0000", style)
	}
}

func TestMinSizeDefaultsAndOverride(t *testing.T) {
	rt, err := EvalFileWithCode(`return`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.MinWidth != 30 || rt.MinHeight != 6 {
		t.Fatalf("defaults = %d/%d, want 30/6", rt.MinWidth, rt.MinHeight)
	}

	rt2, err := EvalFileWithCode(`vars.min_width = 60`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt2.Close()
	if rt2.MinWidth != 60 {
		t.Fatalf("min_width = %d, want 60", rt2.MinWidth)
	}
	if rt2.MinHeight != 6 {
		t.Fatalf("min_height = %d, want default 6", rt2.MinHeight)
	}
}

func TestSandboxNoOS(t *testing.T) {
	// os library must not be available to config scripts.
	_, err := EvalFileWithCode(`keys.set("j", "move_down"); return os.execute("echo hi")`)
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

func TestNotifyRegistered(t *testing.T) {
	// notify must exist and be callable without crashing.
	rt, err := EvalFileWithCode(`notify("hello", "info")`)
	if err != nil {
		t.Fatalf("notify errored: %v", err)
	}
	rt.Close()
}

func TestKeysSetArray(t *testing.T) {
	rt, err := EvalFileWithCode(`keys.set({"j","k"}, move_down)`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Keys["j"] != "move_down" || rt.Keys["k"] != "move_down" {
		t.Errorf("array keys = %v, want both move_down", rt.Keys)
	}
}

func TestThemeNameLoaded(t *testing.T) {
	rt, err := EvalFileWithCode(`theme.name = "dracula"`)
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
theme.name = "dracula"
theme.primary = "#FF0000"
theme.dim = "#555555"
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
	if _, err := EvalFileWithCode(`theme.name = "bogus"`); err == nil {
		t.Fatal("unknown theme name must error")
	}
}

func TestCollapseDepthLoaded(t *testing.T) {
	rt, err := EvalFileWithCode(`vars.collapse_depth = 2`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CollapseDepth != 2 {
		t.Fatalf("collapse_depth = %d, want 2", rt.CollapseDepth)
	}
}

func TestCollapseDepthDefaultsZero(t *testing.T) {
	rt, err := EvalFileWithCode(``)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CollapseDepth != 0 {
		t.Fatalf("collapse_depth default = %d, want 0", rt.CollapseDepth)
	}
}

func TestMaxDescriptionLinesLoaded(t *testing.T) {
	rt, err := EvalFileWithCode(`vars.max_description_lines = 5`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.MaxDescriptionLines != 5 {
		t.Fatalf("max_description_lines = %d, want 5", rt.MaxDescriptionLines)
	}
}

func TestMaxDescriptionLinesDefaultsThree(t *testing.T) {
	rt, err := EvalFileWithCode(``)
	if err != nil {
		t.Fatal(err)
	}
	if rt.MaxDescriptionLines != 3 {
		t.Fatalf("max_description_lines default = %d, want 3", rt.MaxDescriptionLines)
	}
}

func TestMaxDescriptionLinesZeroPreserved(t *testing.T) {
	rt, err := EvalFileWithCode(`vars.max_description_lines = 0`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.MaxDescriptionLines != 0 {
		t.Fatalf("explicit max_description_lines = 0 must be preserved (never ellipsize), got %d", rt.MaxDescriptionLines)
	}
}

func TestEvalDefaultConfigMaxDescriptionLines(t *testing.T) {
	rt, err := EvalFile("../../config.lua")
	if err != nil {
		t.Fatal(err)
	}
	if rt.MaxDescriptionLines != 3 {
		t.Fatalf("config.lua max_description_lines = %d, want 3", rt.MaxDescriptionLines)
	}
}

func TestOldAPISyntaxRejected(t *testing.T) {
	if _, err := EvalFileWithCode(`api.vars.theme.name = "nord"`); err == nil {
		t.Fatal("old api.* syntax must be rejected (no backward compat)")
	}
}
