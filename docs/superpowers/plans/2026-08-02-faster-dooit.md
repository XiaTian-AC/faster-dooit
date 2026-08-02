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
- **重新评估门（评审新增）**：M1/M2（Task 1-4）完成后设检查点——若核心树 + CRUD + 编辑 + 持久化已可用且启动达标，由用户决定是否继续 M3-M7，不强制全做
- **"够快"判定（评审新增）**：以 `go build` 后的二进制冷启动实测为准（不是 `go run`，避免混入编译时间），并记录原版 dooit 冷启动作基线对比
- **默认路径（评审新增）**：DB 默认 `%APPDATA%\faster-dooit\todo.db`，config 默认 `%APPDATA%\faster-dooit\config.lua`（Windows；用 `os.UserConfigDir()`/`os.UserCacheDir()` 推算）；`main.go` 在 `store.New` 前 `os.MkdirAll(dataDir, 0o755)`；`--config` 指向不存在文件时打印 "Config file {path} not found." 并退出（对齐原版）
- **配置合并语义（评审新增）**：config.lua 为**整体替换**——用户直接编辑分发的默认 config.lua（全注释、充当参考文档）；不搞默认+覆盖双层合并
- **CLI 表面（评审新增）**：完整支持 `-c/--config`、`--db`、`-v/--version`（打印 "faster-dooit - x.y.z"）、`-h/--help`；`migrate` 子命令**明确不实现**（全新 schema，README 注明"旧 dooit 数据不导入"）；提供 `config_loc`（或 `config` 子命令）打印已解析 config.lua 路径

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

- [ ] **Step 0: 记录原版基线（评审新增）**

```bash
# 记录原版 dooit 冷启动耗时与用户真实 todo 规模，作为重写前的 before 数据
cd F:\Workspace\Project\dooit\dooit
Measure-Command { dooit --version | Out-Null }   # 导入+初始化基线
# 记录用户实际 DB 规模：SELECT COUNT(*) FROM todo; 等
```

Expected: 得到原版启动耗时（秒级）与用户实际 todo 数量，写入 README 基准对比表。若用户实际 todo 数 < 100，10k 节点目标仅作上限参考，不为此过度设计。

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

`store.go` 要点（评审修正，含 3 个 critical）：`sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")`；**必须 `db.SetMaxOpenConns(1)`**——否则 `:memory:` 测试里每个连接是独立空库、`foreign_keys` 也是每连接生效，测试会不确定失败；`LoadAll` 两条 SELECT 全表读出按父键组装内存树，`order_index` 升序、平级按 `id` 兜底；**新建行（ID==0）走 `INSERT` 并 `LastInsertId()` 回写结构体，已有 ID 才走 `ON CONFLICT(id) DO UPDATE`**（否则 id=0 会真插入、子节点父键全连到 0 冲突）；**新非根 workspace 无父时自动挂到根**（对齐原版 `Workspace.save()`）；`Delete*` 依赖 FK 级联。补测试：FK 确实生效（删 workspace 后查 todo 无孤儿）、两个新节点 id 不同且子节点父键匹配。

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

**评审修正（日期覆盖）**：必须兼容原版 `test_date_parse.py` 的全部用例——`2020-01-01`（ISO）、`july 1 2034`（英文月+日+年）、`jan 1`（英文缩写月+日，默认今年）、`?????`（无效→报错）。README 明确"支持格式为明确子集，**非** python-dateutil 全等价"，不宣称完整日期解析对齐。Step 1 测试代码补充以下用例：

