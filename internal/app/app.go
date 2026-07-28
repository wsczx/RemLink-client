// Package app 实现客户端与前端（原生引导页）交互的 Wails 服务。
//
// 连接模型（原生认证优先，复用服务端 WebAuth JSON API，零改服务端）：
//   - Connect(raw) 仅校验并持久化服务器地址，随后异步发起连接；
//   - 引擎 InitAuth 拿到 sso-v2-login 后提取 state，直接消费 WebAuth JSON API：
//     组选择、账号密码、OTP/短信/RADIUS 挑战全部原生表单（引擎发 webauth 事件，
//     前端渲染，SubmitAuth 回灌答案），认证完成以 state 为 sso-token 建隧道；
//   - 仅企微/飞书等 SSO 组（status=sso）才打开 WebView 扫码，扫码后服务端把
//     令牌 302 到 http://localhost:29786/api/sso/{token}，由本地回调接住；
//   - 连上后 webview 回到主页（连接详情），右键托盘「进入门户」跳门户。
//
// 全程不经过 SPA 桥接、不依赖 window.go、不需要改服务端。
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"remlink-client/internal/config"
	"remlink-client/internal/engine"
	"remlink-client/internal/types"
)

// ClientService 是绑定到前端的服务实例。
type ClientService struct {
	appApp     *application.App // 应用实例，用于创建门户新窗口
	window     *application.WebviewWindow
	tray       *application.SystemTray // 系统托盘句柄，用于把连接状态/错误同步到托盘 tooltip（可见）
	cfg        *config.AppConfig
	lastBase   string // 最近一次连接的网关地址（https://host）
	vpn        engine.Engine
	portalWin  *application.WebviewWindow // 已打开的门户窗口（避免重复创建）
	webauthWin *application.WebviewWindow // 独立的 WebAuth 登录窗口（与门户同尺寸）

	mu         sync.Mutex
	connecting bool // 防止重复 Connect 触发多次建隧道

	// SSO / 状态
	cbOnce      sync.Once
	portalURL   string           // 服务端回跳的门户地址（用于「进入门户」）
	status      types.ConnStatus // 当前连接状态（供 GetStatus 拉取，弥补 webview 跳转期间错过的事件）
	lastTunnel  *types.TunnelInfo
	connectedAt time.Time
	lastError   string

	// P1a：策略可视化 / 通知中心 / 智能选路
	notifier   *notifications.NotificationService // 系统通知（可为 nil，降级为仅列表）
	lastPolicy *types.PolicyInfo                  // 最近一次 config_push（策略卡片数据源）
	epKey      string                             // 接入点缓存 key（用户输入的 base URL）
}

// New 构造服务并加载本地配置。
func New() *ClientService {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.AppConfig{}
	}
	svc := &ClientService{cfg: cfg, vpn: engine.NewReal()}
	initLogging()
	go svc.forwardEngineEvents()
	return svc
}

// SetWindow 注入主窗口引用（用于断开时复位、SSO 跳转）。
func (s *ClientService) SetWindow(w *application.WebviewWindow) { s.window = w }

// Minimize 最小化主窗口（原生标题栏「–」按钮）。
func (s *ClientService) Minimize() {
	if s.window != nil {
		s.window.Minimise()
	}
}

// HideWindow 隐藏主窗口并收进托盘（原生标题栏「×」按钮，与 OS 关闭行为一致）。
func (s *ClientService) HideWindow() {
	if s.window != nil {
		s.window.Hide()
	}
}

// SetApp 注入应用实例（用于「进入门户」时创建更大的新窗口）。
func (s *ClientService) SetApp(a *application.App) {
	s.appApp = a
}

// SetTray 注入系统托盘句柄，用于把连接状态/错误同步到托盘 tooltip（可见）。
func (s *ClientService) SetTray(t *application.SystemTray) { s.tray = t }

