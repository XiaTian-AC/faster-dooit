# 发布流程（Releasing）

本文档说明如何发布新版本。目标是**全自动、可重复**——每次发版只需打一个 tag。

## 发布要素

| 要素 | 位置 | 说明 |
|---|---|---|
| LICENSE | `LICENSE` | MIT，scoop/winget 审核必须 |
| 版本 tag | `v0.1.0` 等 | 触发 CI 构建 + Release |
| CI 工作流 | `.github/workflows/release.yml` | 跨平台编译 → 打 zip → 上传 Release → 更新 scoop manifest |
| 统一 scoop bucket | `XiaTian-AC-bucket` | 所有项目共用的个人 bucket，`scoop install` 用，manifest 由各项目 CI 自动更新 |
| Winget manifest | `.winget/manifests/` | 已归档；用 `wingetcreate submit` 提交 |
| Scoop 官方 | ScoopInstaller/Extras | 已提交 PR |

## 一键发布步骤

```powershell
# 1. 确认代码在 main 上且测试通过
git checkout main
go test ./... && go vet ./...

# 2. 打 tag（触发 CI）
git tag v0.2.0
git push origin v0.2.0

# 3. 等 CI 完成：Actions → release 工作流
#    - release job: 生成 Release（跨平台 zip/tar.gz + checksums）
#    - update-scoop-bucket job: 用 API 更新个人 bucket 的 manifest（真实 hash）

# 4. 验证 scoop 可装
scoop update faster-dooit
scoop info faster-dooit

# 5. Winget 提交新版本（手动，因需要审核）
wingetcreate update --urls "https://github.com/XiaTian-AC/faster-dooit/releases/download/v0.2.0/faster-dooit-windows-amd64.zip" --version 0.2.0 --submit
```

## 必备 Secret

`SCOOP_BUCKET_TOKEN`（仓库 secret）——用于 CI 更新统一 bucket 的 manifest。
这是一个有 `XiaTian-AC-bucket` 仓库写权限的 **Fine-grained PAT**：

1. GitHub → Settings → Developer settings → Fine-grained tokens → Generate
2. 勾选 `XiaTian-AC-bucket`，权限：Contents → Read and write
3. 设为仓库 secret：
   ```powershell
   gh secret set SCOOP_BUCKET_TOKEN --repo XiaTian-AC/faster-dooit
   ```

> ⚠️ 不要用 `gh auth token`（会过期）。用专门的 PAT 才能长期自动更新。

## 发布状态（v0.1.0）

| 渠道 | 状态 | 说明 |
|---|---|---|
| GitHub Release v0.1.0 | ✅ | 跨平台 zip/tar.gz + checksums |
| 个人 bucket | ✅ | `scoop bucket add faster-dooit https://github.com/XiaTian-AC-bucket`，CI 自动更新 manifest |
| Scoop 官方 Extras | ⏳ 待定 | PR #18463；技术检查已过，但卡 **100 star/50 fork** 社区门槛，可能被关或特批 |
| Winget | ⏳ 审核中 | PR #412633；wingetbot 流水线验证中，微软 reviewer 审核 |

**重要认知**：Scoop Extras 对 GitHub 托管的包要求 ≥100 star 或 ≥50 fork（`not-meet-criteria` 标签）。新项目通常达不到，**个人 bucket 是主要分发路径**，官方渠道是"star 上来后的加分项"。

## 主路径（发布后立即可用）

每次发版：

```powershell
git tag v0.2.0 && git push origin v0.2.0
```

CI 自动：跨平台 Release + 更新个人 bucket manifest。用户 `scoop update faster-dooit` 即可升级。

官方渠道（winget/scoop Extras）在新版本发布后需**手动**更新 PR——winget 用 `wingetcreate update`，scoop Extras 更新 `bucket/faster-dooit.json`（`.scoop/` 模板渲染后）。

## 手动触发（不重建 Release）

`workflow_dispatch` 输入版本号（如 `0.1.0`）会跳过 goreleaser，只重跑 bucket manifest 更新——用于修复 hash 或重发同版本。

## 主题机制

`api.vars.theme.name` 选择内置预设主题（nord / catppuccin_mocha / catppuccin_latte / dracula / gruvbox_dark / solarized_light / tokyo_night），单色覆盖（含 dim / selection / border_focused / border_unfocused / urgency_colors）叠加在预设之上。详见 README。

## 常见问题

- **CI 没触发**：确认 tag 格式是 `v*`，且 `on.push.tags` 匹配
- **bucket manifest hash 为 0**：检查 `Publish manifest to scoop bucket` 步骤日志——`TAG` 必须正确（dispatch 时要传 `version` 输入）
- **winget 审核失败**：确认 Release 的 zip 里 exe 名是 `faster-dooit.exe`（NestedInstallerFiles.RelativeFilePath 匹配）
- **scoop 官方审核**：遵循 ScoopInstaller/Extras 的 PR 模板，manifest 需含 `checkver` + `autoupdate`

## 产物命名约定

统一命名为 `fdooit`（命令名）：

- `faster-dooit-windows-amd64.zip`（含 `faster-dooit.exe`，scoop/winget 的 bin 映射到 `fdooit`）
- `faster-dooit-linux-amd64.tar.gz` / `faster-dooit-darwin-*.tar.gz`
