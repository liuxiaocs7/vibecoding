package appbootstrap

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ymhhh/go-common/logger"
	"github.com/ymhhh/vibecoding/internal/api"
	"github.com/ymhhh/vibecoding/internal/applog"
	"github.com/ymhhh/vibecoding/internal/autodev"
	"github.com/ymhhh/vibecoding/internal/config"
	"github.com/ymhhh/vibecoding/internal/db"
	"github.com/ymhhh/vibecoding/internal/llm"
)

// App holds the shared HTTP stack used by both desktop (Wails) and server modes.
type App struct {
	Cfg    *config.Config
	Store  *db.Store
	Server *api.Server
}

// New initializes logging, SQLite, and the API server.
// static may be nil when the frontend is served elsewhere (e.g. Wails AssetServer).
func New(cfg *config.Config, static fs.FS) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := applog.Init(cfg.LogLevel, cfg.LogFormat, cfg.LogOutput); err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}
	store, err := db.Open(cfg.DBPath())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if n, err := store.FailOrphanJobs(); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("fail orphan jobs: %w", err)
	} else if n > 0 {
		logger.L().WithField("count", n).Warn("marked orphan auto-dev jobs as failed after restart")
	}
	hub := autodev.NewHub()
	llmClient := llm.New()
	wtRoot := filepath.Join(cfg.DataDir, "worktrees")
	if err := os.MkdirAll(wtRoot, 0o755); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("worktree root: %w", err)
	}
	runner := &autodev.Runner{Store: store, LLM: llmClient, Hub: hub, WorktreeRoot: wtRoot}
	srv := &api.Server{
		Store:  store,
		LLM:    llmClient,
		Runner: runner,
		Hub:    hub,
		Static: static,
		Token:  cfg.Token,
	}
	return &App{Cfg: cfg, Store: store, Server: srv}, nil
}

// Handler returns the HTTP mux (API + optional embedded SPA).
func (a *App) Handler() http.Handler {
	return a.Server.Handler()
}

// Close releases database resources.
func (a *App) Close() error {
	if a == nil || a.Store == nil {
		return nil
	}
	return a.Store.Close()
}
