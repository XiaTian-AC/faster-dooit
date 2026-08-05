package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/XiaTian-AC/faster-dooit/internal/lua"
	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/XiaTian-AC/faster-dooit/internal/store"
)

// Action is the runtime action registered against a key sequence.
type Action func(*Model) tea.Cmd

// Model is the bubbletea Elm model for the TUI.
type Model struct {
	store *store.Store

	root *model.Workspace

	// focus identifies the active pane (workspace or todo).
	focus int

	// mode is the current input mode.
	mode Mode

	// Per-pane cursors.
	WorkspaceCursor int
	TodoCursor      int

	// Per-pane viewport scroll offsets for short terminals.
	workspaceScroll int
	todoScroll      int

	// pendingResize debounces window resizes: during a drag the terminal
	// reports many sizes in quick succession; we only apply the final one.
	pendingResize *pendingResizeState

	// expanded[id] = true if the node is expanded in its tree view.
	expanded map[int64]bool

	// selectedWorkspaceID is the workspace currently shown in the todo pane.
	selectedWorkspaceID int64

	// clipboard holds a clipped model id + kind for paste-bellow/above.
	clipboard *clipboardEntry

	// keys is the active key manager (single-table for the skeleton).
	keys *keyManager

	// actions is the action registry keyed by action name.
	actions map[string]Action

	// width / height of the rendered area, updated on WindowSizeMsg.
	width  int
	height int

	// version is incremented on every state change; renderers/tests can
	// cache by it. Decoupled from the 1s clock tick.
	version int64

	// notification message displayed in the status line.
	notice      string
	noticeLevel string

	// input is the active text input overlay for INSERT/SEARCH/SORT modes.
	input textinput.Model

	// editField is the model field being edited (description/due/effort/...).
	editField string

	// editPlaceholder is the placeholder shown for a freshly-created item's
	// inline edit ("" for a normal edit of an existing value).
	editPlaceholder string

	// filter is the active search filter (Task 6 narrows the visible list).
	filter string

	// confirmCallback runs when the confirm dialog is confirmed (y/enter).
	confirmCallback func() tea.Cmd

	// helpVisible toggles the help screen (see actionShowHelp).
	helpVisible bool

	// luaCfg holds the evaluated config.lua runtime (may be nil when no
	// config was loaded — the skeleton uses its built-in defaults).
	luaCfg *lua.Runtime

	// rowCache caches rendered rows keyed by (pane, id, version). Decoupled
	// from the 1s clock tick so it is not invalidated every second.
	rowCache map[string]string

	// quitting is set on ctrl+q to break out of the Update loop.
	quitting bool
}

type clipboardEntry struct {
	kind string // "workspace" or "todo"
	id   int64
}

// pendingResizeState holds a debounced terminal-size change.
type pendingResizeState struct {
	w, h int
}

// New constructs the bubbletea model wired to the store and optional Lua
// config runtime.
func New(st *store.Store, luaCfg *lua.Runtime) *Model {
	m := &Model{
		store:    st,
		luaCfg:   luaCfg,
		focus:    PaneWorkspace,
		mode:     ModeNormal,
		expanded: map[int64]bool{},
		keys:     newKeyManager(defaultKeyBindings()), //nolint:exhaustruct
	}
	m.actions = m.defaultActions()
	return m
}

// Init implements tea.Model. Starts the 1s bar tick (drives the clock
// widget; decoupled from row caching) and the resize poll.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.startBarTick(), m.startResizeTick())
}

// RefreshFromStore reloads the in-memory tree from SQLite. Called on init
// and after batch operations.
func (m *Model) RefreshFromStore() error {
	root, err := m.store.LoadAll()
	if err != nil {
		return err
	}
	m.root = root
	if m.selectedWorkspaceID == 0 {
		// Default the todo pane to the first non-root workspace, if any.
		for _, c := range root.Children {
			m.selectedWorkspaceID = c.ID
			m.TodoCursor = 0
			break
		}
	}
	m.version++
	return nil
}

// bumpVersion invalidates renderer caches without mutating state.
func (m *Model) BumpVersion() { m.version++ }

// SetFocus moves focus to pane (0=workspace, 1=todo).
func (m *Model) SetFocus(pane int) {
	if pane != PaneWorkspace && pane != PaneTodo {
		return
	}
	m.focus = pane
}

