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
	"github.com/ymhhh/vibecoding/internal/api"
	"github.com/ymhhh/vibecoding/internal/applog"
	"github.com/ymhhh/vibecoding/internal/autodev"
	"github.com/ymhhh/vibecoding/internal/config"
	"github.com/ymhhh/vibecoding/internal/db"
	"github.com/ymhhh/vibecoding/internal/llm"
)

//go:embed all:dist
var embeddedDist embed.FS

func main() {
	cfg, err := config.Parse()
	if err != nil {
		// Logger not ready yet.
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}
	if err := applog.Init(cfg.LogLevel, cfg.LogFormat, cfg.LogOutput); err != nil {
		os.Stderr.WriteString("logger: " + err.Error() + "\n")
		os.Exit(1)
	}

	store, err := db.Open(cfg.DBPath())
	if err != nil {
		logger.L().WithError(err).Fatal("open database")
	}
	defer store.Close()

	hub := autodev.NewHub()
	llmClient := llm.New()
	runner := &autodev.Runner{Store: store, LLM: llmClient, Hub: hub}

	static, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		logger.L().WithError(err).Fatal("embed frontend")
	}

	srv := &api.Server{
		Store:  store,
		LLM:    llmClient,
		Runner: runner,
		Hub:    hub,
		Static: static,
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
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
