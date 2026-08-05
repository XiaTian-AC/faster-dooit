# faster-dooit

> **Disclaimer: 本项目与 [dooit](https://github.com/dooit-org/dooit) 官方项目无关。**
> faster-dooit 是一个**从零重写**、独立的 vim 风格终端待办管理器，参考了 dooit 的交互设计，但代码、架构与数据格式均不兼容、无关联。
> 本项目由 **AI 协作创作**，作者是一名**初中生业余开发者**。功能可能不完善，欢迎提交 Issue / PR，但请勿期待企业级质量。

A from-scratch, high-performance vim-style TUI todo manager in **Go + Bubble Tea** — inspired by [dooit](https://github.com/dooit-org/dooit)'s two-pane tree, six modes, vim keybindings, natural-language dates, recurrence, search/sort/clipboard, status bar and Lua extension surface — without the Python startup and per-frame cost.

## 安装

### Scoop（推荐）

```powershell
# 一次性添加我的个人 bucket（临时方案）
scoop bucket add faster-dooit https://github.com/XiaTian-AC/scoop-faster-dooit
scoop install faster-dooit
```

安装后命令为 `fdooit`（不是 `faster-dooit`）。

### Winget

```powershell
winget install faster-dooit
```

### 手动构建

Requirements: Go ≥ 1.22 (no CGO, no gcc).

```powershell
git clone https://github.com/XiaTian-AC/faster-dooit.git
cd faster-dooit
go build -o fdooit.exe .
.\fdooit.exe
```

## 快速开始（Tutorial）

1. 启动后，左侧是 **Workspaces**（工作区），右侧是 **Todos**（待办）
2. 按 `a` 在光标处添加一个待办，输入名称后回车
3. 按 `c` 切换完成状态，按 `d` 设置截止日期（支持 `tomorrow`、`3d`、`next monday` 等）
4. 按 `?` 查看帮助，按 `/` 强制刷新终端画面

## 功能一览

### 为什么重写

原版 dooit 是 Python/Textual/SQLAlchemy。它的慢来自每次刷新重建整棵树、N+1 查询、1 秒 DB-mtime 轮询、以及响应式框架开销。

faster-dooit 移除这些根源：

- **无 DB 轮询**：启动时一次性读库到内存（`LoadAll`），编辑立即写回 SQLite，只重渲染受影响行
- **视口渲染**：`Model` 用按窄脏集失效的行缓存；1 秒时钟 tick 与缓存解耦，不会每帧抖缓存
- **直接 SQLite**：`modernc.org/sqlite`（纯 Go 无 CGO），单连接 `SetMaxOpenConns(1)` + `_pragma=foreign_keys(1)`

### 基准

Machine: AMD Ryzen 7 H 260 / Windows 11. `go test ./internal/app/ -bench . -benchmem`.

| Benchmark | Result | Target | Status |
|---|---|---|---|
| `BenchmarkStartup10k` (cold load 10k todos) | ~34 ms | < 200 ms | ✅ |
| `BenchmarkVisibleTodos10k` (flatten 10k visible) | ~204 µs | < 10 ms | ✅ |
| `BenchmarkUpdate` (keypress dispatch) | ~157 µs | < 1 ms | ✅ |
| `BenchmarkRenderRow10k` (render one row) | ~233 µs | — | ✅ |

原版基线（本机）：`import dooit.ui.tui`（Textual + SQLAlchemy，渲染前）≈ **1.9 s**。faster-dooit 整个冷启动数据路径 ~34 ms。

## 数据与配置（Reference）

数据库在首次运行时创建于：

- **Windows**: `%APPDATA%\faster-dooit\todo.db`
- **Linux/macOS**: `$XDG_CONFIG_HOME/faster-dooit/todo.db`（回退 `~/.config`）

配置文件 `config.lua` 位于同一目录。可用 `--db <path>` / `-c, --config <path>` 覆盖。

### 键位（默认）

| Key | Action | Key | Action |
|---|---|---|---|
| `j` / `k` | move down / up | `i` | edit description |
| `d` / `r` / `e` | edit due / recurrence / effort | `a` / `A` | add sibling / child |
| `z` / `Z` | expand / expand parent | `gg` / `G` | go to top / bottom |
| `J` / `K` | shift down / up | `xx` | delete (confirm) |
| `y` / `Y` | copy description / model | `p` / `P` | paste below / above |
| `c` | toggle complete | `=` / `+` / `-` / `_` | increase / decrease urgency |
| `/` | redraw（强制刷新） | `S` / `?` | search / help |
| `tab` / `h` / `l` | switch pane | `enter` | edit description |
| `ctrl+s` | sort | `ctrl+q` / `ctrl+c` | quit |

所有键位都可在 `config.lua` 通过 `api.keys.set(key, api.<action>)` 重映射，例如：

```lua
api.keys.set("i", api.add_sibling)     -- 让 i 变成添加
api.keys.set("x", api.toggle_complete) -- 单击 x 完成
```

### 搜索模式（How-to）

- `/` 进入搜索，输入关键词后 **Enter** 应用过滤并把光标移到结果列表（状态栏显示 `search: <关键词>`）
- 在结果上可正常操作（`j`/`k` 移动、`c` 完成、`A` 加子项）
- **Esc** 清空过滤恢复完整列表
- 搜索中按 `a`/`A` 会应用过滤并直接开始添加（新项不会被过滤隐藏）

### Lua 配置

配置面是原版 Python API 的刻意精简子集：**keys / layouts / formatter / bar / dashboard / subscribe / timer / `vars.theme` / notify**。`api.css` 与插件管理器刻意不提供。找不到 `config.lua` 时用内置默认；无效配置打印 `file:line` 错误并退出。

**主题示例：**

```lua
api.vars.theme.primary   = "#FFB86C"
api.vars.theme.secondary = "#6272A4"
api.vars.theme.background = "#282A36"
```

**格式化器示例：**

```lua
api.formatter.todos.description.add(function(description, model, theme)
  if description and #description > 20 then
    return { text = description:sub(1, 19) .. "…", style = theme.yellow }
  end
  return { text = description, style = "" }
end)
```

## 设计说明（Explanation）

### 完成级联

- R1：完成一个 todo → 整个子树完成
- R2：所有子项完成 → 父项自动完成
- R3：重开任意子项 → 父项自动重开
- R4：循环 todo 永不完成——完成时 `due` 自动推进一个周期并保持 pending

### 排序语义

- `pending` 复合键 `pending→due→order_index` 只用于 `pending` 选项
- 其他字段升序，NULL `due` 排在最后；`reverse` 反转

### 与 dooit 的差异

- 旧 dooit 数据**不迁移**（全新 schema）
- 编辑错误时留在输入框（原版会退出并保留旧值）
- **不实现** `poll_dooit_db` 外部编辑热加载（单进程权衡，刻意不做）

## 从 dooit 迁移

数据不迁移，建议从空库开始。Python API → Lua 的对应关系见上表（多数同名）。

## 开发（Contribution）

```powershell
go test ./...        # unit + parity + e2e
go vet ./...
go test ./internal/app/ -bench . -benchmem   # 性能门禁
```

项目文档：设计规格与实现计划见 [`docs/superpowers/`](docs/superpowers/)。

## 致谢

- [dooit](https://github.com/dooit-org/dooit) —— 交互设计的灵感来源
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) / [lipgloss](https://github.com/charmbracelet/lipgloss) —— Go TUI 渲染栈
- 本项目代码由 AI 协作生成，欢迎批评指正