// SetNotifier 注入系统通知服务（main 注册 notifications 服务后调用）。
func (s *ClientService) SetNotifier(n *notifications.NotificationService) { s.notifier = n }

// clientLog 是客户端诊断日志（落盘到用户缓存目录），用于在无控制台的 GUI
// 程序里排查「连接后无反应」类问题。所有事件/错误都会写入此处。
var clientLog *log.Logger

func initLogging() {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, "remlink-client.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		clientLog = log.New(os.Stderr, "[RemLink] ", log.LstdFlags)
		clientLog.Printf("无法打开日志文件 %s: %v（改为 stderr）", path, err)
		return
	}
	clientLog = log.New(f, "[RemLink] ", log.LstdFlags)
	// 构建指纹：每次启动打印，便于确认运行的 exe 是否包含最新代码
	// （例如「独立 WebAuth 窗口」改动）。若日志首行没有此指纹，说明跑的是旧 exe。
	clientLog.Printf("=== RemLink 客户端启动 [build: webauth-portal-framed] 日志文件: %s ===", path)
}

// Brand 返回品牌信息，供原生引导页渲染。
func (s *ClientService) Brand() map[string]any {
	return map[string]any{
		"name":    "RemLink",
		"tagline": "企业级安全远程接入",
		"version": "0.1.0",
		"primary": "#0EA5E9",
	}
}

// GetServers 返回已保存的服务器列表。
func (s *ClientService) GetServers() ([]types.ServerEntry, error) {
	return s.cfg.Servers, nil
}

// LastServer 返回上次连接的服务器地址（https:// 形式）。
func (s *ClientService) LastServer() (string, error) {
	return s.cfg.LastServer, nil
}

// normalizeServer 把用户输入规范化为 https://host 形式。
func normalizeServer(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("服务器地址不能为空")
	}
	raw = strings.TrimSuffix(raw, "/")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("无效的服务器地址: %s", raw)
	}
	return "https://" + u.Host, nil
}

// Connect 校验并保存服务器地址，随后异步发起 SSO 连接流程。
// 真正的身份认证由引擎驱动：在 webview 中打开 WebAuth 登录页完成，
// 令牌经本地回调（localhost:29786）回传后建 CSTP 隧道。
// 不需要改服务端、不经过 SPA 桥接。
func (s *ClientService) Connect(raw string) (string, error) {
	base, err := normalizeServer(raw)
	if err != nil {
		return "", err
	}
	host := strings.TrimPrefix(base, "https://")

	// upsert
	found := false
	for i := range s.cfg.Servers {
		if s.cfg.Servers[i].Host == base {
			s.cfg.Servers[i].Name = host
			found = true
			break
		}
	}
	if !found {
		s.cfg.Servers = append(s.cfg.Servers, types.ServerEntry{Name: host, Host: base})
	}
	s.cfg.LastServer = base
	if err := s.cfg.Save(); err != nil {
		return "", err
	}
	s.lastBase = base
	if clientLog != nil {
		clientLog.Printf("Connect: lastBase=%s", base)
	}

	// 防止重复 Connect 触发多次建隧道 / 引擎状态错乱：
	// 已有连接在进行中则忽略本次点击（按钮本就处于禁用态）。
	s.mu.Lock()
	if s.connecting {
		s.mu.Unlock()
		return base, nil
	}
	s.connecting = true
	s.mu.Unlock()

	// 启动 SSO 本地回调（localhost:29786），并异步发起连接。
	s.ensureSsoCallback()
	s.epKey = base
	go func() {
		defer func() {
			s.mu.Lock()
			s.connecting = false
			s.mu.Unlock()
		}()
		// 智能选路：对「用户地址 + 服务端下发的接入点缓存」并发测速择优。
		// 无缓存（首连/单入口部署）时直连用户地址，行为不变。
		target := s.pickEndpoint(base)
		s.lastBase = target
		_ = s.vpn.Connect(context.Background(), &engine.Config{Server: target})
	}()
	return base, nil
}

