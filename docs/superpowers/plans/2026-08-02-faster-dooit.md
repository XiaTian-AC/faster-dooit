# Faster-Dooit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 Go + bubbletea 在 `faster-dooit/` 下全面重写 dooit（TUI 任务管理器），功能完整对齐原版，启动 < 200ms、万级节点无感知按键延迟。

**Architecture:** Elm 架构（bubbletea）。启动时一次性把 SQLite 整库读入内存模型，所有操作走内存，写操作同步落库；`view()` 只渲染视口内行。Lua（gopher-lua）配置文件镜像原版 `api` 表面（keys/layouts/formatter/bar/dashboard/subscribe/timer）。两栏树 + 六种模式 + 输入覆盖层。

**Tech Stack:** Go 1.22+、bubbletea、bubbles/textinput、lipgloss、gopher-lua、modernc.org/sqlite、dateparse（自写）、Go 标准测试。

**Spec:** `docs/superpowers/specs/2026-08-02-faster-dooit-design.md`

## Global Constraints

- Go module 名：`github.com/XiaTian-AC/faster-dooit`
- 最低 Go 版本：1.22
- 存储：modernc.org/sqlite（纯 Go，无 CGO）；不引入 ORM，用 `database/sql`
- 依赖锁定：`go.mod` 提交进版本库；构建产物为单一二进制
- 所有 git 命令在 `faster-dooit/` 目录内执行（`git -C F:\Workspace\Project\dooit\faster-dooit ...`）
- 配置：`config.lua`，通过 gopher-lua 求值；默认配置镜像原版 `default_config.py` 行为
- 数据：全新 schema，不迁移旧库
- 模式常量：`NORMAL` / `INSERT` / `DATE` / `SEARCH` / `SORT` / `CONFIRM`
- 函数/命名以本计划「Interfaces」块为准；跨任务引用不得重名
- 每个任务结束必须 `git commit`（提交信息含 `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`）

---

### Task 1: 脚手架 + 领域模型 + 存储层

**Files:**
- Create: `go.mod`
- Create: `internal/model/model.go`
- Create: `internal/model/model_test.go`
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`
- Create: `internal/store/schema.go`

**Interfaces:**
- Consumes: 无（首个任务）
- Produces:
  - `model.Workspace{ID int64; OrderIndex int; Description string; IsRoot bool; ParentID *int64; Children []*Workspace; Todos []*Todo}`
  - `model.Todo{ID int64; OrderIndex int; Description string; Due *time.Time; Effort int; Recurrence *time.Duration; Urgency int; Pending bool; ParentWorkspaceID *int64; ParentTodoID *int64}`
  - `store.New(path string) (*Store, error)` — 打开/创建 SQLite，建表
  - `(*Store).LoadAll() (*model.Workspace, error)` — 返回根 Workspace，树已组装好（含全部后代）
  - `(*Store).SaveWorkspace(*model.Workspace) error` / `(*Store).SaveTodo(*model.Todo) error`
  - `(*Store).DeleteWorkspace(id int64) error` / `(*Store).DeleteTodo(id int64) error`
  - `(*Store).BatchOrder(parentKind string, parentID *int64, items []orderItem) error` — 批量更新 order_index

- [ ] **Step 1: 初始化 module 与依赖**

```bash
cd F:\Workspace\Project\dooit\faster-dooit
go mod init github.com/XiaTian-AC/faster-dooit
go get modernc.org/sqlite
```

- [ ] **Step 2: 写模型测试（失败）**

```go
package model

import (
	"testing"
	"time"
)

func TestTodoStatus(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		todo := &Todo{Pending: true}
		if got := todo.Status(); got != "pending" {
			t.Errorf("Status() = %q, want pending", got)
		}
	})
	t.Run("completed", func(t *testing.T) {
		todo := &Todo{Pending: false}
		if got := todo.Status(); got != "completed" {
			t.Errorf("Status() = %q, want completed", got)
		}
	})
	t.Run("overdue", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		todo := &Todo{Pending: true, Due: &past}
		if got := todo.Status(); got != "overdue" {
			t.Errorf("Status() = %q, want overdue", got)
		}
	})
}

