package app

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	glua "github.com/yuin/gopher-lua"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/XiaTian-AC/faster-dooit/internal/theme"
)

// Row rendering with a per-row cache keyed on (pane, model id, version).
// The 1s clock and bar refresh are decoupled from this cache: the clock
// never bumps the version, so row caches are not invalidated every second.

// appTheme resolves the active theme: the named preset as a base with any
// explicitly-configured color fields applied as overrides. Without config it
// returns the nord preset.
func (m *Model) appTheme() theme.Theme {
	if m.luaCfg == nil {
		th, _ := theme.Resolve("", nil)
		return th
	}
	th, err := theme.Resolve(m.luaCfg.Theme.Name, m.luaCfg.Theme.Explicit)
	if err != nil {
		// Unknown name is caught at config eval; fall back defensively.
		th, _ = theme.Resolve("", nil)
	}
	if len(m.luaCfg.Theme.UrgencyColors) > 0 {
		th.UrgencyColors = m.luaCfg.Theme.UrgencyColors
	}
	return th
}

// ColumnLayout returns the column order for a pane from config.lua, falling
// back to the dooit defaults.
func (m *Model) ColumnLayout(pane int) []string {
	if m.luaCfg != nil {
		switch pane {
		case PaneWorkspace:
			if len(m.luaCfg.Layouts.Workspace) > 0 {
				return m.luaCfg.Layouts.Workspace
			}
		case PaneTodo:
			if len(m.luaCfg.Layouts.Todo) > 0 {
				return m.luaCfg.Layouts.Todo
			}
		}
	}
	if pane == PaneWorkspace {
		return []string{"description"}
	}
	return []string{"status", "description", "due", "effort", "recurrence", "urgency"}
}

// RenderRow renders the row at idx in the given pane, using the row cache.
func (m *Model) RenderRow(pane int, idx int) string {
	var id int64
	var content string
	if pane == PaneWorkspace {
		ws := m.VisibleWorkspaces()
		if idx < 0 || idx >= len(ws) {
			return ""
		}
		id = ws[idx].ID
		content = m.formatWorkspace(ws[idx])
	} else {
		todos := m.visibleTodos()
		if idx < 0 || idx >= len(todos) {
			return ""
		}
		id = todos[idx].ID
		content = m.formatTodo(todos[idx])
	}
	key := cacheKey(pane, id, m.version)
	if m.rowCache == nil {
		m.rowCache = map[string]string{}
	}
	if s, ok := m.rowCache[key]; ok {
		return s
	}
	m.rowCache[key] = content
	return content
}

func cacheKey(pane int, id int64, version int64) string {
	return itoa(pane) + ":" + itoa(int(id)) + ":" + itoa(int(version))
}

// formatWorkspace renders a workspace row per its column layout.
func (m *Model) formatWorkspace(w *model.Workspace) string {
	cols := m.ColumnLayout(PaneWorkspace)
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		parts = append(parts, m.formatWorkspaceColumn(col, w))
	}
	return strings.Join(parts, " ")
}

func (m *Model) formatWorkspaceColumn(field string, w *model.Workspace) string {
	th := m.appTheme()
	switch field {
	case "description":
		return th.Style("primary").Render(w.Description)
	}
	return th.Style("primary").Render(w.Description)
}

// formatTodo renders a todo row per its column layout.
func (m *Model) formatTodo(t *model.Todo) string {
	cols := m.ColumnLayout(PaneTodo)
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		parts = append(parts, m.formatTodoColumn(col, t))
	}
	return strings.Join(parts, " ")
}

