.PHONY: all icon syso build build-linux clean help

VERSION := $(shell cat version 2>/dev/null || echo 0.0.0)

## 生成 Windows 图标 (build/windows/icon.ico)
icon:
	GOWORK=off go run ./tools/genicon

## 生成资源 .syso（图标 + manifest 嵌入 exe）
syso: icon
	bash scripts/build_syso.sh

## 交叉编译 Windows amd64 发布包（含图标）
build:
	bash scripts/build_windows.sh

## 本地 Linux 编译（开发用，.syso 自动忽略）
build-linux:
	GOWORK=off go build -buildvcs=false -o remlink-client .

## 清理构建产物
clean:
	rm -f remlink-client remlink-client.exe wails_windows_amd64.syso
	rm -f build/windows/icon.ico build/windows/info.json

## 查看帮助
help:
	@echo "RemLink 客户端构建命令:"
	@echo ""
	@echo "  make icon        生成 Windows 图标"
	@echo "  make syso        生成资源 .syso（图标 + manifest）"
	@echo "  make build       交叉编译 Windows amd64 发布包（含图标）"
	@echo "  make build-linux 本地 Linux 编译（开发用）"
	@echo "  make clean       清理构建产物"
	@echo "  make help        显示帮助"
	@echo ""
	@echo "当前版本: $(VERSION)"

all: build
