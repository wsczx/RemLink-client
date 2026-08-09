package core

import (
	"context"

	"remlink-client/internal/types"
)

// Transport 是底层传输层（CSTP/DTLS/TUN 驱动）的抽象。
//
// 第一阶段：桌面实现复用 sslcon（内部仍读全局 auth.Prof/session.Sess，但由 host/desktop 适配器
// 在调用前填好），core 不直接引用 sslcon 全局状态。
// 后续 P2：改为实例化的 *transport.Client，每个连接一个实例，彻底消除全局变量。
//
// 设计约定：
//   - Dial 建立并认证；成功后 ConnSession 由 Transport 持有，core 通过 Stats/TunnelInfo 只读读取。
//   - 重连时调用 Reconnect（复用 session-token 重建隧道，无需重新认证）。
//   - Close 拆除隧道并恢复路由；ctx 取消应触发优雅退出。
type Transport interface {
	// Dial 建立 TLS 连接并完成认证（或 SSO 后回灌令牌的后续步骤）。
	// 返回服务端下发的初始 WebAuth 表单（如需分步认证）或 nil（直达 connected）。
	Dial(ctx context.Context, p *Profile) (*WebAuthForm, error)

	// Submit 回灌认证表单答案（组选择/凭据/挑战码），唤醒原生 WebAuth 状态机。
	Submit(answers map[string]string) error

	// SubmitSSOToken 回灌 SSO 令牌（唤醒阻塞在 Connect 中的 SSO 握手）。
	SubmitSSOToken(token string) error

	// Reconnect 在断线后复用 session-token 重建隧道（自动重连路径）。
	Reconnect(ctx context.Context) error

	// Close 主动拆除隧道并恢复系统路由。
	Close() error

	// TunnelInfo 返回当前隧道分配地址（用于 tunnel 事件与策略/Routes 处理）。
	TunnelInfo() *types.TunnelInfo

	// Stats 返回实时流量/通道统计。
	Stats() *types.TunnelStats

	// RunDiagnostics 本地自助排障（网关可达性/DNS/MTU/路由概况）。
	RunDiagnostics() (map[string]any, error)

	// Events 返回 Transport 内部的 core.Event 通道（已由 Transport 把底层引擎事件
	// 转换为 core.Event）。Machine 作为唯一消费者：Dial 读首批，之后 pump goroutine
	// 读后续（connected/tunnel/status），二者顺序消费同一通道，不分片。
	Events() <-chan Event
}
