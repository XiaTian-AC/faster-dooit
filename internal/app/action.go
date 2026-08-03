package app

import (
	"time"

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
		m.syncWorkspaceSelection()
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
	m.BumpVersion()
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
	m.syncWorkspaceSelection()
	m.BumpVersion()
	return nil
}

func (m *Model) actionGoToTop(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		m.WorkspaceCursor = 0
	} else {
		m.TodoCursor = 0
	}
	m.syncWorkspaceSelection()
	m.BumpVersion()
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
	m.syncWorkspaceSelection()
	m.BumpVersion()
	return nil
}

// syncWorkspaceSelection keeps the todo pane pointing at the workspace under
// the workspace cursor, so switching workspaces updates the right side.
func (m *Model) syncWorkspaceSelection() {
	if m.focus != PaneWorkspace {
		return
	}
	if ws := m.selectedWorkspaceByCursor(); ws != nil {
		m.selectedWorkspaceID = ws.ID
	}
}

// Default names assigned to freshly-created items so a new row is never an
// invisible blank. The inline edit shows them as placeholder; leaving the
// input empty on confirm keeps them.
const (
	defaultTaskName      = "New task"
	defaultWorkspaceName = "New workspace"
)

// indexOfTodoByID returns the index of the todo with the given id, or -1.
func indexOfTodoByID(todos []*model.Todo, id int64) int {
	for i, t := range todos {
		if t.ID == id {
			return i
		}
	}
	return -1
}

// indexOfWorkspaceByID returns the index of the workspace with the given id.
func indexOfWorkspaceByID(ws []*model.Workspace, id int64) int {
	for i, w := range ws {
		if w.ID == id {
			return i
		}
	}
	return -1
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
	var newTodo *model.Todo
	if todo == nil {
		ws := m.selectedWorkspace()
		if ws == nil {
			return nil
		}
		newTodo = &model.Todo{Description: defaultTaskName, OrderIndex: len(ws.Todos), Pending: true, Urgency: 1, ParentWorkspaceID: &ws.ID}
		if err := m.store.SaveTodo(newTodo); err != nil {
			return noticeCmd("add sibling failed: " + err.Error())
		}
	} else {
		parentTodo := todo.ParentTodo
		if parentTodo != nil {
			newTodo = &model.Todo{Description: defaultTaskName, Pending: true, OrderIndex: len(parentTodo.Todos), Urgency: 1, ParentTodoID: &parentTodo.ID}
			if err := m.store.SaveTodo(newTodo); err != nil {
				return noticeCmd("add sibling failed: " + err.Error())
			}
		} else {
			ws := todo.ParentWorkspace
			newTodo = &model.Todo{Description: defaultTaskName, Pending: true, OrderIndex: len(ws.Todos), Urgency: 1, ParentWorkspaceID: &ws.ID}
			if err := m.store.SaveTodo(newTodo); err != nil {
				return noticeCmd("add sibling failed: " + err.Error())
			}
		}
	}
	m.RefreshFromStore()
	// Position the cursor on the new item and open the inline edit.
	m.TodoCursor = max(0, indexOfTodoByID(m.visibleTodos(), newTodo.ID))
	return m.startInlineEdit(defaultTaskName)
}

func (m *Model) addWorkspaceChild(parent *model.Workspace) tea.Cmd {
	if parent == nil {
		return nil
	}
	ws := &model.Workspace{Description: defaultWorkspaceName, OrderIndex: len(parent.Children), ParentID: &parent.ID}
	if err := m.store.SaveWorkspace(ws); err != nil {
		return noticeCmd("add workspace failed: " + err.Error())
	}
	m.RefreshFromStore()
	m.WorkspaceCursor = max(0, indexOfWorkspaceByID(m.VisibleWorkspaces(), ws.ID))
	m.selectedWorkspaceID = ws.ID // show the new workspace in the todo pane
	return m.startInlineEdit(defaultWorkspaceName)
}

func (m *Model) actionAddChild(_ *Model) tea.Cmd {
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws == nil {
			return m.actionAddSibling(m)
		}
		return m.addWorkspaceChild(ws)
	}
	// Todo pane: create a nested child under the selected todo (not a
	// sibling), so the new row renders indented.
	todo := m.selectedTodo()
	if todo == nil {
		return m.actionAddSibling(m)
	}
	newTodo := &model.Todo{
		Description: defaultTaskName, Pending: true, Urgency: 1,
		OrderIndex:   len(todo.Todos),
		ParentTodoID: &todo.ID,
	}
	if err := m.store.SaveTodo(newTodo); err != nil {
		return noticeCmd("add child failed: " + err.Error())
	}
	m.RefreshFromStore()
	m.TodoCursor = max(0, indexOfTodoByID(m.visibleTodos(), newTodo.ID))
	return m.startInlineEdit(defaultTaskName)
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
	m.applyCompletionCascade(t)
	m.saveTodoSubtree(t)
	if err := m.RefreshFromStore(); err != nil {
		return noticeCmd("reload failed: " + err.Error())
	}
	m.BumpVersion()
	return nil
}

