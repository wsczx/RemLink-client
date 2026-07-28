//go:build windows

package wintundll

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
// 始终用内嵌版本覆盖同目录的旧 wintun.dll：曾经出现过「旧/损坏的 wintun.dll
// 残留在 exe 目录，embed 的跳过逻辑导致坏 dll 一直被加载，CreateAdapter 报
// 'The system cannot find the file specified'」的坑。客户端与内嵌 dll 版本强绑定，
// 强制覆盖比「跳过」更稳定。
func extractWintunDLL() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dst := filepath.Join(filepath.Dir(exe), "wintun.dll")
	// 先删旧的（忽略「不存在」错误），再写内嵌版本，避免残留坏文件。
	_ = os.Remove(dst)
	if err := os.WriteFile(dst, wintunDLL, 0644); err != nil {
		return err
	}
	return nil
}

func init() {
	if err := extractWintunDLL(); err != nil {
		println("warning: 释放 wintun.dll 失败:", err.Error())
	}
}