```go
// 追加到 dateparse_test.go
func TestParseEnglishMonth(t *testing.T) {
	now := fixedNow()
	got, err := Parse("july 1 2034", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2034, 7, 1, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("july 1 2034 = %v, want %v", got, want)
	}
	got, err = Parse("jan 1", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != now.Year() || got.Month() != time.January || got.Day() != 1 {
		t.Errorf("jan 1 = %v", got)
	}
}
```

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
- **评审修正（组合键）**：`keys` 不能是平面 `map[string]string`——默认键位含 `xx`/`gg` 多键组合。实现 KeyManager：输入缓冲 + 前缀匹配动作表（对齐原版 `keys.py`），按模式分表；`escape` 清缓冲、死键吞掉（按下 `g` 再按 `k` → `k` 丢失）；单按 `x` 无动作、连按 `xx` 才删除。测试：单 `x` 无动作、`xx` 删除、`g`-`k` 行为。
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
	m.ConfirmEdit() // 评审修正：解析失败留在输入态是"有意的改进"（原版是退出编辑并保留旧值）
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
  - `(*lua.Runtime).CallFormatter(l *lua.LFunction, value any, model any, theme Theme) (string, error)` — 调用 Lua 函数返回 `{text, style}`；**必须透传 theme/运行时上下文**（原版每个默认 formatter 都从 `api.vars.theme` 取色）
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
- **评审修正（Lua 表面）**：明确 Lua API 是**封闭子集**（`keys`/`layouts`/`formatter`/`bar`/`dashboard`/`subscribe`/`timer`/`vars.theme`），README 不宣称与原版任意 Python API 全等；不实现 `api.css`/`plugin_manager`。文档如实标注，删掉"完整对齐"的过度表述。

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

- `renderers.go`：列布局来自 `lua.Runtime.Layouts`；每格按字段选格式化器（内置 Go formatter + Lua formatter 调用）。
- **评审修正（缓存设计）**：行渲染缓存**只缓存视口内行**，按行内容（窄脏集）失效；**1s 时钟/条栏刷新与行缓存解耦**——默认布局的行里不含时钟，时钟只更新状态栏一格，绝不触发全行缓存失效。不做全局 `version` 全量失效（10k 行场景每帧重建是原版的老毛病）。
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
**评审修正**：在 README 明确 `poll_dooit_db`（外部改库热刷新）为**单进程取舍、明确不实现**（而非藏在"设计缺口"括注里）；对拍矩阵里把日期解析的已支持格式清单与不支持项列成表格。测试策略补强（见 Task 7 之后）：排序/循环/搜索/粘贴位置的语义用表驱动测试覆盖；并用 bubbletea `tea.TestModel` 写一条端到端测试（按键→动作→持久化）驱动真实按键消息。
**评审修正（完成级联，重要 parity 缺口）**：原版 `update_hooks.py` 的完成/循环级联必须实现——`c` 切换完成时：完成 todo 级联完成**整个子树**；父 todo 仅当**全部子项**完成才自动完成；重新打开任一子项会重新打开父项；**循环 todo 完成时 `due += recurrence` 且强制 pending=true（循环任务永不 completed）**。用表驱动测试覆盖四条规则。排序语义补正：复合键 `pending→due→order_index` **仅用于 `pending` 排序**；其余字段按升序 + `nulls_last`（due 为 NULL 排最后）；排序菜单 todo 7 项 / workspace 2 项；`reverse` 原地反转；平级按 `order_index, id` 兜底。

- [ ] **Step 4: 启动时间验证**

Run: `go build` 后计时生成的 `.exe`（**不要用 `go run`**，那会混入冷编译时间）；同时用 Task 1 Step 0 记录的原版基线对比
Expected: 二进制冷启动到界面 < 200ms，且低于原版基线（记录两者实测值入 README 基准表）

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

---

## GSTACK REVIEW REPORT

### Phase 1 (CEO) — Dual Voice Consensus

Claude 子代理独立声部已运行（codex 不可用，单声部）。主评审核对了原版源码（`model_tree.py`、`tui.py`、`date_parser.py`、`test_date_parse.py`）。