// columnWidths assigns a fixed width per column so rows line up like a table.
// description is elastic (absorbs the remaining budget); other columns are
// sized to their typical content. `cols` is the explicit column list (already
// narrowed by visibleColumns for the pane width and by activeColumns for the
// row's non-empty cells). `paneW` is the total width budget available for the
// columns (markers/indent are handled by the caller). When the budget is
// enough, widths are sized so sum(widths) + (ncols-1) gaps == paneW; with a
// tight budget the elastic columns are floored at 1 each, so the sum can
// exceed paneW (indent is accounted for by the caller).
func (m *Model) columnWidths(cols []string, paneW int) map[string]int {
	widths := make(map[string]int, len(cols))
	fixed := 0
	var elastic []string
	for _, c := range cols {
		if w := fixedWidth(c); w > 0 {
			widths[c] = w
			fixed += w
		} else {
			elastic = append(elastic, c)
		}
	}
	gaps := len(cols) - 1
	avail := paneW - fixed - gaps
	if len(elastic) > 0 {
		// Elastic columns absorb the remaining budget even when it is tight
		// (deep indent/markers already deducted by the caller); a fixed
		// fallback would overflow the pane. Floor at 1 so a cell stays visible.
		ew := avail / len(elastic)
		if ew < 1 {
			ew = 1
		}
		for _, c := range elastic {
			widths[c] = ew
		}
	}
	return widths
}

// minDescWidth is the minimum display width kept for the elastic description
// column before less-important fixed columns are dropped.
const minDescWidth = 10

// dropOrder lists columns from least to most important; when the pane is too
// narrow to keep every fixed column and leave minDescWidth for description,
// columns are removed in this order until the remaining fixed widths fit.
// description is elastic and always survives.
var dropOrder = []string{"urgency", "effort", "due", "recurrence"}

func fixedWidth(col string) int {
	switch col {
	case "status":
		return 1
	case "due":
		return 16
	case "effort":
		return 4
	case "recurrence":
		return 6
	case "urgency":
		return 4
	}
	return 0 // elastic (description)
}

// visibleColumns returns the columns to render for pane given the column
// budget paneW. It starts from the configured layout and drops the least
// important fixed columns (in dropOrder) until the fixed columns + gaps fit
// in paneW with at least minDescWidth left for the elastic description.
// The drop is always applied when the budget is tight, so narrow panes never
// overflow regardless of the overall layout mode.
func (m *Model) visibleColumns(pane int, paneW int) []string {
	cols := append([]string{}, m.ColumnLayout(pane)...)
	drop := append([]string{}, dropOrder...)
	for len(cols) > 0 {
		fixed := 0
		elastic := 0
		for _, c := range cols {
			if w := fixedWidth(c); w > 0 {
				fixed += w
			} else {
				elastic++
			}
		}
		gaps := len(cols) - 1
		if elastic == 0 || fixed+gaps+minDescWidth <= paneW {
			break
		}
		// Drop the least important fixed column still present.
		dropped := false
		for _, d := range drop {
			for i, c := range cols {
				if c == d {
					cols = append(cols[:i], cols[i+1:]...)
					dropped = true
					break
				}
			}
			if dropped {
				break
			}
		}
		if !dropped {
			break
		}
	}
	return cols
}

// activeColumns returns the columns whose cell is non-empty for this todo,
// so empty due/effort/recurrence columns don't consume width (and their gap).
// status and description are always kept.
func (m *Model) activeColumns(t *model.Todo, cols []string) []string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if c == "status" || c == "description" {
			out = append(out, c)
			continue
		}
		if m.formatTodoColumn(c, t) != "" {
			out = append(out, c)
		}
	}
	return out
}

// renderTodoRow joins the todo's active columns into one aligned row. The
// description cell is passed in (styled separately for collapsed vs expanded
// rendering); every cell is fit to its column width.
func (m *Model) renderTodoRow(t *model.Todo, cols []string, widths map[string]int, descCell string) string {
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		cell := descCell
		if col != "description" {
			cell = m.formatTodoColumn(col, t)
		}
		if w := widths[col]; w > 0 {
			cell = fitColumn(cell, w)
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, " ")
}

