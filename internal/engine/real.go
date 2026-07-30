// Package engine 的真实隧道实现（纯 Go，基于 sslcon 的 CSTP/DTLS/TUN）。
//
// 设计：直接使用 sslcon 的「标准 AnyConnect 协议」路径（auth 包），
// 与 sslcon/AnyConnect 原生客户端完全一致：
//  1. auth.InitAuth()   建立 TLS 并向网关 POST <config-auth type="init">，
//     服务端返回标准凭据表单（UA 含 openconnect 时）；
//  2. auth.PasswordAuth() POST <config-auth type="auth-reply"> 提交用户名/密码，
//     服务端回 <config-auth type="complete"> 并签发 session-token；
//  3. vpn.SetupTunnel() 用 session.Sess.SessionToken 作为 webvpn cookie
//     发起 CONNECT /CSCOSSLC/tunnel，完成 CSTP 协商 + 建 TUN + 路由 + DTLS。
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
	"fmt"
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
	events     chan types.Event
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	server     string
	ssoTokenCh chan string            // SSO v2：app 层本地回调捕获令牌后写入，唤醒 Connect 中的握手
	answersCh  chan map[string]string // 原生认证表单：UI 提交答案后写入，唤醒 WebAuth 状态机
	ctlCancel  context.CancelFunc     // 控制通道生命周期（重连/断开时重建或停止）
}

// NewReal 创建真实引擎（纯 Go，无 cgo / 无 libopenconnect）。
func NewReal() Engine {
	return &realEngine{
		events:     make(chan types.Event, 64),
		ssoTokenCh: make(chan string, 1),
		answersCh:  make(chan map[string]string, 1),
	}
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

	// 原生认证（AnyConnect 同款模型）：客户端直接消费服务端 WebAuth JSON API，
	// 组选择、账号密码、OTP/短信/RADIUS 全部原生 UI；只有企微/飞书等 SSO 组
	// 才打开 WebView 扫码。认证完成拿到 sso-token（即 WebAuth state）。
	ssoToken, err := r.nativeWebAuth(ctx)
	if err != nil {
		if ctx.Err() != nil {
			// 用户主动取消（Disconnect）：状态已由取消路径下发
			emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
			return ctx.Err()
		}
		emit(r.events, types.Event{Kind: "error", Message: "认证失败: " + err.Error()})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return err
	}
	if ssoToken == "" {
		emit(r.events, types.Event{Kind: "error", Message: "未获取到 SSO 令牌"})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return errors.New("未获取到 SSO 令牌")
	}

	// 用 SSO 令牌提交 auth-reply，handleSsoToken 还原会话并签发 session-token。
	auth.Prof.SsoToken = ssoToken

	// 令牌已捕获，进入「认证换会话 + 建隧道」阶段。该阶段若因服务端无响应 /
	// TUN/路由设置阻塞而挂起，用户会一直卡在「正在建立安全连接」登录页；
	// 用看门狗在 90s 内未完成时上报超时，让真实错误暴露出来（而非无限转圈）。
	done := make(chan error, 1)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := auth.SsoAuth(); err != nil {
			done <- err
			return
		}
		// session.Sess.SessionToken 已由 SsoAuth 设置（来自服务端签发的 session-token）。
		emit(r.events, types.Event{Kind: "log", Message: "认证成功，正在建立安全隧道…"})
		r.server = cfg.Server
		if err := vpn.SetupTunnel(); err != nil {
			done <- err
			return
		}
		// SetupTunnel 返回即隧道已建立（TUN/路由已配置，数据通道在后台运行）。
		r.emitConnected()
		// 控制通道：与数据隧道并行的 WebSocket 长连，状态感知与服务端管控的权威通道
		r.startControl(ctx)
		// 看护本次会话：非主动断开（DPD 超时/网络切换）时自动重连
		go r.watch(ctx)
		done <- nil
	}()

	watchdog := time.NewTimer(90 * time.Second)
	defer watchdog.Stop()
	select {
	case err := <-done:
		if err != nil {
			emit(r.events, types.Event{Kind: "error", Message: "认证/隧道建立失败: " + err.Error()})
			emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
			return err
		}
		return nil
	case <-watchdog.C:
		emit(r.events, types.Event{Kind: "error", Message: "连接超时：认证或隧道建立 90s 内未完成（请查看客户端日志定位卡点）"})
		emit(r.events, types.Event{Kind: "status", Status: types.StatusError})
		return errors.New("连接超时")
	}
}

