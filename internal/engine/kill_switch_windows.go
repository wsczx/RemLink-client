//go:build windows

package engine

import "fmt"

// Windows 断网保护（P1d）：需 WFP（Windows Filtering Platform）内核过滤，
// 纯 Go 无稳定绑定，且需 cgo/签名驱动，列为真机实现项。
//
// 当前版本未启用（返回未实现错误，由调用方降级为「仅提示」），
// 不阻断连接。后续可用 WFP 的 Win32 API（或第三方 Go 绑定）实现等价规则。
func EnableKillSwitch(server string, port int) error {
	return fmt.Errorf("Windows 断网保护(WFP)尚未实现，请联系管理员或暂关闭该策略")
}

// DisableKillSwitch Windows 端暂无规则可清，安全返回。
func DisableKillSwitch() error {
	return nil
}
