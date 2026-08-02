package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registers as "sqlite"

	"github.com/XiaTian-AC/faster-dooit/internal/model"
)

// Workspace and Todo alias the model types so callers of this package consume
// model.Workspace / model.Todo while the store package keeps the same names.
type Workspace = model.Workspace
type Todo = model.Todo

type Store struct {
	db *sql.DB
}

// orderItem carries an id -> order_index pair for BatchOrder.
type orderItem struct {
	ID    int64
	Order int
}

// New opens (creating if needed) the SQLite database at path and ensures the
// schema and the root workspace row exist.
//
// Review critical #1: the DSN uses the "file:" URI form so foreign_keys and
// busy_timeout pragmas apply, and the pool is pinned to a single connection.
// Without SetMaxOpenConns(1) an in-memory DB (":memory:") would be a separate,
// empty database per pooled connection and the per-connection foreign_keys
// pragma would be lost.
func New(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: create schema: %w", err)
	}
	if err := s.ensureRoot(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ensureRoot inserts the single root workspace row on first open.
func (s *Store) ensureRoot() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM workspace WHERE is_root = 1`).Scan(&n); err != nil {
		return fmt.Errorf("store: check root: %w", err)
	}
	if n == 0 {
		if _, err := s.db.Exec(`INSERT INTO workspace (order_index, description, is_root, parent_id) VALUES (-1, '', 1, NULL)`); err != nil {
			return fmt.Errorf("store: create root: %w", err)
		}
	}
	return nil
}

func (s *Store) rootID() (int64, error) {
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM workspace WHERE is_root = 1 LIMIT 1`).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// LoadAll reads every workspace and todo and assembles the in-memory tree.
// Siblings are ordered by order_index ascending, tie-broken by id.
func (s *Store) LoadAll() (*Workspace, error) {
	rows, err := s.db.Query(`SELECT id, order_index, description, is_root, parent_id FROM workspace ORDER BY order_index, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int64]*Workspace)
	var ordered []*Workspace
	var root *Workspace
	for rows.Next() {
		var w Workspace
		var isRoot int
		var parentID sql.NullInt64
		if err := rows.Scan(&w.ID, &w.OrderIndex, &w.Description, &isRoot, &parentID); err != nil {
			return nil, err
		}
		w.IsRoot = isRoot == 1
		if parentID.Valid {
			id := parentID.Int64
			w.ParentID = &id
		}
		byID[w.ID] = &w
		ordered = append(ordered, &w)
		if w.IsRoot {
			root = &w
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	todoRows, err := s.db.Query(`SELECT id, order_index, description, due, effort, recurrence, urgency, pending, parent_workspace_id, parent_todo_id FROM todo ORDER BY order_index, id`)
	if err != nil {
		return nil, err
	}
	defer todoRows.Close()

	// First pass: collect every todo flat so we can resolve parent pointers
	// in a second pass (the todos may be intermixed with their parents).
	todoByID := make(map[int64]*Todo)
	for todoRows.Next() {
		var t Todo
		var due sql.NullString
		var recurrence sql.NullInt64
		var pws, ptd sql.NullInt64
		var pending int
		if err := todoRows.Scan(&t.ID, &t.OrderIndex, &t.Description, &due, &t.Effort, &recurrence, &t.Urgency, &pending, &pws, &ptd); err != nil {
			return nil, err
		}
		t.Pending = pending == 1
		if due.Valid {
			d, err := time.Parse(time.RFC3339, due.String)
			if err != nil {
				return nil, fmt.Errorf("store: parse due %q: %w", due.String, err)
			}
			t.Due = &d
		}
		if recurrence.Valid {
			d := time.Duration(recurrence.Int64)
			t.Recurrence = &d
		}
		if pws.Valid {
			id := pws.Int64
			t.ParentWorkspaceID = &id
		}
		if ptd.Valid {
			id := ptd.Int64
			t.ParentTodoID = &id
		}
		todoByID[t.ID] = &t
	}
	if err := todoRows.Err(); err != nil {
		return nil, err
	}

	// Second pass: link todos into the workspace / todo trees.
	for _, t := range todoByID {
		if t.ParentWorkspaceID != nil {
			if ws, ok := byID[*t.ParentWorkspaceID]; ok {
				ws.Todos = append(ws.Todos, t)
				t.ParentWorkspace = ws
				continue
			}
		}
		if t.ParentTodoID != nil {
			if p, ok := todoByID[*t.ParentTodoID]; ok {
				p.Todos = append(p.Todos, t)
				t.ParentTodo = p
			}
		}
	}

	// Wire up Workspace.Parent + Children.
	for _, w := range ordered {
		if w.ParentID != nil {
			if parent, ok := byID[*w.ParentID]; ok {
				parent.Children = append(parent.Children, w)
				w.Parent = parent
			}
		}
	}

	if root == nil {
		return nil, fmt.Errorf("store: no root workspace found")
	}
	return root, nil
}

// SaveWorkspace inserts a new workspace (ID == 0) or updates an existing one.
//
// Review critical #2: a new row goes through INSERT and LastInsertId() is
// written back into the struct; an existing ID goes through an upsert. Naively
// running an upsert with id=0 would insert a literal row with id=0 and break
// every parent key.
//
// Review critical #3: a new non-root workspace with no parent is auto-attached
// to the root, matching the original dooit Workspace.save() behaviour.
func (s *Store) SaveWorkspace(w *Workspace) error {
	if w.ID == 0 {
		if !w.IsRoot && w.ParentID == nil {
			rootID, err := s.rootID()
			if err != nil {
				return err
			}
			w.ParentID = &rootID
		}
		res, err := s.db.Exec(
			`INSERT INTO workspace (order_index, description, is_root, parent_id) VALUES (?, ?, ?, ?)`,
			w.OrderIndex, w.Description, boolToInt(w.IsRoot), nullableInt64(w.ParentID))
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		w.ID = id
		return nil
	}

	_, err := s.db.Exec(
		`INSERT INTO workspace (id, order_index, description, is_root, parent_id)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   order_index=excluded.order_index, description=excluded.description,
		   is_root=excluded.is_root, parent_id=excluded.parent_id`,
		w.ID, w.OrderIndex, w.Description, boolToInt(w.IsRoot), nullableInt64(w.ParentID))
	return err
}

// SaveTodo inserts a new todo (ID == 0) or updates an existing one, following
// the same INSERT + LastInsertId() writeback / upsert split as SaveWorkspace.
func (s *Store) SaveTodo(t *Todo) error {
	if t.ID == 0 {
		res, err := s.db.Exec(
			`INSERT INTO todo (order_index, description, due, effort, recurrence, urgency, pending, parent_workspace_id, parent_todo_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.OrderIndex, t.Description, nullableTime(t.Due), t.Effort,
			nullableDuration(t.Recurrence), t.Urgency, boolToInt(t.Pending),
			nullableInt64(t.ParentWorkspaceID), nullableInt64(t.ParentTodoID))
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		t.ID = id
		return nil
	}

	_, err := s.db.Exec(
		`INSERT INTO todo (id, order_index, description, due, effort, recurrence, urgency, pending, parent_workspace_id, parent_todo_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   order_index=excluded.order_index, description=excluded.description,
		   due=excluded.due, effort=excluded.effort, recurrence=excluded.recurrence,
		   urgency=excluded.urgency, pending=excluded.pending,
		   parent_workspace_id=excluded.parent_workspace_id, parent_todo_id=excluded.parent_todo_id`,
		t.ID, t.OrderIndex, t.Description, nullableTime(t.Due), t.Effort,
		nullableDuration(t.Recurrence), t.Urgency, boolToInt(t.Pending),
		nullableInt64(t.ParentWorkspaceID), nullableInt64(t.ParentTodoID))
	return err
}

// DeleteWorkspace removes the workspace by id; descendants are removed by the
// ON DELETE CASCADE foreign keys.
func (s *Store) DeleteWorkspace(id int64) error {
	_, err := s.db.Exec(`DELETE FROM workspace WHERE id = ?`, id)
	return err
}

// DeleteTodo removes the todo by id; nested todos are removed by cascade.
func (s *Store) DeleteTodo(id int64) error {
	_, err := s.db.Exec(`DELETE FROM todo WHERE id = ?`, id)
	return err
}

// BatchOrder updates order_index for the given rows in one transaction.
// A nil/empty items slice is a no-op.
func (s *Store) BatchOrder(parentKind string, parentID *int64, items []orderItem) error {
	switch parentKind {
	case "workspace", "todo":
	default:
		return fmt.Errorf("store: unknown parentKind %q", parentKind)
	}
	if len(items) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after successful Commit

	stmt, err := tx.Prepare(`UPDATE ` + parentKind + ` SET order_index = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, it := range items {
		if _, err := stmt.Exec(it.Order, it.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func nullableDuration(d *time.Duration) any {
	if d == nil {
		return nil
	}
	return int64(*d)
}