// nativeWebAuth 原生认证状态机：消费服务端 WebAuth JSON API，返回 sso-token。
//
// 服务端 handleSsoToken 支持「原始 state 直查」（与外部浏览器模式同一路径），
// 因此凭据类认证 done 后直接以 state 作为 sso-token 提交 auth-reply，
// 不经过 complete_url / cookie / localhost 回调。
// SSO 组（企微/飞书）返回 status=sso + redirect_url，此时才回退 WebView：
// 扫码后 IdP 回调 → 服务端续跑管道 → 302 localhost:29786/api/sso/{state}，
// 令牌仍经现有本地回调回传（SubmitSSOToken 唤醒）。
func (r *realEngine) nativeWebAuth(ctx context.Context) (string, error) {
	state := auth.WebAuthStateParam(auth.Prof.SsoV2Login)
	if state == "" {
		// 取不到 state（异常 URL / 旧版服务端）：回退整页 WebView 流程
		return r.webviewSso(ctx, auth.Prof.SsoV2Login)
	}

	st, err := auth.WebAuthStart(state)
	if err != nil {
		return "", err
	}

	// 整个交互式认证共用一个总超时（组选择 + 输入凭据 + 挑战码）
	deadline := time.NewTimer(10 * time.Minute)
	defer deadline.Stop()

	var lastForm *types.WebAuthForm // 上一次下发的表单，凭据错误时携带错误信息重发

	// 组过滤开关开启时（RequireIdentify=true）：展示组清单前先收集用户名，
	// 按 User.Groups 过滤可选组，缩短清单并避免展示无权限组。
	// 开关关闭（默认）则直接展示全量启用组，保持改造前的默认行为。
	if st.Status == "select_group" && st.RequireIdentify {
		origGroups := st.Groups
		if filtered, ferr := r.doIdentify(ctx, state, deadline); ferr == nil {
			st = filtered
		} else {
			st = &auth.WebAuthState{Status: "select_group", Groups: origGroups}
		}
	}

	for {
		switch st.Status {
		case "done":
			return state, nil

		case "sso":
			u := st.RedirectURL
			if u == "" {
				// 服务端未给出 IdP 跳转地址（如 saml 类型）：回退 WebAuth 整页
				u = auth.Prof.SsoV2Login
			}
			return r.webviewSso(ctx, u)

		case "error":
			// 无表单上下文（start/选组前置失败）或会话已失效：终止
			if lastForm == nil || strings.Contains(st.Message, "过期") {
				if st.Message == "" {
					st.Message = "认证失败"
				}
				return "", errors.New(st.Message)
			}
			// 凭据类错误：原表单就地重发（带错误信息），允许用户重试
			f := *lastForm
			f.Error = st.Message
			emit(r.events, types.Event{Kind: "webauth", WebAuth: &f})
			ans, werr := r.waitAnswers(ctx, deadline)
			if werr != nil {
				return "", werr
			}
			st, err = webAuthSubmit(state, lastForm.Status, ans)

		case "select_group":
			form := &types.WebAuthForm{Status: "select_group", Groups: st.Groups, Hint: st.Hint}
			lastForm = form
			emit(r.events, types.Event{Kind: "webauth", WebAuth: form})
			ans, werr := r.waitAnswers(ctx, deadline)
			if werr != nil {
				return "", werr
			}
			st, err = webAuthSubmit(state, "select_group", ans)

		case "credentials", "otp", "sms", "sms_phone", "radius", "change_pwd":
			form := &types.WebAuthForm{
				Status:       st.Status,
				Hint:         st.Hint,
				Username:     st.Username,
				PhoneMasked:  st.PhoneMasked,
				ChallengeMsg: st.ChallengeMsg,
			}
			lastForm = form
			emit(r.events, types.Event{Kind: "webauth", WebAuth: form})
			ans, werr := r.waitAnswers(ctx, deadline)
			if werr != nil {
				return "", werr
			}
			st, err = webAuthSubmit(state, st.Status, ans)

		default:
			return "", fmt.Errorf("服务端返回未知认证状态: %s", st.Status)
		}
		if err != nil {
			return "", err
		}
	}
}