// applyCompletionCascade implements the reference update_hooks.py rules:
//
//	R1 completing a todo completes its whole subtree
//	R2 a parent auto-completes only when all its children complete
//	R3 reopening any child reopens the parent
//	R4 a recurring todo never completes — due += recurrence, pending stays true
func (m *Model) applyCompletionCascade(t *model.Todo) {
	if t.Pending {
		// completing
		if t.Recurrence != nil {
			nd := time.Now().Add(*t.Recurrence)
			if t.Due != nil {
				nd = t.Due.Add(*t.Recurrence)
			}
			t.Due = &nd
			// R4: a recurring todo never completes — due advances and it
			// stays pending. Only its subtree completes (R1), matching the
			// reference update_hooks ordering.
			for _, c := range t.Todos {
				c.SetSubtreePending(false)
			}
		} else {
			t.Pending = false
			t.SetSubtreePending(false) // R1
		}
		t.ParentAutoComplete() // R2
	} else {
		t.Pending = true
		t.ReopenParents() // R3
	}
}

// saveTodoSubtree persists t and every descendant.
func (m *Model) saveTodoSubtree(t *model.Todo) {
	_ = m.store.SaveTodo(t)
	for _, c := range t.Todos {
		m.saveTodoSubtree(c)
	}
}

// actionSort sorts the current model's siblings by field (or reverses),
// then persists the new order.
func (m *Model) actionSort(field string) tea.Cmd {
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws == nil || ws.IsRoot {
			return nil
		}
		ws.SortSiblings(field, field == "reverse")
		m.persistSiblingOrder()
	} else {
		t := m.selectedTodo()
		if t == nil {
			return nil
		}
		t.SortSiblings(field, field == "reverse")
		m.persistSiblingOrder()
	}
	return nil
}

// persistSiblingOrder persists the current in-memory order for every
// workspace and todo (topological), then reloads.
func (m *Model) persistSiblingOrder() {
	if err := m.store.ReorderAll(m.root); err != nil {
		m.Notify("sort persist failed: "+err.Error(), "error")
		return
	}
	if err := m.RefreshFromStore(); err != nil {
		m.Notify("sort persist failed: "+err.Error(), "error")
	}
	m.BumpVersion()
}

