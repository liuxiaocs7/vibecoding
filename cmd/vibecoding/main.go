package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/ymhhh/go-common/logger"
	"github.com/ymhhh/vibecoding/internal/appbootstrap"
	"github.com/ymhhh/vibecoding/internal/config"
)

//go:embed all:dist
var embeddedDist embed.FS

func main() {
	cfg, err := config.Parse()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	static, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		os.Stderr.WriteString("embed frontend: " + err.Error() + "\n")
		os.Exit(1)
	}

	app, err := appbootstrap.New(cfg, static)
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	defer app.Close()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + cfg.Addr
	logger.L().WithFields(logger.Fields{
		"addr":      cfg.Addr,
		"data_dir":  cfg.DataDir,
		"log_level": cfg.LogLevel,
		"log_fmt":   cfg.LogFormat,
	}).Info("vibecoding listening")

	if cfg.Open {
		go func() {
			time.Sleep(400 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				logger.L().WithError(err).Warn("open browser")
			}
		}()
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.L().WithError(err).Fatal("http server stopped")
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
