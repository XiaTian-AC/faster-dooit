package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme-driven styles. The palette is resolved from config.lua (preset +
// overrides); there are no hardcoded UI colors.
func (m *Model) titleStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Primary))
}

func (m *Model) focusedBorder() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(th.BorderFocused)).Padding(0, 1)
}

func (m *Model) dimBorder() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(th.BorderUnfocused)).Padding(0, 1)
}

func (m *Model) cursorStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(th.Green))
}

func (m *Model) dimStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Foreground(lipgloss.Color(th.Dim))
}

func (m *Model) statusStyle() lipgloss.Style {
	th := m.appTheme()
	return lipgloss.NewStyle().Foreground(lipgloss.Color(th.Secondary))
}

// layoutMode selects the responsive layout for the current terminal size.
// Width and height are evaluated independently; either being too small
// wins over everything.
type layoutMode int

const (
	layoutNormal  layoutMode = iota // dual pane side-by-side
	layoutStacked                   // stacked, single column
	layoutTooSmall                  // stop rendering, show notice
)

const (
	layoutWStack = 100 // width >= this: dual pane
)

func (m *Model) layoutMode() layoutMode {
	mw, mh := 40, 12
	if m.luaCfg != nil {
		if m.luaCfg.MinWidth > 0 {
			mw = m.luaCfg.MinWidth
		}
		if m.luaCfg.MinHeight > 0 {
			mh = m.luaCfg.MinHeight
		}
	}
	if m.width < mw || m.height < mh {
		return layoutTooSmall
	}
	if m.width < layoutWStack {
		return layoutStacked
	}
	return layoutNormal
}

func (m *Model) minSize() (int, int) {
	mw, mh := 40, 12
	if m.luaCfg != nil {
		if m.luaCfg.MinWidth > 0 {
			mw = m.luaCfg.MinWidth
		}
		if m.luaCfg.MinHeight > 0 {
			mh = m.luaCfg.MinHeight
		}
	}
	return mw, mh
}

func (m *Model) renderTooSmall() string {
	mw, mh := m.minSize()
	return fmt.Sprintf("Terminal size too small: Width = %d Height = %d\nNeeded for current config: Width = %d Height = %d",
		m.width, m.height, mw, mh)
}

