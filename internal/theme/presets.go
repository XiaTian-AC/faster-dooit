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
