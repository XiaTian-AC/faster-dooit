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
	Children    []*Workspace
	Todos       []*Todo
}

func (w *Workspace) IsRootNode() bool { return w.IsRoot }

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
