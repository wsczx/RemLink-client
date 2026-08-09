package core

import (
	"fmt"
	"time"

	"remlink-client/internal/types"
)

// ConnStatus 连接状态机。与 internal/types.ConnStatus 一一对应（core 不 import types，避免环依赖）。
type ConnStatus int

const (
	StatusIdle ConnStatus = iota
	StatusConnecting
	StatusNeedAuth
	StatusConnected
	StatusReconnecting
	StatusDisconnected
	StatusError
)

func (s ConnStatus) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusConnecting:
		return "connecting"
	case StatusNeedAuth:
		return "need_auth"
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusDisconnected:
		return "disconnected"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// MarshalJSON 让事件里的 status 以字符串（而非整数）下发，便于前端/宿主判断。
func (s ConnStatus) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// Profile 一次连接的网关入口与凭据（替代 sslcon 的全局 auth.Prof）。
// 实例级别保存，移动端多 Profile 天然支持。
type Profile struct {
	Name      string // 用户保存的配置名（如“公司 VPN”）
	Host      string // vpn.example.com 或 https://vpn.example.com
	Group     string // 隧道组（可选，未填由服务端/WebAuth 决定）
	Username  string
	Password  string
	SecretKey string // 服务端可选的共享密钥（URL ?key=）

	AppVersion string // 上报给服务端的客户端版本
	// 设备指纹字段（原 auth.Prof 的 ComputerName/UniqueId 等）由 Transport 在连接时填充，
	// 这里仅保留宿主可覆盖的入口。
}

// WebAuthForm 原生认证表单（引擎消费服务端 WebAuth JSON API 后下发给 UI）。
// Status 决定 UI 渲染哪种输入：
//
//	select_group → 组下拉框；credentials → 用户名/密码；otp → 动态码；
//	sms_phone → 手机号；sms → 短信码；radius → 挑战码；change_pwd → 新密码×2。
//
// Error 非空表示上一次提交失败，UI 就地显示错误并允许重新输入。
type WebAuthForm struct {
	Status       string   `json:"status"`
	Groups       []string `json:"groups,omitempty"`
	Hint         string   `json:"hint,omitempty"`
	Error        string   `json:"error,omitempty"`
	Username     string   `json:"username,omitempty"`
	PhoneMasked  string   `json:"phone_masked,omitempty"`
	ChallengeMsg string   `json:"challenge_msg,omitempty"`
}

// DeviceBinding 设备绑定条目（控制通道 device_list 下推）。
// 复用 types.DeviceBinding（前端「设备管理」视图据此渲染，支持解绑）。
type DeviceBinding = types.DeviceBinding

// Event 是 Machine → 宿主的事件通道载荷。
// Kind 取值：status | webauth | tunnel | log | error | sso-login | policy | notice | devices
// 数据承载字段（Tunnel/Policy/Stats/Notice/Devices）直接复用 types 包类型，
// 避免与前端 DTO 重复定义；仅 Status 用 core.ConnStatus（整数状态机），下发时转为字符串。
type Event struct {
	Kind    string                `json:"kind"`
	Status  ConnStatus            `json:"status,omitempty"`
	WebAuth *WebAuthForm          `json:"webauth,omitempty"`
	Tunnel  *types.TunnelInfo     `json:"tunnel,omitempty"`
	Message string                `json:"message,omitempty"`
	URL     string                `json:"url,omitempty"`     // sso-login: 由 SSOHandler 打开的地址
	Policy  *types.PolicyInfo     `json:"policy,omitempty"`  // policy: 服务端 config_push
	Notice  *types.Notice         `json:"notice,omitempty"`  // notice: 通知中心条目
	Notices []types.Notice        `json:"notices,omitempty"` // notice 批量（状态快照）
	Devices []types.DeviceBinding `json:"devices,omitempty"` // devices: 控制通道下推设备列表
	Stats   *types.TunnelStats    `json:"stats,omitempty"`   // stats: 实时流量/通道统计
}

// HumanBytes 把字节数格式化为人类可读串。
func HumanBytes(n uint64) string {
	const units = "BKMGTPE"
	var v float64 = float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %c", n, units[0])
	}
	return fmt.Sprintf("%.2f %cB", v, units[i])
}

// HumanDuration 把秒数格式化为 时:分:秒。
func HumanDuration(secs uint64) string {
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// NowSecs 返回当前 Unix 秒，便于时长计算。
func NowSecs() uint64 {
	return uint64(time.Now().Unix())
}
