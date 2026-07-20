//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// extractWintunDLL 把内嵌的 wintun.dll 释放到 exe 同目录。
//
// golang.zx2c4.com/wintun 用
//   windows.LoadLibraryEx("wintun.dll", 0, LOAD_LIBRARY_SEARCH_APPLICATION_DIR|LOAD_LIBRARY_SEARCH_SYSTEM32)
// 加载，只会在 exe 目录与 system32 查找 wintun.dll。
// 因此必须把 dll 放到 exe 旁边，否则建隧道时报
//   "Error loading wintun.dll DLL: Unable to load library"。
//
// 若同目录已存在 wintun.dll 则跳过（避免重复写，也允许用户自带更新版本）。
func extractWintunDLL() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dst := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if _, err := os.Stat(dst); err == nil {
		return nil // 已存在，无需重复释放
	}
	return os.WriteFile(dst, wintunDLL, 0644)
}

func init() {
	if err := extractWintunDLL(); err != nil {
		println("warning: 释放 wintun.dll 失败:", err.Error())
	}
}
