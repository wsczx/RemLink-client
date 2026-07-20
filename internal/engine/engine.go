// Package engine 定义 VPN 引擎抽象与事件通道。
//
// 设计目标（与 Rust 版一致）：
//   - 引擎只负责“隧道”，UI 通过 Events() 通道被动接收状态/表单/流量；
//   - 认证表单由引擎回调产生，UI 渲染后通过 Submit() 回灌答案；
//   - mock 引擎无需 libopenconnect，用于 UI 联调与无依赖构建；
//   - real 引擎（build tag: openconnect）通过 cgo 调用 libopenconnect，
//     仅在装好 libopenconnect-dev 时编译。
package engine

import (
	"context"

	"remlink-client/internal/types"
)

// Config 一次连接所需的参数。
// 采用标准 AnyConnect 协议（与 sslcon/AnyConnect 原生客户端一致）：
// 客户端直接用 username/password 向网关认证，由服务端签发 session-token，
// 再建 CSTP 隧道。不需要 SPA、不需要改服务端。
type Config struct {
	Server   string // 网关地址（https://host 或 host）
	Username string // 认证用户名
	Password string // 认证密码
	Group    string // 预选用户组（可空，由服务端提供选择）
	Secret   string // 动态码 / OTP（可空）
}

// Engine 是 VPN 引擎的统一接口。
type Engine interface {
	// Connect 启动连接流程（在内部 goroutine 运行，立即返回）。
	Connect(ctx context.Context, cfg *Config) error
	// Submit 提交认证表单答案（在 NeedAuth 状态下调用）。
	Submit(answers map[string]string) error
	// SubmitSSOToken 在 SSO v2 流程中，由 app 层本地回调捕获令牌后调用，
	// 唤醒阻塞在 Connect 中的 SSO 握手。
	SubmitSSOToken(token string) error
	// Disconnect 断开并释放资源。
	Disconnect() error
	// Events 返回引擎 → UI 的事件通道。
	Events() <-chan types.Event
}

// emit 非阻塞发送事件，通道满时丢弃最新（避免阻塞引擎）。
func emit(ch chan types.Event, e types.Event) {
	select {
	case ch <- e:
	default:
	}
}
