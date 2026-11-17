package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"banshee/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	app := NewApp()

	var trayCtrl *tray.Controller

	err := wails.Run(&options.App{
		Title:             "Banshee",
		Width:             1080,
		Height:            760,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 7, G: 8, B: 10, A: 1},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "banshee.turing.relay.desktop.v1",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				app.showWindow()
			},
		},
		OnStartup: func(ctx context.Context) {
			app.wailsEvents = true
			trayCtrl = tray.Start(trayIcon, tray.Actions{
				OnShow: app.showWindow,
				OnQuit: app.requestQuit,
			})
			app.setTraySink(trayCtrl)
			app.startup(ctx)
		},
		OnBeforeClose: app.beforeClose,
		OnShutdown: func(ctx context.Context) {
			app.shutdown(ctx)
			if trayCtrl != nil {
				trayCtrl.Stop()
			}
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