// pickEndpoint 智能选路：候选 = 用户地址 + 缓存的服务端下发接入点，
// 并发 TCP 测延迟选最快；全部失败时回退用户地址（交由连接流程报错）。
func (s *ClientService) pickEndpoint(base string) string {
	cached := s.cfg.CachedEndpoints[base]
	if len(cached) == 0 {
		return base
	}
	// 候选去重（host[:port] 归一化为 https://host[:port]）
	seen := map[string]bool{base: true}
	candidates := []string{base}
	for _, ep := range cached {
		b, err := normalizeServer(ep)
		if err != nil || seen[b] {
			continue
		}
		seen[b] = true
		candidates = append(candidates, b)
	}
	if len(candidates) == 1 {
		return base
	}

	type result struct {
		base string
		rtt  time.Duration
	}
	ch := make(chan result, len(candidates))
	for _, c := range candidates {
		go func(b string) {
			host := strings.TrimPrefix(b, "https://")
			if _, _, err := net.SplitHostPort(host); err != nil {
				host = net.JoinHostPort(host, "443")
			}
			start := time.Now()
			conn, err := net.DialTimeout("tcp", host, 3*time.Second)
			if err != nil {
				ch <- result{b, -1}
				return
			}
			_ = conn.Close()
			ch <- result{b, time.Since(start)}
		}(c)
	}
	best := result{base, -1}
	for range candidates {
		r := <-ch
		if r.rtt < 0 {
			continue
		}
		if best.rtt < 0 || r.rtt < best.rtt {
			best = r
		}
	}
	if best.rtt < 0 {
		return base // 全部不可达：回退用户地址，由连接流程给出明确错误
	}
	if clientLog != nil {
		clientLog.Printf("智能选路: 候选 %d 个，选中 %s (%s)", len(candidates), best.base, best.rtt.Round(time.Millisecond))
	}
	if best.base != base {
		s.handleEvent(types.Event{Kind: "log",
			Message: "智能选路: 已选择最快接入点 " + strings.TrimPrefix(best.base, "https://") +
				" (" + strconv.FormatInt(best.rtt.Milliseconds(), 10) + "ms)"})
	}
	return best.base
}

// OpenPortal 在新窗口中打开门户（门户信息多、需要更大窗口）。
// 复用同一 WebView 引擎的数据目录（cookie 仍生效，自动认证）；
// 若门户窗口已存在则直接聚焦，避免重复创建。
func (s *ClientService) OpenPortal() error {
	if s.appApp == nil {
		return errors.New("应用尚未就绪")
	}
	u := s.portalURL
	if u == "" {
		u = s.lastBase + "/ui/#/portal"
	}
	// 已打开则聚焦
	if s.portalWin != nil {
		s.portalWin.Show()
		s.portalWin.Focus()
		return nil
	}
	w := s.appApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "RemLink 门户",
		BackgroundColour: application.NewRGB(27, 33, 56),
		Width:            1000,
		Height:           720,
		MinWidth:         820,
		MinHeight:        600,
	})
	s.portalWin = w
	// 关闭后清空引用，下次「进入门户」重新创建
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s.portalWin = nil
	})
	w.Show()
	// 用 SetURL 加载外部门户地址，避免被全局 AssetServer Middleware 拦截返回空白。
	w.SetURL(u)
	return nil
}

