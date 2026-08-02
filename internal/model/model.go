package model

import (
	"strings"
	"time"
)

type Workspace struct {
	ID          int64
	OrderIndex  int
	Description string
	IsRoot      bool
	ParentID    *int64
	Parent      *Workspace // populated by store.LoadAll
	Children    []*Workspace
	Todos       []*Todo
}

func (w *Workspace) IsRootNode() bool { return w.IsRoot }

// NestLevel returns how deep this node is from the root (root = 0).
func (w *Workspace) NestLevel() int {
	lvl := 0
	cur := w.Parent
	for cur != nil && !cur.IsRoot {
		lvl++
		cur = cur.Parent
	}
	return lvl
}

// ShiftUp moves the workspace one slot up among its siblings. Returns
// true if the order changed.
func (w *Workspace) ShiftUp() bool {
	if w.IsRoot || w.Parent == nil {
		return false
	}
	sibs := w.Parent.Children
	idx := indexOfWorkspace(sibs, w)
	if idx <= 0 {
		return false
	}
	sibs[idx-1], sibs[idx] = sibs[idx], sibs[idx-1]
	w.reindexSiblings()
	return true
}

// ShiftDown moves the workspace one slot down among its siblings.
func (w *Workspace) ShiftDown() bool {
	if w.IsRoot || w.Parent == nil {
		return false
	}
	sibs := w.Parent.Children
	idx := indexOfWorkspace(sibs, w)
	if idx < 0 || idx >= len(sibs)-1 {
		return false
	}
	sibs[idx+1], sibs[idx] = sibs[idx], sibs[idx+1]
	w.reindexSiblings()
	return true
}

func (w *Workspace) reindexSiblings() {
	if w.Parent == nil {
		return
	}
	for i, s := range w.Parent.Children {
		s.OrderIndex = i
	}
}

func indexOfWorkspace(s []*Workspace, target *Workspace) int {
	for i, s := range s {
		if s == target {
			return i
		}
	}
	return -1
}

type Todo struct {
	ID                int64
	OrderIndex        int
	Description       string
	Due               *time.Time
	Effort            int
	Recurrence        *time.Duration
	Urgency           int
	Pending           bool
	ParentWorkspaceID *int64
	ParentTodoID      *int64
	ParentWorkspace   *Workspace // populated by store.LoadAll
	ParentTodo        *Todo      // populated by store.LoadAll
	Todos             []*Todo
}

func (t *Todo) Status() string {
	if !t.Pending {
		return "completed"
	}
	if t.Due != nil && t.Due.Before(time.Now()) {
		return "overdue"
	}
	return "pending"
}

func (t *Todo) Tags() []string {
	var out []string
	for _, w := range strings.Fields(t.Description) {
		if len(w) > 0 && w[0] == '@' {
			out = append(out, w)
		}
	}
	return out
}

// NestLevel for a todo (root-level todo = 0; nested = parent + 1).
func (t *Todo) NestLevel() int {
	if t.ParentTodo != nil {
		return t.ParentTodo.NestLevel() + 1
	}
	return 0
}

// ShiftUp moves the todo one slot up among its siblings.
func (t *Todo) ShiftUp() bool {
	sibs := t.siblings()
	idx := indexOfTodo(sibs, t)
	if idx <= 0 {
		return false
	}
	sibs[idx-1], sibs[idx] = sibs[idx], sibs[idx-1]
	reindexTodoSiblings(sibs)
	return true
}

// ShiftDown moves the todo one slot down among its siblings.
func (t *Todo) ShiftDown() bool {
	sibs := t.siblings()
	idx := indexOfTodo(sibs, t)
	if idx < 0 || idx >= len(sibs)-1 {
		return false
	}
	sibs[idx+1], sibs[idx] = sibs[idx], sibs[idx+1]
	reindexTodoSiblings(sibs)
	return true
}

func (t *Todo) siblings() []*Todo {
	if t.ParentTodo != nil {
		return t.ParentTodo.Todos
	}
	if t.ParentWorkspace != nil {
		return t.ParentWorkspace.Todos
	}
	return nil
}

func reindexTodoSiblings(s []*Todo) {
	for i, t := range s {
		t.OrderIndex = i
	}
}

func indexOfTodo(s []*Todo, target *Todo) int {
	for i, s := range s {
		if s == target {
			return i
		}
	}
	return -1
}
