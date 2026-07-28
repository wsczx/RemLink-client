package auth

// WebAuth JSON API 客户端（专属客户端原生认证）。
//
// 服务端 WebAuth 端点（link_webauth.go）本为 SPA 设计，返回结构化 JSON；
// 专属客户端直接消费同一套 API：组选择、账号密码、OTP/短信/RADIUS 挑战
// 全部原生 UI 完成，零 WebView；仅企微/飞书等 SSO 组（status=sso）才需要
// 打开 WebView 完成扫码。
//
// 完整链路：
//   InitAuth 拿 sso-v2-login → 提取 state → WebAuthStart 取组列表 →
//   WebAuthSelectGroup 得该组认证方式 → WebAuthStep 提交凭据/验证码 →
//   status=done 后直接以 state 为 <sso-token> 提交 auth-reply。
//   服务端 handleSsoToken 支持原始 state 直查（外部浏览器模式同款路径），
//   因此原生流程无需经过 complete_url/cookie/localhost 回调。

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"sslcon/base"
)

// WebAuthState 服务端 WebAuth JSON 响应（字段并集，按 Status 取用）。
type WebAuthState struct {
	Status       string   `json:"status"`        // select_group|credentials|sms_phone|otp|sms|radius|change_pwd|sso|done|error
	Message      string   `json:"message"`       // error：错误原因
	Groups       []string `json:"groups"`        // select_group：可选组列表
	Hint         string   `json:"hint"`          // 输入提示
	Username     string   `json:"username"`      // credentials：预填用户名
	PhoneMasked  string   `json:"phone_masked"`  // sms：脱敏手机号
	ChallengeMsg string   `json:"challenge_msg"` // radius：挑战提示
	SsoType      string   `json:"sso_type"`      // sso：wxwork|feishu|saml
	RedirectURL  string   `json:"redirect_url"`  // sso：IdP 跳转地址（WebView 打开）
	CompleteURL  string   `json:"complete_url"`  // done：完成端点（原生流程不使用）
}

// WebAuthStateParam 从 sso-v2-login URL（/+CSCOE+/web-auth/sp/login?state=XXX）
// 提取 state。取不到返回空串，调用方据此回退整页 WebView 流程。
func WebAuthStateParam(ssoV2Login string) string {
	u, err := url.Parse(ssoV2Login)
	if err != nil {
		return ""
	}
	return u.Query().Get("state")
}

// WebAuthStart 开始认证：证书组直接进入管道，否则返回组列表（select_group）。
func WebAuthStart(state string) (*WebAuthState, error) {
	return webAuthDo(http.MethodGet, "/web-auth/start", state, nil)
}

// WebAuthSelectGroup 选定组，返回该组首步认证方式（credentials/sms_phone/sso/…）。
func WebAuthSelectGroup(state, group string) (*WebAuthState, error) {
	return webAuthDo(http.MethodPost, "/web-auth/groups", state, map[string]string{"group": group})
}

// WebAuthStep 提交凭据或挑战码，推进认证管道。
// payload 键：username / password / phone / otp_code / sms_code（按需提交）。
func WebAuthStep(state string, payload map[string]string) (*WebAuthState, error) {
	return webAuthDo(http.MethodPost, "/web-auth/step", state, payload)
}

// WebAuthChangePassword 强制改密提交，改密成功后管道续跑（可能进入 otp 等后续步骤）。
func WebAuthChangePassword(state, newPwd, confirm string) (*WebAuthState, error) {
	return webAuthDo(http.MethodPost, "/web-auth/change_password", state,
		map[string]string{"new_password": newPwd, "new_password_confirm": confirm})
}

// webAuthDo 执行一次 WebAuth JSON 请求。独立短连接（非聚合认证的裸 TLS 长连），
// 与 SPA 浏览器调用同构，服务端无需任何改动。
func webAuthDo(method, path, state string, payload map[string]string) (*WebAuthState, error) {
	u := fmt.Sprintf("%s%s%s?state=%s", Prof.Scheme, Prof.HostWithPort, path, url.QueryEscape(state))

	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: base.Cfg.InsecureSkipVerify},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webauth %s: %s", path, resp.Status)
	}

	st := new(WebAuthState)
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("webauth %s: 响应解析失败: %w", path, err)
	}
	return st, nil
}
