#!/bin/bash
# 生成 Windows 资源 .syso（图标 + manifest + 版本信息）
#
# 供 make syso / make build 调用。仅生成资源，不编译二进制。
# 依赖：wails3 CLI（go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117）
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

if ! command -v wails3 &>/dev/null; then
    export PATH="$(go env GOPATH)/bin:$PATH"
fi
if ! command -v wails3 &>/dev/null; then
    echo "❌ 未找到 wails3，请先安装："
    echo "   go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117"
    exit 1
fi

ver=$(cat version 2>/dev/null || echo "0.0.0")

# 1. 图标
echo "生成图标 build/windows/icon.ico ..."
GOWORK=off go run ./tools/genicon

# 2. 版本信息（单来源：version 文件）
echo "生成版本信息 build/windows/info.json ..."
mkdir -p build/windows
cat > build/windows/info.json <<JSON
{
  "fixed": {
    "file_version": "$ver"
  },
  "info": {
    "0000": {
      "ProductVersion": "$ver",
      "CompanyName": "RemLink",
      "FileDescription": "RemLink 安全远程接入客户端",
      "LegalCopyright": "Copyright © 2026 RemLink",
      "ProductName": "RemLink",
      "Comments": ""
    }
  }
}
JSON

# 3. 资源 .syso（图标 + manifest 嵌入 exe）
echo "生成 wails_windows_amd64.syso ..."
wails3 generate syso -arch amd64 \
  -icon build/windows/icon.ico \
  -manifest build/windows/wails.exe.manifest \
  -info build/windows/info.json \
  -out wails_windows_amd64.syso

echo "完成: $SCRIPT_DIR/../wails_windows_amd64.syso"