// workspaceCursor returns the current workspace cursor (read-only).
// (Cursor is a public field; this getter exists for tests.)
func (m *Model) workspaceCursor() int { return m.WorkspaceCursor }

// todoCursor returns the current todo cursor (read-only).
func (m *Model) todoCursor() int { return m.TodoCursor }

// VisibleWorkspaces returns the flat list of non-root workspaces in tree
// order. Expansion state is applied; for the skeleton all nodes are
// considered expanded (task 6 narrows this).
func (m *Model) VisibleWorkspaces() []*model.Workspace {
	if m.root == nil {
		return nil
	}
	out := make([]*model.Workspace, 0, len(m.root.Children))
	var walk func(ws *model.Workspace)
	walk = func(ws *model.Workspace) {
		out = append(out, ws)
		if m.expanded[ws.ID] || true { // skeleton: always expand
			for _, c := range ws.Children {
				walk(c)
			}
		}
	}
	for _, c := range m.root.Children {
		walk(c)
	}
	return out
}

// visibleTodos returns the todos for the currently selected workspace,
// filtered by the active search filter when set.
func (m *Model) visibleTodos() []*model.Todo {
	ws := m.selectedWorkspace()
	if ws == nil {
		return nil
	}
	var out []*model.Todo
	var walk func(t *model.Todo)
	walk = func(t *model.Todo) {
		if m.filter == "" || matchesFilter(t, m.filter) {
			out = append(out, t)
		}
		if m.expanded[t.ID] || true { // skeleton: always expand
			for _, c := range t.Todos {
				walk(c)
			}
		}
	}
	for _, t := range ws.Todos {
		walk(t)
	}
	return out
}

// matchesFilter reports whether a todo matches the active search filter.
func matchesFilter(t *model.Todo, filter string) bool {
	return strings.Contains(strings.ToLower(t.Description), strings.ToLower(filter))
}

// selectedWorkspace returns the workspace currently displayed in the todo
// pane, or nil if none.
func (m *Model) selectedWorkspace() *model.Workspace {
	if m.root == nil || m.selectedWorkspaceID == 0 {
		return nil
	}
	return findWorkspace(m.root, m.selectedWorkspaceID)
}

// selectedTodo returns the todo under the cursor in the todo pane, or nil.
func (m *Model) selectedTodo() *model.Todo {
	ws := m.selectedWorkspace()
	if ws == nil {
		return nil
	}
	todos := m.visibleTodos()
	if m.TodoCursor < 0 || m.TodoCursor >= len(todos) {
		return nil
	}
	return todos[m.TodoCursor]
}

// selectedWorkspaceByCursor returns the workspace under the workspace cursor.
func (m *Model) selectedWorkspaceByCursor() *model.Workspace {
	ws := m.VisibleWorkspaces()
	if len(ws) == 0 {
		return nil
	}
	if m.WorkspaceCursor < 0 {
		m.WorkspaceCursor = 0
	}
	if m.WorkspaceCursor >= len(ws) {
		m.WorkspaceCursor = len(ws) - 1
	}
	return ws[m.WorkspaceCursor]
}

func findWorkspace(root *model.Workspace, id int64) *model.Workspace {
	if root == nil {
		return nil
	}
	if root.ID == id {
		return root
	}
	for _, c := range root.Children {
		if w := findWorkspace(c, id); w != nil {
			return w
		}
	}
	return nil
}

// findTodoInWorkspace returns the todo with the given id anywhere under a
// workspace tree (its own todos plus those of child workspaces).
func findTodoInWorkspace(ws *model.Workspace, id int64) *model.Todo {
	if ws == nil {
		return nil
	}
	for _, t := range ws.Todos {
		if t.ID == id {
			return t
		}
		if f := findTodoInTodo(t, id); f != nil {
			return f
		}
	}
	for _, c := range ws.Children {
		if f := findTodoInWorkspace(c, id); f != nil {
			return f
		}
	}
	return nil
}

// findTodoInTodo searches a todo and its descendants by id.
func findTodoInTodo(t *model.Todo, id int64) *model.Todo {
	for _, c := range t.Todos {
		if c.ID == id {
			return c
		}
		if f := findTodoInTodo(c, id); f != nil {
			return f
		}
	}
	return nil
}
