package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
)

// defaultActions registers all actions wired into the keymap. Returns
// nil for actions that are placeholders (full implementation lands in
// Task 4 / Task 7).
func (m *Model) defaultActions() map[string]Action {
	// wrap binds an unbound method to m so the registry stores
	// `func(*Model) tea.Cmd` rather than a `func() tea.Cmd` method value.
	wrap := func(fn func(*Model) tea.Cmd) Action { return fn }
	return map[string]Action{
		"move_down":              wrap(m.actionMoveDown),
		"move_up":                wrap(m.actionMoveUp),
		"go_to_top":              wrap(m.actionGoToTop),
		"go_to_bottom":           wrap(m.actionGoToBottom),
		"add_sibling":            wrap(m.actionAddSibling),
		"add_child":              wrap(m.actionAddChild),
		"delete":                 wrap(m.actionDelete),
		"toggle_complete":        wrap(m.actionToggleComplete),
		"increase_urgency":       wrap(m.actionIncreaseUrgency),
		"decrease_urgency":       wrap(m.actionDecreaseUrgency),
		"shift_down":             wrap(m.actionShiftDown),
		"shift_up":               wrap(m.actionShiftUp),
		"toggle_expand":          wrap(m.actionToggleExpand),
		"toggle_expand_parent":   wrap(m.actionToggleExpandParent),
		"copy_description":       wrap(m.actionCopyDescription),
		"copy_model":             wrap(m.actionCopyModel),
		"paste_below":            wrap(m.actionPasteBelow),
		"paste_above":            wrap(m.actionPasteAbove),
		"switch_focus":           wrap(m.actionSwitchFocus),
		"enter_edit_description": wrap(m.actionEnterEditDescription),
		"start_search":           wrap(m.actionStartSearch),
		"start_sort":             wrap(m.actionStartSort),
		"edit_description":       wrap(m.actionEditDescription),
		"edit_due":               wrap(m.actionEditDue),
		"edit_recurrence":        wrap(m.actionEditRecurrence),
		"edit_effort":            wrap(m.actionEditEffort),
		"show_help":              wrap(m.actionShowHelp),
		"quit":                   wrap(m.actionQuit),
	}
}

func (m *Model) placeholder(name string) Action {
	return func(*Model) tea.Cmd {
		return func() tea.Msg { return noticeMsg("TODO: " + name) }
	}
}

// ----- cursor movement -----

func (m *Model) actionMoveDown(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		max := len(m.VisibleWorkspaces())
		if max == 0 {
			m.WorkspaceCursor = 0
			return nil
		}
		if m.WorkspaceCursor < max-1 {
			m.WorkspaceCursor++
		}
	} else {
		max := len(m.visibleTodos())
		if max == 0 {
			m.TodoCursor = 0
			return nil
		}
		if m.TodoCursor < max-1 {
			m.TodoCursor++
		}
	}
	m.bumpVersion()
	return nil
}

func (m *Model) actionMoveUp(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		if m.WorkspaceCursor > 0 {
			m.WorkspaceCursor--
		}
	} else {
		if m.TodoCursor > 0 {
			m.TodoCursor--
		}
	}
	m.bumpVersion()
	return nil
}

func (m *Model) actionGoToTop(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		m.WorkspaceCursor = 0
	} else {
		m.TodoCursor = 0
	}
	m.bumpVersion()
	return nil
}

func (m *Model) actionGoToBottom(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		if n := len(m.VisibleWorkspaces()); n > 0 {
			m.WorkspaceCursor = n - 1
		}
	} else {
		if n := len(m.visibleTodos()); n > 0 {
			m.TodoCursor = n - 1
		}
	}
	m.bumpVersion()
	return nil
}

// ----- CRUD: add / delete -----

func (m *Model) actionAddSibling(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		parent := m.selectedWorkspaceByCursor()
		if parent == nil {
			return m.addWorkspaceChild(m.root)
		}
		return m.addWorkspaceChild(parent)
	}
	todo := m.selectedTodo()
	if todo == nil {
		ws := m.selectedWorkspace()
		if ws == nil {
			return nil
		}
		newTodo := &model.Todo{Description: "", OrderIndex: len(ws.Todos), Pending: true, ParentWorkspaceID: &ws.ID}
		if err := m.store.SaveTodo(newTodo); err != nil {
			return noticeCmd("add sibling failed: " + err.Error())
		}
		m.RefreshFromStore()
		return nil
	}
	parentTodo := todo.ParentTodo
	if parentTodo != nil {
		newTodo := &model.Todo{Description: "", Pending: true, OrderIndex: len(parentTodo.Todos), ParentTodoID: &parentTodo.ID}
		if err := m.store.SaveTodo(newTodo); err != nil {
			return noticeCmd("add sibling failed: " + err.Error())
		}
	} else {
		ws := todo.ParentWorkspace
		newTodo := &model.Todo{Description: "", Pending: true, OrderIndex: len(ws.Todos), ParentWorkspaceID: &ws.ID}
		if err := m.store.SaveTodo(newTodo); err != nil {
			return noticeCmd("add sibling failed: " + err.Error())
		}
	}
	m.RefreshFromStore()
	return nil
}

func (m *Model) addWorkspaceChild(parent *model.Workspace) tea.Cmd {
	if parent == nil {
		return nil
	}
	ws := &model.Workspace{Description: "", OrderIndex: len(parent.Children), ParentID: &parent.ID}
	if err := m.store.SaveWorkspace(ws); err != nil {
		return noticeCmd("add workspace failed: " + err.Error())
	}
	m.RefreshFromStore()
	return nil
}