```
CEO DUAL VOICES — CONSENSUS TABLE (codex 不可用，单声部):
  Dimension                           Claude  Consensus
  1. Premises valid?                   MEDIUM  ACCEPT (用户已确认前提)
  2. Right problem to solve?           CHALLENGE → 提升到最终门（#2）
  3. Scope calibration correct?        HIGH    ACCEPT（评审修正入计划）
  4. Alternatives sufficiently explored? CHALLENGE → 提升到最终门（#2）
  5. Competitive/market risks covered? HIGH    ACCEPT（公开仓库风险→最终门 #6）
  6. 6-month trajectory sound?         HIGH    ACCEPT（重评估门已加）
```

| # | 发现 | 严重度 | 自动决策 | 结果 |
|---|------|--------|---------|------|
| 1 | 问题未测量（无原版基线/真实数据量） | CRITICAL | 采纳 | Task 1 加 Step 0 基线；README 基准表 |
| 2 | 全量重写 vs 原地优化/温驻留 | CRITICAL | **不自动决策 → 最终门** | 用户已选重写，门处让用户确认 |
| 3 | Lua formatter 桥漏传 theme/api | HIGH | 采纳（P1/P5） | `CallFormatter` 加 theme 参数；Lua 表面标注为封闭子集 |
| 4 | 手写 dateparse 无法对齐 dateutil | HIGH | 采纳（P5，显式胜于聪明） | 补英文月份格式 + 明确子集声明 |
| 5 | `go run` 会伪造启动指标 | HIGH | 采纳（P3） | Task 7 改 `go build` 后计时 + 原版基线 |
| 6 | 公开仓库 vs 活跃上游（60x stars） | HIGH | **不自动决策 → 最终门** | 用户已要求公开仓库，门处确认风险 |
| 7 | 无时间预估/无重评估门 | HIGH | 采纳（P2/P6） | Global Constraints 加重评估门 + "够快"判定 |
| 8 | 全局版本缓存与 1s 时钟耦合 | MEDIUM | 采纳（P5） | 视口内行缓存 + 时钟与行缓存解耦 |
| 9 | 测试没覆盖难点 | MEDIUM | 采纳（P1） | 表驱动语义测试 + tea.TestModel e2e |
| 10 | poll_dooit_db 取舍措辞不诚实 | MEDIUM | 采纳（P3/P5） | README 明确单进程取舍 |

### Decision Audit Trail

<!-- AUTONOMOUS DECISION LOG -->
| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
|---|-------|----------|-----------|-----------|----------|----------|
| 1 | CEO | 采纳：Task 1 加原版基线测量 | Mechanical | P3 | 无 before 数据无法证明"变快" | 无 |
| 2 | CEO | 采纳：Lua formatter 透传 theme + 表面标封闭子集 | Mechanical | P5/P1 | 原版默认 formatter 依赖 api.vars.theme | 无 |
| 3 | CEO | 采纳：dateparse 补英文月格式 + 明确子集声明 | Mechanical | P5 | 对齐原版 test_date_parse 用例 | 无 |
| 4 | CEO | 采纳：`go build` 后计时 + 原版基线 | Mechanical | P3 | `go run` 混编译时间 | 无 |
| 5 | CEO | 采纳：Global Constraints 加重评估门 | Mechanical | P2/P6 | 给项目退出条件 | 无 |
| 6 | CEO | 采纳：视口内行缓存 + 时钟解耦 | Mechanical | P5 | 10k 行每帧重建是原版老毛病 | 无 |
| 7 | CEO | 采纳：测试补语义表驱动 + e2e | Mechanical | P1 | 对拍矩阵需自动化锚点 | 无 |
| 8 | CEO | 采纳：README 明确 poll_dooit_db 取舍 | Mechanical | P5 | 措辞诚实 | 无 |
| 9 | CEO | #2（重写 vs 原地优化）→ 最终门 | User Challenge | — | 用户已明确选重写；门处确认 | — |
| 10 | CEO | #6（公开仓库 vs 上游）→ 最终门 | User Challenge | — | 用户已明确要公开仓库；门处确认 | — |

