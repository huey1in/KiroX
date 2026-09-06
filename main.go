package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	runDesktopApp()
}

// runDesktopApp 运行桌面应用
func runDesktopApp() {
	app := NewApp()

	appOpts := &options.App{
		Title:  "Kiro 注册机",
		Width:  900,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 240, B: 235, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.huey1in.kirox",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				app.restoreWindow()
			},
		},
		StartHidden: false,
		Frameless:   true,
		Bind: []interface{}{
			app,
		},
	}

	// 应用平台特定选项
	for _, opt := range getPlatformOptions() {
		opt(appOpts)
	}

	err := wails.Run(appOpts)

	if err != nil {
		log.Fatal(err)
	}
}
