package transport

import (
	"context"
	"testing"
	"time"

	"remlink-client/core"
	"remlink-client/internal/engine"
)

// TestMachineViaTransport 用 mock engine 验证 core.Machine 经 transport.Client
// 完整跑通连接流程（connect → webauth → submit → connected → disconnect），
// 证明平台无关状态机（core）与底层引擎（engine）已通过 transport 正确解耦。
func TestMachineViaTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 用 mock engine 跑通 UI 流程（无需真实网关/sslcon）。
	tr := New(engine.NewMock())
	m := core.NewMachine(tr, nil)

	evCh := m.Events()
	if err := m.Connect(ctx, core.Profile{Host: "mock-gateway", Username: "u", Password: "p"}); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// 期望收到 connecting → form(webauth) → need_auth
	gotForm := false
	for i := 0; i < 3; i++ {
		select {
		case e := <-evCh:
			if e.Kind == "webauth" {
				gotForm = true
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting events")
		}
	}
	if !gotForm {
		t.Fatal("expected webauth form event")
	}

	// 提交表单答案，应进入 connected
	m.Submit("credentials", map[string]string{"username": "u", "password": "p"})

	connected := false
	for i := 0; i < 3; i++ {
		select {
		case e := <-evCh:
			if e.Kind == "status" && e.Status == core.StatusConnected {
				connected = true
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting connected")
		}
	}
	if !connected {
		t.Fatal("expected connected status")
	}

	// 统计与诊断应能返回（mock 有值）
	if m.Stats() == nil {
		t.Fatal("expected non-nil stats")
	}
	if _, err := m.RunDiagnostics(); err != nil {
		t.Fatalf("diagnostics: %v", err)
	}

	m.Disconnect()
	select {
	case e := <-evCh:
		if e.Kind == "status" && e.Status != core.StatusDisconnected {
			t.Fatalf("expected disconnected, got %v", e.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting disconnected")
	}
}
