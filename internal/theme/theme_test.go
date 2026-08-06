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
