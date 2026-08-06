# faster-dooit

[English](README.md) | **简体中文**

> _与官方 [dooit](https://github.com/dooit-org/dooit) 项目无关——这是一个独立的从零重写移植。_

你的待办工具要等两秒才打开，每次按键都卡一下。**faster-dooit** 是一个 vim 风格终端待办管理器，冷启动加载 **1 万个待办只需 ~34 ms**，而且从不轮询数据库——哪怕你把整个人生都放进去，它也快得感觉不到。

```
scoop bucket add faster-dooit https://github.com/XiaTian-AC/XiaTian-AC-bucket
scoop install faster-dooit
fdooit
```

```
┌─ Workspaces ─────────────────────┐  ┌─ Todos ──────────────────────────────┐
│  Work                            │  │  o  写发布说明                @today  │
│  Personal                        │  │  o  写设计规格                        │
└──────────────────────────────────┘  └──────────────────────────────────────┘
```

`j`/`k` 移动，`a` 添加，`d` 设置截止日期（`d 3d` 或 `d next monday`），`c` 完成，`?` 查看帮助。这是 vim，用在你的待办上。

## 为什么不用你现在用的那个？

| 换成它 | 为什么用 faster-dooit |
|---|---|
| **dooit**（Python） | 冷启动 1.9 s vs **34 ms**；它每帧重建整棵树、N+1 查询、每秒轮询数据库 |
| **网页应用 / 手机清单** | 没有浏览器、没有同步、没有云——任务就在你拥有的本地 SQLite 文件里 |
| **taskwarrior / 纯文本** | 双栏树 + 层级嵌套 + 完成级联 + 状态栏——全程不用离开键盘 |

## 功能

| 痛点 | faster-dooit |
|---|---|
| 打开待办工具要等几秒 | 1 万个待办载入 **~34 ms**（原版 dooit 本机约 1.9 s） |
| 每次按键都整树重渲染 | 视口渲染——按窄脏集失效的行缓存，只重绘受影响行 |
| 后台每秒轮询数据库 | 启动时一次性读库到内存；编辑立即写回 SQLite |
| 要装 Python 和一串依赖 | 纯 Go、无 CGO、单个静态二进制 |
| 配置不听话 | Lua 配置 + 7 套内置主题（`nord`、`dracula`、`catppuccin_mocha`…）支持单色覆盖 |
| 打错一个字就被踢出编辑 | 校验错误会留在输入框直到你改对 |
| 空应用里没法添加任务 | 在空状态下按 `a` 会顺便创建第一个工作区和第一个任务 |

## 安装

### Scoop（推荐）

```powershell
# 一次性添加我的个人 bucket
scoop bucket add faster-dooit https://github.com/XiaTian-AC/XiaTian-AC-bucket
scoop install faster-dooit
```

安装后命令为 `fdooit`（不是 `faster-dooit`）。个人 bucket 的 manifest 由发布 CI 自动更新——打一个 tag，bucket 就自动同步。

### Winget

```powershell
winget install faster-dooit
```

> 状态：已提交官方 winget-pkgs PR（微软审核中）。在此之前可用 Scoop bucket 或源码构建。

### 手动构建

Requires Go ≥ 1.22 (no CGO, no gcc).

```powershell
git clone https://github.com/XiaTian-AC/faster-dooit.git
cd faster-dooit
go build -o fdooit.exe .
.\fdooit.exe
```

## 快速开始

1. 启动后，左侧是 **Workspaces**（工作区），右侧是 **Todos**（待办）
2. 按 `a` 在光标处添加一个待办，输入名称后回车
3. 按 `c` 切换完成，按 `d` 设置截止日期（`tomorrow`、`3d`、`next monday`）
4. 按 `?` 查看帮助，按 `/` 强制整屏刷新

自然语言日期、循环、搜索、排序、剪贴板复制粘贴、Lua 扩展面全部默认内置——无需装任何插件。

## 参考

### 数据与配置

- 数据库（首次运行创建）：`%APPDATA%\faster-dooit\todo.db`（Windows）/ `$XDG_CONFIG_HOME/faster-dooit/todo.db`（Linux/macOS）
- 配置文件：与数据库同目录的 `config.lua`。可用 `--db <path>` / `-c, --config <path>` 覆盖
- 无效配置打印 `file:line` 错误并退出

### 键位（默认）

| Key | Action | Key | Action |
|---|---|---|---|
| `j` / `k` | move down / up | `i` | edit description |
| `d` / `r` / `e` | edit due / recurrence / effort | `a` / `A` | add sibling / child |
| `z` / `Z` | expand / expand parent | `gg` / `G` | go to top / bottom |
| `J` / `K` | shift down / up | `xx` | delete (confirm) |
| `y` / `Y` | copy description / model | `p` / `P` | paste below / above |
| `c` | toggle complete | `=` / `+` / `-` / `_` | increase / decrease urgency |
| `/` | force redraw | `S` / `?` | search / help |
| `tab` / `h` / `l` | switch pane | `enter` | edit description |
| `ctrl+s` | sort | `ctrl+q` / `ctrl+c` | quit |

所有键位都可在 `config.lua` 里重映射：

```lua
api.keys.set("i", api.add_sibling)     -- 让 i 变成添加
api.keys.set("x", api.toggle_complete) -- 单击 x 完成
```

### 搜索模式

`/` 边输入边过滤；Enter 把光标移到第一个匹配项。结果上可正常操作（`c` 完成、`A` 加子项）。Esc 清除过滤——光标按 ID 停留在同一个条目上。

### 主题

选一套内置主题，再在它之上覆盖任意颜色：

```lua
api.vars.theme.name = "dracula"
api.vars.theme.primary = "#FF79C6"      -- 可选：单色覆盖
-- api.vars.theme.background = "transparent"  -- 保留终端自身背景
```

内置主题：`nord`、`catppuccin_mocha`、`catppuccin_latte`、`dracula`、`gruvbox_dark`、`solarized_light`、`tokyo_night`。可覆盖：8 个基础色加 `dim`、`selection`、`border_focused`、`border_unfocused`、`urgency_colors`。未知主题名会报错退出。

### 基准

Machine: AMD Ryzen 7 H 260 / Windows 11. `go test ./internal/app/ -bench . -benchmem`.

| Benchmark | Result | Target | Status |
|---|---|---|---|
| `BenchmarkStartup10k` (cold load 10k todos) | ~34 ms | < 200 ms | ✅ |
| `BenchmarkVisibleTodos10k` (flatten 10k visible) | ~204 µs | < 10 ms | ✅ |
| `BenchmarkUpdate` (keypress dispatch) | ~157 µs | < 1 ms | ✅ |
| `BenchmarkRenderRow10k` (render one row) | ~233 µs | — | ✅ |

原版基线（本机）：`import dooit.ui.tui` 渲染前 ≈ **1.9 s**。faster-dooit 整个冷启动路径 ~34 ms。

## 设计说明

- **完成级联** — 完成一个 todo 会完成整棵子树；父项仅在所有子项都完成时自动完成；重开任意子项会重开父项；循环 todo 永不完成（due 自动推进一个周期）
- **与 dooit 的差异** — 旧数据**不迁移**（全新 schema）；编辑错误时留在输入框；刻意不实现 `poll_dooit_db` 外部编辑热加载

## 开发（Contribution）

```powershell
go test ./...        # unit + parity + e2e
go vet ./...
go test ./internal/app/ -bench . -benchmem   # 性能门禁
```

设计规格与实现计划见 [`docs/superpowers/`](docs/superpowers/)。

## 致谢

- [dooit](https://github.com/dooit-org/dooit) —— 交互设计的灵感来源
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) / [lipgloss](https://github.com/charmbracelet/lipgloss) —— Go TUI 渲染栈
- 由 AI 协作构建，欢迎反馈