// formatTodoAligned renders a todo row with each column padded to a fixed
// width (a table layout). Empty due/effort/recurrence columns are dropped per
// row so the freed width (and gaps) goes to the elastic description column.
// An expanded long description returns multiple lines: the first keeps all
// columns, continuation lines show only the description indented to its
// column (bounded by maxDescLines; 0 = never ellipsized).
func (m *Model) formatTodoAligned(t *model.Todo, cols []string, paneW int) []string {
	active := m.activeColumns(t, cols)
	widths := m.columnWidths(active, paneW)

	if !m.expandedDesc[t.ID] {
		return []string{m.renderTodoRow(t, active, widths, m.formatTodoColumn("description", t))}
	}

	descW := widths["description"]
	if descW < 1 {
		descW = 1
	}
	wrapped := wrapDescription(t.Description, descW)
	// The expanded first line uses the first wrap fragment (styled) so line 0
	// and the continuation lines agree on the split point.
	first := m.renderTodoRow(t, active, widths, m.appTheme().Style("primary").Render(wrapped[0]))

	out := []string{first}
	maxLines := m.maxDescLines
	if maxLines > 0 {
		for i := 1; i < len(wrapped) && len(out) < maxLines; i++ {
			out = append(out, wrapped[i])
		}
		// More description remains than fits: the last rendered line signals it
		// with an ellipsis.
		if len(wrapped) > len(out) {
			out[len(out)-1] = ellipsizeTail(out[len(out)-1], descW)
		}
	} else {
		// maxDescLines == 0: never ellipsize — always show the full
		// description, even if it pushes the row past the default line count.
		out = append(out, wrapped[1:]...)
	}

	// Continuation lines align under the first line's description column
	// (marker/indent are prepended by the caller on every line).
	offset := 0
	for _, c := range active {
		if c == "description" {
			break
		}
		offset += widths[c] + 1 // +1 gap
	}
	for i := 1; i < len(out); i++ {
		out[i] = strings.Repeat(" ", offset) + out[i]
	}
	return out
}

