# 发布流程（Releasing）

本文档说明如何发布新版本。目标是**全自动、可重复**——每次发版只需打一个 tag。

## 发布要素

| 要素 | 位置 | 说明 |
|---|---|---|
| LICENSE | `LICENSE` | MIT，scoop/winget 审核必须 |
| 版本 tag | `v0.1.0` 等 | 触发 CI 构建 + Release |
| CI 工作流 | `.github/workflows/release.yml` | 跨平台编译 → 打 zip → 上传 Release → 更新 scoop manifest |
| 个人 scoop bucket | `XiaTian-AC/scoop-faster-dooit` | `scoop install` 用，manifest 由 CI 自动更新 |
| Winget manifest | winget-pkgs | 通过 `wingetcreate submit` 提交 |

## 一键发布步骤

```powershell
# 1. 确认代码在 main 上且测试通过
git checkout main
go test ./... && go vet ./...

# 2. 打 tag（触发 CI）
git tag v0.1.0
git push origin v0.1.0

# 3. 等 CI 完成：Actions → release 工作流 → 生成 Release 页面
#    Release 里应出现：fdooit-windows-amd64.zip, fdooit-linux-amd64.tar.gz 等

# 4. 确认 scoop bucket 的 manifest 已被 CI 更新（自动提交新 hash）
```

## 常见问题

- **CI 没触发**：确认 tag 格式是 `v*`，且 `.github/workflows/release.yml` 的 `on.push.tags` 匹配
- **scoop manifest 没更新**：检查 bucket 仓库的 CI 是否使用了写权限的 token（`GH_TOKEN`）
- **winget 更新**：新版本发布后运行：
  ```powershell
  wingetcreate update --urls https://github.com/XiaTian-AC/faster-dooit/releases/download/v0.2.0/fdooit-windows-amd64.zip --version 0.2.0 --submit
  ```

## 产物命名约定

统一命名为 `fdooit`（命令名），避免 `faster-dooit` 过长：

- `fdooit-windows-amd64.zip`（含 `fdooit.exe`）
- `fdooit-linux-amd64.tar.gz`（含 `fdooit`）
- `fdooit-darwin-amd64.tar.gz` / `fdooit-darwin-arm64.tar.gz`
