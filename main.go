package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "snbApp",
		Description: "snB 망연계 파일전송 클라이언트",
		Services:    []application.Service{application.NewService(&AppService{})},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "snbApp",
		Width:     1100,
		Height:    720,
		MinWidth:  800,
		MinHeight: 560,
		URL:       "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
