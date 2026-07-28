//go:build !windows

// 非 Windows 平台不需要释放 wintun.dll，仅保留空包使本包可在全平台被导入。
package wintundll
