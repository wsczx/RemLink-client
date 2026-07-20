// Package types 定义 RemLink 客户端的核心领域模型与工具函数。
// 这些类型与 Rust 版 client/core/src/types.rs 一一对应，便于两端对齐。
package types

import (
	"fmt"
	"time"
)

// ConnStatus 连接状态机。
type ConnStatus int

const (
	StatusIdle ConnStatus = iota
	StatusConnecting
	StatusNeedAuth
	StatusConnected
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
	case StatusDisconnected:
		return "disconnected"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// MarshalJSON 让引擎事件里的 status 以字符串（而非整数）下发，便于前端判断。
func (s ConnStatus) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// ServerEntry 用户保存的连接配置（一个 VPN 网关入口）。
type ServerEntry struct {
	Name     string `json:"name"`
	Host     string `json:"host"` // 例如 vpn.example.com 或 https://vpn.example.com
	Group    string `json:"group"`
	Username string `json:"username"`
}

// TunnelInfo 隧道建立后分配的地址与实时流量统计。
type TunnelInfo struct {
	IPv4         string   `json:"ipv4"`
	IPv6         string   `json:"ipv6"`
	DNS          []string `json:"dns"`
	MTU          uint32   `json:"mtu"`
	RxBytes      uint64   `json:"rx_bytes"`
	TxBytes      uint64   `json:"tx_bytes"`
	DurationSecs uint64   `json:"duration_secs"`
	Server       string   `json:"server"`
}

// FormField 单个认证输入字段。
type FormField struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	FieldType string `json:"field_type"` // "text" | "password"
	Value     string `json:"value"`
}

// AuthForm 需要用户在 UI 中填写的认证表单（由引擎回调产生）。
type AuthForm struct {
	Banner        string      `json:"banner"`
	Message       string      `json:"message"`
	Error         string      `json:"error"`
	Fields        []FormField `json:"fields"`
	GroupSelected bool        `json:"group_selected"`
}

// Event 是引擎 → UI 的事件通道载荷。
type Event struct {
	Kind    string      `json:"kind"` // status | form | tunnel | log | error | sso-login
	Status  ConnStatus  `json:"status,omitempty"`
	Form    *AuthForm   `json:"form,omitempty"`
	Tunnel  *TunnelInfo `json:"tunnel,omitempty"`
	Message string      `json:"message,omitempty"`
	URL     string      `json:"url,omitempty"` // sso-login: WebAuth 登录地址，由 app 层打开 webview
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
