#!/usr/bin/env bash
#
# faster-dooit 一键安装脚本（macOS / Linux）
#
# 策略（按优先级）：
#   1. Homebrew tap（推荐）：brew tap XiaTian-AC/XiaTian-AC-bucket && brew install faster-dooit
#   2. 无 brew：直接下载 GitHub Release 的 tar.gz 到 ~/.local/bin/fdooit
#
# 用法：
#   bash <(curl -fsSL https://raw.githubusercontent.com/XiaTian-AC/faster-dooit/main/install.sh)
#   或下载后本地执行： ./install.sh
#
set -euo pipefail

REPO="XiaTian-AC/faster-dooit"
TAP="XiaTian-AC/XiaTian-AC-bucket"
BIN_NAME="fdooit"

log()  { printf '\033[1;32m[install]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[install]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[install] %s\033[0m\n' "$*" >&2; exit 1; }

detect_os_arch() {
  case "$(uname -s)" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux" ;;
    *) die "不支持的平台: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) ARCH="arm64" ;;
    x86_64|amd64)  ARCH="amd64" ;;
    *) die "不支持的架构: $(uname -m)" ;;
  esac
}

install_via_brew() {
  log "检测到 Homebrew，通过 tap 安装…"
  if ! brew tap "$TAP" 2>/dev/null; then
    warn "brew tap $TAP 失败，尝试完整 URL"
    brew tap "$TAP" "https://github.com/XiaTian-AC/homebrew-XiaTian-AC-bucket"
  fi
  brew install faster-dooit
  log "安装完成：$(command -v $BIN_NAME)"
}

download_binary() {
  local os="$1" arch="$2"
  log "直接下载 $os/$arch 二进制…"
  local tag
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4) \
    || die "无法获取最新版本"
  # 归档名用 ProjectName（faster-dooit），二进制名是 fdooit。
  local archive="faster-dooit-$os-$arch.tar.gz"
  local url="https://github.com/$REPO/releases/download/$tag/$archive"
  local dest_dir="${XDG_BIN_HOME:-$HOME/.local/bin}"
  mkdir -p "$dest_dir"
  local tmp
  tmp=$(mktemp -d)
  curl -fsSL "$url" -o "$tmp/$archive"
  tar -xzf "$tmp/$archive" -C "$tmp"
  cp "$tmp/$BIN_NAME" "$dest_dir/$BIN_NAME"
  chmod +x "$dest_dir/$BIN_NAME"
  rm -rf "$tmp"
  log "已安装到 $dest_dir/$BIN_NAME"
  if [[ ":$PATH:" != *":$dest_dir:"* ]]; then
    warn "请将 $dest_dir 加入 PATH（例如 export PATH=\"$dest_dir:\$PATH\"）"
  fi
}

main() {
  detect_os_arch
  if command -v brew >/dev/null 2>&1; then
    install_via_brew
  else
    warn "未检测到 Homebrew，改用直接下载"
    download_binary "$OS" "$ARCH"
  fi
  log "验证：$BIN_NAME --version"
  "$(command -v $BIN_NAME || echo "$HOME/.local/bin/$BIN_NAME")" --version || warn "运行失败，请检查 PATH"
}

main "$@"
