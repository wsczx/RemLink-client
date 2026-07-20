#!/bin/bash
# Windows 交叉编译（含图标嵌入）
#
# 流程：
#   1. 生成资源 .syso（图标 + manifest + 版本信息，见 build_syso.sh）
#   2. GOOS=windows 交叉编译，链接器自动把 .syso 嵌进 exe 的 .rsrc 段
#
# 依赖：wails3 CLI（go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117）
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

ver=$(cat version 2>/dev/null || echo "0.0.0")
commitId=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
buildDate=$(date -Iseconds)

echo "=============================="
echo "  版本:   $ver"
echo "  Commit: ${commitId:0:8}"
echo "  日期:   $buildDate"
echo "=============================="

go mod tidy

# 1. 资源（图标 + manifest + 版本信息）
bash "$SCRIPT_DIR/build_syso.sh"

# 2. 交叉编译
echo ""
echo "交叉编译 Windows amd64 ..."
GOWORK=off GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -buildvcs=false -trimpath -ldflags "-H=windowsgui -w -s" -o remlink-client.exe .

echo ""
echo "=============================="
ls -lh remlink-client.exe
echo "---"
file remlink-client.exe
echo "=============================="

# 编译产物已生成，删除临时 .syso（仅链接期使用，无需留存）
rm -f wails_windows_amd64.syso
echo "已清理临时资源文件 wails_windows_amd64.syso"

echo "完成: $SCRIPT_DIR/../remlink-client.exe"
