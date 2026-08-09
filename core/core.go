// Package core 是 RemLink 客户端的平台无关连接内核。
//
// 设计目标（架构级重构第一阶段，2026-08-09）：
//   - 不依赖 Wails / webview / 系统托盘 / 任何具体 UI；
//   - 不直接依赖 sslcon 内部全局状态（auth.Prof / session.Sess / base.Cfg）；
//   - 通过 Transport 接口与底层传输层（CSTP/DTLS/TUN）解耦；
//   - 通过 SSOHandler 接口与具体 SSO 承载方式（桌面 webview / 移动端系统浏览器）解耦；
//   - 仅以 Event 对外广播状态、以 Input 接收用户/宿主输入。
//
// 这样 Wails 桌面、iOS NetworkExtension、Android VpnService 都只是 core 的“宿主适配器”，
// 复用同一套连接状态机，无需为每端重写认证/重连/策略逻辑。
package core

import (
	"context"
	"sync"

	"remlink-client/internal/types"
)

// Machine 是连接状态机的宿主入口。由 NewMachine 构造，宿主（桌面/移动端）持有其实例，
// 调用 Connect/Submit/SubmitSSOToken/Disconnect 驱动状态流转，并通过 Events() 订阅事件。
type Machine struct {
	cfg    *Profile
	tr     Transport
	sso    SSOHandler
	bus    *eventBus
	impl   *machineImpl
	stopCh chan struct{} // 关闭后 pump goroutine 退出

	// 缓存最近一次状态快照，供宿主在任意时刻查询（无需依赖事件流时序）。
	mu         sync.RWMutex
	lastStatus ConnStatus
	lastStats  *types.TunnelStats
	lastPolicy *types.PolicyInfo
	lastNotice []types.Notice
	lastTun    *types.TunnelInfo
	lastDevice []types.DeviceBinding
}

// NewMachine 构造一个连接状态机实例。
//   - tr：底层传输层实现（CSTP/DTLS/TUN 驱动）。第一阶段的桌面实现复用 sslcon，
//     后续 P2 改为实例化的 *transport.Client。
//   - sso：SSO 承载方式。桌面传 Wails webview 适配器；移动端传系统浏览器适配器；
//     纯账密/OTP 场景下 SSO 不会被触发，可传 nil。
func NewMachine(tr Transport, sso SSOHandler) *Machine {
	return &Machine{
		tr:     tr,
		sso:    sso,
		bus:    newEventBus(64),
		impl:   newMachineImpl(),
		stopCh: make(chan struct{}),
	}
}

// ensure 保证 impl 已初始化（防御性，正常路径 NewMachine 已建好）。
func (m *Machine) ensure() {
	if m.impl == nil {
		m.impl = newMachineImpl()
	}
}

// Events 返回事件订阅通道。宿主应常驻消费，直至 Disconnect + 取消 ctx。
func (m *Machine) Events() <-chan Event {
	return m.bus.sub()
}

// Connect 发起一次新连接（idle → connecting → auth → connected）。
// cfg 描述本次连接的网关入口与凭据/组选择。
func (m *Machine) Connect(ctx context.Context, cfg Profile) error {
	m.cfg = &cfg
	if err := m.connect(ctx); err != nil {
		return err
	}
	// Dial 已消费首批事件（webauth/connected），后续事件由 pump 持续转发到 bus。
	// Machine 是 tr.Events() 的唯一消费者，Drain 与 Dial 顺序进行，不分片。
	go m.pump()
	return nil
}

// pump 持续把 Transport 的后续事件转发到事件总线，直到 stopCh 关闭（Disconnect 时）。
func (m *Machine) pump() {
	if m.tr == nil {
		return
	}
	for {
		select {
		case <-m.stopCh:
			return
		case e := <-m.tr.Events():
			m.cacheAndPublish(e)
		}
	}
}

// Submit 提交一份认证表单答案（原生 WebAuth 状态机分步推进）。
// kind 对应服务端 WebAuth 阶段（credentials/otp/sms/radius/change_pwd/select_group），
// 透传给底层 Transport（当前实现忽略 kind，直接以 fields 作为 answers 回灌）。
func (m *Machine) Submit(kind string, fields map[string]string) {
	m.submit(kind, fields)
}

// Stats 返回当前隧道实时统计（委托 Transport）。
func (m *Machine) Stats() *types.TunnelStats {
	if m.tr == nil {
		return nil
	}
	return m.tr.Stats()
}

// RunDiagnostics 本地自助排障（委托 Transport）。
func (m *Machine) RunDiagnostics() (map[string]any, error) {
	if m.tr == nil {
		return nil, nil
	}
	return m.tr.RunDiagnostics()
}

// SubmitSSOToken 在 SSO 完成后回灌令牌（由 SSOHandler 内部捕获并调用）。
func (m *Machine) SubmitSSOToken(token string) {
	m.submitSSOToken(token)
}

// Disconnect 主动断开（connected/reconnecting → disconnected）。
// 由宿主在用户点击“断开”或进程退出时调用；ctx 取消亦可触发优雅退出。
func (m *Machine) Disconnect() {
	m.disconnect()
}

// ---- 状态快照取值（宿主在任意时刻查询，无需依赖事件流时序） ----

// Status 返回最近一次连接状态（disconnected/connecting/auth/connected/reconnecting）。
func (m *Machine) Status() ConnStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastStatus
}

// Snapshot 返回一份合并的状态快照，便于宿主（Wails）一次性序列化给前端。
func (m *Machine) Snapshot() Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Event{
		Kind:    "status",
		Status:  m.lastStatus,
		Stats:   m.lastStats,
		Policy:  m.lastPolicy,
		Notices: m.lastNotice,
		Tunnel:  m.lastTun,
		Devices: m.lastDevice,
	}
}

// Tunnel 返回最近一次隧道信息（IP/DNS/MTU 等）。
func (m *Machine) Tunnel() *types.TunnelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastTun
}

// Devices 返回最近一次设备绑定列表（控制通道下推）。
func (m *Machine) Devices() []DeviceBinding {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastDevice
}
