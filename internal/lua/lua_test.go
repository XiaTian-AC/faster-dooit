package lua

import (
	"testing"
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
	if len(rt.Layouts.Todo) != 4 {
		t.Errorf("todo layout = %v, want 4 columns", rt.Layouts.Todo)
	}
	if len(rt.Dashboard) == 0 {
		t.Error("expected dashboard lines")
	}
}

func TestThemeLoaded(t *testing.T) {
	rt, _ := EvalFile("../../config.lua")
	if rt.Theme.Primary == "" {
		t.Error("theme.primary not loaded")
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

func TestSandboxNoOS(t *testing.T) {
	// os library must not be available to config scripts.
	_, err := EvalFileWithCode(`api.keys.set("j", "move_down"); return os.execute("echo hi")`)
	if err == nil {
		t.Error("expected os.execute to fail in sandboxed config")
	}
}