// wrapDescription splits a description into display lines of at most w
// columns, word-wrapping where possible and hard-breaking over-wide fragments
// (e.g. CJK text without spaces). Preserves words for space-separated text.
func wrapDescription(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	if s == "" {
		return []string{""}
	}
	var out []string
	for _, ln := range strings.Split(ansi.Wordwrap(s, w, " "), "\n") {
		if lipgloss.Width(ln) <= w {
			out = append(out, ln)
			continue
		}
		for _, frag := range strings.Split(ansi.Hardwrap(ln, w, false), "\n") {
			out = append(out, frag)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// ellipsizeTail appends (or squeezes in) an ellipsis at the end of a
// truncated line, padded to w columns so the "…" visibly signals more text.
func ellipsizeTail(s string, w int) string {
	if cur := lipgloss.Width(s); cur < w {
		return padRight(s+"…", w)
	}
	// s fills (or exceeds) the width: keep w-1 columns plus the ellipsis.
	// (ansi.Truncate would return s unchanged when it already fits.)
	return padRight(ansi.Truncate(s, w-1, "")+"…", w)
}

// fitColumn clips a cell to at most n visible columns (with an ellipsis) and
// pads it up to n — keeping rows from overflowing the pane and breaking to the
// next terminal line. Truncation is by display width (full-width CJK chars
// count as 2), not rune count.
func fitColumn(s string, n int) string {
	if cur := lipgloss.Width(s); cur > n {
		if n > 1 {
			s = ansi.Truncate(s, n, "…")
		} else {
			s = "…"
		}
	}
	return padRight(s, n)
}

// truncateByWidth returns the longest prefix of s whose display width is at
// most n columns, handling full-width characters and ANSI escape sequences
// (styled cells must keep their escape codes intact).
func truncateByWidth(s string, n int) string {
	return ansi.Truncate(s, n, "")
}

// padRight pads s (visible width) to at least n columns with trailing spaces.
func padRight(s string, n int) string {
	cur := lipgloss.Width(s)
	if cur >= n {
		return s
	}
	return s + strings.Repeat(" ", n-cur)
}

func (m *Model) formatTodoColumn(field string, t *model.Todo) string {
	th := m.appTheme()

	// Prefer a Lua formatter registered for this field.
	if m.luaCfg != nil {
		var fns []*glua.LFunction
		switch field {
		case "status":
			fns = m.luaCfg.Formatters.Todos.Status
		case "description":
			fns = m.luaCfg.Formatters.Todos.Description
		case "due":
			fns = m.luaCfg.Formatters.Todos.Due
		case "urgency":
			fns = m.luaCfg.Formatters.Todos.Urgency
		case "effort":
			fns = m.luaCfg.Formatters.Todos.Effort
		case "recurrence":
			fns = m.luaCfg.Formatters.Todos.Recurrence
		}
		// Try in reverse registration order (matches formatter_store.py).
		for i := len(fns) - 1; i >= 0; i-- {
			val := fieldValue(field, t)
			text, style, err := m.luaCfg.CallFormatter(fns[i], val, t, m.luaCfg.Theme)
			if err == nil && text != "" {
				// The formatter's style is a hex color (config.lua returns
				// theme.<color>); fall back to primary when empty.
				if style == "" {
					style = th.Primary
				}
				return lipgloss.NewStyle().Foreground(lipgloss.Color(style)).Render(text)
			}
		}
	}

	// Built-in formatter fallback.
	switch field {
	case "status":
		switch t.Status() {
		case "completed":
			return th.Style("green").Bold(true).Render("x")
		case "overdue":
			return th.Style("red").Bold(true).Render("!")
		default:
			return th.Style("yellow").Bold(true).Render("o")
		}
	case "description":
		return th.Style("primary").Render(t.Description)
	case "due":
		if t.Due == nil {
			return ""
		}
		return th.Style("secondary").Render(dueString(*t.Due))
	case "urgency":
		if t.Urgency == 0 {
			return ""
		}
		col := m.appTheme().UrgencyColor(t.Urgency)
		return lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Render("!" + itoa(t.Urgency))
	case "effort":
		if t.Effort == 0 {
			return ""
		}
		return th.Style("secondary").Render(itoa(t.Effort))
	case "recurrence":
		if t.Recurrence == nil {
			return ""
		}
		return th.Style("secondary").Render(durationString(*t.Recurrence))
	}
	return ""
}

func fieldValue(field string, t *model.Todo) any {
	switch field {
	case "status":
		return t.Status()
	case "description":
		return t.Description
	case "due":
		if t.Due == nil {
			return nil
		}
		return dueString(*t.Due)
	case "urgency":
		return t.Urgency
	case "effort":
		return t.Effort
	case "recurrence":
		if t.Recurrence == nil {
			return nil
		}
		return durationString(*t.Recurrence)
	}
	return ""
}

// dueString renders a due time as YYYY-MM-DD [HH:MM].
func dueString(d time.Time) string {
	if d.Hour() != 0 || d.Minute() != 0 {
		return d.Format("2006-01-02 15:04")
	}
	return d.Format("2006-01-02")
}

// DashboardLines returns the config dashboard content.
func (m *Model) DashboardLines() []string {
	if m.luaCfg != nil && len(m.luaCfg.Dashboard) > 0 {
		return m.luaCfg.Dashboard
	}
	return []string{"Welcome to Faster Dooit!", "", "Press '?' for help."}
}

// renderRowMarker returns the leading marker slot for a row: the fold arrow
// for collapsible nodes, blank for leaves. Collapsed nodes show ">", expanded
// nodes show "⌄" (a downward-pointing caret). Selection is conveyed by the row
// highlight, not by a cursor arrow here.
func (m *Model) renderRowMarker(hasChildren, expanded bool) string {
	if !hasChildren {
		return "  "
	}
	ch := ">"
	if expanded {
		ch = "⌄"
	}
	return m.appTheme().Style("primary").Render(ch + " ")
}