// openWebAuthWindow 在独立窗口中打开 WebAuth 登录页（与门户同尺寸 1000×720）。
// 不复用主窗口（主窗口仅 420×700，登录页会拥挤），也不走系统浏览器。
// 该窗口与门户共用同一 WebView 数据目录，认证产生的 webvpn/session cookie
// 在窗口提升为门户时仍生效，门户可自动认证、无需再次登录。
func (s *ClientService) openWebAuthWindow(u string) {
	if clientLog != nil {
		clientLog.Printf("openWebAuthWindow: url=%s appAppNil=%v", u, s.appApp == nil)
	}
	// 优先用注入的 app；若未注入（极端情况），尝试从全局获取，避免退回主窗口。
	app := s.appApp
	if app == nil {
		if g := application.Get(); g != nil {
			app = g
			s.appApp = g // 缓存，后续复用
			if clientLog != nil {
				clientLog.Printf("openWebAuthWindow: appApp 为 nil，已用 application.Get() 兜底")
			}
		}
	}
	if app == nil {
		// 真正兜底：应用未就绪时退回主窗口加载（保持旧行为）。
		if clientLog != nil {
			clientLog.Printf("openWebAuthWindow: 无可用 app 实例，退回主窗口加载")
		}
		if s.window != nil {
			s.window.SetURL(u)
		}
		return
	}
	if s.webauthWin != nil {
		s.webauthWin.SetURL(u)
		s.webauthWin.Show()
		s.webauthWin.Focus()
		return
	}
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "RemLink 登录",
		BackgroundColour: application.NewRGB(27, 33, 56),
		Width:            1000,
		Height:           720,
		MinWidth:         820,
		MinHeight:        600,
	})
	s.webauthWin = w
	if clientLog != nil {
		clientLog.Printf("openWebAuthWindow: 已创建独立登录窗口 webauthWin=%v url=%s", w != nil, u)
	}
	// 关闭时清空引用；若连接尚未成功（用户主动关掉登录窗、未完成 SSO），
	// 取消进行中的连接，使主窗口复位到「可重新连接」状态，避免卡在「连接中」。
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s.mu.Lock()
		connected := s.status == types.StatusConnected
		s.mu.Unlock()
		s.webauthWin = nil
		s.portalWin = nil
		if !connected {
			s.Disconnect()
		}
	})
	w.Show()
	// 关键：用 SetURL 而非 NewWithOptions.URL 加载外部 SSO 地址。
	// SetURL 会经 assetserver.GetStartURL 正确处理外部 https URL；
	// 若直接用 NewWithOptions.URL，外部地址会被全局 AssetServer Middleware
	// 拦截、交给引导页 handler 返回空白页。
	w.SetURL(u)
	w.Focus()
}

// promoteWebAuthToPortal 连接成功后，把登录窗口原地提升为门户窗口：
// 直接在同一 WebView 内跳转门户地址（cookie 连续，自动认证），避免再开新窗口。
// 若登录窗口已被用户手动关闭（webauthWin==nil），则不做处理，
// 用户可稍后从托盘「进入门户」重新打开。
func (s *ClientService) promoteWebAuthToPortal() {
	s.mu.Lock()
	url := s.portalURL
	s.mu.Unlock()
	if url == "" {
		url = s.lastBase + "/ui/#/portal"
	}
	if s.webauthWin == nil {
		return
	}
	s.portalWin = s.webauthWin
	s.webauthWin = nil
	s.portalWin.SetTitle("RemLink 门户")
	// 同 WebView 内 cookie 连续、自动认证，门户无需再次登录。
	s.portalWin.SetURL(url)
	s.portalWin.Focus()
}

// closeWebAuthWindow 断开连接时关闭仍打开的登录窗口，避免残留。
func (s *ClientService) closeWebAuthWindow() {
	if s.webauthWin != nil {
		s.webauthWin.Close()
		s.webauthWin = nil
	}
}

// GetStatus 返回当前连接状态，供主页在 webview 跳转后拉取（弥补错过的事件）。
func (s *ClientService) GetStatus() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := map[string]any{
		"status": s.status.String(),
		"server": s.lastBase,
	}
	if s.lastTunnel != nil {
		m["tunnel"] = s.lastTunnel
	}
	if s.status == types.StatusConnected && !s.connectedAt.IsZero() {
		m["duration_secs"] = uint64(time.Since(s.connectedAt).Seconds())
	}
	if (s.status == types.StatusError || s.status == types.StatusDisconnected) && s.lastError != "" {
		m["error"] = s.lastError
	}
	return m
}

