package engine

import (
	"context"
	"sync"
	"time"

	"remlink-client/internal/types"
)

// mockEngine 用模拟数据跑通完整 UI 流程：连接 → 认证表单 →
// 建连 → 实时流量/时长 → 断开。无需 libopenconnect，便于无依赖联调。
type mockEngine struct {
	events    chan types.Event
	submitted chan struct{}
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewMock 创建一个 mock 引擎实例。
func NewMock() Engine {
	return &mockEngine{
		events: make(chan types.Event, 64),
	}
}

func (m *mockEngine) Events() <-chan types.Event { return m.events }

func (m *mockEngine) Connect(ctx context.Context, cfg *Config) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.submitted = make(chan struct{}, 1)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		emit(m.events, types.Event{Kind: "status", Status: types.StatusConnecting})
		emit(m.events, types.Event{Kind: "log", Message: "正在进行认证..."})

		// 模拟认证表单
		select {
		case <-time.After(800 * time.Millisecond):
		case <-ctx.Done():
			return
		}
		form := &types.AuthForm{
			Banner:  "RemLink 演示网关",
			Message: "请输入登录凭据",
			Fields: []types.FormField{
				{Name: "username", Label: "用户名", FieldType: "text", Value: cfg.Username},
				{Name: "password", Label: "密码", FieldType: "password"},
			},
		}
		emit(m.events, types.Event{Kind: "form", Form: form})
		emit(m.events, types.Event{Kind: "status", Status: types.StatusNeedAuth})

		// 等待用户提交或断开
		select {
		case <-m.submitted:
		case <-ctx.Done():
			emit(m.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
			return
		}

		emit(m.events, types.Event{Kind: "status", Status: types.StatusConnected})
		emit(m.events, types.Event{Kind: "log", Message: "隧道已建立，连接成功"})

		// 实时流量/时长
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		var rx, tx uint64
		for {
			select {
			case <-ctx.Done():
				emit(m.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
				return
			case <-ticker.C:
				rx += 1536
				tx += 768
				emit(m.events, types.Event{Kind: "tunnel", Tunnel: &types.TunnelInfo{
					IPv4:         "10.8.0.42",
					DNS:          []string{"8.8.8.8", "1.1.1.1"},
					MTU:          1400,
					RxBytes:      rx,
					TxBytes:      tx,
					DurationSecs: uint64(time.Since(start).Seconds()),
					Server:       cfg.Server,
				}})
			}
		}
	}()
	return nil
}

func (m *mockEngine) Submit(answers map[string]string) error {
	if m.submitted != nil {
		select {
		case m.submitted <- struct{}{}:
		default:
		}
	}
	return nil
}

func (m *mockEngine) SubmitSSOToken(token string) error { return nil }

func (m *mockEngine) Stats() *types.TunnelStats {
	return &types.TunnelStats{RxBytes: 1536, TxBytes: 768, Channel: "DTLS", Cipher: "MOCK-CIPHER"}
}

func (m *mockEngine) RunDiagnostics() (map[string]any, error) {
	return map[string]any{
		"gw_ping":      true,
		"gw_addr":      "mock-gateway",
		"vpn_addr":     "10.8.0.42",
		"mtu":          1400,
		"dns":          true,
		"dns_server":   "8.8.8.8",
		"routes_include": 0,
		"routes_exclude": 0,
	}, nil
}

func (m *mockEngine) Disconnect() error {
	if m.cancel != nil {
		m.cancel()
	}
	return nil
}
