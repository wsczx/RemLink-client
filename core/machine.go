package core

import (
	"context"
	"sync"

	"remlink-client/internal/types"
)

// machineImpl 持有状态机的可变运行态。Machine 是它对外暴露的不可变句柄。
// 这样把“状态”与“入口方法”分离，便于后续把重连/控制通道等协程挂到 impl 上。
type machineImpl struct {
	mu        sync.Mutex
	status    ConnStatus
	reconnWG  sync.WaitGroup
	active    bool // 是否有进行中的连接/重连
	onDisconn chan struct{}
}

func newMachineImpl() *machineImpl {
	return &machineImpl{
		status: StatusIdle,
	}
}

// setStatus 切换状态并向订阅者广播 status 事件。
func (m *Machine) setStatus(s ConnStatus) {
	m.impl.mu.Lock()
	m.impl.status = s
	m.impl.mu.Unlock()
	m.emit(Event{Kind: "status", Status: s})
}

// cacheAndPublish 更新本地状态缓存并向订阅者广播事件。Transport 后续事件与
// Machine 主动发出的状态/隧道事件都经此汇聚，保证缓存与前端事件一致。
func (m *Machine) cacheAndPublish(e Event) {
	switch e.Kind {
	case "status":
		if e.Status != StatusIdle {
			m.lastStatus = e.Status
		}
		if e.Tunnel != nil {
			m.lastTun = e.Tunnel
		}
		if e.Stats != nil {
			m.lastStats = e.Stats
		}
		if e.Policy != nil {
			m.lastPolicy = e.Policy
		}
		if e.Notices != nil {
			m.lastNotice = e.Notices
		}
		if e.Devices != nil {
			m.lastDevice = e.Devices
		}
	case "tunnel":
		m.lastTun = e.Tunnel
	case "stats":
		m.lastStats = e.Stats
	case "policy":
		m.lastPolicy = e.Policy
	case "notice":
		if e.Notice != nil {
			m.lastNotice = append([]types.Notice{*e.Notice}, m.lastNotice...)
			if len(m.lastNotice) > 50 {
				m.lastNotice = m.lastNotice[:50]
			}
		}
		if e.Notices != nil {
			m.lastNotice = e.Notices
		}
	case "devices":
		m.lastDevice = e.Devices
	}
	m.bus.publish(e)
}

// emit 向订阅者广播一个通用事件（带缓存更新）。
func (m *Machine) emit(e Event) {
	m.cacheAndPublish(e)
}

// connect 发起连接：idle → connecting → (auth | connected)。
// 第一阶段直接调用 Transport.Dial；后续 P3 把 nativeWebAuth 分步逻辑接入此处。
func (m *Machine) connect(ctx context.Context) error {
	m.ensure()
	m.impl.active = true
	m.setStatus(StatusConnecting)

	form, err := m.tr.Dial(ctx, m.cfg)
	if err != nil {
		m.emit(Event{Kind: "error", Message: err.Error()})
		m.setStatus(StatusError)
		m.impl.active = false
		return err
	}

	if form != nil {
		// 需要分步认证（原生 WebAuth 或 SSO）
		m.setStatus(StatusNeedAuth)
		m.emit(Event{Kind: "webauth", WebAuth: form})
		return nil
	}

	// 直达 connected
	m.setStatus(StatusConnected)
	if ti := m.tr.TunnelInfo(); ti != nil {
		m.emit(Event{Kind: "tunnel", Tunnel: ti})
	}
	return nil
}

// submit 提交认证表单答案，透传给底层 Transport（唤醒 nativeWebAuth 状态机）。
// P3 将把 real.go 的 nativeWebAuth 分步推进逻辑整体搬入 core，由 core 持有当前待填表单。
func (m *Machine) submit(kind string, fields map[string]string) {
	m.ensure()
	m.emit(Event{Kind: "log", Message: "submit " + kind})
	if m.tr != nil {
		if err := m.tr.Submit(fields); err != nil {
			m.emit(Event{Kind: "log", Message: "submit error: " + err.Error()})
		}
	}
}

// submitSSOToken 回灌 SSO 令牌（由 SSOHandler 内部捕获并调用），透传给 Transport。
func (m *Machine) submitSSOToken(token string) {
	m.ensure()
	m.emit(Event{Kind: "log", Message: "sso token received"})
	if m.tr != nil {
		if err := m.tr.SubmitSSOToken(token); err != nil {
			m.emit(Event{Kind: "log", Message: "sso submit error: " + err.Error()})
		}
	}
}

// disconnect 主动断开。
func (m *Machine) disconnect() {
	m.ensure()
	m.impl.active = false
	if err := m.tr.Close(); err != nil {
		m.emit(Event{Kind: "log", Message: "disconnect close: " + err.Error()})
	}
	// 停止 pump goroutine（避免 Disconnect 后继续从已关闭的 Transport 读事件）。
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	if m.impl.onDisconn != nil {
		close(m.impl.onDisconn)
	}
	m.setStatus(StatusDisconnected)
	m.bus.close()
}
