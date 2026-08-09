package app

import (
	"net"
	"net/http"
	"strings"
)

// ensureSsoCallback 启动 SSO 本地回调服务器（监听 127.0.0.1:29786 与 [::1]:29786），
// 接住服务端 SAMLACLogin 302 过来的令牌：http://localhost:29786/api/sso/{token}?return=<门户>。
//
// 这是原生 AnyConnect 的 SSO 令牌回传机制：WebAuth 登录完成后，服务端把令牌
// 拼进 localhost:29786 的 URL 让客户端本地进程接收——无需改服务端、无需 SPA 桥接、
// 不依赖 window.go。客户端捕获令牌后唤醒引擎，用 sso-token 换 session-token 建隧道。
func (s *ClientService) ensureSsoCallback() {
	s.cbOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/sso/", s.handleSsoCallback)
		for _, addr := range []string{"127.0.0.1:29786", "[::1]:29786"} {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				if clientLog != nil {
					clientLog.Printf("SSO 回调监听 %s 失败: %v", addr, err)
				}
				continue
			}
			go func(l net.Listener) {
				_ = http.Serve(l, mux)
			}(ln)
		}
	})
}

// handleSsoCallback 处理 /api/sso/{token}：提取令牌，唤醒引擎握手，并回一个友好页面。
func (s *ClientService) handleSsoCallback(w http.ResponseWriter, r *http.Request) {
	// 路径形如 /api/sso/{token}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	token := ""
	if len(parts) >= 3 {
		token = parts[2]
	}
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	// 记录门户回跳地址（用于「进入门户」）。
	if ret := r.URL.Query().Get("return"); ret != "" {
		s.mu.Lock()
		s.portalURL = ret
		s.mu.Unlock()
	}

	// 唤醒阻塞在连接状态机中的 SSO 握手（machine 经 transport 转交引擎 SubmitSSOToken）。
	if s.machine != nil {
		s.machine.SubmitSSOToken(token)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<html><body style="font-family:-apple-system,Segoe UI,Roboto,sans-serif;text-align:center;padding-top:90px;background:#1b2138;color:#e2e8f0"><h2>认证成功</h2><p>正在建立安全连接，请返回客户端…</p></body></html>`))
}