// webAuthSubmit 按表单类型把 UI 答案提交到对应端点。
func webAuthSubmit(state, formStatus string, ans map[string]string) (*auth.WebAuthState, error) {
	switch formStatus {
	case "select_group":
		return auth.WebAuthSelectGroup(state, ans["group"])
	case "change_pwd":
		return auth.WebAuthChangePassword(state, ans["new_password"], ans["new_password_confirm"])
	default:
		// credentials/otp/sms/sms_phone/radius：键名与服务端 WebAuthStep 对齐
		return auth.WebAuthStep(state, ans)
	}
}

// doIdentify 收集用户名并提交到 /web-auth/identify，取回按权限过滤后的可选组列表。
func (r *realEngine) doIdentify(ctx context.Context, state string, deadline *time.Timer) (*auth.WebAuthState, error) {
	form := &types.WebAuthForm{Status: "identify", Hint: "请输入用户名以加载可用用户组"}
	emit(r.events, types.Event{Kind: "webauth", WebAuth: form})
	ans, werr := r.waitAnswers(ctx, deadline)
	if werr != nil {
		return nil, werr
	}
	username := strings.TrimSpace(ans["username"])
	if username == "" {
		return nil, errors.New("请输入用户名")
	}
	return auth.WebAuthIdentify(state, username)
}

// waitAnswers 阻塞等待 UI 提交表单答案（Submit 写入 answersCh）。
func (r *realEngine) waitAnswers(ctx context.Context, deadline *time.Timer) (map[string]string, error) {
	select {
	case ans := <-r.answersCh:
		return ans, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-deadline.C:
		return nil, errors.New("认证超时，请重新连接")
	}
}

// webviewSso 打开 WebView 完成 SSO（企微/飞书扫码或整页 WebAuth 回退），
// 等待本地回调（localhost:29786）捕获令牌。
func (r *realEngine) webviewSso(ctx context.Context, url string) (string, error) {
	emit(r.events, types.Event{Kind: "sso-login", URL: url})
	ssoWait := time.NewTimer(10 * time.Minute)
	defer ssoWait.Stop()
	select {
	case token := <-r.ssoTokenCh:
		return token, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-ssoWait.C:
		return "", errors.New("WebAuth 登录超时，请重新连接")
	}
}

// startControl 重建控制通道（先停掉上一条，避免重连后双通道并存）。
func (r *realEngine) startControl(ctx context.Context) {
	if r.ctlCancel != nil {
		r.ctlCancel()
	}
	cctx, cancel := context.WithCancel(ctx)
	r.ctlCancel = cancel
	go r.controlLoop(cctx)
}

// emitConnected 下发连接成功状态与隧道信息。
func (r *realEngine) emitConnected() {
	cSess := session.Sess.CSess
	emit(r.events, types.Event{Kind: "status", Status: types.StatusConnected})
	emit(r.events, types.Event{Kind: "log", Message: "隧道已建立，连接成功"})
	if cSess != nil {
		emit(r.events, types.Event{Kind: "tunnel", Tunnel: &types.TunnelInfo{
			IPv4:   cSess.VPNAddress,
			IPv6:   cSess.VPNAddress6,
			DNS:    append(append([]string{}, cSess.DNS...), cSess.DNS6...),
			MTU:    uint32(cSess.MTU),
			Server: r.server,
		}})
	}
}