### Phase 2 (Design) — Dual Voice Consensus

Claude 设计子代理独立声部已运行（单声部）。12 条发现全部为规格补强（无用户方向冲突，全部自动决策采纳），核心修正已写入设计文档「UI 规格（评审增补 2026-08-02）」：

```
DESIGN DUAL VOICES — CONSENSUS TABLE (codex 不可用，单声部):
  Dimension                           Claude  Consensus
  1. Frame ratio / hierarchy          HIGH    ACCEPT → UI 规格 #1
  2. Dashboard model (right pane)     HIGH    ACCEPT → UI 规格 #2（修正为全栏欢迎页）
  3. Focus indication                 HIGH    ACCEPT → UI 规格 #3
  4. Empty states                     HIGH    ACCEPT → UI 规格 #4
  5. Search/filter semantics          MED-HI  ACCEPT → UI 规格 #5
  6. Cursor policy across panes       MED     ACCEPT → UI 规格 #6
  7. Scroll / viewport / resize       HIGH    ACCEPT → UI 规格 #7
  8. Column alignment                 HIGH    ACCEPT → UI 规格 #8
  9. Default style values enumerated  MED     ACCEPT → UI 规格 #9
  10. Right-pane auto-link            MED     ACCEPT → UI 规格 #10
  11. Confirm dialog default-no       LOW-MED ACCEPT → UI 规格 #11
  12. DATE dead mode / mouse          LOW     ACCEPT → UI 规格 #12、#13
```

**关键设计修正**：仪表盘从"TODO 栏顶部区域"改为原版语义——右栏未选中 workspace 时显示全栏欢迎页；列对齐/滚动/焦点/空状态均已落到具体规格，实现时逐条对齐。

### Phase 3 (Eng) — Dual Voice Consensus

Claude 工程子代理独立声部已运行（单声部），核对了原版 `update_hooks.py`/`todo.py`/`workspace.py`/`keys.py`/`plug.py`/`formatter_store.py`/`model_inputs.py`。18 条发现全部采纳（无用户方向冲突）。

```
ENG DUAL VOICES — CONSENSUS TABLE (codex 不可用，单声部):
  Dimension                           Claude  Consensus
  1. Architecture sound?              HIGH    ACCEPT（3 个 critical 修入 Task 1）
  2. Test coverage sufficient?        MED-HI  ACCEPT（测试缺口并入实施指引）
  3. Performance risks addressed?     MED-HI  ACCEPT（列宽/批量/时钟耦合修正）
  4. Security threats covered?        MED     ACCEPT（Lua 沙箱化）
  5. Error paths handled?             MED-HI  ACCEPT（写失败顺序/日期失败语义）
  6. Deployment risk manageable?      N/A     ACCEPT（无部署面）
```

