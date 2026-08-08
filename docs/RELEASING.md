# 发布流程（Releasing）

本文档说明如何发布新版本。目标是**全自动、可重复**——每次发版只需打一个 tag。

## 发布要素

| 要素 | 位置 | 说明 |
|---|---|---|
| LICENSE | `LICENSE` | MIT |
| 版本 tag | `vX.Y.Z` | 触发 CI 构建 + Release |
| CI 工作流 | `.github/workflows/release.yml` | 跨平台编译 → 打 zip/tar.gz → 上传 Release → 更新 scoop bucket + Homebrew tap |
| 统一 scoop bucket | `XiaTian-AC-bucket` | 所有项目共用的个人 bucket，`scoop install` 用，manifest 由各项目 CI 自动更新 |
| Homebrew tap | `homebrew-XiaTian-AC-bucket` | `brew install faster-dooit` 用，formula 由各项目 CI 自动更新 |
| 一键安装脚本 | `install.sh` / `install.ps1` | 仓库根目录，分别面向 macOS/Linux 与 Windows |

官方 Scoop Extras 与 Winget **不在 CI 流程内**（需人工审核，且社区门槛高），以个人渠道为主。

## 一键发布步骤

```powershell
# 1. 确认代码在 main 上且测试通过
git checkout main
go test ./... && go vet ./...

# 2. 打 tag（触发 CI）
git tag v0.3.1
git push origin v0.3.1

# 3. 等 CI 完成：Actions → release 工作流
#    - release job: 生成 Release（跨平台 zip/tar.gz + checksums）
#    - update-scoop-bucket: 更新个人 bucket 的 manifest（真实 hash）
#    - update-homebrew-tap: 更新 Homebrew tap 的 formula（4 平台 hash）

# 4. 验证渠道
scoop update faster-dooit   # Windows
brew update && brew upgrade faster-dooit   # macOS/Linux
```

## 必备 Secret

`SCOOP_BUCKET_TOKEN`（仓库 secret）——用于 CI 更新统一 bucket 与 Homebrew tap。
这是一个有 **两个仓库** 写权限的 **Fine-grained PAT**：

1. GitHub → Settings → Developer settings → Fine-grained tokens → Generate
2. 勾选 `XiaTian-AC-bucket` 与 `homebrew-XiaTian-AC-bucket`，权限：Contents → Read and write
3. 设为仓库 secret：
   ```powershell
   gh secret set SCOOP_BUCKET_TOKEN --repo XiaTian-AC/faster-dooit
   ```

> ⚠️ 不要用 `gh auth token`（会过期）。用专门的 PAT 才能长期自动更新。

## 发布渠道

| 渠道 | 状态 | 说明 |
|---|---|---|
| GitHub Release | ✅ | 跨平台 zip/tar.gz + checksums |
| 统一 scoop bucket | ✅ | `scoop bucket add faster-dooit https://github.com/XiaTian-AC/XiaTian-AC-bucket`，CI 自动更新 |
| Homebrew tap | ✅ | `brew tap XiaTian-AC/XiaTian-AC-bucket && brew install faster-dooit`，CI 自动更新 |

## 主路径（发布后立即可用）

每次发版：

```powershell
git tag v0.3.1 && git push origin v0.3.1
```

CI 自动：跨平台 Release + 更新 scoop bucket manifest + 更新 Homebrew formula。
用户通过 `scoop update faster-dooit` 或 `brew upgrade faster-dooit` 升级，或重跑一键安装脚本。

## 手动触发（不重建 Release）

`workflow_dispatch` 输入版本号（如 `0.1.0`）会跳过 goreleaser，只重跑 bucket/tap 的 manifest 更新——用于修复 hash 或重发同版本。

## 主题机制

`api.vars.theme.name` 选择内置预设主题（nord / catppuccin_mocha / catppuccin_latte / dracula / gruvbox_dark / solarized_light / tokyo_night），单色覆盖（含 dim / selection / border_focused / border_unfocused / urgency_colors）叠加在预设之上。详见 README。

## 常见问题

- **CI 没触发**：确认 tag 格式是 `v*`，且 `on.push.tags` 匹配
- **bucket/tap manifest hash 为 0**：检查对应 job 日志——`TAG` 必须正确（dispatch 时要传 `version` 输入）
- **Homebrew formula 报 sha256 不匹配**：确认 `.brew/faster-dooit.rb.tmpl` 的 4 个占位符都被替换（检查 `sha()` 提取结果）
- **PAT 403**：确认 Fine-grained PAT 同时授权了 `XiaTian-AC-bucket` 和 `homebrew-XiaTian-AC-bucket`

## 产物命名约定

- **二进制名**：`fdooit`（命令名）
- **归档名**（GoReleaser 用 ProjectName）：`faster-dooit-{os}-{arch}.{ext}`
  - Windows: `faster-dooit-windows-amd64.zip`
  - Linux/macOS: `faster-dooit-linux-{amd64,arm64}.tar.gz`、`faster-dooit-darwin-{amd64,arm64}.tar.gz`
- **scoop manifest**：`.scoop/faster-dooit.json.tmpl` → `XiaTian-AC-bucket/faster-dooit.json`
- **Homebrew formula**：`.brew/faster-dooit.rb.tmpl` → `homebrew-XiaTian-AC-bucket/Formula/faster-dooit.rb`