// GetStats 返回当前隧道实时统计（流量/数据通道/密码套件），未连接返回 nil。
// 前端在连接详情页轮询调用。
func (s *ClientService) GetStats() *types.TunnelStats {
	if s.vpn == nil {
		return nil
	}
	return s.vpn.Stats()
}

// GetPolicy 返回最近一次服务端下发的策略快照（策略卡片数据源），未下发返回 nil。
func (s *ClientService) GetPolicy() *types.PolicyInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPolicy
}

// GetNotices 返回通知中心列表（最新在前，最多 50 条）。
func (s *ClientService) GetNotices() []types.Notice {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Notices
}

// ClearNotices 清空通知中心。
func (s *ClientService) ClearNotices() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Notices = nil
	return s.cfg.Save()
}

// RemoveServer 删除一个已保存的服务器。
func (s *ClientService) RemoveServer(raw string) error {
	base, err := normalizeServer(raw)
	if err != nil {
		return err
	}
	kept := s.cfg.Servers[:0]
	for _, srv := range s.cfg.Servers {
		if srv.Host != base {
			kept = append(kept, srv)
		}
	}
	s.cfg.Servers = kept
	if s.cfg.LastServer == base {
		s.cfg.LastServer = ""
	}
	return s.cfg.Save()
}

// SubmitAuth 提交原生认证表单答案（组选择/账号密码/OTP/短信/挑战码/改密），
// 由前端认证卡片调用，唤醒引擎中阻塞等待的 WebAuth 状态机。
func (s *ClientService) SubmitAuth(answers map[string]string) error {
	if s.vpn == nil {
		return errors.New("引擎未就绪")
	}
	return s.vpn.Submit(answers)
}

// Disconnect 断开连接并复位回原生主页（真实隧道释放由引擎负责）。
func (s *ClientService) Disconnect() error {
	if s.vpn != nil {
		_ = s.vpn.Disconnect()
	}
	if s.window != nil {
		s.window.SetURL("/")
	}
	return nil
}

// RunDiagnostics 本地网络诊断（工具页“运行网络诊断”调用），需已建立隧道。
func (s *ClientService) RunDiagnostics() (map[string]any, error) {
	if s.vpn == nil {
		return nil, errors.New("引擎未就绪")
	}
	return s.vpn.RunDiagnostics()
}

// forwardEngineEvents 把隧道引擎事件转发到前端与本地处理。
func (s *ClientService) forwardEngineEvents() {
	for e := range s.vpn.Events() {
		s.handleEvent(e)
	}
}