| # | 发现 | 严重度 | 决策 | 处置 |
|---|------|--------|------|------|
| 1 | SQLite 连接模型（:memory:/FK 每连接） | CRITICAL | 采纳(P5/P1) | Task 1：`SetMaxOpenConns(1)` + `file:` URI + `_pragma=foreign_keys(1)` |
| 2 | Upsert 不回写新 ID | CRITICAL | 采纳(P1) | Task 1：INSERT+`LastInsertId()` 回写，已知 ID 才 upsert |
| 3 | gopher-lua 非线程安全 | CRITICAL | 采纳(P1/P5) | 见下方「实施指引 #3」 |
| 4 | 完成/循环级联缺失 | HIGH | 采纳(P1) | Task 7 已补：子树级联/父自动完成/循环推进 |
| 5 | 列宽 max-content 是 O(n×Lua) | HIGH | 采纳(P5) | 见实施指引 #5 |
| 6 | 过期状态跨 tick 陈旧 | HIGH | 采纳(P3) | 见实施指引 #6 |
| 7 | 排序语义不准 | HIGH | 采纳(P1/P5) | Task 7 已补：nulls_last/仅 pending 复合键/7+2 项 |
| 8 | 组合键 xx/gg 无法用平面 map | HIGH | 采纳(P1) | Task 3 已补：KeyManager 输入缓冲 |
| 9 | Lua 桥接缝隙（fn 命名/多 formatter/bar 值持有） | MED | 采纳(P5) | 见实施指引 #9 |
| 10 | DB 写失败内存/磁盘分叉 | MED | 采纳(P5) | 见实施指引 #10 |
| 11 | 批量操作阻塞事件循环 | MED | 采纳(P3/P5) | 见实施指引 #11 |
| 12 | 新节点未自动挂根 | MED | 采纳(P1) | Task 1 已补：root 自动连线 |
| 13 | due 失败语义与原版相反 | MED | 采纳(P3) | Task 4 已注：留在输入态是**有意改进** |
| 14 | Lua 执行攻击面 | MED | 采纳(P1) | 见实施指引 #14 |
| 15 | 测试缺口 | MED | 采纳(P1) | 见实施指引 #15 |
| 16 | 粘贴/克隆排序未定义 | MED | 采纳(P5) | 见实施指引 #16 |
| 17 | 日期/循环过度宣称 | MED | 采纳(P5) | 见实施指引 #17 |
| 18 | 杂项（SortBar 空选/always_expand/WindowSizeMsg 0 尺寸） | LOW | 采纳(P3) | 见实施指引 #18 |

### Eng 实施指引（采纳发现的落地要点，实现时并入对应任务）

- **#3 Lua 线程模型**：声明不变式——**Lua 只在 bubbletea 主 goroutine 调用**（Update/View 内）。`tea.Tick` 回调只返回携带 `time.Now()` 的 Msg，不碰 Lua；Lua 调用全部在 Update 里。跑 `go test -race` 验证。
- **#5 列宽计算**：只对**可见窗口**算 max-content，设硬上限（既限宽度也限计算量），按版本缓存；某行溢出上限时重算（对齐原版 `cache_clear` 溢出语义）。加列宽路径的 benchmark，不只测 `VisibleRows`。
- **#6 过期翻转**：tick Msg（主 goroutine，廉价）只对 `due ∈ (last_tick, now]` 的行重算 status 格（O(1)），不动其余行缓存；固定 `now` 的测试。
- **#9 Lua 桥**：加 fn→名字注册表（`map[*lua.LFunction]string` 解 `keys.set("j", api.move_down)`）；`layouts` 表用 `__newindex` 拦截写入 `Runtime.Layouts`；formatter 是**多 formatter store**（对齐原版 `formatter_store.py`：类型 1 反向注册序直到非 nil）；bar widget 是**值持有者**（对齐 `plug.py`：订阅/定时函数把结果暂存在函数上，bar 渲染时读值），支持 width-0/width-1 分隔符；formatter 传完整 api 上下文对象（不只 theme）。
- **#10 写失败顺序**：明确"先落库成功再改内存"，或失败时从 DB 重载替换 root 回滚内存；加失败注入测试（如关掉 DB 后执行动作）。
- **#11 批量操作**：`BatchOrder` 单事务 + prepared statement；显式 benchmark 排序 10k；超预算则加脏队列后台刷盘。
- **#14 Lua 沙箱**：从 LState 剥离 `os.execute`/`io`/`package`/`debug`，`SetMx` 指令上限，每个 Lua 调用点 `recover()` 转通知栏；README 文档化沙箱。
- **#15 测试补强**：FK 级联（嵌套 workspace+子 todo）、upsert ID 回写、单连接内存库、排序表驱动（nulls_last/仅 pending 复合/ties by id）、剪贴板粘贴顺序+子树克隆、循环推进+完成级联、组合键时序、tick 不清行缓存+过期翻转、`WindowSizeMsg` 0 尺寸、空串 due 清空、`tea.TestModel` e2e 作为正式任务。
- **#16 粘贴**：克隆子树插入内存树游标 ±1，用 `BatchOrder` 重排受影响兄弟（或依赖 `ORDER BY order_index, id`）；跨类型粘贴（workspace 贴到 todo 下）报错通知。
- **#17 日期/循环**：循环输入对齐原版**单 token** `^(\d+)[mhdw]$`（不做 `1d 2h`）；`next monday` 当今天是周一时 → +7 天；`due TEXT` 用 `time.RFC3339`（带偏移）明确编码，加往返存储测试。
- **#18 杂项**：SortBar 无高亮项时定义错误路径；`always_expand_todos`/`always_expand_workspaces` 给默认值；`WindowSizeMsg` 0 尺寸初始渲染。

