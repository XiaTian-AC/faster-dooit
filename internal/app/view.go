package app

import (
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

	// Help screen overlays the whole view.
	if m.helpVisible {
		return lipgloss.JoinVertical(lipgloss.Left, m.HelpView(), m.renderStatusBar())
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
	content := lipgloss.JoinVertical(lipgloss.Left, combined, status)

	// Vertically center the two panes when they fit inside the terminal;
	// fall back to top-aligned once the content overflows the height.
	if m.height > 0 {
		lines := strings.Count(content, "\n") + 1
		if top := (m.height - lines) / 2; top > 0 {
			content = strings.Repeat("\n", top) + content
		}
	}
	return content
}

func (m *Model) renderWorkspacePane(w int) string {
	th := m.appTheme()
	title := th.Style("primary").Bold(true).Render("Workspaces")
	if m.root == nil {
		return title
	}
	ws := m.VisibleWorkspaces()
	if len(ws) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, dimStyle.Render("(no workspaces)"))
	}
	lines := make([]string, 0, len(ws)+1)
	lines = append(lines, title)
	for i := range ws {
		marker := "  "
		if i == m.WorkspaceCursor && m.focus == PaneWorkspace {
			marker = th.Style("green").Render("> ")
		}
		// indent before marker so the cursor aligns with the row's text column.
		indent := strings.Repeat("  ", ws[i].NestLevel())
		// Inline edit: the focused row renders the text input instead of the row.
		if m.mode == ModeInsert && m.focus == PaneWorkspace && i == m.WorkspaceCursor {
			lines = append(lines, indent+marker+m.input.View())
			continue
		}
		lines = append(lines, indent+marker+m.RenderRow(PaneWorkspace, i))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderTodoPane(w int) string {
	th := m.appTheme()
	title := th.Style("primary").Bold(true).Render("Todos")
	if m.root == nil {
		return title
	}
	ws := m.selectedWorkspace()
	if ws == nil {
		// UI 规格 #2: right pane shows the dashboard welcome until a workspace
		// is selected.
		return m.renderDashboard()
	}
	todos := m.visibleTodos()
	if len(todos) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, dimStyle.Render("(no items to display)"))
	}
	lines := make([]string, 0, len(todos)+1)
	lines = append(lines, title)
	for i := range todos {
		marker := "  "
		if i == m.TodoCursor && m.focus == PaneTodo {
			marker = th.Style("green").Render("> ")
		}
		// indent before marker so the cursor aligns with the row's text column.
		indent := strings.Repeat("  ", todos[i].NestLevel())
		// Inline edit: the focused row renders the text input instead of the row.
		if m.mode == ModeInsert && m.focus == PaneTodo && i == m.TodoCursor {
			lines = append(lines, indent+marker+m.input.View())
			continue
		}
		lines = append(lines, indent+marker+m.RenderRow(PaneTodo, i))
	}
	return strings.Join(lines, "\n")
}

// renderDashboard renders the config dashboard in the todo pane.
func (m *Model) renderDashboard() string {
	th := m.appTheme()
	lines := m.DashboardLines()
	out := make([]string, 0, len(lines)+1)
	out = append(out, th.Style("primary").Bold(true).Render("Dashboard"))
	for _, l := range lines {
		out = append(out, th.Style("secondary").Render(l))
	}
	return strings.Join(out, "\n")
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