// watch 监视当前会话：连接异常中断（DPD 超时、网络切换、服务端重启）时，
// 复用已有 session-token 自动重建隧道（无需重新认证），指数退避。
// 状态机收敛（P0）：
//   - 用户主动断开 / 服务端下发断开（ActiveClose）→ 不重连，状态由对应路径下发；
//   - CSTP 收到 0x05 DISCONNECT（ServerClose）→ 会话已被服务端回收，直接「已断开」；
//   - 重连最多 maxRetry 次（约 1 分钟窗口，覆盖网络切换/短暂断网），
//     全部失败后稳定显示「已断开」，不再无限「重连中」掩盖真实状态。
func (r *realEngine) watch(ctx context.Context) {
	// 提前捕获会话引用：CloseChan 触发后 session.Sess.CSess 已被置 nil，
	// 而 ServerClose 兜底路径仍需要它恢复路由/DNS。
	cSess := session.Sess.CSess
	closeChan := session.Sess.CloseChan
	select {
	case <-closeChan:
	case <-ctx.Done():
		return
	}
	if session.Sess.ActiveClose {
		return // 主动断开/服务端指令断开，状态事件由对应路径下发
	}
	if session.Sess.ServerClose {
		// CSTP 层收到服务端 DISCONNECT（无控制通道时的兜底路径）
		if cSess != nil {
			vpnc.ResetRoutes(cSess)
		}
		// 先发 status 再发 error：app 层在 disconnected 时会清空 lastError，
		// 反序会把断开原因抹掉（GetStatus 拉不到）。
		emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
		emit(r.events, types.Event{Kind: "error", Message: "服务端已断开连接"})
		return
	}

	emit(r.events, types.Event{Kind: "status", Status: types.StatusReconnecting})
	emit(r.events, types.Event{Kind: "log", Message: "连接中断，正在自动重连…"})

	// 2+4+8+16+30 ≈ 60s：覆盖网络切换/短暂断网；超过则判定服务端不可达
	const maxRetry = 5
	backoff := 2 * time.Second
	const maxBackoff = 30 * time.Second
	for attempt := 1; attempt <= maxRetry; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		// 重连等待期间控制通道可能已下发断开指令（踢人/停机），此时终止重连，
		// 避免 401 的「会话已过期」文案覆盖真实断开原因。
		if session.Sess.ActiveClose {
			return
		}

		err := r.retryTunnel()
		if err == nil {
			r.emitConnected()
			emit(r.events, types.Event{Kind: "log", Message: "自动重连成功"})
			r.startControl(ctx)
			go r.watch(ctx)
			return
		}

		emit(r.events, types.Event{Kind: "log", Message: fmt.Sprintf("第 %d 次重连失败: %v", attempt, err)})
		// 服务端已回收会话（token 失效返回 401），重试无意义，需用户重新认证
		if strings.Contains(err.Error(), "401") {
			emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
			emit(r.events, types.Event{Kind: "error", Message: "会话已过期，请重新连接"})
			return
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	// 重连额度用尽：稳定显示「已断开」，把决定权交还用户（避免假「重连中」）
	emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
	emit(r.events, types.Event{Kind: "error", Message: fmt.Sprintf("自动重连 %d 次均失败，已断开连接", maxRetry)})
}

// Stats 返回当前隧道实时统计（流量、数据通道、密码套件）；未连接时返回 nil。
func (r *realEngine) Stats() *types.TunnelStats {
	cSess := session.Sess.CSess
	if cSess == nil || cSess.Stat == nil {
		return nil
	}
	st := &types.TunnelStats{
		RxBytes: cSess.Stat.BytesReceived,
		TxBytes: cSess.Stat.BytesSent,
	}
	if cSess.DtlsConnected.Load() {
		st.Channel = "DTLS"
		st.Cipher = cSess.DTLSCipherSuite
	} else {
		st.Channel = "TLS"
		st.Cipher = cSess.TLSCipherSuite
	}
	return st
}

// retryTunnel 用已有 session-token 重建隧道：重新探测出口网卡（网络可能已切换）、
// 重拨 TLS、重新协商 CSTP/DTLS 并重建 TUN。
func (r *realEngine) retryTunnel() error {
	if err := vpnc.GetLocalInterface(); err != nil {
		return err
	}
	if err := auth.Dial(); err != nil {
		return err
	}
	return vpn.SetupTunnel()
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

// Submit 回灌原生认证表单答案（组选择/凭据/挑战码），唤醒 nativeWebAuth 状态机。
func (r *realEngine) Submit(answers map[string]string) error {
	if r.answersCh != nil {
		select {
		case r.answersCh <- answers:
		default:
		}
	}
	return nil
}

func (r *realEngine) Disconnect() error {
	if r.cancel != nil {
		r.cancel()
	}
	// 停止控制通道（随连接生命周期）
	if r.ctlCancel != nil {
		r.ctlCancel()
		r.ctlCancel = nil
	}
	// 先标记主动断开，避免 watch 看护协程误判为异常中断而触发自动重连。
	// ActiveClose 置位后 tun 协程不再兜底恢复路由/DNS，须在此显式恢复。
	session.Sess.ActiveClose = true
	if session.Sess.CSess != nil {
		vpnc.ResetRoutes(session.Sess.CSess)
		session.Sess.CSess.Close()
	}
	if auth.Conn != nil {
		_ = auth.Conn.Close()
	}
	// P1d 断网保护：主动断开时解除防火墙规则，避免把自己锁死
	disableKillSwitchSafe()
	emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
	return nil
}