func (m *Model) actionAddChild(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws == nil {
			return m.actionAddSibling(m)
		}
		return m.addWorkspaceChild(ws)
	}
	return m.actionAddSibling(m)
}

func (m *Model) actionDelete(_ *Model) tea.Cmd {
	// Deletion goes through the confirm dialog (default-NO, matching the
	// reference). The actual delete runs in doDelete after confirm.
	return m.StartConfirm()
}

// ----- toggle complete / urgency -----

func (m *Model) actionToggleComplete(_ *Model) tea.Cmd {
	t := m.selectedTodo()
	if t == nil {
		return nil
	}
	t.Pending = !t.Pending
	if err := m.store.SaveTodo(t); err != nil {
		return noticeCmd("toggle failed: " + err.Error())
	}
	m.bumpVersion()
	return nil
}

func (m *Model) actionIncreaseUrgency(_ *Model) tea.Cmd {
	t := m.selectedTodo()
	if t == nil {
		return nil
	}
	t.Urgency++
	if err := m.store.SaveTodo(t); err != nil {
		return noticeCmd("urgency failed: " + err.Error())
	}
	m.bumpVersion()
	return nil
}

func (m *Model) actionDecreaseUrgency(_ *Model) tea.Cmd {
	t := m.selectedTodo()
	if t == nil {
		return nil
	}
	if t.Urgency > 0 {
		t.Urgency--
	}
	if err := m.store.SaveTodo(t); err != nil {
		return noticeCmd("urgency failed: " + err.Error())
	}
	m.bumpVersion()
	return nil
}

// ----- ordering -----

func (m *Model) actionShiftDown(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws != nil && !ws.IsRoot {
			ws.ShiftDown()
			m.RefreshFromStore()
		}
	} else {
		t := m.selectedTodo()
		if t != nil {
			t.ShiftDown()
			m.RefreshFromStore()
		}
	}
	return nil
}

func (m *Model) actionShiftUp(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws != nil && !ws.IsRoot {
			ws.ShiftUp()
			m.RefreshFromStore()
		}
	} else {
		t := m.selectedTodo()
		if t != nil {
			t.ShiftUp()
			m.RefreshFromStore()
		}
	}
	return nil
}

// ----- expand / collapse -----

func (m *Model) actionToggleExpand(_ *Model) tea.Cmd {
	var id int64
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws == nil {
			return nil
		}
		id = ws.ID
	} else {
		t := m.selectedTodo()
		if t == nil {
			return nil
		}
		id = t.ID
	}
	m.expanded[id] = !m.expanded[id]
	m.bumpVersion()
	return nil
}

func (m *Model) actionToggleExpandParent(_ *Model) tea.Cmd {
	var id int64
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws == nil || ws.IsRoot {
			return nil
		}
		if ws.ParentID == nil {
			return nil
		}
		id = *ws.ParentID
	} else {
		t := m.selectedTodo()
		if t == nil {
			return nil
		}
		if t.ParentTodo != nil {
			id = t.ParentTodo.ID
		} else if t.ParentWorkspace != nil {
			id = t.ParentWorkspace.ID
		} else {
			return nil
		}
	}
	m.expanded[id] = !m.expanded[id]
	m.bumpVersion()
	return nil
}

// ----- clipboard -----

func (m *Model) actionCopyDescription(_ *Model) tea.Cmd {
	return nil
}

func (m *Model) actionCopyModel(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws == nil {
			return nil
		}
		m.clipboard = &clipboardEntry{kind: "workspace", id: ws.ID}
	} else {
		t := m.selectedTodo()
		if t == nil {
			return nil
		}
		m.clipboard = &clipboardEntry{kind: "todo", id: t.ID}
	}
	return nil
}

func (m *Model) actionPasteBelow(_ *Model) tea.Cmd {
	if m.clipboard == nil {
		return nil
	}
	return noticeCmd("paste (skeleton): " + m.clipboard.kind)
}

func (m *Model) actionPasteAbove(_ *Model) tea.Cmd {
	return m.actionPasteBelow(m)
}

// ----- focus / edit entry -----

func (m *Model) actionSwitchFocus(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		m.SetFocus(PaneTodo)
	} else {
		m.SetFocus(PaneWorkspace)
	}
	return nil
}

func (m *Model) actionEnterEditDescription(_ *Model) tea.Cmd {
	return m.StartEdit("description")
}

func (m *Model) actionEditDescription(_ *Model) tea.Cmd { return m.StartEdit("description") }
func (m *Model) actionEditDue(_ *Model) tea.Cmd         { return m.StartEdit("due") }
func (m *Model) actionEditRecurrence(_ *Model) tea.Cmd  { return m.StartEdit("recurrence") }
func (m *Model) actionEditEffort(_ *Model) tea.Cmd      { return m.StartEdit("effort") }

func (m *Model) actionStartSearch(_ *Model) tea.Cmd { return m.StartSearch() }
func (m *Model) actionStartSort(_ *Model) tea.Cmd   { return m.StartSort() }

func (m *Model) actionShowHelp(_ *Model) tea.Cmd {
	m.helpVisible = !m.helpVisible
	m.bumpVersion()
	return nil
}

func (m *Model) actionQuit(_ *Model) tea.Cmd {
	m.quitting = true
	return tea.Quit
}

// ----- helpers -----

func noticeCmd(s string) tea.Cmd {
	return func() tea.Msg { return noticeMsg(s) }
}

type noticeMsg string
