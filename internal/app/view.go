package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette mirrors the default config.lua (Nord-ish). Task 6 wires
// these to a real theme struct; this is a stable local palette so the
// skeleton renders.
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8FBCBB"))
	focusedBorder = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#8FBCBB")).Padding(0, 1)
	dimBorder     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#3B4252")).Padding(0, 1)
	cursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A3BE8C"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#4C566A"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#81A1C1"))
)

// View implements tea.Model. Renders two side-by-side trees with a status
// bar underneath. Real theming/formatting lands in Task 6.
func (m *Model) View() string {
	if m.quitting {
		return "bye.\n"
	}
	if m.width == 0 || m.height == 0 {
		return "loading…\n"
	}

	paneW := m.width / 4 // workspace pane ≈ 20% per UI 规格 #1
	if paneW < 16 {
		paneW = 16
	}
	rightW := m.width - paneW - 2

	left := m.renderWorkspacePane(paneW)
	right := m.renderTodoPane(rightW)

	var combined string
	if m.focus == PaneWorkspace {
		combined = lipgloss.JoinHorizontal(lipgloss.Top,
			focusedBorder.Width(paneW).Render(left),
			dimBorder.Width(rightW).Render(right),
		)
	} else {
		combined = lipgloss.JoinHorizontal(lipgloss.Top,
			dimBorder.Width(paneW).Render(left),
			focusedBorder.Width(rightW).Render(right),
		)
	}

	status := m.renderStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, combined, status)
}

func (m *Model) renderWorkspacePane(w int) string {
	title := titleStyle.Render("Workspaces")
	if m.root == nil {
		return title
	}
	ws := m.VisibleWorkspaces()
	if len(ws) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, dimStyle.Render("(no workspaces)"))
	}
	lines := make([]string, 0, len(ws)+1)
	lines = append(lines, title)
	for i, w := range ws {
		marker := "  "
		if i == m.WorkspaceCursor && m.focus == PaneWorkspace {
			marker = cursorStyle.Render("> ")
		}
		indent := strings.Repeat("  ", w.NestLevel())
		lines = append(lines, fmt.Sprintf("%s%s%s", marker, indent, truncate(w.Description, 40)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderTodoPane(w int) string {
	title := titleStyle.Render("Todos")
	if m.root == nil {
		return title
	}
	ws := m.selectedWorkspace()
	if ws == nil {
		return lipgloss.JoinVertical(lipgloss.Left, title, dimStyle.Render("(select a workspace)"))
	}
	todos := m.visibleTodos()
	if len(todos) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, dimStyle.Render("(no items to display)"))
	}
	lines := make([]string, 0, len(todos)+1)
	lines = append(lines, title)
	for i, t := range todos {
		marker := "  "
		if i == m.TodoCursor && m.focus == PaneTodo {
			marker = cursorStyle.Render("> ")
		}
		indent := strings.Repeat("  ", t.NestLevel())
		glyph := "o"
		if !t.Pending {
			glyph = "x"
		}
		urgency := ""
		if t.Urgency > 0 {
			urgency = fmt.Sprintf(" !%d", t.Urgency)
		}
		lines = append(lines, fmt.Sprintf("%s%s[%s] %s%s", marker, indent, glyph, truncate(t.Description, 60), urgency))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderStatusBar() string {
	mode := string(m.mode)
	left := fmt.Sprintf(" %s ", mode)
	right := ""
	if m.notice != "" {
		right = " " + m.notice + " "
	}
	pad := strings.Repeat(" ", max(0, m.width-len(left)-len(right)))
	return statusStyle.Render(left + pad + right)
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
