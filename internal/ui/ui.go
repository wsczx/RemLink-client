// Package ui 提供客户端原生「引导页」的静态资源（品牌头部 + 服务器地址输入），
// 以及一个运行时生成的品牌图标（窗口/托盘使用）。
//
// 生产模式下，引导页只负责「选择/输入服务器地址」；点击连接后由 Go 侧
// 让 WebView 跳转到真实服务器的 WebAuth 认证页（https://<server>/ui/#/web-auth），
// 认证与门户均由真实服务器提供，客户端不再内置任何 mock 数据。
package ui

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"net/http"
)

//go:embed all:assets
var assetsFS embed.FS

// Handler 返回 http.Handler，提供客户端原生引导页（index.html / styles.css / app.js / logo.svg）。
func Handler() http.Handler {
	mux := http.NewServeMux()
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return mux
}

// Icon 生成 256x256 的 PNG 图标，与 RemLink 真实 logo 保持一致：
// 纯色 #0EA5E9 圆角矩形 + 浅色箭头折线 + 三个节点圆点，用于窗口与系统托盘。
func Icon() []byte {
	const size = 256
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// 背景圆角矩形 #0EA5E9（64→256 放大 4 倍：x=8,y=8,w=240,h=240,rx=56）
	drawRoundRect(img, 8, 8, 248, 248, 56, color.RGBA{14, 165, 233, 255})

	light := color.RGBA{240, 249, 255, 255} // #F0F9FF
	// 箭头折线（坐标按 64→256 放大 4 倍）
	drawThickLine(img, 56, 160, 120, 96, 20, light)  // M14 40 L30 24
	drawThickLine(img, 120, 96, 152, 128, 20, light) // L38 32
	drawThickLine(img, 120, 96, 200, 96, 20, light)  // M30 24 L50 24
	drawThickLine(img, 200, 96, 168, 136, 20, light) // L42 34

	// 节点圆点
	drawDisk(img, 56, 160, 20, color.RGBA{186, 230, 253, 255}) // #BAE6FD (14,40)
	drawDisk(img, 200, 96, 20, color.RGBA{186, 230, 253, 255}) // #BAE6FD (50,24)
	drawDisk(img, 120, 96, 16, light)                          // #F0F9FF (30,24)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func drawRoundRect(img *image.RGBA, x0, y0, x1, y1, r int, c color.RGBA) {
	w, h := x1-x0, y1-y0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if inRoundRect(x-x0, y-y0, w, h, r) {
				img.Set(x, y, c)
			}
		}
	}
}

func inRoundRect(x, y, w, h, r int) bool {
	if x < r && y < r {
		return math.Hypot(float64(x-r), float64(y-r)) <= float64(r)
	}
	if x >= w-r && y < r {
		return math.Hypot(float64(x-(w-r)), float64(y-r)) <= float64(r)
	}
	if x < r && y >= h-r {
		return math.Hypot(float64(x-r), float64(y-(h-r))) <= float64(r)
	}
	if x >= w-r && y >= h-r {
		return math.Hypot(float64(x-(w-r)), float64(y-(h-r))) <= float64(r)
	}
	return true
}

func drawDisk(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if math.Hypot(float64(x-cx), float64(y-cy)) <= float64(r) {
				img.Set(x, y, c)
			}
		}
	}
}

// drawThickLine 以 r=w/2 的实心圆沿线段填充，天然得到圆头线段。
func drawThickLine(img *image.RGBA, x0, y0, x1, y1, w int, c color.RGBA) {
	steps := int(math.Hypot(float64(x1-x0), float64(y1-y0))) * 2
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(float64(x0) + t*float64(x1-x0))
		y := int(float64(y0) + t*float64(y1-y0))
		drawDisk(img, x, y, w/2, c)
	}
}