### Phase 3.5 (DX) — Dual Voice Consensus

Claude DX 子代理独立声部已运行（单声部），核对了原版 `__main__.py`（CLI）/`plug.py`/`loader.py`/帮助页/wiki。15 条发现全部采纳（无用户方向冲突）。

```
DX DUAL VOICES — CONSENSUS TABLE (codex 不可用，单声部):
  Dimension                           Claude  Consensus
  1. Getting started < 5 min?          MED-HI  ACCEPT（首次运行冒烟 + TTHW 文档）
  2. API/CLI naming guessable?         HIGH    ACCEPT（CLI 表面/默认路径已补）
  3. Error messages actionable?        MED-HI  ACCEPT（cause+fix 错误表、日期格式提示）
  4. Docs findable & complete?         MED     ACCEPT（README 展开 + Moving from dooit）
  5. Upgrade path safe?                LOW     ACCEPT（无升级面；旧库不导入已注明）
  6. Dev environment friction-free?    MED     ACCEPT（LICENSE/CI/维护者 quickstart）
```

| # | 发现 | 严重度 | 决策 | 处置 |
|---|------|--------|------|------|
| 1 | 无端到端首次运行冒烟 | MED | 采纳 | Task 3/4 加：空 DB → 欢迎页 → `a` 进 INSERT → `enter` 持久化 |
| 2 | 默认 DB 路径未定义、目录不建 | HIGH | 采纳 | Global Constraints：默认路径 + `os.MkdirAll` |
| 3 | CLI 表面未定义 | HIGH | 采纳 | Global Constraints：CLI 表面；不实现 migrate |
| 4 | 默认 config.lua 位置未定义、无发现/校验 | HIGH | 采纳 | 搜索序 `--config`→`%APPDATA%`→bundled；`config_loc`；缺失/非法 Lua 错误 |
| 5 | 默认与用户配置合并语义未定义 | MED | 采纳 | Global Constraints：整体替换 |
| 6 | 设计文档 Lua 示例非法 | MED | 采纳(P5) | 已改为可运行 Lua；事件名用字符串；`api.no_op` 解绑 |
| 7 | `api.notify` 缺失 | LOW | 采纳 | Lua 表面加 `api.notify(message, level)` |
| 8 | 配置/DB 错误缺 cause+fix；回滚策略二选一 | MED | 采纳 | 错误表补 cause+fix；**选"先落库成功再改内存"** |
| 9 | 文档一行、无迁移指南 | MED | 采纳 | Task 7 README 展开（见下方 DX 实施指引 #9） |
| 10 | 无分发/发布机制 | LOW | 采纳 | README 给 `go build -o faster-dooit.exe .` |
| 11 | 维护者上手/License/CI | MED | 采纳 | MIT LICENSE + GitHub Actions + quickstart + `gofmt -l .` |
| 12 | dateparse 文档与计划不一致 | MED | 采纳 | 设计文档已对齐（去 `2 weeks from now`） |
| 13 | 循环输入/显示不一致 | LOW | 采纳 | 设计文档已注明：单 token 输入、多 token 显示 |
| 14 | Theme 固定子集丢未知键 | LOW | 采纳 | 未知 theme 键 → 警告通知 |
| 15 | 200ms 目标终端未钉死 | LOW | 采纳 | 以 Windows Terminal/ConPTY 为基线，记录终端 |