func TestTodoTags(t *testing.T) {
	todo := &Todo{Description: "buy milk @grocery @errand"}
	got := todo.Tags()
	if len(got) != 2 || got[0] != "@grocery" || got[1] != "@errand" {
		t.Errorf("Tags() = %v", got)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/model/ -run TestTodoStatus -v`
Expected: FAIL — `Status` / `Tags` 未定义

- [ ] **Step 4: 实现模型**

```go
package model

import "time"

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
```

- [ ] **Step 5: 运行模型测试确认通过**

Run: `go test ./internal/model/`
Expected: PASS

- [ ] **Step 6: 写存储测试（失败）**

```go
package store

import (
	"testing"
	"time"
)

func TestStoreCRUD(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	root, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if root == nil || !root.IsRoot {
		t.Fatal("expected a root workspace")
	}

	ws := &Workspace{Description: "Work"}
	root.Children = append(root.Children, ws)
	if err := st.SaveWorkspace(ws); err != nil {
		t.Fatal(err)
	}

	due := time.Now().Add(24 * time.Hour)
	todo := &Todo{Description: "ship", Due: &due, ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	if err := st.SaveTodo(todo); err != nil {
		t.Fatal(err)
	}

	got, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Children) != 1 || len(got.Children[0].Todos) != 1 {
		t.Fatalf("reload mismatch: children=%d todos=%d", len(got.Children), len(got.Children[0].Todos))
	}
	if got.Children[0].Todos[0].Description != "ship" {
		t.Fatalf("desc = %q", got.Children[0].Todos[0].Description)
	}
}

func TestStoreDelete(t *testing.T) {
	st, _ := New(":memory:")
	root, _ := st.LoadAll()
	ws := &Workspace{Description: "x"}
	root.Children = append(root.Children, ws)
	st.SaveWorkspace(ws)
	todo := &Todo{Description: "t", ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	st.SaveTodo(todo)

	if err := st.DeleteWorkspace(ws.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadAll()
	if len(got.Children) != 0 {
		t.Fatalf("expected 0 children, got %d", len(got.Children))
	}
}
```

- [ ] **Step 7: 运行存储测试确认失败**

Run: `go test ./internal/store/`
Expected: FAIL — `New`/`LoadAll` 未定义

- [ ] **Step 8: 实现 schema 与存储**

`schema.go`:
```go
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
```

`store.go` 要点：`sql.Open("sqlite", path)`；`LoadAll` 用两条 SELECT 全表读出，按 `parent_id`/`parent_workspace_id`/`parent_todo_id` 组装内存树，`order_index` 升序；`Save*` 用 `INSERT ... ON CONFLICT(id) DO UPDATE`；`Delete*` 依赖外键级联（`PRAGMA foreign_keys = ON`，通过 `_pragma=foreign_keys(1)` DSN 参数）。

- [ ] **Step 9: 运行存储测试确认通过**

Run: `go test ./internal/store/`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git -C F:\Workspace\Project\dooit\faster-dooit add go.mod go.sum internal/
git -C F:\Workspace\Project\dooit\faster-dooit commit -m "feat: scaffold model and sqlite store"
```

---

### Task 2: 自然语言日期解析

**Files:**
- Create: `internal/dateparse/dateparse.go`
- Create: `internal/dateparse/dateparse_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `dateparse.Parse(input string, now time.Time) (time.Time, error)` — 解析绝对格式/相对词/快捷记法；`dateparse.Normalize(input string, now time.Time) (string, error)` — 供编辑回显用（原样返回规范化时间串）

- [ ] **Step 1: 写失败测试**

```go
package dateparse

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 8, 2, 12, 0, 0, 0, time.Local)
}

func TestParseAbsolute(t *testing.T) {
	now := fixedNow()
	for _, in := range []string{"2026-08-02", "2026-08-02 15:30", "2026/08/02"} {
		if _, err := Parse(in, now); err != nil {
			t.Errorf("Parse(%q) err = %v", in, err)
		}
	}
}

func TestParseRelative(t *testing.T) {
	now := fixedNow()
	tomorrow := now.AddDate(0, 0, 1)
	got, err := Parse("tomorrow", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, tomorrow) {
		t.Errorf("tomorrow = %v, want %v", got, tomorrow)
	}

	nextMon := nextWeekday(now, time.Monday)
	got, err = Parse("next monday", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, nextMon) {
		t.Errorf("next monday = %v, want %v", got, nextMon)
	}

	got, err = Parse("in 3 days", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, now.AddDate(0, 0, 3)) {
		t.Errorf("in 3 days = %v", got)
	}

	got, err = Parse("3d", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, now.AddDate(0, 0, 3)) {
		t.Errorf("3d = %v", got)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse("not a date at all", fixedNow()); err == nil {
		t.Error("expected error for invalid input")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/dateparse/`
Expected: FAIL

- [ ] **Step 3: 实现**

规则顺序：`today`/`tomorrow` → 快捷记法 `(\d+)([dhw])` → `next <weekday>` → `in <n> <unit>s?` → 绝对 `YYYY-MM-DD[ HH:MM]` 与 `YYYY/MM/DD`。相对词以 `now` 为基准，`today` 归零到当天 0 点，`tomorrow` 为次日 0 点。解析失败返回错误（含原文提示）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/dateparse/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C F:\Workspace\Project\dooit\faster-dooit add internal/dateparse/
git -C F:\Workspace\Project\dooit\faster-dooit commit -m "feat: natural language date parsing"
```

---

### Task 3: bubbletea 骨架 + 两栏树 + NORMAL 导航 + CRUD

**Files:**
- Create: `main.go`
- Create: `internal/app/app.go`（`Model`、`New`）
- Create: `internal/app/update.go`（消息处理）
- Create: `internal/app/view.go`
- Create: `internal/app/mode.go`（模式常量与状态）
- Create: `internal/app/tree.go`（树组件）
- Create: `internal/app/keymap.go`（默认键位）
- Create: `internal/app/action.go`（动作注册表：move_down/move_up/add_sibling/...）
- Create: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `model.Workspace`、`model.Todo`、`store.Store`
- Produces:
  - `app.New(st *store.Store, luaCfg interface{}) *app.Model`
  - `(*app.Model).Init() tea.Cmd`、`Update(tea.Msg) (tea.Model, tea.Cmd)`、`View() string`
  - `type Mode string`，常量 `ModeNormal/ModeInsert/ModeDate/ModeSearch/ModeSort/ModeConfirm`
  - `(*app.Model).VisibleTodos() []*model.Todo`、`(*app.Model).SetFocus(pane int)`（0=workspace,1=todo）
  - 动作注册表：`app.Action` 为 `func(*Model) tea.Cmd`；`app.actions map[string]Action`

- [ ] **Step 1: 写失败测试（update 纯逻辑）**

```go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/XiaTian-AC/faster-dooit/internal/model"
	"github.com/XiaTian-AC/faster-dooit/internal/store"
)

func newTestApp(t *testing.T) *Model {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := st.LoadAll()
	ws := &model.Workspace{Description: "Work"}
	root.Children = append(root.Children, ws)
	st.SaveWorkspace(ws)
	todo := &model.Todo{Description: "a", ParentWorkspaceID: &ws.ID}
	ws.Todos = append(ws.Todos, todo)
	st.SaveTodo(todo)
	m := New(st, nil)
	m.refreshFromStore()
	return m
}

func TestFocusWorkspacePane(t *testing.T) {
	m := newTestApp(t)
	if m.focus != 0 {
		t.Fatalf("initial focus = %d, want 0", m.focus)
	}
	m.SetFocus(1)
	if m.focus != 1 {
		t.Fatalf("focus after SetFocus = %d, want 1", m.focus)
	}
}

func TestMoveDown(t *testing.T) {
	m := newTestApp(t)
	if m.cursor() != 0 {
		t.Fatal("expected cursor 0")
	}
	m.actionMoveDown()
	if m.cursor() != 0 {
		t.Fatalf("single item: cursor should stay 0, got %d", m.cursor())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/app/`
Expected: FAIL

- [ ] **Step 3: 实现骨架**

- `app.go`：`Model` 持有 `store`、`root *model.Workspace`、`focus int`、`mode Mode`、`cursor int`、`expanded map[int64]bool`、`clipboard *model.Todo`（或 workspace 类型标记）、`keys map[string]string`（按键→动作名）。`New` 里 `root = st.LoadAll()`（Task 1 产物）。
- `tree.go`：`VisibleWorkspaces()` / `VisibleTodos()` 返回展平可见列表（`filter_refresh`/`always_expand` 稍后 Task 6 接入），只维护游标。
- `keymap.go`：默认键位镜像原版——`j/k` 上下、`i/d/r/e` 编辑四字段、`a/A` 兄弟/子、`z/Z` 展开/父、`gg/G` 首末、`J/K` 下移/上移、`xx` 删除、`y/Y` 复制描述/节点、`p/P` 粘贴下/上、`c` 完成、`=/+` 增急、`-/_` 减急、`/` 搜索、`ctrl+s` 排序、`ctrl+q` 退出、`?` 帮助、`tab` 切焦、`enter` 进入编辑描述。
- `action.go`：动作函数操作内存模型并落库（`Save*`/`BatchOrder`），返回 `tea.Cmd`（可空）。
- `view.go`：用 lipgloss 画两栏；TODO 栏取当前 workspace 的可见 todo。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 5: 可运行验证**

Run: `go run .`（终端内验证启动秒开、j/k 移动、a 新增、xx 删除、c 完成切换）
Expected: 应用可启动，基本导航与 CRUD 生效

- [ ] **Step 6: Commit**

```bash
git -C F:\Workspace\Project\dooit\faster-dooit add main.go internal/app/
git -C F:\Workspace\Project\dooit\faster-dooit commit -m "feat: bubbletea skeleton with two-pane tree and CRUD"
```

---

### Task 4: 模式与输入（编辑/搜索/排序/确认/帮助）

**Files:**
- Create: `internal/app/input.go`（输入覆盖层，基于 `bubbles/textinput`）
- Create: `internal/app/bars.go`（通知栏/确认栏）
- Create: `internal/app/help.go`（帮助页）
- Modify: `internal/app/update.go`、`internal/app/keymap.go`
- Create: `internal/app/input_test.go`

**Interfaces:**
- Consumes: Task 3 的 `Mode`、动作注册表
- Produces:
  - `(*app.Model).StartEdit(field string) tea.Cmd` — 进入 INSERT，按 field 选输入语义
  - `(*app.Model).StartSearch() tea.Cmd` / `(*app.Model).StartSort() tea.Cmd`
  - `(*app.Model).StartConfirm() tea.Cmd` — 删除确认
  - `(*app.Model).Notify(msg string, level string) tea.Cmd` — 通知栏
  - `(*app.Model).HelpView() string` — 帮助页渲染

- [ ] **Step 1: 写失败测试**

```go
package app

import "testing"

func TestModeTransitions(t *testing.T) {
	m := newTestApp(t)
	m.StartEdit("description")
	if m.mode != ModeInsert {
		t.Fatalf("mode = %v, want INSERT", m.mode)
	}
	m.ConfirmEdit()
	if m.mode != ModeNormal {
		t.Fatalf("mode = %v, want NORMAL after confirm", m.mode)
	}
}

func TestEditDueParsesDate(t *testing.T) {
	m := newTestApp(t)
	m.StartEdit("due")
	m.input.SetValue("tomorrow")
	m.ConfirmEdit() // due 解析失败应留在输入态
	if m.mode != ModeInsert {
		t.Fatalf("invalid due should stay editing, mode = %v", m.mode)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/app/ -run TestModeTransitions -v`
Expected: FAIL

- [ ] **Step 3: 实现**

- `input.go`：一个 `textinput.Model` 复用；`StartEdit` 按字段（description 纯文本 / due 走 `dateparse` / effort、urgency 整数 / recurrence 走时长解析）设置校验。非 NORMAL 模式时 `Update` 将按键路由给 input，`enter` 提交、`escape` 取消回 NORMAL。
- 编辑语义对齐原版：描述编辑提交后触发 `TodoDescriptionChanged`；due 失败通知且不清空。
- `bars.go`：通知栏带 auto-exit（气泡消息，几秒后清除）；确认栏 `y/n`。
- `help.go`：按键表渲染。
- `keymap.go`：把 `?`→help、`/`→search、`ctrl+s`→sort、`xx`→confirm 等接到新动作。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 5: 可运行验证**

Run: `go run .`
Expected: `i` 编辑描述、`d` 编辑日期（输入 `tomorrow` 生效）、`/` 搜索、`ctrl+s` 排序、`xx` 确认删除、`?` 帮助

- [ ] **Step 6: Commit**

```bash
git -C F:\Workspace\Project\dooit\faster-dooit add internal/app/
git -C F:\Workspace\Project\dooit\faster-dooit commit -m "feat: modes and input overlays for edit/search/sort/confirm/help"
```

---

### Task 5: Lua 配置系统 + 默认 config.lua

**Files:**
- Create: `internal/lua/lua.go`
- Create: `internal/lua/lua_test.go`
- Create: `config.lua`
- Modify: `internal/app/app.go`、`main.go`

**Interfaces:**
- Consumes: 动作注册表（Task 3）、主题/布局/格式化器（Task 6 会用到，这里先定义数据结构）
- Produces:
  - `lua.EvalFile(path string) (*lua.Runtime, error)` — 返回运行时（含 `api` 表）
  - `type lua.Runtime struct { Keys map[string]string; Layouts Layouts; Formatters Formatters; Bar []BarWidget; Dashboard []string; Theme Theme; Subscribers []Subscriber; Timers []Timer }`
  - `type Theme struct{ Primary, Secondary, Background, Background1, Green, Yellow, Orange, Red string }`
  - `type BarWidget struct { Name string; Fn *lua.LFunction; EverySec float64 }`
  - `(*lua.Runtime).CallFormatter(l *lua.LFunction, value any, model any) (string, error)` — 调用 Lua 函数返回 `{text, style}`
  - `(*lua.Runtime).Emit(event string, args ...any)` — 触发 `subscribe` 注册的回调

- [ ] **Step 1: 写失败测试**

```go
package lua

import "testing"

func TestEvalDefaultConfig(t *testing.T) {
	rt, err := EvalFile("../../../config.lua")
	if err != nil {
		t.Fatal(err)
	}
	if rt.Keys["j"] != "move_down" {
		t.Errorf("j key = %q, want move_down", rt.Keys["j"])
	}
	if len(rt.Bar) == 0 {
		t.Error("expected bar widgets")
	}
	if len(rt.Layouts.Todo) != 4 {
		t.Errorf("todo layout = %v, want 4 columns", rt.Layouts.Todo)
	}
}

func TestThemeLoaded(t *testing.T) {
	rt, _ := EvalFile("../../../config.lua")
	if rt.Theme.Primary == "" {
		t.Error("theme.primary not loaded")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/lua/`
Expected: FAIL（config.lua 不存在 / 绑定未实现）

- [ ] **Step 3: 实现绑定**

- `lua.go`：`lua.NewState(L)`；构造 `api` 表并注册函数：`keys.set`、`layouts.*`（赋值表）、`formatter.todos.<field>.add(fn)`、`bar.set([])`、`dashboard.set([])`、`subscribe(event, fn)`、`timer(n, fn)`、`api.vars.theme`、动作函数（`api.move_down` 等转发到 Go 动作表）。
- `config.lua`：镜像原版 `default_config.py`——模式指示、时钟、用户名 widget；四个 todo formatter（status/due/urgency/recurrence）；键位全集；两栏 layout；状态栏顺序；dashboard 文案。
- 桥接：Lua 返回 `{text=..., style=...}` 表 → `string`；主题色从 Lua 表读入 `Theme` 结构。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/lua/`
Expected: PASS

- [ ] **Step 5: 接入启动**

`main.go`：读 `--config`/默认 `config.lua` → `lua.EvalFile` → `app.New(st, rt)`。配置错误时打印错误屏（文件+行号）。

- [ ] **Step 6: Commit**

```bash
git -C F:\Workspace\Project\dooit\faster-dooit add internal/lua/ config.lua main.go internal/app/app.go
git -C F:\Workspace\Project\dooit\faster-dooit commit -m "feat: lua config system with default config.lua"
```

---

### Task 6: 布局/格式化器/条栏/仪表盘/主题渲染

**Files:**
- Create: `internal/theme/theme.go`
- Create: `internal/app/renderers.go`（列渲染 + 版本缓存）
- Create: `internal/app/dashboard.go`
- Modify: `internal/app/view.go`、`internal/app/bars.go`、`internal/app/tree.go`
- Create: `internal/app/renderers_test.go`

**Interfaces:**
- Consumes: `lua.Runtime`（Layouts/Formatters/Bar/Theme/Dashboard）
- Produces:
  - `(*app.Model).ColumnLayout(pane int) []string` — 当前列布局
  - `(*app.Model).RenderRow(pane int, idx int) string` — 单行渲染（含 formatter），带版本缓存
  - `(*app.Model).BumpVersion()` — 状态变化时自增版本，失效缓存
  - 主题应用到 view：lipgloss 样式按 `Theme` 字段生成

- [ ] **Step 1: 写失败测试**

```go
package app

import "testing"

func TestRenderRowCaches(t *testing.T) {
	m := newTestAppLua(t) // 用真实 config.lua 构造
	v0 := m.RenderRow(1, 0)
	v1 := m.RenderRow(1, 0)
	if v0 != v1 {
		t.Fatal("cache miss on unchanged state")
	}
	m.BumpVersion()
	v2 := m.RenderRow(1, 0)
	if v2 == "" {
		t.Fatal("expected rendered row after bump")
	}
}

func TestColumnLayoutFromConfig(t *testing.T) {
	m := newTestAppLua(t)
	cols := m.ColumnLayout(1)
	if len(cols) == 0 {
		t.Fatal("no todo layout columns")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/app/ -run TestRenderRowCaches -v`
Expected: FAIL

- [ ] **Step 3: 实现**

- `renderers.go`：列布局来自 `lua.Runtime.Layouts`；每格按字段选格式化器（内置 Go formatter + Lua formatter 调用）；行字符串缓存 `map[int64]string` 以 `version` 为 key 失效。`BumpVersion` 在每次 update 落库后调用。
- `bars.go`：状态栏按 `Bar` widget 数组渲染，`timer` 用 bubbletea `tea.Tick` 驱动（时钟 1s）；通知栏渲染在底部。
- `dashboard.go`：`Dashboard []string` 渲染到 TODO 栏顶部区域（原版 dashboard）。
- `theme.go`：`Theme` 结构 → lipgloss 颜色常量；从 `lua.Runtime.Theme` 填充。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 5: 可运行验证**

Run: `go run .`
Expected: 状态栏显示模式/时钟/用户名，todo 列按 status/description/due/urgency 渲染，颜色来自 config.lua

- [ ] **Step 6: Commit**

```bash
git -C F:\Workspace\Project\dooit\faster-dooit add internal/theme/ internal/app/renderers.go internal/app/dashboard.go
git -C F:\Workspace\Project\dooit\faster-dooit commit -m "feat: layout, formatters, bars, dashboard, theme rendering"
```

---

### Task 7: 对齐打磨 + 性能验证 + README

**Files:**
- Modify: 上述各文件（按对拍结果）
- Create: `README.md`
- Create: `benchmarks_test.go`（`internal/app/`）

**Interfaces:**
- Consumes: 全部
- Produces: 无新接口；对拍矩阵与基准数据

- [ ] **Step 1: 写性能基准测试**

```go
package app

import (
	"testing"
)

func BenchmarkVisibleRows10k(b *testing.B) {
	m := newTestAppWithTodos(t, 10000) // 构造 1 万 todo 的测试环境
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.VisibleTodos()
	}
}

func BenchmarkUpdate(b *testing.B) {
	m := newTestAppWithTodos(t, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.actionMoveDown()
	}
}
```

- [ ] **Step 2: 运行基准并记录**

Run: `go test ./internal/app/ -bench . -benchmem`
Expected: 记录 `VisibleRows10k` 与 `Update` 的 ns/op；目标 `Update` < 1ms、`VisibleRows` 10k 行 < 10ms。超标的优化：剪枝只渲染视口、避免每帧重建。

- [ ] **Step 3: 与原版功能对拍矩阵**

逐项核对（原版行为来源见 spec 与源码）：两栏树、六模式、键位全集、自然语言日期、循环完成推进、搜索过滤、排序（pending 优先→due→order_index 语义）、剪贴板复制/粘贴上下、通知/确认/搜索/排序栏、帮助页、仪表盘、Lua 扩展（subscribe/timer/formatter/bar/theme/layout/keys）。每项运行验证，记录差异。

- [ ] **Step 4: 启动时间验证**

Run: `time go run .`（或 `go build` 后测二进制）
Expected: 启动到界面 < 200ms（记录实测值）

- [ ] **Step 5: 写 README**

安装、`config.lua` 指南、键位表、与原版 dooit 的关系。

- [ ] **Step 6: Commit**

```bash
git -C F:\Workspace\Project\dooit\faster-dooit add README.md internal/app/benchmarks_test.go
git -C F:\Workspace\Project\dooit\faster-dooit commit -m "feat: parity polish, perf validation, README"
```

---

## 收尾（实现完成后，由本机执行）

- 创建公开 GitHub 仓库 `faster-dooit`（`gh repo create faster-dooit --public --source F:\Workspace\Project\dooit\faster-dooit --remote origin --push`）
- 确认 `git remote -v`、`git log --oneline` 正常

## 设计缺口（评审阶段标记，见 /autoplan 决策日志）

- 原版 `poll_dooit_db`（外部改库热刷新）不实现——单进程应用，按 spec 砍掉
- `dateparse` 覆盖度按原版 test_date_parse 对拍，缺失格式列入对拍矩阵
