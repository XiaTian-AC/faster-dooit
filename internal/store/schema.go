package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS workspace (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_index INTEGER NOT NULL DEFAULT -1,
  description TEXT NOT NULL DEFAULT '',
  is_root INTEGER NOT NULL DEFAULT 0,
  parent_id INTEGER REFERENCES workspace(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS todo (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  order_index INTEGER NOT NULL DEFAULT -1,
  description TEXT NOT NULL DEFAULT '',
  due TEXT,
  effort INTEGER NOT NULL DEFAULT 0,
  recurrence INTEGER,
  urgency INTEGER NOT NULL DEFAULT 1,
  pending INTEGER NOT NULL DEFAULT 1,
  parent_workspace_id INTEGER REFERENCES workspace(id) ON DELETE CASCADE,
  parent_todo_id INTEGER REFERENCES todo(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_workspace_parent ON workspace(parent_id);
CREATE INDEX IF NOT EXISTS idx_todo_parent_ws ON todo(parent_workspace_id);
CREATE INDEX IF NOT EXISTS idx_todo_parent_todo ON todo(parent_todo_id);
`
