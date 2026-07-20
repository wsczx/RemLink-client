// Package webauth 实现 RemLink 服务端 WebAuth 多步 JSON 认证管道。
//
// 协议契约见服务端 handler/link_webauth.go。整体流程：
//
//	start        GET  /+CSCOE+/web-auth/start?state=...
//	select_group POST /+CSCOE+/web-auth/select_group?state=...
//	step         POST /+CSCOE+/web-auth/step?state=...
//	continue     GET  /+CSCOE+/web-auth/continue?state=...   (轮询)
//	complete     GET  <complete_url>                          (拿 webvpn cookie)
//
// state 来自 libopenconnect 的 webview 回调（SAML XML 中的 sso-v2-login URL）。
// 本包为纯 Go（net/http + encoding/json），可跨平台复用，也是从 Rust 客户端迁移到
// Go 后“复用服务端协议”的核心收益点。
package webauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// Step 是服务端每一步返回的通用结构。UI 按 Status 分支渲染。
type Step struct {
	Status       string   `json:"status"`        // select_group|credentials|otp|sms_phone|sms|radius|change_pwd|sso|done|error
	Message      string   `json:"message"`       // error 时的错误信息
	Hint         string   `json:"hint"`          // 输入框提示
	Groups       []string `json:"groups"`        // select_group
	Username     string   `json:"username"`      // credentials 回填
	SSOType      string   `json:"sso_type"`      // sso: wxwork|feishu
	RedirectURL  string   `json:"redirect_url"`  // sso: 浏览器打开地址
	CompleteURL  string   `json:"complete_url"`  // done: 完成端点（相对/绝对）
	SessionToken string   `json:"session_token"` // done: webvpn 隧道令牌（Sid@Token），供原生客户端直接建隧道
	PortalURL    string   `json:"portal_url"`    // done: 门户地址
	PhoneMasked  string   `json:"phone_masked"`  // sms: 脱敏手机号
	ChallengeMsg string   `json:"challenge_msg"` // radius: 挑战提示
	Success      bool     `json:"success"`       // done
}

// StepRequest 提交认证凭据/验证码。
type StepRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Phone    string `json:"phone,omitempty"`
	OtpCode  string `json:"otp_code,omitempty"`
	SmsCode  string `json:"sms_code,omitempty"`
}

// Client 封装一个 WebAuth 会话（含 cookie jar，用于携带 acSamlv2Token / webvpn）。
type Client struct {
	base  string // 例如 https://vpn.example.com
	state string
	http  *http.Client
}

// New 创建 WebAuth 客户端。base 为网关地址（可带 https:// 前缀）。
func New(base string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		base: normalizeBase(base),
		http: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
			// 跟随重定向，complete 会 302 到 saml_ac_login.html 并写入 webvpn cookie
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
	}
}

// SetState 设置服务端下发的 state（来自 SSO 回调 URL）。
func (c *Client) SetState(state string) { c.state = state }

// State 返回当前 state。
func (c *Client) State() string { return c.state }

func (c *Client) url(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return c.base + path
}

func (c *Client) get(path string) (*Step, error) {
	resp, err := c.http.Get(c.url(path))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	return decodeStep(resp)
}

func (c *Client) post(path string, body interface{}) (*Step, error) {
	b, _ := json.Marshal(body)
	resp, err := c.http.Post(c.url(path), "application/json", strings.NewReader(string(b)))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	return decodeStep(resp)
}

func decodeStep(resp *http.Response) (*Step, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	var s Step
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w (body=%s)", err, string(data))
	}
	return &s, nil
}

// Start 拉取初始状态（可能是 select_group 或证书自动认证后的结果）。
func (c *Client) Start() (*Step, error) {
	return c.get(fmt.Sprintf("/+CSCOE+/web-auth/start?state=%s", url.QueryEscape(c.state)))
}

// SelectGroup 选择用户组，服务端据此决定首步认证方式。
func (c *Client) SelectGroup(group, username string) (*Step, error) {
	return c.post(fmt.Sprintf("/+CSCOE+/web-auth/select_group?state=%s", url.QueryEscape(c.state)),
		map[string]string{"group": group, "username": username})
}

// Step 提交凭据/验证码，推进管道。
func (c *Client) Step(req StepRequest) (*Step, error) {
	return c.post(fmt.Sprintf("/+CSCOE+/web-auth/step?state=%s", url.QueryEscape(c.state)), req)
}

// Continue 轮询管道状态（SSO 浏览器登录后恢复）。
func (c *Client) Continue() (*Step, error) {
	return c.get(fmt.Sprintf("/+CSCOE+/web-auth/continue?state=%s", url.QueryEscape(c.state)))
}

// Complete 访问完成端点，跟随重定向并读取 webvpn cookie 值。
// 返回 webvpn cookie 的纯值（不含 "webvpn=" 前缀），供喂给隧道引擎。
func (c *Client) Complete(completeURL string) (string, error) {
	resp, err := c.http.Get(c.url(completeURL))
	if err != nil {
		return "", fmt.Errorf("完成请求失败: %w", err)
	}
	resp.Body.Close()

	// webvpn cookie 由服务端在 complete 链（自身域名）下 Set-Cookie，
	// 需按网关域名读取，而非最终 302 到的 localhost。
	u, _ := url.Parse(c.base + "/")
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "webvpn" && ck.Value != "" {
			return ck.Value, nil
		}
	}
	return "", fmt.Errorf("未获取到 webvpn cookie（服务端需在 complete 链设置 webvpn）")
}

func normalizeBase(base string) string {
	b := strings.TrimSpace(base)
	if b == "" {
		return b
	}
	if !strings.HasPrefix(b, "http://") && !strings.HasPrefix(b, "https://") {
		b = "https://" + b
	}
	return strings.TrimRight(b, "/")
}

// ExtractState 从 SSO 回调 URL 中解析出 WebAuth 的 state 参数。
func ExtractState(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Query().Get("state"), nil
}