func (m *Model) actionIncreaseUrgency(_ *Model) tea.Cmd {
	t := m.selectedTodo()
	if t == nil {
		return nil
	}
	if t.Urgency < 5 {
		t.Urgency++
	}
	if err := m.store.SaveTodo(t); err != nil {
		return noticeCmd("urgency failed: " + err.Error())
	}
	m.BumpVersion()
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
	m.BumpVersion()
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
	m.BumpVersion()
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
	m.BumpVersion()
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

func (m *Model) actionPasteBelow(_ *Model) tea.Cmd { return m.paste("below") }

func (m *Model) actionPasteAbove(_ *Model) tea.Cmd { return m.paste("above") }

// paste clones the clipped subtree into the focused pane at the cursor,
// one slot below/above the highlighted node. Cross-type paste (a workspace
// into the todo tree or a todo into the workspace tree) is an error,
// matching the reference paste_model_from_clipboard + the Eng review #16.
func (m *Model) paste(position string) tea.Cmd {
	if m.clipboard == nil {
		return m.Notify("No model in clipboard", "error")
	}
	if m.focus == PaneWorkspace {
		return m.pasteWorkspace(position)
	}
	return m.pasteTodo(position)
}

func (m *Model) pasteWorkspace(position string) tea.Cmd {
	if m.clipboard.kind != "workspace" {
		return m.Notify("cannot paste a todo into the workspace tree", "error")
	}
	src := findWorkspace(m.root, m.clipboard.id)
	if src == nil {
		return m.Notify("clipboard source not found", "error")
	}
	cur := m.selectedWorkspaceByCursor()
	if cur == nil || cur.IsRoot {
		return nil
	}
	parent := cur.Parent
	if parent == nil {
		return nil
	}
	sibs := parent.Children
	idx := indexOfWorkspaceIn(sibs, cur)
	if position == "below" {
		idx++
	}
	clone := src.Clone()
	clone.Parent = parent
	clone.ParentID = &parent.ID
	parent.Children = insertWorkspaceAt(sibs, idx, clone)
	reindexWorkspaceSlice(parent.Children)
	m.persistWorkspaceClone(clone)
	return m.finishPaste()
}

func (m *Model) pasteTodo(position string) tea.Cmd {
	if m.clipboard.kind != "todo" {
		return m.Notify("cannot paste a workspace into the todo tree", "error")
	}
	src := findTodoInWorkspace(m.root, m.clipboard.id)
	if src == nil {
		return m.Notify("clipboard source not found", "error")
	}
	cur := m.selectedTodo()
	if cur == nil {
		ws := m.selectedWorkspace()
		if ws == nil {
			return nil
		}
		clone := src.Clone()
		clone.ParentWorkspace = ws
		clone.ParentWorkspaceID = &ws.ID
		ws.Todos = append(ws.Todos, clone)
		reindexTodoSlice(ws.Todos)
		m.persistTodoClone(clone)
		return m.finishPaste()
	}
	var clone *model.Todo
	if cur.ParentTodo != nil {
		sibs := cur.ParentTodo.Todos
		idx := indexOfTodoIn(sibs, cur)
		if position == "below" {
			idx++
		}
		clone = src.Clone()
		clone.ParentTodo = cur.ParentTodo
		clone.ParentTodoID = &cur.ParentTodo.ID
		cur.ParentTodo.Todos = insertTodoAt(sibs, idx, clone)
		reindexTodoSlice(cur.ParentTodo.Todos)
	} else if cur.ParentWorkspace != nil {
		sibs := cur.ParentWorkspace.Todos
		idx := indexOfTodoIn(sibs, cur)
		if position == "below" {
			idx++
		}
		clone = src.Clone()
		clone.ParentWorkspace = cur.ParentWorkspace
		clone.ParentWorkspaceID = &cur.ParentWorkspace.ID
		cur.ParentWorkspace.Todos = insertTodoAt(sibs, idx, clone)
		reindexTodoSlice(cur.ParentWorkspace.Todos)
	} else {
		return nil
	}
	m.persistTodoClone(clone)
	return m.finishPaste()
}

// finishPaste persists the reindexed order of every sibling, reloads, and
// invalidates renderer caches.
func (m *Model) finishPaste() tea.Cmd {
	if err := m.store.ReorderAll(m.root); err != nil {
		return m.Notify("paste persist failed: "+err.Error(), "error")
	}
	if err := m.RefreshFromStore(); err != nil {
		return m.Notify("paste reload failed: "+err.Error(), "error")
	}
	m.BumpVersion()
	return nil
}

// persistTodoClone saves a fresh cloned todo and every descendant, wiring
// the new parent_todo_id as it descends (fresh IDs are INSERTed).
func (m *Model) persistTodoClone(t *model.Todo) {
	_ = m.store.SaveTodo(t)
	for _, c := range t.Todos {
		c.ParentTodoID = &t.ID
		m.persistTodoClone(c)
	}
}

// persistWorkspaceClone saves a fresh cloned workspace and every descendant
// workspace + todo, wiring parent ids along the way.
func (m *Model) persistWorkspaceClone(w *model.Workspace) {
	_ = m.store.SaveWorkspace(w)
	for _, c := range w.Children {
		c.ParentID = &w.ID
		m.persistWorkspaceClone(c)
	}
	for _, t := range w.Todos {
		t.ParentWorkspaceID = &w.ID
		m.persistTodoClone(t)
	}
}

func indexOfWorkspaceIn(s []*model.Workspace, cur *model.Workspace) int {
	for i, x := range s {
		if x == cur {
			return i
		}
	}
	return 0
}

func indexOfTodoIn(s []*model.Todo, cur *model.Todo) int {
	for i, x := range s {
		if x == cur {
			return i
		}
	}
	return 0
}

func insertWorkspaceAt(s []*model.Workspace, idx int, w *model.Workspace) []*model.Workspace {
	s = append(s, nil)
	copy(s[idx+1:], s[idx:])
	s[idx] = w
	return s
}

func insertTodoAt(s []*model.Todo, idx int, t *model.Todo) []*model.Todo {
	s = append(s, nil)
	copy(s[idx+1:], s[idx:])
	s[idx] = t
	return s
}

func reindexWorkspaceSlice(s []*model.Workspace) {
	for i, x := range s {
		x.OrderIndex = i
	}
}

func reindexTodoSlice(s []*model.Todo) {
	for i, t := range s {
		t.OrderIndex = i
	}
}

// startInlineEdit enters INSERT mode for a freshly-created item, showing the
// given default name as the input placeholder (so an empty confirm keeps it).
func (m *Model) startInlineEdit(placeholder string) tea.Cmd {
	m.editField = "description"
	m.editPlaceholder = placeholder
	m.input = newInput(placeholder)
	m.mode = ModeInsert
	m.BumpVersion()
	return nil
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
	m.BumpVersion()
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
