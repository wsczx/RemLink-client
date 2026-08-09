package core

import "context"

// SSOHandler 抽象 SSO 的承载方式，解耦具体 UI 技术。
//
// 桌面（Wails）：打开 webview 加载 url，并由本地回调（localhost:29786）捕获令牌后返回。
// iOS（NetworkExtension）：用 ASWebAuthenticationSession 打开系统浏览器，回调 scheme 捕获令牌。
// Android（VpnService）：用 Custom Tabs，深度链接捕获令牌。
//
// 纯账密/OTP 场景不会触发 SSO，Machine 不会调用 Open；可传 nil。
type SSOHandler interface {
	// Open 打开 SSO 登录页，阻塞直到用户完成登录并由宿主捕获令牌，或 ctx 取消/用户取消。
	// 成功返回服务端下发的 sso-token；用户取消返回 error（含 ErrSSOCancelled）。
	Open(ctx context.Context, url string) (token string, err error)
}

// ErrSSOCancelled 表示用户在 SSO 流程中主动取消（非错误，转为 need_auth 重渲染）。
type ssoCancelError struct{}

func (ssoCancelError) Error() string { return "sso cancelled by user" }

// ErrSSOCancelled 由 SSOHandler 在用户取消时返回。
var ErrSSOCancelled error = ssoCancelError{}
