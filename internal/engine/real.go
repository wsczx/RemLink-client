// Package engine 的真实隧道实现（纯 Go，基于 sslcon 的 CSTP/DTLS/TUN）。
//
// 设计：直接使用 sslcon 的「标准 AnyConnect 协议」路径（auth 包），
// 与 sslcon/AnyConnect 原生客户端完全一致：
//   1. auth.InitAuth()   建立 TLS 并向网关 POST <config-auth type="init">，
//      服务端返回标准凭据表单（UA 含 openconnect 时）；
//   2. auth.PasswordAuth() POST <config-auth type="auth-reply"> 提交用户名/密码，
//      服务端回 <config-auth type="complete"> 并签发 session-token；
//   3. vpn.SetupTunnel() 用 session.Sess.SessionToken 作为 webvpn cookie
//      发起 CONNECT /CSCOSSLC/tunnel，完成 CSTP 协商 + 建 TUN + 路由 + DTLS。
//
// 这正是「成熟客户端零改动即可连上 RemLink」的原因：RemLink 本身就是
// AnyConnect 兼容服务端，标准协议路径原生就支持，无需 SPA、无需改服务端。
//
// 传输层来自 https://github.com/tlslink/sslcon （MIT），已 vendor 至
// ./third_party/sslcon，零 cgo、零 libopenconnect，三平台可用。
//
// 注意：真实隧道在 iOS/Android 上必须置于 NetworkExtension / VpnService，
// 此实现仅覆盖桌面端；移动端需额外原生集成（独立硬仗）。
package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"sslcon/auth"
	"sslcon/base"
	"sslcon/session"
	"sslcon/utils/vpnc"
	"sslcon/vpn"

	"remlink-client/internal/types"
)

// realEngine 通过 sslcon 的纯 Go CSTP 客户端建隧道。
type realEngine struct {
	events      chan types.Event
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	server      string
	ssoTokenCh  chan string // SSO v2：app 层本地回调捕获令牌后写入，唤醒 Connect 中的握手
}

// NewReal 创建真实引擎（纯 Go，无 cgo / 无 libopenconnect）。
func NewReal() Engine {
	return &realEngine{events: make(chan types.Event, 64), ssoTokenCh: make(chan string, 1)}
}

func (r *realEngine) Events() <-chan types.Event { return r.events }

// hostPort 把 "host" 或 "https://host" 规范化为 "host:443"。
func hostPort(server string) string {
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimPrefix(server, "http://")
	if !strings.Contains(server, ":") {
		server += ":443"
	}
	return server
}

