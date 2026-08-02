// Package theme maps the config theme colors onto lipgloss styles.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme is the color palette read from config.lua's api.vars.theme.
type Theme struct {
	Primary     string
	Secondary   string
	Background  string
	Background1 string
	Green       string
	Yellow      string
	Orange      string
	Red         string
}

// Color returns the raw hex for a named color, falling back to Primary.
func (t Theme) Color(name string) string {
	switch name {
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
	}
	return t.Primary
}

// Style builds a foreground-styled lipgloss style from a named color.
func (t Theme) Style(name string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Color(name)))
}

// Bg builds a background-styled lipgloss style from a named color.
func (t Theme) Bg(name string) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(t.Color(name)))
}