// handleEvent 统一处理一个事件：① 落盘诊断日志；② 更新本地状态；
// ③ SSO 登录页在 webview 中打开（非系统浏览器）；④ 连接成功/失败/断开
//
//	回到客户端主页；⑤ 推到前端（window.go.events.on("event", cb) 接收）；
//
// ⑥ 同步到系统托盘 tooltip，让连接状态/失败原因在 GUI 程序里可见。
func (s *ClientService) handleEvent(e types.Event) {
	// 更新本地状态（供 GetStatus 拉取，弥补 webview 跳转期间错过的事件）
	s.mu.Lock()
	switch e.Kind {
	case "status":
		s.status = e.Status
		if e.Status == types.StatusConnected {
			s.connectedAt = time.Now()
			s.lastError = ""
		}
		if e.Status == types.StatusDisconnected || e.Status == types.StatusError {
			s.connectedAt = time.Time{}
			s.lastPolicy = nil // 策略随会话失效，避免详情页残留旧策略
			if e.Kind == "status" && e.Status == types.StatusDisconnected {
				s.lastError = ""
			}
		}
		// 连接进入终态（成功/失败/断开）后解除 connecting 锁，允许再次连接。
		if e.Status == types.StatusConnected || e.Status == types.StatusError || e.Status == types.StatusDisconnected {
			s.connecting = false
		}
	case "tunnel":
		if e.Tunnel != nil {
			s.lastTunnel = e.Tunnel
		}
	case "error":
		s.lastError = e.Message
	case "policy":
		if e.Policy != nil {
			s.lastPolicy = e.Policy
			// 缓存服务端下发的接入点列表（下次连接前测速择优）
			if s.epKey != "" {
				if s.cfg.CachedEndpoints == nil {
					s.cfg.CachedEndpoints = map[string][]string{}
				}
				// 空列表也覆盖：管理员清空配置后客户端同步回单入口
				s.cfg.CachedEndpoints[s.epKey] = e.Policy.Endpoints
				_ = s.cfg.Save()
			}
		}
	case "notice":
		if e.Notice != nil {
			// 通知中心：头插保留最近 50 条并持久化
			s.cfg.Notices = append([]types.Notice{*e.Notice}, s.cfg.Notices...)
			if len(s.cfg.Notices) > 50 {
				s.cfg.Notices = s.cfg.Notices[:50]
			}
			_ = s.cfg.Save()
		}
	}
	s.mu.Unlock()

	// 系统通知（列表持久化之外的即时提醒；服务未注入时静默降级）
	if e.Kind == "notice" && e.Notice != nil && s.notifier != nil {
		title := "RemLink 消息"
		if e.Notice.Level == "warn" || e.Notice.Level == "error" {
			title = "RemLink 警告"
		}
		_ = s.notifier.SendNotification(notifications.NotificationOptions{
			ID:    "remlink-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			Title: title,
			Body:  e.Notice.Text,
		})
	}

	if clientLog != nil {
		clientLog.Printf("event kind=%s status=%s msg=%s", e.Kind, e.Status, e.Message)
	}

	// SSO 登录页：在独立的「登录窗口」中打开（与门户同尺寸 1000×720，
	// 非系统浏览器、不经过 SPA 桥接）。认证成功后该窗口会被原地提升为门户窗口。
	if e.Kind == "sso-login" && e.URL != "" {
		if clientLog != nil {
			clientLog.Printf("handleEvent: 收到 sso-login 事件，准备开独立窗口 url=%s", e.URL)
		}
		// 弹出独立「登录窗口」加载 SSO 页（与门户同尺寸 1000×720，非系统浏览器、
		// 不经过 SPA 桥接）。认证成功后该窗口会被原地提升为门户窗口。
		// 当前主窗口保持 SPA 不变，避免白屏突兀。
		s.openWebAuthWindow(e.URL)
	}
	// 连接状态变化：主窗口回到连接详情页；连接成功时把登录窗口提升为门户。
	if e.Kind == "status" && s.window != nil {
		switch e.Status {
		case types.StatusConnected:
			s.window.SetURL("/")
			s.promoteWebAuthToPortal()
		case types.StatusError:
			// 出错时主窗口回到首页展示错误，并关闭登录窗口——
			// 否则用户会一直卡在「正在建立安全连接」页（该页由本地 SSO 回调返回，
			// 停在 webauth 窗口上），真实的「认证/隧道建立失败」原因被挡在背后。
			s.window.SetURL("/")
			s.closeWebAuthWindow()
		case types.StatusDisconnected:
			s.window.SetURL("/")
			s.closeWebAuthWindow()
		}
	}

	if app := application.Get(); app != nil {
		app.Event.Emit("event", e)
	}

	if s.tray != nil {
		switch e.Kind {
		case "error":
			s.tray.SetTooltip("RemLink：连接失败")
		case "status":
			switch e.Status {
			case types.StatusConnected:
				s.tray.SetTooltip("RemLink：已连接 " + s.lastBase)
			case types.StatusConnecting:
				s.tray.SetTooltip("RemLink：连接中…")
			case types.StatusReconnecting:
				s.tray.SetTooltip("RemLink：重连中…")
			case types.StatusDisconnected:
				s.tray.SetTooltip("RemLink：未连接")
			}
		}
	}
}
