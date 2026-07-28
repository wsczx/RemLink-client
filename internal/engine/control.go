// 控制通道客户端（RemLink 专属）。
//
// 与 CSTP 数据隧道并行的 WebSocket 长连（wss://<host>/api/client/ctl），
// 是「连接状态 + 服务端可控」的权威通道：
//   - 客户端每 15s 上报 heartbeat（状态/数据通道/流量/能力声明）；
//   - 服务端可下发 disconnect（踢人/停机/过期/配额，秒级感知）与 message（管理员消息）。
//
// 兼容性：老服务端（无 /api/client/ctl 端点）握手失败时静默降级为纯 CSTP DPD
// 行为，不影响连接；控制通道存活期间意外掉线则视为失去管控——短暂重试后
// 主动拆 CSTP，交由 watch() 自动重连（避免孤儿隧道）。
package engine

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"

	"sslcon/auth"
	"sslcon/session"
	"sslcon/utils/vpnc"

	"remlink-client/internal/types"
)

// ctlCommand 服务端→客户端命令
type ctlCommand struct {
	Type   string `json:"type"`             // disconnect | message | config_push | device_challenge | diag_request
	Reason string `json:"reason,omitempty"` // disconnect 原因码
	Level  string `json:"level,omitempty"`  // message: info|warn|error
	Text   string `json:"text,omitempty"`   // message 正文
	ID     string `json:"id,omitempty"`     // device_challenge nonce 标识 / diag_request 请求 id
	Nonce  string `json:"nonce,omitempty"`  // device_challenge: 服务端下发的随机挑战值
}

// ctlUp 客户端→服务端上行回执（device_proof / diag_result）
type ctlUp struct {
	Type    string                 `json:"type"`
	ID      string                 `json:"id,omitempty"`
	Nonce   string                 `json:"nonce,omitempty"`
	PubKey  string                 `json:"pubkey,omitempty"` // device_proof: 设备公钥(hex)
	Sig     string                 `json:"sig,omitempty"`    // device_proof: 对 nonce 的 Ed25519 签名(hex)
	Results map[string]interface{} `json:"results,omitempty"` // diag_result: 各项诊断结果
}

// ctlHeartbeat 客户端→服务端心跳
type ctlHeartbeat struct {
	Type    string   `json:"type"`
	Status  string   `json:"status"`
	Channel string   `json:"channel"`
	Rx      uint64   `json:"rx"`
	Tx      uint64   `json:"tx"`
	Caps    []string `json:"caps"`
}

const ctlHeartbeatInterval = 15 * time.Second

// ctlReasonText 服务端断开原因码 → 用户可读文案
func ctlReasonText(reason string) string {
	switch reason {
	case "admin_kick":
		return "已被管理员断开连接"
	case "server_shutdown":
		return "服务端已停止服务"
	case "session_expired":
		return "会话已过期，请重新连接"
	case "account_expired":
		return "账号已过期"
	case "idle_timeout":
		return "连接空闲超时，已断开"
	case "quota_exceeded":
		return "流量配额已用尽，已断开"
	default:
		return "服务端已断开连接"
	}
}

// ctlDial 建立控制通道 WebSocket（session-token 认证，跳过证书校验与主隧道一致）。
func ctlDial(ctx context.Context) (*websocket.Conn, error) {
	u := "wss://" + auth.Prof.HostWithPort + "/api/client/ctl"
	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 10 * time.Second,
	}
	hdr := http.Header{}
	hdr.Set("X-RemLink-Token", session.Sess.SessionToken)
	conn, resp, err := websocket.Dial(ctx, u, &websocket.DialOptions{
		HTTPClient: hc,
		HTTPHeader: hdr,
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return conn, err
}

// controlLoop 维护控制通道：拨号 → 心跳 + 命令分发；由 startControl 启动，
// ctx 取消（Disconnect/重建）时退出。
func (r *realEngine) controlLoop(ctx context.Context) {
	conn, err := ctlDial(ctx)
	if err != nil {
		// 老服务端无此端点：降级为纯 CSTP DPD 行为（不影响隧道）
		emit(r.events, types.Event{Kind: "log", Message: "控制通道不可用（可能是旧版服务端），状态感知降级为 DPD"})
		return
	}
	emit(r.events, types.Event{Kind: "log", Message: "控制通道已建立（服务端可实时管控）"})

	for {
		kicked := r.ctlSession(ctx, conn)
		_ = conn.CloseNow()
		if kicked || ctx.Err() != nil {
			return
		}

		// 控制通道意外掉线（隧道可能仍在）：短暂重试，约一个心跳周期
		redialed := false
		for i := 0; i < 3; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			if conn, err = ctlDial(ctx); err == nil {
				redialed = true
				emit(r.events, types.Event{Kind: "log", Message: "控制通道已恢复"})
				break
			}
		}
		if redialed {
			continue
		}

		// 重试失败：视为失去服务端管控，主动拆 CSTP 避免孤儿隧道；
		// watch() 会接管自动重连（成功后重建控制通道）。
		// ctx 已取消说明本通道已被新连接替换/主动断开，勿误杀新会话。
		if ctx.Err() != nil {
			return
		}
		emit(r.events, types.Event{Kind: "log", Message: "控制通道失联，正在重置连接…"})
		if cSess := session.Sess.CSess; cSess != nil {
			cSess.Close()
		}
		return
	}
}

