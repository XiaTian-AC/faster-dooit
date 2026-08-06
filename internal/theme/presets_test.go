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