// View implements tea.Model. Renders the two panes (side-by-side or stacked
// depending on terminal width) with a status bar underneath.
func (m *Model) View() string {
	if m.quitting {
		return "bye.\n"
	}
	if m.width == 0 || m.height == 0 {
		return "loading…\n"
	}
	if m.layoutMode() == layoutTooSmall {
		return m.renderTooSmall()
	}

	// Help screen overlays the whole view.
	if m.helpVisible {
		return lipgloss.JoinVertical(lipgloss.Left, m.HelpView(), m.renderStatusBar())
	}

	var combined string
	if m.layoutMode() == layoutStacked {
		combined = m.renderStacked()
	} else {
		combined = m.renderDualPane()
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
	return m.fillBackground(content)
}

// fillBackground applies the theme's Background color to the full rendered
// output, padding every line to the terminal width and extending the output
// to the terminal height so the background spans the whole screen. Rows that
// already carry their own background (the selected row's Selection highlight)
// keep it: the fill only prepends the base background and re-applies it after
// plain resets. A transparent Background (or any non-hex value) skips the
// fill entirely, leaving the terminal's own background.
func (m *Model) fillBackground(content string) string {
	th := m.appTheme()
	bg := ansiBackground(th.Background)
	if bg == "" || m.width <= 0 {
		return content
	}
	reset := "\x1b[0m"
	lines := strings.Split(content, "\n")
	if m.height > 0 {
		for len(lines) < m.height {
			lines = append(lines, "")
		}
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if pad := m.width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		out = append(out, bg+strings.ReplaceAll(line, reset, reset+bg)+reset)
	}
	return strings.Join(out, "\n")
}

// renderDualPane lays the two panes side by side. Each pane box is rendered
// at Width(n) which yields n+2 total columns (2 border columns sit outside
// the width budget); the panes receive the content width n-2 so rows fill
// the box content area exactly. Both panes are viewport-scrolled to the
// available height so short terminals still navigate correctly.
func (m *Model) renderDualPane() string {
	paneW := m.width / 4 // workspace pane ≈ 20% per UI 规格 #1
	if paneW < 16 {
		paneW = 16
	}
	rightW := m.width - paneW - 4
	// Each pane gets the full height minus the status bar.
	maxLines := m.height - 1
	// Content width inside the border: Width(n).Render gives n+2 columns, so
	// content area is (n-2); pass the content width to the pane renderers.
	left := m.renderWorkspacePaneClipped(paneW-2, maxLines)
	right := m.renderTodoPaneClipped(rightW-2, maxLines)

	var combined string
	if m.focus == PaneWorkspace {
		combined = lipgloss.JoinHorizontal(lipgloss.Top,
			m.focusedBorder().Width(paneW).Render(left),
			m.dimBorder().Width(rightW).Render(right),
		)
	} else {
		combined = lipgloss.JoinHorizontal(lipgloss.Top,
			m.dimBorder().Width(paneW).Render(left),
			m.focusedBorder().Width(rightW).Render(right),
		)
	}
	return combined
}

// renderStacked lays the two panes vertically, giving ~70% of the height to
// the focused pane and the rest to the other. Each pane's rows are viewport
// clipped by its own scroll offset.
func (m *Model) renderStacked() string {
	statusH := 1
	avail := m.height - statusH
	focusH := avail * 7 / 10
	otherH := avail - focusH
	if otherH < 3 {
		otherH = 3
		focusH = avail - otherH
	}
	// Content width inside the bordered box: Width(n).Render yields n+2 total
	// columns (border outside), and the content area is n-4 (border 2 +
	// padding 2). Pass the content width to the pane renderers.
	contentW := m.width - 4
	boxW := m.width - 2

	var top, bottom string
	if m.focus == PaneWorkspace {
		top = m.focusedBorder().Width(boxW).Render(m.renderWorkspacePaneClipped(contentW, focusH))
		bottom = m.dimBorder().Width(boxW).Render(m.renderTodoPaneClipped(contentW, otherH))
	} else {
		top = m.dimBorder().Width(boxW).Render(m.renderWorkspacePaneClipped(contentW, otherH))
		bottom = m.focusedBorder().Width(boxW).Render(m.renderTodoPaneClipped(contentW, focusH))
	}
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

// renderWorkspacePaneClipped renders the workspace pane limited to maxLines
// rows, scrolled so the cursor row stays visible. The title line is pinned;
// only the content rows scroll.
func (m *Model) renderWorkspacePaneClipped(contentW, maxLines int) string {
	ws := m.VisibleWorkspaces()
	m.clampWorkspaceScroll(len(ws), maxLines)
	return m.renderWorkspacePaneViewport(contentW, m.workspaceScroll, maxLines)
}

// renderTodoPaneClipped renders the todo pane limited to maxLines rows,
// scrolled so the cursor row stays visible. The title line is pinned.
func (m *Model) renderTodoPaneClipped(contentW, maxLines int) string {
	todos := m.visibleTodos()
	m.clampTodoScroll(len(todos), maxLines)
	return m.renderTodoPaneViewport(contentW, m.todoScroll, maxLines)
}

// clampWorkspaceScroll keeps the viewport window positioned so the cursor
// row stays inside the visible maxLines rows. scroll counts content rows only
// (the title is pinned above the viewport).
func (m *Model) clampWorkspaceScroll(total, maxLines int) {
	if total == 0 {
		m.workspaceScroll = 0
		return
	}
	if maxLines <= 1 {
		m.workspaceScroll = 0
		return
	}
	// maxLines includes the pinned title, so the content window is one shorter.
	contentLines := maxLines - 1
	cursorLine := m.WorkspaceCursor
	m.workspaceScroll = clampScroll(m.workspaceScroll, cursorLine, total, contentLines)
}

// clampTodoScroll keeps the todo cursor row visible inside the viewport.
// scroll counts content rows only (the title is pinned).
func (m *Model) clampTodoScroll(total, maxLines int) {
	if total == 0 {
		m.todoScroll = 0
		return
	}
	if maxLines <= 1 {
		m.todoScroll = 0
		return
	}
	contentLines := maxLines - 1
	cursorLine := m.TodoCursor
	m.todoScroll = clampScroll(m.todoScroll, cursorLine, total, contentLines)
}

// clampScroll returns the scroll offset that keeps cursorLine (0-based in the
// content rows, title excluded) within a contentLines window.
func clampScroll(scroll, cursorLine, total, contentLines int) int {
	// Keep the cursor in [scroll, scroll+contentLines).
	if cursorLine < scroll {
		scroll = cursorLine
	}
	if cursorLine >= scroll+contentLines {
		scroll = cursorLine - contentLines + 1
	}
	// Clamp to valid range.
	maxScroll := total - contentLines
	if scroll < 0 {
		scroll = 0
	}
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}

// renderWorkspacePaneViewport renders the workspace pane for the window
// [scroll, scroll+contentLines). contentLines <= 0 renders every row.
func (m *Model) renderWorkspacePaneViewport(w, scroll, contentLines int) string {
	th := m.appTheme()
	title := th.Style("primary").Bold(true).Render("Workspaces")
	if m.root == nil {
		return title
	}
	ws := m.VisibleWorkspaces()
	if len(ws) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, m.dimStyle().Render("(no workspaces)"))
	}
	if m.WorkspaceCursor >= len(ws) {
		m.WorkspaceCursor = len(ws) - 1
	}
	if m.WorkspaceCursor < 0 {
		m.WorkspaceCursor = 0
	}
	// Window bounds over the content rows (index 0 == first workspace).
	// contentLines (the pane height) includes the pinned title, so the
	// scrollable content row count is one fewer. contentLines <= 0 renders
	// every row with no scrolling, so no scrollbar either.
	contentRows := contentLines - 1
	if contentRows < 1 {
		contentRows = 1
	}
	var thumb int
	hasScrollbar := false
	if contentLines > 1 {
		thumb, hasScrollbar = scrollbarThumb(len(ws), contentRows, scroll)
	}
	lo, hi := 0, len(ws)
	if contentLines > 0 {
		if scroll < 0 {
			scroll = 0
		}
		lo = scroll
		hi = scroll + contentLines - 1
		if hi > len(ws) {
			hi = len(ws)
		}
	}
	contentW := w
	if hasScrollbar {
		contentW = w - 1
	}
	lines := make([]string, 0, hi-lo+2)
	if hasScrollbar {
		title += strings.Split(m.scrollbarColumn(contentRows, thumb), "\n")[0]
	}
	lines = append(lines, title)
	for i := lo; i < hi; i++ {
		selected := i == m.WorkspaceCursor && m.focus == PaneWorkspace
		marker := "  "
		if selected {
			marker = th.Style("green").Render("> ")
		}
		// indent before marker so the cursor aligns with the row's text column.
		indent := strings.Repeat("  ", ws[i].NestLevel())
		// Inline edit: the focused row renders the text input instead of the row.
		if m.mode == ModeInsert && selected {
			row := indent + marker + m.input.View()
			if hasScrollbar {
				row = m.appendScrollbar(row, contentW, contentRows, thumb, i-lo)
			}
			lines = append(lines, row)
			continue
		}
		row := indent + marker + m.RenderRow(PaneWorkspace, i)
		if selected {
			row = m.renderSelectedRow(row, contentW)
		}
		if hasScrollbar {
			row = m.appendScrollbar(row, contentW, contentRows, thumb, i-lo)
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// renderTodoPane renders the full todo pane (all rows). Used by direct call
// sites and tests; the interactive view goes through renderTodoPaneViewport.
func (m *Model) renderTodoPane(w int) string {
	return m.renderTodoPaneViewport(w, 0, 0)
}

// renderTodoPaneViewport renders the todo pane for the window
// [scroll, scroll+contentLines). contentLines <= 0 renders every row (title
// pinned, all content). Column widths are budgeted from the FULL row set so
// they stay stable while scrolling.
func (m *Model) renderTodoPaneViewport(w, scroll, contentLines int) string {
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
		return lipgloss.JoinVertical(lipgloss.Left, title, m.dimStyle().Render("(no items to display)"))
	}
	if m.TodoCursor >= len(todos) {
		m.TodoCursor = len(todos) - 1
	}
	if m.TodoCursor < 0 {
		m.TodoCursor = 0
	}
	// Budget the columns for the pane content width. The marker ("> " or
	// "  ") and any nest indent are prepended in front of the columns below,
	// so we leave room for them by passing a reduced width to columnWidths.
	maxIndent := 0
	for _, t := range todos {
		if t.NestLevel() > maxIndent {
			maxIndent = t.NestLevel()
		}
	}
	markerW := 2
	// Reserve one column for the scrollbar when the content overflows the
	// viewport; the scrollbar column is appended after each row. contentLines
	// (the pane height) includes the pinned title, so the scrollable content
	// row count is one fewer. contentLines <= 0 renders every row with no
	// scrolling, so no scrollbar either.
	contentRows := contentLines - 1
	if contentRows < 1 {
		contentRows = 1
	}
	var thumb int
	hasScrollbar := false
	if contentLines > 1 {
		thumb, hasScrollbar = scrollbarThumb(len(todos), contentRows, scroll)
	}
	contentW := w
	if hasScrollbar {
		contentW = w - 1
	}
	budget := contentW - markerW - maxIndent*2
	if budget < 8 {
		budget = 8
	}
	widths := m.columnWidths(PaneTodo, budget)
	cols := m.visibleColumns(PaneTodo, budget)

	// Window bounds over the content rows (index 0 == first todo).
	lo, hi := 0, len(todos)
	if contentLines > 0 {
		if scroll < 0 {
			scroll = 0
		}
		lo = scroll
		hi = scroll + contentLines - 1 // -1 for the pinned title
		if hi > len(todos) {
			hi = len(todos)
		}
	}

	lines := make([]string, 0, hi-lo+2)
	if hasScrollbar {
		title += strings.Split(m.scrollbarColumn(contentRows, thumb), "\n")[0]
	}
	lines = append(lines, title)
	for i := lo; i < hi; i++ {
		selected := i == m.TodoCursor && m.focus == PaneTodo
		marker := "  "
		if selected {
			marker = th.Style("green").Render("> ")
		}
		// indent before marker so the cursor aligns with the row's text column.
		indent := strings.Repeat("  ", todos[i].NestLevel())
		// Inline edit: the whole row becomes the text input (full width), so
		// small columns like effort don't clip what the user is typing.
		if m.mode == ModeInsert && selected {
			row := indent + marker + m.input.View()
			if hasScrollbar {
				row = m.appendScrollbar(row, contentW, contentRows, thumb, i-lo)
			}
			lines = append(lines, row)
			continue
		}
		row := indent + marker + m.formatTodoAligned(todos[i], cols, widths)
		if selected {
			row = m.renderSelectedRow(row, contentW)
		}
		if hasScrollbar {
			row = m.appendScrollbar(row, contentW, contentRows, thumb, i-lo)
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// renderSelectedRow applies a full-row background highlight so the active
// row reads as selected at a glance, and pads with plain spaces to the pane
// width. The background is injected as raw ANSI so it survives the row's own
// color resets (a lipgloss .Background would be dropped by the first \x1b[0m
// inside a colored cell, leaving only the cursor arrow highlighted).
func (m *Model) renderSelectedRow(row string, w int) string {
	visible := lipgloss.Width(row)
	if pad := w - visible; pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	th := m.appTheme()
	bg := ansiBackground(th.Selection)
	if bg == "" {
		return row
	}
	reset := "\x1b[0m"
	// Re-apply the background after every reset so the highlight spans the row.
	return bg + strings.ReplaceAll(row, reset, reset+bg) + reset
}

// ansiBackground converts a #RRGGBB color to a 24-bit ANSI background sequence,
// or "" for an unparseable value.
func ansiBackground(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	r, err1 := strconv.ParseInt(hex[0:2], 16, 32)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 32)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 32)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// scrollbarThumb returns the content row (0..contentRows-1) the scrollbar
// thumb sits on for total items, contentRows visible content rows (title
// excluded), and the current scroll offset. ok=false when the content fits
// the view (no scrollbar needed).
func scrollbarThumb(total, contentRows, scroll int) (int, bool) {
	if contentRows <= 0 || total <= contentRows {
		return 0, false
	}
	maxScroll := total - contentRows
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	// Thumb position proportional to scroll progress across the content rows.
	pos := 0
	if maxScroll > 0 {
		pos = scroll * (contentRows - 1) / maxScroll
	}
	return pos, true
}

// scrollbarColumn renders a 1-column vertical scrollbar spanning the whole
// pane (title row + contentRows rows): a primary-colored thumb on its row,
// dim-colored track elsewhere. contentThumb is the content row the thumb
// sits on; it maps to pane row contentThumb+1 (row 0 is the pinned title).
func (m *Model) scrollbarColumn(contentRows, contentThumb int) string {
	th := m.appTheme()
	thumbStyle := th.Style("primary")
	trackStyle := th.Style("dim")
	rows := make([]string, 0, contentRows+1)
	for i := 0; i <= contentRows; i++ {
		if i == contentThumb+1 {
			rows = append(rows, thumbStyle.Render("█"))
		} else {
			rows = append(rows, trackStyle.Render("│"))
		}
	}
	return strings.Join(rows, "\n")
}

// appendScrollbar appends the scrollbar column row for content row idx to a
// pane row. contentRows is the visible content-row count, contentThumb the
// thumb's content row. Rows are padded to the pane content width first so the
// scrollbar sits flush against the pane's right edge on every row.
func (m *Model) appendScrollbar(row string, contentW, contentRows, contentThumb, idx int) string {
	row = padRight(row, contentW)
	bar := m.scrollbarColumn(contentRows, contentThumb)
	return row + strings.Split(bar, "\n")[idx+1]
}
