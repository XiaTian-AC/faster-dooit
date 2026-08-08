# AGENTS.md — faster-dooit 项目约定

## 版本号

- 主版本号由 git tag（`vX.Y.Z`）决定，发布时通过 GoReleaser ldflags 注入 `main.version`。
- **在合适的时候询问用户是否要前进版本号**：当 main 上积累了值得发布的改动、或用户要求"发版"、或功能集有明显增长时，先问用户是否 bump 版本，再打 tag。
- 不要未经询问就主动打新 tag 或发布 Release。
- 本地 `go build` 时 `main.version` 回落到手写默认值。

## 提交与推送

- **不需要每个改动都 push**。多个相关改动可以合并为一个 push 批次，避免冗余的远程往返。
- push 时机：用户明确要求、或一批改动自然收尾（如一个完整功能/修复完成）时。
- 提交保持小而聚焦，推送可以攒批。

## 发布流程

详见 `docs/RELEASING.md`。核心：`git tag vX.Y.Z && git push origin vX.Y.Z` 触发 CI 自动构建 Release + 更新统一 scoop bucket（`XiaTian-AC/XiaTian-AC-bucket`）+ Homebrew tap（`XiaTian-AC/homebrew-XiaTian-AC-bucket`）。官方 Scoop Extras / Winget 不在 CI 内。

## 默认路径

- 数据库：`%APPDATA%\faster-dooit\todo.db`（Windows）/ `$XDG_CONFIG_HOME/faster-dooit/todo.db`（Linux/macOS）
- 配置：`~/.config/faster-dooit/config.lua`（全平台统一）
