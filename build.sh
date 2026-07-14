#!/usr/bin/env bash
# 构建脚本（Linux/macOS）。
# 用法： ./build.sh
# 产物输出到 dist-bin/ 目录，包含面板与 Agent 的多平台二进制。

set -euo pipefail

echo "==> 构建前端"
(cd web && npm install && npm run build)

OUT=dist-bin
mkdir -p "$OUT"

# 纯 Go 的 SQLite 驱动，无需 CGO，可直接交叉编译。
export CGO_ENABLED=0

build() {
  local os=$1 arch=$2 ext=$3 tag="$1-$2"
  echo "==> 构建面板 $tag"
  GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$OUT/probe-server-$tag$ext" ./cmd/server
  echo "==> 构建 Agent $tag"
  GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$OUT/probe-agent-$tag$ext" ./cmd/agent
}

build linux   amd64 ""
build linux   arm64 ""
build windows amd64 ".exe"
build darwin  arm64 ""

echo "==> 完成，产物在 $OUT/"
ls -lh "$OUT"
