# 构建脚本（Windows PowerShell）。
# 用法： .\build.ps1
# 产物输出到 dist-bin/ 目录，包含面板与 Agent 的多平台二进制。

$ErrorActionPreference = "Stop"

Write-Host "==> 构建前端"
Push-Location web
npm install
npm run build
Pop-Location

$out = "dist-bin"
New-Item -ItemType Directory -Force -Path $out | Out-Null

# 纯 Go 的 SQLite 驱动，无需 CGO，可直接交叉编译。
$env:CGO_ENABLED = "0"

# 目标平台列表： GOOS/GOARCH/后缀
$targets = @(
    @{os="linux";   arch="amd64"; ext=""},
    @{os="linux";   arch="arm64"; ext=""},
    @{os="windows"; arch="amd64"; ext=".exe"},
    @{os="darwin";  arch="arm64"; ext=""}
)

foreach ($t in $targets) {
    $env:GOOS = $t.os
    $env:GOARCH = $t.arch
    $tag = "$($t.os)-$($t.arch)"

    Write-Host "==> 构建面板 $tag"
    go build -trimpath -ldflags "-s -w" -o "$out/probe-server-$tag$($t.ext)" ./cmd/server

    Write-Host "==> 构建 Agent $tag"
    go build -trimpath -ldflags "-s -w" -o "$out/probe-agent-$tag$($t.ext)" ./cmd/agent
}

Write-Host "==> 完成，产物在 $out/"
Get-ChildItem $out
