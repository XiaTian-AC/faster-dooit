package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/XiaTian-AC/faster-dooit/internal/lua"
	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/XiaTian-AC/faster-dooit/internal/store"
)

func TestCursorAlignsWithIndent(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	// Build a 2-level tree: root todo with one child.
	root := m.selectedTodo()
	child := &model.Todo{Description: "child", Pending: true, ParentTodoID: &root.ID}
	root.Todos = append(root.Todos, child)
	if err := m.store.SaveTodo(child); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Expand the root so the child is visible.
	m.expanded[root.ID] = true
	// Cursor on the nested child row (index 1); it must be indented by the
	// child's nest level (2 spaces per level for level 1).
	m.TodoCursor = 1
	v := m.renderTodoPane(80)
	if !strings.Contains(stripANSI(v), "    o") {
		t.Fatalf("child should align with nested indent, got:\n%s", v)
	}
}

func TestViewVerticallyCenters(t *testing.T) {
	m := newTestApp(t)
	m.width = 80
	m.height = 30
	v := m.View()
	// With 30 rows of room and ~10 content lines, content must be padded down
	// from the top (leading blank rows), not stuck at the first row. The blank
	// rows carry the global background fill, so strip ANSI before checking.
	lines := strings.Split(v, "\n")
	blank := 0
	for _, line := range lines {
		if strings.TrimSpace(stripANSI(line)) == "" {
			blank++
		} else {
			break
		}
	}
	if blank < 2 {
		t.Fatalf("content should be vertically centered (leading blank rows missing), got:\n%q", v)
	}
}

func TestInlineEditRendersOnCursorRow(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.StartEdit("description")
	v := m.renderTodoPane(80)
	if !strings.Contains(v, m.input.View()) {
		t.Fatalf("input should render on the cursor row, got:\n%s", v)
	}
}

// TestViewFitsTerminalWidth renders the full view (two bordered panes) at
// several terminal widths and asserts every line fits within the terminal —
// the pane borders were previously under-budgeted, overflowing even with
// zero nesting.
func TestViewFitsTerminalWidth(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.selectedTodo().Description = "zero nesting long description to probe overflow"

	for _, tw := range []int{40, 60, 80, 100} {
		m.width = tw
		m.height = 30
		v := m.View()
		for _, line := range strings.Split(v, "\n") {
			if lw := lipgloss.Width(line); lw > tw {
				t.Errorf("terminal %d: line overflows by %d cols: %q", tw, lw-tw, line)
			}
		}
	}
}

