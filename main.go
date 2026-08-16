//go:build desktop

package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/ymhhh/go-common/logger"
	"github.com/ymhhh/vibecoding/internal/appbootstrap"
	"github.com/ymhhh/vibecoding/internal/config"
)

//go:embed all:web/dist
var embeddedFrontend embed.FS

func main() {
	cfg, err := config.Parse()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	boot, err := appbootstrap.New(cfg, nil)
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	// Closed from App.shutdown so Wails can tear down cleanly.
	app := NewApp(cfg, boot)

	assets, err := fs.Sub(embeddedFrontend, "web/dist")
	if err != nil {
		_ = boot.Close()
		os.Stderr.WriteString("embed frontend: " + err.Error() + "\n")
		os.Exit(1)
	}

	apiHandler := boot.Handler()
	logger.L().WithFields(logger.Fields{
		"data_dir":  cfg.DataDir,
		"log_level": cfg.LogLevel,
		"log_fmt":   cfg.LogFormat,
	}).Info("vibecoding desktop starting")

	err = wails.Run(&options.App{
		Title:  "Vibecoding",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/api") {
						apiHandler.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
	})
	if err != nil {
		os.Stderr.WriteString("wails: " + err.Error() + "\n")
		os.Exit(1)
	}
}
