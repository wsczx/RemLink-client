package main

import (
	"net/http"
	"strings"

	"remlink-client/internal/app"
	"remlink-client/internal/ui"
	_ "remlink-client/wintun_dll" // Windows 下启动时把内嵌的 wintun.dll 释放到 exe 同目录

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

func main() {
	svc := app.New()
	notifier := notifications.New()
	svc.SetNotifier(notifier)

	app := application.New(application.Options{
		Name:        "RemLink",
		Description: "RemLink 安全远程接入客户端",
		Icon:        ui.Icon(),
		// 用 Middleware 而非 Handler：把 /wails/*（运行时脚本 /wails/runtime.js）
		// 交给 Wails 默认 handler 处理，其余路径由客户端引导页 handler 提供。
		// 否则自定义 Handler 会替换掉默认 server，导致 /wails/runtime.js 无法加载、
		// 前端拿不到 window.go（报“客户端尚未就绪”）。
		Assets: application.AssetOptions{
			Middleware: func(next http.Handler) http.Handler {
				uiHandler := ui.Handler()
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/wails/") {
						next.ServeHTTP(w, r)
						return
					}
					uiHandler.ServeHTTP(w, r)
				})
			},
		},
		Services: []application.Service{
			application.NewService(svc),
			application.NewService(notifier), // 系统通知（通知中心即时提醒）
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "RemLink",
		BackgroundColour: application.NewRGB(27, 33, 56),
		URL:              "/",
		Frameless:        true,
		Width:            960,
		Height:           720,
		MinWidth:         960,
		MinHeight:        720,
		// 固定窗口大小：不允许用户拖拽缩放。
		DisableResize: true,
		// 禁用 WebView 默认右键菜单（返回/前进/刷新/另存等）。
		DefaultContextMenuDisabled: true,
	})

	svc.SetWindow(win)
	svc.SetApp(app)

	// 关闭窗口（点 X）时最小化到系统托盘，而不是直接退出程序。
	// RegisterHook 在内部默认关闭监听之前执行：Cancel() 阻止窗口被销毁，
	// 改为 Hide() 隐藏窗口（托盘仍在，可随时唤回）。各平台关闭事件最终都
	// 映射到 events.Common.WindowClosing（见 wails events/defaults.go）。
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
	})

	// 系统托盘：显示窗口（默认回到连接详情页）/ 进入门户 / 断开连接 / 退出
	tray := app.SystemTray.New()
	tray.SetIcon(ui.Icon())
	tray.SetMenu(application.NewMenuFromItems(
		application.NewMenuItem("显示窗口").OnClick(func(*application.Context) {
			win.SetURL("/") // 已连接→连接详情页；未连接→连接页
			win.Show()
			win.Focus()
		}),
		application.NewMenuItem("进入门户").OnClick(func(*application.Context) {
			_ = svc.OpenPortal()
		}),
		application.NewMenuItem("断开连接").OnClick(func(*application.Context) {
			_ = svc.Disconnect()
		}),
		application.NewMenuItem("退出").OnClick(func(*application.Context) {
			app.Quit()
		}),
	))
	tray.OnClick(func() {
		win.SetURL("/")
		win.Show()
		win.Focus()
	})
	tray.Run()

	svc.SetTray(tray)

	if err := app.Run(); err != nil {
		println("error:", err.Error())
	}
}
