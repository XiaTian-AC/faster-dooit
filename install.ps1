# faster-dooit 一键安装脚本（Windows / PowerShell 7+）
#
# 策略（按优先级）：
#   1. Scoop（推荐）：scoop bucket add faster-dooit https://github.com/XiaTian-AC/XiaTian-AC-bucket
#      && scoop install faster-dooit
#   2. 无 scoop：直接下载 GitHub Release 的 zip 到 %LOCALAPPDATA%\Programs\fdooit
#
# 用法：
#   iwr -useb https://raw.githubusercontent.com/XiaTian-AC/faster-dooit/main/install.ps1 | iex
#   或下载后本地执行： .\install.ps1
#
param(
  [switch]$SkipScoop
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Repo = 'XiaTian-AC/faster-dooit'
$Bucket = 'https://github.com/XiaTian-AC/XiaTian-AC-bucket'
$BinName = 'fdooit'

function Write-Log  { Write-Host "[install] $args" -ForegroundColor Green }
function Write-Warn { Write-Host "[install] $args" -ForegroundColor Yellow }

function Get-LatestVersion {
  try {
    $r = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ 'User-Agent' = 'fdooit-install' }
    return $r.tag_name
  } catch {
    throw "无法获取最新版本: $_"
  }
}

function Install-ViaScoop {
  Write-Log "检测到 Scoop，通过个人 bucket 安装…"
  if (-not (scoop bucket list | Select-String -Quiet '^faster-dooit\b')) {
    scoop bucket add faster-dooit $Bucket
  }
  scoop install faster-dooit
  Write-Log "安装完成：$(Get-Command fdooit -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source)"
}

function Install-Zip {
  Write-Log "直接下载 Windows zip…"
  $version = Get-LatestVersion
  # 归档名用 ProjectName（faster-dooit），二进制名是 fdooit。
  $url = "https://github.com/$Repo/releases/download/$version/faster-dooit-windows-amd64.zip"
  $dest = Join-Path $env:LOCALAPPDATA 'Programs\fdooit'
  $tmp = Join-Path $env:TEMP "fdooit-$([guid]::NewGuid()).zip"
  New-Item -ItemType Directory -Force -Path $dest | Out-Null
  try {
    Invoke-WebRequest -Uri $url -OutFile $tmp
    Expand-Archive -Path $tmp -DestinationPath $dest -Force
  } finally {
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
  }
  $exe = Join-Path $dest "$BinName.exe"
  if (-not (Test-Path $exe)) { throw "解压后未找到 $exe" }
  Write-Log "已安装到 $exe"
  Write-Warn "请将 $dest 加入 PATH：\$env:Path = '$dest;' + `$env:Path"
}

$hasScoop = (Get-Command scoop -ErrorAction SilentlyContinue) -ne $null
if (-not $SkipScoop -and $hasScoop) {
  Install-ViaScoop
} else {
  if ($SkipScoop) { Write-Warn "跳过 Scoop（-SkipScoop），直接下载 zip" }
  else { Write-Warn "未检测到 Scoop，改用直接下载 zip（安装 Scoop: https://scoop.sh）" }
  Install-Zip
}

Write-Log "验证：$BinName --version"
& $BinName --version