### DX 实施指引（采纳发现的落地要点）

- **#9 README 展开**（Task 7 Step 5 从一行扩为清单）：Windows 安装/构建（`go build -o faster-dooit.exe .`）、3 个 config.lua 示例（改键 / 加 formatter / 自定义主题）、完整键位表、"Moving from dooit" 章节（Python API→Lua 对照表 + "旧 dooit 数据不迁移" 声明）。
- **#11 仓库卫生**：加 MIT LICENSE 文件任务；GitHub Actions 最小 workflow（`go test ./...` + `go build`，windows/linux 矩阵）；维护者 quickstart 块（build/run/test/vet 命令 + bubbletea/gopher-lua 入门阅读）；`gofmt -l .` 纳入收尾检查。
- **#8 错误表**：每条补 cause+fix；DB 写失败采用**先落库成功再改内存**（内存是派生状态）；日期解析错误文本带支持格式提示。
- **#14 主题**：`Theme` 扩展到原版全色板（含 background3 等），未知键警告。

### 架构依赖图（Eng Phase 1 产出）

```
main.go ──► internal/lua ──► config.lua（gopher-lua 求值 → Runtime{Keys,Layouts,Formatters,Bar,Dashboard,Theme}）
   │                │
   │                ▼
   └──► internal/app（bubbletea Elm）
             │  Model{root *model.Workspace, focus, mode, cursors[2], expanded, clipboard, keys}
             ├─► Update(Msg) ──► 动作（改内存 + store 落库）──► return Cmd
             ├─► View() ──► viewport 行渲染（lipgloss + Runtime 格式化器）
             └─► tea.Tick(1s) ──► 仅返回时间 Msg（Lua 只在主 goroutine）
                    │
   internal/store ◄─┘（modernc.org/sqlite，SetMaxOpenConns(1)，先落库后改内存）
   internal/dateparse（纯 Go 子集）
   internal/theme（Theme 结构 → lipgloss 色）
```

### NOT in scope（评审确认后明确排除）

- **`poll_dooit_db` 外部改库热刷新**——单进程取舍（README 明示）
- **`migrate` 子命令**——全新 schema，旧 dooit 数据不导入
- **`api.css` / `plugin_manager`**（原版 Python 插件机制）——Lua 封闭子集不含
- **DATE 模式 UI**——保留常量、永不激活
- **鼠标支持**——纯键盘
- **10k 节点目标**——仅作上限参考，以用户实际数据量为准（若 <100 条不为此过度设计）

### 跨阶段主题（2+ 阶段独立出现的高置信信号）

1. **「别过度宣称 parity/性能」**：CEO #2/#5、Eng #6/#17、DX #12/#13 独立指出——Lua 表面、日期解析、循环输入、启动计时、poll_dooit_db 取舍都需要如实标注边界。已统一处理：所有 "完整对齐" 改为精确边界声明，启动计时改用 `go build` 后计时 + 原版基线。
2. **Lua 桥的正确性**：CEO #3、Eng #9、DX #6/#7 独立指出 Lua 桥（formatter 透传 api、fn 命名、bar 值持有、合法 Lua 语法、事件名字符串）。已统一处理并给出实施指引。
3. **先落库后改内存**：Eng #10 与 DX #8 一致指向同一回滚策略——已确定为单一方案。

### VERDICT

**APPROVED**（2026-08-02，用户批准）。两用户挑战（重写方向 #2、公开仓库 #6）按用户原始方向成立。53 项自动决策全部采纳并已并入计划/设计文档。进入实现。

`NO UNRESOLVED DECISIONS`