// ctlSession 在一条已建立的控制通道上跑心跳与命令分发；
// 返回 true 表示收到服务端 disconnect（已执行断开），false 表示通道掉线。
func (r *realEngine) ctlSession(ctx context.Context, conn *websocket.Conn) bool {
	// 接入后上报设备公钥（P1b）：供服务端 device_challenge 校验用。
	if pubHex, err := devicePubKeyHex(); err == nil {
		r.ctlSendUp(ctx, conn, ctlUp{Type: "device_register", PubKey: pubHex})
	}
	// 心跳协程：写失败即退出（读循环会随之出错返回）
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go func() {
		ticker := time.NewTicker(ctlHeartbeatInterval)
		defer ticker.Stop()
		for {
			hb := ctlHeartbeat{Type: "heartbeat", Status: "connected", Caps: r.buildCaps()}
			if st := r.Stats(); st != nil {
				hb.Channel = st.Channel
				hb.Rx = st.RxBytes
				hb.Tx = st.TxBytes
			}
			data, _ := json.Marshal(hb)
			wctx, cancel := context.WithTimeout(hbCtx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
			select {
			case <-ticker.C:
			case <-hbCtx.Done():
				return
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return false // 通道掉线（或 ctx 取消），由 controlLoop 决定重试/拆隧道
		}
		var cmd ctlCommand
		if json.Unmarshal(data, &cmd) != nil {
			continue
		}
		switch cmd.Type {
		case "disconnect":
			r.onServerDisconnect(cmd.Reason)
			return true
		case "message":
			// 走通知中心（app 层负责系统通知 + 列表持久化 + UI 角标）
			level := cmd.Level
			if level == "" {
				level = "info"
			}
			emit(r.events, types.Event{Kind: "notice", Notice: &types.Notice{
				Level: level, Text: cmd.Text, Time: time.Now().Unix(),
			}})
		case "config_push":
			// 策略快照：策略可视化 + 智能选路候选（app 层缓存 endpoints）
		var pi types.PolicyInfo
		if json.Unmarshal(data, &pi) == nil {
			emit(r.events, types.Event{Kind: "policy", Policy: &pi})
			// P1d 断网保护：按服务端策略启用/关闭
			r.applyKillSwitch(pi.Policy.KillSwitch)
		}
		case "diag_request":
			// 网络诊断：服务端请求自助排障，客户端本地执行后回传 diag_result。
			res, err := r.runDiagnostics()
			if err != nil {
				res = map[string]interface{}{"error": err.Error()}
			}
			r.ctlSendUp(ctx, conn, ctlUp{Type: "diag_result", ID: cmd.ID, Results: res})
			emit(r.events, types.Event{Kind: "log", Message: "已执行网络诊断并上报"})
		case "device_challenge":
			// 设备持续信任挑战（P1b）：用设备私钥签名 nonce 回 device_proof。
			pubHex, sigHex, err := signDeviceChallenge(cmd.Nonce)
			if err != nil {
				emit(r.events, types.Event{Kind: "log", Message: "设备挑战签名失败: " + err.Error()})
				break
			}
			r.ctlSendUp(ctx, conn, ctlUp{
				Type:   "device_proof",
				ID:     cmd.ID,
				Nonce:  cmd.Nonce,
				PubKey: pubHex,
				Sig:    sigHex,
			})
			emit(r.events, types.Event{Kind: "log", Message: "已回应设备挑战（持续信任证明）"})
		}
	}
}

// ctlSendUp 经控制通道上行一条回执消息（device_proof / diag_result）。
func (r *realEngine) ctlSendUp(ctx context.Context, conn *websocket.Conn, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = conn.Write(wctx, websocket.MessageText, data)
}

// runDiagnostics 本地自助排障：网关可达性 / DNS / MTU / 路由概况。
// 纯标准库实现（TCP 探测替代 ICMP，免 root），真机调优可换 ICMP echo。
func (r *realEngine) runDiagnostics() (map[string]interface{}, error) {
	cSess := session.Sess.CSess
	if cSess == nil {
		return nil, errors.New("隧道尚未建立")
	}
	res := map[string]interface{}{}
	gw := cSess.ServerAddress
	host := gw
	port := "443"
	if h, p, err := net.SplitHostPort(gw); err == nil {
		host = h
		if p != "" {
			port = p
		}
	}
	gwOK := false
	if host != "" {
		if c, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second); err == nil {
			_ = c.Close()
			gwOK = true
		}
	}
	res["gw_ping"] = gwOK
	res["gw_addr"] = gw
	res["vpn_addr"] = cSess.VPNAddress
	if cSess.VPNAddress6 != "" {
		res["vpn_addr6"] = cSess.VPNAddress6
	}
	res["mtu"] = cSess.MTU
	// DNS 可用性（走系统解析器，作为「DNS 通道」指标）
	dnsOK := true
	if len(cSess.DNS) > 0 {
		res["dns_server"] = cSess.DNS[0]
		if _, err := net.LookupHost("www.baidu.com"); err != nil {
			dnsOK = false
		}
	}
	res["dns"] = dnsOK
	res["routes_include"] = len(cSess.SplitInclude) + len(cSess.SplitInclude6)
	res["routes_exclude"] = len(cSess.SplitExclude) + len(cSess.SplitExclude6)
	return res, nil
}

// onServerDisconnect 处理服务端下发的断开命令：拆隧道、恢复路由，
// 并向 UI 显式下发「已断开(原因)」；不触发自动重连（会话已被服务端回收）。
func (r *realEngine) onServerDisconnect(reason string) {
	msg := ctlReasonText(reason)
	emit(r.events, types.Event{Kind: "log", Message: "收到服务端断开指令: " + reason})

	// 先标记，阻止 watch() 把服务端主动断开误判为异常中断而自动重连
	session.Sess.ActiveClose = true
	session.Sess.ServerClose = true
	if cSess := session.Sess.CSess; cSess != nil {
		vpnc.ResetRoutes(cSess)
		cSess.Close()
	}
	if auth.Conn != nil {
		_ = auth.Conn.Close()
	}
	// 先发 status 再发 error：app 层在 disconnected 时会清空 lastError，
	// 反序会把断开原因抹掉（GetStatus 拉不到）。
	emit(r.events, types.Event{Kind: "notice", Notice: &types.Notice{
		Level: "error", Text: msg, Time: time.Now().Unix(),
	}})
	emit(r.events, types.Event{Kind: "status", Status: types.StatusDisconnected})
	emit(r.events, types.Event{Kind: "error", Message: msg})
	// P1d 断网保护：断开时务必解除防火墙规则，避免把自己锁死
	disableKillSwitchSafe()
}

// buildCaps 根据当前隧道实际能力动态构造能力声明，替代写死的 ["ipv6"]。
// 仅声明客户端确已实现的项，避免服务端据未实现的能力（compression/kill-switch/tap）误判。
func (r *realEngine) buildCaps() []string {
	var caps []string
	cSess := session.Sess.CSess
	if cSess == nil {
		return caps
	}
	if cSess.VPNAddress6 != "" {
		caps = append(caps, "ipv6")
	}
	if len(cSess.DNS) > 0 || len(cSess.DNS6) > 0 {
		caps = append(caps, "split-dns")
	}
	return caps
}

// RunDiagnostics 暴露本地自助排障给前端（工具页“网络诊断”调用）。
func (r *realEngine) RunDiagnostics() (map[string]any, error) {
	return r.runDiagnostics()
}

// applyKillSwitch 按服务端策略启用/关闭断网保护（P1d）。
func (r *realEngine) applyKillSwitch(on bool) {
	if session.Sess.CSess == nil {
		return
	}
	host := auth.Prof.HostWithPort
	port := 443
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if pp, e := strconv.Atoi(p); e == nil {
			port = pp
		}
	}
	if !on {
		disableKillSwitchSafe()
		return
	}
	if err := EnableKillSwitch(host, port); err != nil {
		emit(r.events, types.Event{Kind: "log", Message: "断网保护启用失败: " + err.Error()})
		return
	}
	emit(r.events, types.Event{Kind: "log", Message: "断网保护已启用（断连窗口阻断明文出网）"})
}

// disableKillSwitchSafe 安全解除断网保护（recover 包裹，跨平台实现差异不致命）。
func disableKillSwitchSafe() {
	defer func() { _ = recover() }()
	_ = DisableKillSwitch()
}