func (r *realEngine) Connect(ctx context.Context, cfg *Config) error {
	if cfg.Server == "" {
		emit(r.events, types.Event{Kind: "error", Message: "缺少服务器地址"})
		return errors.New("缺少服务器地址")
	}

	// 用可取消 context 包裹：使 Disconnect() 能在 SSO 等待阶段（用户关闭登录窗）
	// 取消阻塞，让连接回到「可重新连接」状态，避免主窗口卡在「连接中」。
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	// 关键：使用 sslcon 的标准 AnyConnect 协议路径（与 sslcon/AnyConnect 原生客户端一致）。
	// 这里刻意让 UA 不含 "openconnect"（用 "AnyConnect"），服务端因此返回 SSO v2 响应
	// （sso-v2-login 指向 WebAuth 登录页），由客户端在 webview 内打开完成认证——
	// 这正是原生 AnyConnect 的 SSO 行为，无需改服务端、无需 SPA 桥接。
	base.Cfg.InsecureSkipVerify = true
	base.Cfg.LogLevel = "Info"
	// 注意：不能含 "openconnect" 子串（服务端据此分支），也不能设 CiscoCompat=true
	// （SetCommonHeader 里 CiscoCompat 会把 AgentName 强制改回 "AnyConnect"）。
	base.Cfg.AgentName = "AnyConnect"
	base.Cfg.AgentVersion = "4.10.07062"
	base.InitLog()

	// 探测本地出口网卡，填充 base.LocalInterface（Ip4/Gateway/Mac），
	// 否则 SetRoutes 会因 localInterface 为 nil 而失败（表现为「连上但没推送路由」）。
	if err := vpnc.GetLocalInterface(); err != nil {
		emit(r.events, types.Event{Kind: "log", Message: "探测本地接口失败（路由可能不全）: " + err.Error()})
	}

	hp := hostPort(cfg.Server)
	auth.Prof.Host = strings.TrimSuffix(
		strings.TrimPrefix(strings.TrimPrefix(cfg.Server, "https://"), "http://"), ":443")
	auth.Prof.HostWithPort = hp
	auth.Prof.Scheme = "https://"
	auth.Prof.Username = cfg.Username
	auth.Prof.Password = cfg.Password
	auth.Prof.Group = cfg.Group
	auth.Prof.SecretKey = cfg.Secret
	auth.Prof.Initialized = true

	emit(r.events, types.Event{Kind: "status", Status: types.StatusConnecting})
	emit(r.events, types.Event{Kind: "log", Message: "正在与网关进行身份认证…"})

	// 标准协议握手：init → 服务端返回 SSO v2（sso-v2-login 指向 WebAuth 登录页）。
	if err := auth.InitAuth(); err != nil {
		emit(r.events, types.Event{Kind: "error", Message: "认证初始化失败: " + err.Error()})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return err
	}

	// 服务端未返回 SSO v2 登录地址：说明该网关未启用 WebAuth，或 UA 被识别为
	// 凭据模式。客户端目前只走 WebAuth/SSO 路径，故直接报错提示。
	if auth.Prof.SsoV2Login == "" {
		emit(r.events, types.Event{Kind: "error", Message: "服务器未返回 WebAuth 登录地址，请确认服务端已启用 WebAuth"})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return errors.New("服务器未返回 WebAuth 登录地址")
	}

	// 通知 app 层在 webview 中打开 WebAuth 登录页（不是系统浏览器）。
	emit(r.events, types.Event{Kind: "sso-login", URL: auth.Prof.SsoV2Login})

	// 阻塞等待 app 层本地回调（localhost:29786）捕获 SSO 令牌后唤醒。
	// 在此期间 webview 已展示 WebAuth 页，用户可正常交互（登录/扫码）。
	// 若用户关闭登录窗（Disconnect 取消 ctx）或 10 分钟未认证，则放弃本次连接，
	// 回到可重新连接状态，避免主窗口卡在「连接中」。
	ssoWait := time.NewTimer(10 * time.Minute)
	defer ssoWait.Stop()
	var ssoToken string
	select {
	case ssoToken = <-r.ssoTokenCh:
	case <-ctx.Done():
		emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
		return ctx.Err()
	case <-ssoWait.C:
		emit(r.events, types.Event{Kind: "error", Message: "WebAuth 登录超时，请重新连接"})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return errors.New("sso 等待超时")
	}
	if ssoToken == "" {
		emit(r.events, types.Event{Kind: "error", Message: "未获取到 SSO 令牌"})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return errors.New("未获取到 SSO 令牌")
	}

	// 用 SSO 令牌提交 auth-reply，handleSsoToken 还原会话并签发 session-token。
	auth.Prof.SsoToken = ssoToken
	if err := auth.SsoAuth(); err != nil {
		emit(r.events, types.Event{Kind: "error", Message: "身份认证失败: " + err.Error()})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return err
	}

	// session.Sess.SessionToken 已由 SsoAuth 设置（来自服务端签发的 session-token）。
	emit(r.events, types.Event{Kind: "log", Message: "认证成功，正在建立安全隧道…"})
	r.server = cfg.Server

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := vpn.SetupTunnel(); err != nil {
			emit(r.events, types.Event{Kind: "error", Message: "隧道建立失败: " + err.Error()})
			emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
			return
		}
		// SetupTunnel 返回即隧道已建立（TUN/路由已配置，数据通道在后台运行）。
		cSess := session.Sess.CSess
		emit(r.events, types.Event{Kind: "status", Status: types.StatusConnected})
		emit(r.events, types.Event{Kind: "log", Message: "隧道已建立，连接成功"})
		if cSess != nil {
			emit(r.events, types.Event{Kind: "tunnel", Tunnel: &types.TunnelInfo{
				IPv4:   cSess.VPNAddress,
				DNS:    cSess.DNS,
				MTU:    uint32(cSess.MTU),
				Server: r.server,
			}})
		}
	}()
	return nil
}

// SubmitSSOToken 由 app 层本地回调（localhost:29786/api/sso/{token}）捕获令牌后调用，
// 唤醒阻塞在 Connect 中的 SSO 握手。
func (r *realEngine) SubmitSSOToken(token string) error {
	if r.ssoTokenCh != nil {
		select {
		case r.ssoTokenCh <- token:
		default:
		}
	}
	return nil
}

func (r *realEngine) Submit(answers map[string]string) error {
	// 标准协议下认证已在 Login 阶段由 sslcon auth 包完成，无需表单回灌。
	return nil
}

func (r *realEngine) Disconnect() error {
	if r.cancel != nil {
		r.cancel()
	}
	if auth.Conn != nil {
		_ = auth.Conn.Close()
	}
	if session.Sess.CSess != nil {
		session.Sess.CSess.Close()
	}
	emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
	return nil
}
