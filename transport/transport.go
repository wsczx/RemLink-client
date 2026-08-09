// Package transport 把现有 engine.Engine 桥接为 core.Transport 实例，
// 让 core.Machine（平台无关状态机）能真实驱动桌面连接逻辑，而不直接依赖 sslcon 全局状态。
//
// 阶段定位（2026-08-09 P2）：
//   - transport.Client 持有一个 engine.Engine 实例（real 或 mock），并启动转发 goroutine
//     把 engine 的原生事件（types.Event）转换为 core.Event，统一从 Events() 输出；
//   - core.Machine 是 Events() 的唯一消费者：Dial 读首批（webauth/connected），之后由
//     Machine 的 pump goroutine 续读后续（connected/tunnel/status/disconnected）；
//   - 后续 P3 将把 engine 的全局状态（auth.Prof/session.Sess/base.Cfg）收敛进 Client 实例，
//     并让 host/desktop 正式改用 core.Machine + transport.Client，替代 app.go 直接持有 realEngine。
package transport

import (
	"context"
	"fmt"
	"sync/atomic"

	"remlink-client/core"
	"remlink-client/internal/engine"
	"remlink-client/internal/types"
)

// Client 实现 core.Transport，封装一个 engine.Engine 实例。
type Client struct {
	eng     engine.Engine
	out     chan core.Event // 转换后的 core.Event 通道（Machine 唯一消费）
	closeCh chan struct{}
	closed  atomic.Bool
}

// New 用给定的 engine 实例构造 transport.Client，并启动事件转发 goroutine。
// 该 eng 的 Events() 不应被其他消费者读取（否则事件被分片）。
func New(eng engine.Engine) *Client {
	c := &Client{
		eng:     eng,
		out:     make(chan core.Event, 64),
		closeCh: make(chan struct{}),
	}
	go c.pump()
	return c
}

// pump 持续把 engine 原生事件转换为 core.Event 转发到 out。
// engine 事件约定（兼容 mock/real 差异）：
//   - 认证表单：real 用 Kind="webauth"+WebAuth 字段；mock 用 Kind="form"+Form 字段；
//   - 其余（status/tunnel/log/error/sso-login/policy/notice）字段同构，直接复制。
func (c *Client) pump() {
	for {
		select {
		case <-c.closeCh:
			return
		case e := <-c.eng.Events():
			if c.closed.Load() {
				return
			}
			select {
			case c.out <- core.Event{
				Kind:    e.Kind,
				Status:  core.ConnStatus(e.Status),
				WebAuth: bridgeWebAuth(e),
				Tunnel:  bridgeTunnel(e.Tunnel),
				Message: e.Message,
				URL:     e.URL,
				Policy:  bridgePolicy(e.Policy),
				Notice:  bridgeNotice(e.Notice),
			}:
			default:
			}
		}
	}
}

// Events 返回转换后的 core.Event 通道（Machine 唯一消费者）。
func (c *Client) Events() <-chan core.Event { return c.out }

// Dial 发起连接，从 out 通道阻塞读取直到首个认证表单（need_auth）或直达 connected/error。
// 返回 *core.WebAuthForm 表示需要分步认证；nil 表示已直连；error 表示失败。
func (c *Client) Dial(ctx context.Context, p *core.Profile) (*core.WebAuthForm, error) {
	cfg := &engine.Config{
		Server:   p.Host,
		Username: p.Username,
		Password: p.Password,
		Group:    p.Group,
		Secret:   p.SecretKey,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- c.eng.Connect(ctx, cfg) }()

	for {
		select {
		case e, ok := <-c.out:
			if !ok {
				return nil, fmt.Errorf("连接通道已关闭")
			}
			switch e.Kind {
			case "webauth", "form":
				if e.WebAuth != nil {
					return e.WebAuth, nil
				}
			case "status":
				switch e.Status {
				case core.StatusConnected:
					return nil, nil
				case core.StatusError:
					return nil, fmt.Errorf("连接失败：服务端返回错误状态")
				}
			case "error":
				return nil, fmt.Errorf("%s", e.Message)
			}
		case err := <-errCh:
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Submit 把认证表单答案回灌给 engine（唤醒 nativeWebAuth 状态机）。
func (c *Client) Submit(answers map[string]string) error {
	return c.eng.Submit(answers)
}

// SubmitSSOToken 回灌 SSO 令牌（唤醒阻塞在 Connect 中的 SSO 握手）。
func (c *Client) SubmitSSOToken(token string) error {
	return c.eng.SubmitSSOToken(token)
}

// Reconnect 复用 session-token 重建隧道（自动重连路径，由 core 重连逻辑调用）。
// 当前 engine 的 watch 协程已内置自动重连；显式 Reconnect 用于移动端/多会话场景（P3 接入）。
func (c *Client) Reconnect(ctx context.Context) error {
	return nil
}

// Close 主动断开并释放资源，并停止内部 pump goroutine。
// 不关闭 out 通道（由 Machine.pump 受自身 stopCh 控制退出），避免 close 后写 panic。
func (c *Client) Close() error {
	c.closed.Store(true)
	close(c.closeCh)
	return c.eng.Disconnect()
}

// TunnelInfo 当前由 Engine 通过 "tunnel" 事件下发，transport 不缓存，
// 故直接返回 nil；Machine 从 Events() 获取 TunnelInfo（见 pump）。
func (c *Client) TunnelInfo() *types.TunnelInfo {
	return nil
}

// Stats 委托 engine.Stats()。
func (c *Client) Stats() *types.TunnelStats {
	return c.eng.Stats()
}

// RunDiagnostics 委托 engine.RunDiagnostics()。
func (c *Client) RunDiagnostics() (map[string]any, error) {
	return c.eng.RunDiagnostics()
}

// ---- 桥接辅助：engine/types → core ----

func bridgeWebAuth(e types.Event) *core.WebAuthForm {
	if e.WebAuth != nil {
		f := e.WebAuth
		return &core.WebAuthForm{
			Status:       f.Status,
			Groups:       f.Groups,
			Hint:         f.Hint,
			Error:        f.Error,
			Username:     f.Username,
			PhoneMasked:  f.PhoneMasked,
			ChallengeMsg: f.ChallengeMsg,
		}
	}
	if e.Form != nil {
		f := e.Form
		wf := &core.WebAuthForm{Status: "credentials", Hint: f.Message, Error: f.Error}
		for _, fld := range f.Fields {
			if fld.Name == "username" {
				wf.Username = fld.Value
			}
		}
		return wf
	}
	return nil
}

func bridgeTunnel(t *types.TunnelInfo) *types.TunnelInfo {
	return t
}

func bridgePolicy(p *types.PolicyInfo) *types.PolicyInfo {
	return p
}

func bridgeNotice(n *types.Notice) *types.Notice {
	return n
}
