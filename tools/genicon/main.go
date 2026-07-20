// Command genicon 生成 Windows 图标 build/windows/icon.ico。
//
// 复用客户端运行时图标（internal/ui.Icon() 的 256x256 PNG），
// 经 winres 缩放为多尺寸 .ico，供 `wails3 generate syso` 嵌入 exe。
//
// 用法：go run ./tools/genicon
package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
	"remlink-client/internal/ui"
)

func main() {
	pngBytes := ui.Icon()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		panic(err)
	}

	// 多尺寸：16/32/48/64/128/256，覆盖资源管理器/任务栏/开始菜单各场景。
	ico, err := winres.NewIconFromResizedImage(img, []int{16, 32, 48, 64, 128, 256})
	if err != nil {
		panic(err)
	}

	out := filepath.Join("build", "windows", "icon.ico")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		panic(err)
	}
	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := ico.SaveICO(f); err != nil {
		panic(err)
	}
	println("wrote", out)
}
