package model

import (
	"sort"
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

// SetSubtreePending sets Pending on t and every descendant (completion
// cascade rule R1).
func (t *Todo) SetSubtreePending(v bool) {
	t.Pending = v
	for _, c := range t.Todos {
		c.SetSubtreePending(v)
	}
}

// ParentAutoComplete walks up the todo parent chain completing each parent
// once all of its direct children are completed (rule R2).
func (t *Todo) ParentAutoComplete() {
	p := t.ParentTodo
	for p != nil {
		allDone := len(p.Todos) > 0
		for _, c := range p.Todos {
			if c.Pending {
				allDone = false
				break
			}
		}
		if !allDone {
			return
		}
		p.Pending = false
		p = p.ParentTodo
	}
}

// ReopenParents walks up the chain reopening every parent (rule R3).
func (t *Todo) ReopenParents() {
	p := t.ParentTodo
	for p != nil {
		p.Pending = true
		p = p.ParentTodo
	}
}

// TodoComparableFields are the fields a todo can be sorted by (excluding
// id/order_index and parent keys), matching the reference comparable_fields.
var TodoComparableFields = []string{"description", "due", "effort", "recurrence", "urgency", "pending"}

// SortSiblings sorts t's siblings in place by field using the reference
// semantics: the composite key (pending→due→order_index) applies ONLY to
// the "pending" option; other fields sort ascending with NULL due last
// (nulls_last). reverse=true reverses the resulting order.
func (t *Todo) SortSiblings(field string, reverse bool) {
	sibs := t.siblings()
	if reverse {
		reverseTodoSlice(sibs)
		reindexTodoSiblings(sibs)
		return
	}
	idx := indexOfString(TodoComparableFields, field)
	if idx < 0 {
		return
	}
	sort.SliceStable(sibs, func(i, j int) bool {
		a, b := sibs[i], sibs[j]
		if field == "pending" {
			if a.Pending != b.Pending {
				return a.Pending // pending first
			}
			ad, bd := a.Due, b.Due
			if ad == nil {
				ad = &time.Time{}
			}
			if bd == nil {
				bd = &time.Time{}
			}
			if !ad.Equal(*bd) {
				return ad.Before(*bd)
			}
			return a.OrderIndex < b.OrderIndex
		}
		switch field {
		case "description":
			return a.Description < b.Description
		case "due":
			if a.Due == nil {
				return false // nulls last
			}
			if b.Due == nil {
				return true
			}
			return a.Due.Before(*b.Due)
		case "effort":
			return a.Effort < b.Effort
		case "recurrence":
			return durationLess(a.Recurrence, b.Recurrence)
		case "urgency":
			return a.Urgency < b.Urgency
		}
		return a.ID < b.ID
	})
	reindexTodoSiblings(sibs)
}

// WorkspaceComparableFields are the fields a workspace can be sorted by.
var WorkspaceComparableFields = []string{"description"}

// SortSiblings sorts a workspace's siblings in place.
func (w *Workspace) SortSiblings(field string, reverse bool) {
	if w.Parent == nil {
		return
	}
	sibs := w.Parent.Children
	if reverse {
		reverseWorkspaceSlice(sibs)
		reindexWorkspaceSiblings(sibs)
		return
	}
	sort.SliceStable(sibs, func(i, j int) bool {
		a, b := sibs[i], sibs[j]
		if field == "description" {
			return a.Description < b.Description
		}
		return a.ID < b.ID
	})
	reindexWorkspaceSiblings(sibs)
}

func durationLess(a, b *time.Duration) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return *a < *b
}

func indexOfString(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func reverseTodoSlice(s []*Todo) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func reverseWorkspaceSlice(s []*Workspace) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func reindexWorkspaceSiblings(s []*Workspace) {
	for i, x := range s {
		x.OrderIndex = i
	}
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

// Clone deep-copies t and its whole subtree with fresh (zero) IDs so the
// copy can be saved as new rows. Parent pointers are reset; the caller
// re-wires the clone into the target parent.
func (t *Todo) Clone() *Todo {
	c := &Todo{
		OrderIndex:  t.OrderIndex,
		Description: t.Description,
		Due:         t.Due,
		Effort:      t.Effort,
		Recurrence:  t.Recurrence,
		Urgency:     t.Urgency,
		Pending:     t.Pending,
	}
	for _, child := range t.Todos {
		cc := child.Clone()
		cc.ParentTodo = c
		c.Todos = append(c.Todos, cc)
	}
	return c
}

// Clone deep-copies w and its whole subtree (child workspaces + todos) with
// fresh IDs. Parent pointers are reset; the caller re-wires the clone.
func (w *Workspace) Clone() *Workspace {
	c := &Workspace{
		OrderIndex:  w.OrderIndex,
		Description: w.Description,
	}
	for _, child := range w.Children {
		cc := child.Clone()
		cc.Parent = c
		c.Children = append(c.Children, cc)
	}
	for _, t := range w.Todos {
		tt := t.Clone()
		tt.ParentWorkspace = c
		c.Todos = append(c.Todos, tt)
	}
	return c
}
