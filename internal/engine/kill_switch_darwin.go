//go:build darwin

package engine

import "fmt"

// macOS 断网保护（P1d）：需用 pf 锚点规则（/etc/pf.conf 或 anchor），
// 命令行加载需管理员权限，列为真机实现项。
//
// 当前版本未启用（返回未实现错误，由调用方降级为「仅提示」）。
func EnableKillSwitch(server string, port int) error {
	return fmt.Errorf("macOS 断网保护(pf)尚未实现，请联系管理员或暂关闭该策略")
}

// DisableKillSwitch macOS 端暂无规则可清，安全返回。
func DisableKillSwitch() error {
	return nil
}