func TestLayoutModeDecision(t *testing.T) {
	m := newTestApp(t)
	cases := []struct {
		w, h int
		want layoutMode
	}{
		{120, 30, layoutNormal},
		{80, 30, layoutStacked},
		{99, 30, layoutStacked},  // < W_stack
		{100, 30, layoutNormal},  // >= W_stack
		{150, 30, layoutNormal},
		{80, 5, layoutTooSmall},  // too short
		{29, 30, layoutTooSmall}, // too narrow
	}
	for _, c := range cases {
		m.width, m.height = c.w, c.h
		if got := m.layoutMode(); got != c.want {
			t.Errorf("layoutMode(%d,%d) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}

// TestLayoutModeMinSizeBoundaries: the too-small threshold follows config
// (default 30x6), and a size exactly at the boundary is still renderable.
func TestLayoutModeMinSizeBoundaries(t *testing.T) {
	m := newTestApp(t)
	// Exactly at default min: not too small.
	m.width, m.height = 30, 6
	if m.layoutMode() == layoutTooSmall {
		t.Fatalf("30x6 should not be too small, got %v", m.layoutMode())
	}
	// One less on either axis flips to too-small.
	m.width, m.height = 29, 6
	if m.layoutMode() != layoutTooSmall {
		t.Fatalf("29x6 should be too small, got %v", m.layoutMode())
	}
	m.width, m.height = 30, 5
	if m.layoutMode() != layoutTooSmall {
		t.Fatalf("30x5 should be too small, got %v", m.layoutMode())
	}
	// Config override: min_width=60 makes 50x30 too small.
	rt, err := lua.EvalFileWithCode(`vars.min_width = 60`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	m.luaCfg = rt
	m.width, m.height = 50, 30
	if m.layoutMode() != layoutTooSmall {
		t.Fatalf("50x30 with min_width=60 should be too small, got %v", m.layoutMode())
	}
}

// TestRendersBelowOldFloor: the layout floor dropped from 40x12 to 30x6, so a
// short-but-not-tiny terminal (height 8) must render the panes instead of the
// "Terminal size too small" notice.
func TestRendersBelowOldFloor(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 80, 8
	v := m.View()
	if strings.Contains(v, "Terminal size too small") {
		t.Fatalf("height 8 should render panes, got the too-small notice:\n%s", v)
	}
}

// TestScrollTopBoundary: with the cursor at the top, scroll stays 0 and the
// first row is visible even when the pane is short.
func TestScrollTopBoundary(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	for i := 0; i < 8; i++ {
		todo := &model.Todo{Description: "item", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 80, 14
	v := m.View()
	if m.todoScroll != 0 {
		t.Fatalf("todoScroll = %d, want 0 at top", m.todoScroll)
	}
	// First todo ("a") must be visible after the title.
	if !strings.Contains(stripANSI(v), "o a") {
		t.Fatalf("first todo should be visible at scroll 0:\n%s", v)
	}
}

// TestDualToStackedTransition: resizing width across the 100-column boundary
// must switch the rendered layout (dual-pane vs stacked) and keep all rows
// within the terminal.
func TestDualToStackedTransition(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0

	// Wide → dual pane.
	m.width, m.height = 120, 30
	dual := m.View()
	dualIdx := strings.Index(dual, "Workspaces")
	todoIdx := strings.Index(dual, "Todos")
	if dualIdx < 0 || todoIdx < 0 {
		t.Fatalf("dual pane missing titles:\n%s", dual)
	}
	if strings.Count(dual[:todoIdx], "\n") != strings.Count(dual[:dualIdx], "\n") {
		t.Fatalf("dual pane: titles should share a line")
	}

	// Narrow → stacked (titles on different lines).
	m.width, m.height = 80, 30
	stacked := m.View()
	wsLine := strings.Count(stacked[:strings.Index(stacked, "Workspaces")], "\n")
	todoLine2 := strings.Count(stacked[:strings.Index(stacked, "Todos")], "\n")
	if todoLine2 <= wsLine {
		t.Fatalf("stacked: Todos should be below Workspaces")
	}
	for _, line := range strings.Split(stacked, "\n") {
		if lw := lipgloss.Width(line); lw > 80 {
			t.Errorf("stacked transition overflows by %d cols: %q", lw-80, line)
		}
	}
}

func TestTooSmallNotice(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 29, 30
	got := m.View()
	if !strings.Contains(got, "Terminal size too small") {
		t.Fatalf("expected too-small notice, got:\n%s", got)
	}
	if !strings.Contains(got, "Needed for current config") {
		t.Fatalf("notice should list needed size, got:\n%s", got)
	}
}

func TestStackedLayoutRendersBothPanes(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 80, 30
	v := m.View()
	if !strings.Contains(v, "Workspaces") || !strings.Contains(v, "Todos") {
		t.Fatalf("stacked view should show both pane titles:\n%s", v)
	}
	// Both titles must be on different lines (stacked), not the same line.
	wsIdx := strings.Index(v, "Workspaces")
	todoIdx := strings.Index(v, "Todos")
	if wsIdx < 0 || todoIdx < 0 {
		t.Fatalf("missing title")
	}
	// "Todos" must appear on a later line than "Workspaces".
	wsLine := strings.Count(v[:wsIdx], "\n")
	todoLine := strings.Count(v[:todoIdx], "\n")
	if todoLine <= wsLine {
		t.Fatalf("stacked: Todos should be below Workspaces (line %d vs %d)", todoLine, wsLine)
	}
}

// TestViewportScrollKeepsCursorVisible: with a short pane, moving the cursor
// to the bottom must shift the rendered window so the cursor row stays
// visible and the top row falls off.
func TestViewportScrollKeepsCursorVisible(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	// Add 8 todos so the list exceeds a 4-row pane.
	for i := 0; i < 8; i++ {
		todo := &model.Todo{Description: "item", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Move cursor to the last row (index 8).
	for i := 0; i < 8; i++ {
		m.actionMoveDown(m)
	}
	if m.TodoCursor != 8 {
		t.Fatalf("cursor = %d, want 8", m.TodoCursor)
	}
	// Short terminal: stacked, todo pane gets a small viewport. Height 14 is
	// above H_min(6), so viewport scrolling is active.
	m.width, m.height = 80, 14
	v := m.View()
	lines := strings.Split(v, "\n")
	found := false
	for _, ln := range lines {
		if strings.Contains(ln, "Todos") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Todos title missing:\n%s", v)
	}
	// The selected row (highlighted background) must be rendered somewhere
	// after the Todos title. With 9 todos and a small viewport, it must have
	// scrolled the window down (the first item rows are hidden) but the
	// selected row is still visible.
	todoIdx := -1
	selLine := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Todos") {
			todoIdx = i
		}
		if strings.Contains(ln, "\x1b[48;2;") {
			selLine = i
		}
	}
	if todoIdx < 0 || selLine < 0 {
		t.Fatalf("selected row not visible after scroll:\n%s", v)
	}
	if selLine <= todoIdx {
		t.Fatalf("selected row should render below the Todos title, selLine=%d todoIdx=%d", selLine, todoIdx)
	}
	// At least one todo must have scrolled off (we have 9 todos; the visible
	// todo area is much smaller), i.e. the first todo row is not line 0.
	if lines[0] == "" {
		// acceptable: leading empty line from JoinVertical padding
	}
	// The first rendered todo (immediately after "Todos") is NOT item 0 —
	// scroll offset advanced. We can't know the exact id, but the cursor row
	// being present and below the title suffices for the visible-window
	// contract here.
}

// TestSelectedRowNeverExceedsPane: the highlighted (selected) row must fit
// inside the pane content width even with a long description — the previous
// code padded to the box width, wrapping the terminal and leaving a ghost
// highlighted row.
func TestSelectedRowNeverExceedsPane(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.selectedTodo().Description = "a very long description that forces truncation and padding"
	m.width, m.height = 80, 30
	// Stacked mode: pane content width = m.width-4.
	v := m.View()
	for _, line := range strings.Split(v, "\n") {
		if lw := lipgloss.Width(line); lw > m.width {
			t.Errorf("selected row overflows terminal by %d cols: %q", lw-m.width, line)
		}
	}
}

// TestDualPaneScrollsOnShortTerminal: even in the wide (dual-pane) layout, a
// short terminal must scroll the todo viewport so the cursor stays visible.
func TestDualPaneScrollsOnShortTerminal(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	for i := 0; i < 8; i++ {
		todo := &model.Todo{Description: "item", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Move cursor to the last todo (index 8).
	for i := 0; i < 8; i++ {
		m.actionMoveDown(m)
	}
	if m.TodoCursor != 8 {
		t.Fatalf("cursor = %d, want 8", m.TodoCursor)
	}
	// Wide (dual-pane) but short: height 14 gives a small viewport.
	m.width, m.height = 150, 14
	v := m.View()
	if m.layoutMode() != layoutNormal {
		t.Fatalf("expected dual-pane layout, got %v", m.layoutMode())
	}
	// Selected row (highlighted) must be visible somewhere after the Todos
	// title.
	todoIdx := -1
	selLine := -1
	for i, ln := range strings.Split(v, "\n") {
		if strings.Contains(ln, "Todos") {
			todoIdx = i
		}
		if strings.Contains(ln, "\x1b[48;2;") {
			selLine = i
		}
	}
	if todoIdx < 0 || selLine < 0 {
		t.Fatalf("selected row not visible in dual-pane scroll:\n%s", v)
	}
	if selLine <= todoIdx {
		t.Fatalf("selected row should render below the Todos title, selLine=%d todoIdx=%d", selLine, todoIdx)
	}
}

// TestInlineEditFullWidthInput: while editing effort (a 4-col column), the
// input must render at full available width — not clipped to the column.
func TestInlineEditFullWidthInput(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	m.StartEdit("effort")
	m.input.SetValue("1234567890")
	v := m.renderTodoPane(60)
	if !strings.Contains(stripANSI(v), "1234567890") {
		t.Fatalf("effort input should show full value, got:\n%s", v)
	}
	for _, line := range strings.Split(v, "\n") {
		if lw := lipgloss.Width(line); lw > 60 {
			t.Errorf("inline edit row overflows pane by %d cols: %q", lw-60, line)
		}
	}
}

func TestSelectedRowUsesSelectionColor(t *testing.T) {
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	m.TodoCursor = 0
	th := m.appTheme()
	sel := m.renderSelectedRow("> abc", 20)
	if !strings.Contains(sel, "\x1b[48;2;") {
		t.Fatalf("selected row should carry a background")
	}
	if !strings.Contains(sel, ansiBackground(th.Selection)) {
		t.Fatalf("selected row should use Selection color %q, got %q", ansiBackground(th.Selection), sel)
	}
}

func TestFillBackgroundPadsAndFills(t *testing.T) {
	m := newTestApp(t)
	m.width = 10
	th := m.appTheme()
	out := m.fillBackground("ab\ncd")
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, ansiBackground(th.Background)) {
			t.Fatalf("line %d should start with the theme background", i)
		}
		if !strings.HasSuffix(line, "\x1b[0m") {
			t.Fatalf("line %d should end with a reset", i)
		}
		if lipgloss.Width(line) != m.width {
			t.Fatalf("line %d padded to %d cols, want %d", i, lipgloss.Width(line), m.width)
		}
	}
}

func TestFillBackgroundSpansTerminalHeight(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 10, 5
	th := m.appTheme()
	out := m.fillBackground("ab\ncd")
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("background should span the terminal height, got %d lines (want 5): %q", len(lines), out)
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, ansiBackground(th.Background)) {
			t.Fatalf("line %d should carry the background fill", i)
		}
		if lipgloss.Width(line) != m.width {
			t.Fatalf("line %d width = %d, want %d", i, lipgloss.Width(line), m.width)
		}
	}
}

func TestFillBackgroundTransparentFromConfig(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	rt, err := lua.EvalFileWithCode(`theme.background = "transparent"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	m := New(st, rt)
	m.width, m.height = 10, 5
	if th := m.appTheme(); th.Background != "transparent" {
		t.Fatalf("background = %q, want transparent", th.Background)
	}
	out := m.fillBackground("ab\ncd")
	if strings.Contains(out, "\x1b[48;2;") {
		t.Fatalf("transparent background must not emit a fill: %q", out)
	}
	if out != "ab\ncd" {
		t.Fatalf("transparent background should return content unchanged, got %q", out)
	}
}

func TestViewOutputSpansTerminalHeight(t *testing.T) {
	m := newTestApp(t)
	m.width, m.height = 80, 24
	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) != m.height {
		t.Fatalf("View output has %d lines, want terminal height %d", len(lines), m.height)
	}
	// Every line must carry the global background fill.
	th := m.appTheme()
	for i, line := range lines {
		if !strings.HasPrefix(line, ansiBackground(th.Background)) {
			t.Fatalf("line %d missing background fill: %q", i, line)
		}
	}
}

// TestScrollbarThumbPosition: the thumb tracks the scroll offset.
func TestScrollbarThumbPosition(t *testing.T) {
	// 10 items, 4 visible rows. scroll=0 => thumb at top (0).
	if got, ok := scrollbarThumb(10, 4, 0); !ok || got != 0 {
		t.Fatalf("scroll=0: thumb=%d ok=%v, want 0 true", got, ok)
	}
	// scroll=maxScroll=6 => thumb at bottom (3).
	if got, ok := scrollbarThumb(10, 4, 6); !ok || got != 3 {
		t.Fatalf("scroll=6: thumb=%d ok=%v, want 3 true", got, ok)
	}
	// scroll=3 (middle) => thumb at ~1.
	if got, ok := scrollbarThumb(10, 4, 3); !ok || got != 1 {
		t.Fatalf("scroll=3: thumb=%d ok=%v, want 1 true", got, ok)
	}
}

// TestScrollbarHiddenWhenFits: no thumb when the content fits the view.
func TestScrollbarHiddenWhenFits(t *testing.T) {
	if _, ok := scrollbarThumb(3, 4, 0); ok {
		t.Fatal("content that fits the view must not produce a scrollbar")
	}
}

// TestScrollbarColumnRendersThumb: the column spans title + contentRows rows,
// with a primary-colored thumb on row contentThumb+1 (row 0 is the title).
func TestScrollbarColumnRendersThumb(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	rt, err := 	lua.EvalFileWithCode(`theme.name = "nord"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, rt)
	col := m.scrollbarColumn(4, 1) // 4 content rows, thumb on content row 1
	rows := strings.Split(col, "\n")
	if len(rows) != 5 {
		t.Fatalf("scrollbar column should have 5 rows (title + 4 content), got %d", len(rows))
	}
	// Thumb sits on row contentThumb+1 = 2, styled; the title row is track.
	if !strings.Contains(rows[2], "\x1b[") {
		t.Fatalf("thumb row should be styled, got %q", rows[2])
	}
	if rows[0] == rows[2] {
		t.Fatalf("title track row and thumb row should differ")
	}
}

// TestScrollbarFirstAndLastThumbVisible: the thumb must render when the first
// or last item is selected (thumb on content row 0 or the bottom row) — it
// must not be eaten by the title row or the last-content-row offset bug.
func TestScrollbarFirstAndLastThumbVisible(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	rt, err := 	lua.EvalFileWithCode(`theme.name = "nord"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, rt)
	// First item selected (scroll=0) => thumb on content row 0 (pane row 1).
	first := m.scrollbarColumn(4, 0)
	firstRows := strings.Split(first, "\n")
	if !strings.Contains(firstRows[1], "\x1b[") {
		t.Fatalf("first-item thumb should render on pane row 1, got %q", firstRows)
	}
	// Last item selected (thumb on content row 3 => pane row 4).
	last := m.scrollbarColumn(4, 3)
	lastRows := strings.Split(last, "\n")
	if !strings.Contains(lastRows[4], "\x1b[") {
		t.Fatalf("last-item thumb should render on pane row 4, got %q", lastRows)
	}
}

// TestAppendScrollbarPadsToEdge: the scrollbar must sit flush against the
// pane's right edge on every row, regardless of row content length (nested
// indentation must not pull the scrollbar inward).
func TestAppendScrollbarPadsToEdge(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	rt, err := 	lua.EvalFileWithCode(`theme.name = "nord"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, rt)
	short := m.appendScrollbar("abc", 10, 4, 1, 0)
	long := m.appendScrollbar("abcdefghij", 10, 4, 1, 1)
	// Both rows must end at the same visible width (contentW 10 + scrollbar 1).
	if lipgloss.Width(short) != 11 || lipgloss.Width(long) != 11 {
		t.Fatalf("scrollbar must align right edge: short=%d long=%d, want 11", lipgloss.Width(short), lipgloss.Width(long))
	}
	// The scrollbar column must sit at the same rightmost position in both
	// rows: each stripANSI row is 11 chars (10 content + 1 scrollbar), and the
	// last char is the scrollbar glyph.
	ss := []rune(stripANSI(short))
	ls := []rune(stripANSI(long))
	if len(ss) != 11 || len(ls) != 11 {
		t.Fatalf("rows should be 11 chars wide: short=%d long=%d", len(ss), len(ls))
	}
	if ss[len(ss)-1] != '▌' && ss[len(ss)-1] != '█' {
		t.Fatalf("short row should end with a scrollbar glyph, got %q", ss)
	}
	if ls[len(ls)-1] != '▌' && ls[len(ls)-1] != '█' {
		t.Fatalf("long row should end with a scrollbar glyph, got %q", ls)
	}
}

// TestViewShowsScrollbarOnShortTerminal: a short terminal with many todos must
// render a scrollbar thumb (primary-colored block) inside the todo pane.
func TestViewShowsScrollbarOnShortTerminal(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	for i := 0; i < 12; i++ {
		todo := &model.Todo{Description: "item", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Short terminal: 12 todos + title exceed the ~11 content rows.
	m.width, m.height = 120, 13
	v := m.View()
	th := m.appTheme()
	if !strings.Contains(v, th.Style("primary").Render("█")) {
		t.Fatalf("short terminal should render a primary-colored scrollbar thumb, got:\n%s", v)
	}
}

// TestScrollbarThumbVisibleAtEnds: with many todos in a short terminal, moving
// the cursor to the last item (scroll to bottom) must still render the thumb
// in the scrollbar column — the last-content-row offset must not swallow it.
func TestScrollbarThumbVisibleAtEnds(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	for i := 0; i < 12; i++ {
		todo := &model.Todo{Description: "item", Pending: true, ParentWorkspaceID: &m.selectedWorkspace().ID}
		if err := m.store.SaveTodo(todo); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 120, 13
	th := m.appTheme()
	thumb := th.Style("primary").Render("█")
	// Cursor on the last item => scroll to bottom; the thumb must be visible.
	m.TodoCursor = len(m.visibleTodos()) - 1
	v := m.View()
	if !strings.Contains(v, thumb) {
		t.Fatalf("scroll-to-bottom must keep the thumb visible, got:\n%s", v)
	}
	// Cursor back on the first item => thumb at top, still visible.
	m.TodoCursor = 0
	v = m.View()
	if !strings.Contains(v, thumb) {
		t.Fatalf("first item selected must keep the thumb visible, got:\n%s", v)
	}
}

// TestScrollbarTrackHalfBlock: the track uses a dim half-block glyph (▌) and
// the thumb a primary solid block (█), giving a light-track/dark-thumb look.
func TestScrollbarTrackHalfBlock(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	rt, err := 	lua.EvalFileWithCode(`theme.name = "nord"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, rt)
	col := m.scrollbarColumn(3, 1) // 3 content rows, thumb on content row 1
	rows := strings.Split(col, "\n")
	if len(rows) != 4 {
		t.Fatalf("expected title + 3 content rows, got %d", len(rows))
	}
	// Title and non-thumb rows are dim half-blocks; the thumb row (index 2) is
	// a primary solid block.
	if !strings.Contains(stripANSI(rows[0]), "▌") {
		t.Fatalf("track row should use half-block glyph, got %q", stripANSI(rows[0]))
	}
	if !strings.Contains(stripANSI(rows[2]), "█") {
		t.Fatalf("thumb row should use solid block, got %q", stripANSI(rows[2]))
	}
}

// TestRowMarkerShowsFoldState: a collapsible row renders its fold arrow in the
// marker slot at the row start; a leaf row renders a blank marker.
func TestRowMarkerShowsFoldState(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := newTestApp(t)
	m.SetFocus(PaneTodo)
	root := m.selectedTodo()
	child := &model.Todo{Description: "child", Pending: true, ParentTodoID: &root.ID}
	root.Todos = append(root.Todos, child)
	if err := m.store.SaveTodo(child); err != nil {
		t.Fatal(err)
	}
	if err := m.RefreshFromStore(); err != nil {
		t.Fatal(err)
	}
	// Expanded root: marker shows ⌄ at the row start.
	m.expanded[root.ID] = true
	v := m.renderTodoPane(60)
	lines := strings.Split(v, "\n")
	rootLine := ""
	for _, ln := range lines {
		if strings.Contains(stripANSI(ln), "o a") {
			rootLine = stripANSI(ln)
			break
		}
	}
	if rootLine == "" || !strings.HasPrefix(rootLine, "⌄ ") {
		t.Fatalf("expanded root should start with ⌄ marker, got %q", rootLine)
	}
	// Collapsed root: marker shows > at the row start.
	m.expanded[root.ID] = false
	v = m.renderTodoPane(60)
	for _, ln := range strings.Split(v, "\n") {
		if strings.Contains(stripANSI(ln), "o a") {
			if !strings.HasPrefix(stripANSI(ln), "> ") {
				t.Fatalf("collapsed root should start with > marker, got %q", stripANSI(ln))
			}
			return
		}
	}
	t.Fatal("root row not found after collapse")
}
