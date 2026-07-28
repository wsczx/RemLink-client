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

// TunnelStats 连接期间的实时统计（UI 轮询用）。
type TunnelStats struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
	Channel string `json:"channel"` // 数据通道："DTLS" 或 "TLS"
	Cipher  string `json:"cipher"`  // 当前数据通道的密码套件
}

// PolicySummary 服务端下发的策略摘要（config_push.policy）。
type PolicySummary struct {
	PreferIPv6  bool  `json:"prefer_ipv6"`
	Compression bool  `json:"compression"`
	FakeDNS     bool  `json:"fakedns"`
	SplitDNS    bool  `json:"split_dns"`
	AllowLan    bool  `json:"allow_lan"`
	KillSwitch  bool  `json:"kill_switch"` // 断网保护（P1d）：隧道断连窗口阻断明文出网
	AclCount    int   `json:"acl_count"`
	Bandwidth   int   `json:"bandwidth,omitempty"`
	BandwidthUp int   `json:"bandwidth_up,omitempty"`
	Quota       int64 `json:"quota,omitempty"`
}

// PolicyInfo 控制通道 config_push 的完整载荷（策略可视化 + 智能选路候选）。
// json tag 与服务端 CtlConfigPush 对齐。
type PolicyInfo struct {
	Group         string        `json:"group,omitempty"`
	PolicyName    string        `json:"policy_name,omitempty"`
	Policy        PolicySummary `json:"policy"`
	RoutesInclude []string      `json:"routes_include,omitempty"`
	RoutesExclude []string      `json:"routes_exclude,omitempty"`
	DNS           []string      `json:"dns,omitempty"`
	Endpoints     []string      `json:"endpoints,omitempty"`
}

// Notice 通知中心条目（服务端消息/断开原因等）。
type Notice struct {
	Level string `json:"level"` // info | warn | error
	Text  string `json:"text"`
	Time  int64  `json:"time"` // Unix 秒
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

// Event 是引擎 → UI 的事件通道载荷。
type Event struct {
	Kind    string       `json:"kind"` // status | form | webauth | tunnel | log | error | sso-login | policy | notice
	Status  ConnStatus   `json:"status,omitempty"`
	Form    *AuthForm    `json:"form,omitempty"`
	WebAuth *WebAuthForm `json:"webauth,omitempty"` // webauth: 原生认证表单（组选择/凭据/挑战码）
	Tunnel  *TunnelInfo  `json:"tunnel,omitempty"`
	Message string       `json:"message,omitempty"`
	URL     string       `json:"url,omitempty"`    // sso-login: WebAuth 登录地址，由 app 层打开 webview
	Policy  *PolicyInfo  `json:"policy,omitempty"` // policy: 服务端 config_push
	Notice  *Notice      `json:"notice,omitempty"` // notice: 通知中心条目
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
