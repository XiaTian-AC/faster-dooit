# Better-Dooit 设计文档

> 状态：已获用户批准（2026-08-02）。基于原版 [dooit](https://github.com/dooit-org/dooit) v3.3.4 的全面重写。

## 背景与动机

原版 dooit 是 Python + Textual + SQLAlchemy 的 TUI 任务管理器。用户反馈两个痛点：

1. **启动慢**：从敲命令到界面出现有可见等待
2. **整体感知慢**：按键响应、交互感觉笨重、占用高

代码层面的根源（已核对源码）：

- **Textual 框架开销**：响应式 reactive + CSS + widget 树，每次按键走完整事件管线
- **全量重建**：`model_tree.py` 的 `_force_refresh()` 每次 `clear_options()` 后全量重建选项，`refresh_options()` 对每个 option 逐个 `replace_option_prompt` —— O(n)
- **N+1 查询**：`nest_level` 沿父链 lazy-load 遍历、`siblings` 每次取列表、`Todo.from_id()` 逐个查库
- **1 秒 DB-mtime 轮询器**：`tui.py` 的 `poll_dooit_db()` 每秒检测 SQLite 文件 mtime，变化即刷新所有树
- **每次操作即 commit**：所有写操作立即 `manager.commit()`

## 决策记录（2026-08-02 用户确认）

| 决策点 | 结论 |
|---|---|
| 技术栈 | **Go + bubbletea + gopher-lua + modernc.org/sqlite**（无 CGO） |
| 性能目标 | 启动 < 200ms；万级节点无感知按键延迟 |
| 配置/扩展 | **Lua**（`config.lua` 镜像原版 `default_config.py` 的 `api` 表面） |
| 数据 | **全新 schema，不迁移旧库** |
| 功能范围 | **完整对齐原版 dooit** |
| Git 约束 | 所有 git 命令在 `better-dooit/` 目录内执行；完成后创建**公开**仓库 `better-dooit` 并 push |

### 为什么 Go 而非 Rust

- 性能上两者等价——dooit 慢的根源是 Python/Textual/SQLAlchemy，不是语言；两者都彻底解决
- 用户对两门语言都是新手、单人维护 → 学习曲线与维护成本成为决定性权重，Go 占优
- bubbletea + bubbles 现成组件（TextInput/Help）直接对应 dooit 的输入栏/帮助页，开发效率高
- Windows 下 modernc.org/sqlite 纯 Go，无需 C 编译器（rusqlite 需 MSVC）
- 代价：自然语言日期解析需自写 ~150 行小层；gopher-lua 比 LuaJIT 慢但仅在配置加载时运行，无感

## 架构

### 目录结构

```
better-dooit/
├── go.mod
├── main.go                    # 入口：加载配置 → 启动 bubbletea
├── config.lua                 # 默认配置（镜像 dooit default_config.py）
├── internal/
│   ├── model/                 # 领域模型 Workspace / Todo（纯结构体）
│   ├── store/                 # SQLite 持久化（modernc.org/sqlite）
│   ├── dateparse/             # 自然语言日期解析
│   ├── lua/                   # gopher-lua 绑定，暴露 `api` 表
│   ├── app/                   # bubbletea 应用（Elm 三件套）
│   │   ├── model.go  update.go  view.go
│   │   ├── tree.go            # 自定义树组件
│   │   ├── bars.go  input.go  help.go  dashboard.go
│   └── theme/                 # 颜色/主题
└── docs/superpowers/specs/    # 本设计文档
```

### Elm 架构映射

| bubbletea 概念 | 本应用 |
|---|---|
| Model | AppState：树内存模型、模式、高亮行、条栏状态、剪贴板、主题/布局/格式化器/按键表（Lua 填充） |
| Msg | KeyMsg、ModeChanged、TodoSelected、StartSearch/Sort/Confirm、BarNotification、TimerTick(1s 时钟)、LuaEventMsg |
| Update | 处理消息 → 改内存 → CRUD 落库 → 返回 Cmd |
| View | 纯函数：状态 → 两栏渲染（lipgloss），bubbletea 自动 diff 输出 |

### 数据模型

```go
type Workspace struct {
    ID         int64
    OrderIndex int
    Description string
    IsRoot     bool
    ParentID   *int64        // 自引用嵌套
    Children   []*Workspace
    Todos      []*Todo
}
type Todo struct {
    ID        int64
    Description string
    Due        *time.Time
    Effort     int
    Recurrence *time.Duration   // 秒存储
    Urgency    int
    Pending    bool
    OrderIndex int
    ParentWorkspaceID *int64
    ParentTodoID      *int64
}
```

存储：`workspace` / `todo` 两张表 + 自引用父键，纯 `database/sql`（无 ORM）。

### 性能方案

1. **启动 < 200ms**：编译产物毫秒级启动；启动时一次性把整库读进内存（单条 SELECT）；Lua 配置求值一次
2. **按键延迟**：`update()` O(1)/O(logn)；`view()` 只渲染视口内行；bubbletea diff 输出
3. **砍掉 1 秒 DB-mtime 轮询器**：单进程应用不需要；定时器只驱动时钟（1s 更新状态栏一格）
4. **消除 N+1**：全部操作走内存模型；`VisibleRows()` 用版本号 memo
5. **格式化文本缓存**：每行按版本缓存，仅失效重算

## UI 组成（完整对齐 dooit）

- **两栏布局**：左 `WorkspaceTree`，右 `TodosTree`，`<tab>` 切换焦点
- **六种模式**：NORMAL / INSERT / DATE / SEARCH / SORT / CONFIRM；非 NORMAL 时按键路由到 `bubbles/textinput` 输入覆盖层
- **条栏**：状态栏（可配 widget 数组：模式指示、时钟、用户名…）、通知栏（info/warning/error，自动退出）、确认栏（y/n）
- **帮助页**：`bubbles/help` 现成
- **仪表盘**：可配置文字/进度内容
- **树组件**（自定义）：可见行列表按展开状态 + 版本号 memo；列布局按配置渲染，每格走格式化器

## Lua 配置系统

启动时求值 `config.lua`，暴露全局 `api` 表。默认 `config.lua` 镜像 `default_config.py`：

```lua
api.keys.set("j", api.move_down)
api.keys.set(["=", "+"], api.increase_urgency)
api.layouts.todo_layout = {"status", "description", "due", "urgency"}
api.formatter.todos.status.add(todo_status_formatter)   -- 返回 {text, style}
api.bar.set({ get_mode, clock, get_user })
api.dashboard.set({"Welcome to Better Dooit!", ""})
subscribe(ModeChanged, function(api, ev) ... end)
timer(1, function(api) ... end)
```

对应表：

| dooit 原版 API | Lua 版 |
|---|---|
| `api.keys.set()` | 按键 → 动作字符串，动作注册表内置 |
| `api.layouts.*` | 列布局表 |
| `api.formatter.todos.<field>.add(fn)` | 注册 Lua 格式化函数 |
| `api.bar.set()` / `api.dashboard.set()` | 条栏/仪表盘 widget |
| `@subscribe(Event)` / `@timer(n)` | `subscribe(Event, fn)` / `timer(n, fn)` |
| `api.vars.theme` | 颜色表 |

Lua 格式化函数返回 `{text, style}`，桥接到 lipgloss；每行结果按版本缓存。

## 日期解析（internal/dateparse，纯 Go ~150 行）

- 绝对格式：`2026-08-02`、`2026-08-02 15:30`、`2026/08/02`
- 相对词：`today`、`tomorrow`、`next monday`、`in 3 days`、`2 weeks from now`
- 快捷：`3d`、`2w`、`1h`、`1w 2d`
- 失败 → 通知栏报错，留在输入态

## 循环任务（Recurrence）

- 秒存储；输入 `1d`/`2w`/`3h`/`1d 2h`，显示 "1d 2h" 风格
- 完成时：`due += recurrence`，`pending` 重置 true（对齐 dooit：设置循环即强制 pending）

## 错误处理

| 场景 | 策略 |
|---|---|
| 存储写失败 | 回滚事务 + `BarNotification("error")` |
| Lua 配置错误 | 启动时错误屏（文件+行号） |
| 日期解析失败 | 通知栏内联报错，停留输入态 |
| 运行时 panic | bubbletea 恢复 + stderr |

## 测试

- Go 标准测试：`dateparse`（表驱动，镜像原版 test_date_parse）、`store`（内存 SQLite）、`model` 操作（排序/移动/克隆/循环语义）、`lua` 桥接、键位解析
- `update()` 纯逻辑可无终端单测

## 里程碑

1. **M0 脚手架**：go.mod、store、model、dateparse + 测试
2. **M1 骨架 UI**：bubbletea 两栏树、NORMAL 导航、CRUD
3. **M2 模式与输入**：edit/search/sort/confirm/help
4. **M3 Lua 配置**：api 表面 + 默认 config.lua
5. **M4 条栏/仪表盘/主题/格式化**
6. **M5 对齐打磨**：与原版逐项对拍 + 性能验证（benchmark 启动时间、10k 节点延迟）
