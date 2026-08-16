//go:build desktop

package main

import (
	"context"

	"github.com/ymhhh/vibecoding/internal/appbootstrap"
	"github.com/ymhhh/vibecoding/internal/config"
)

// App is the Wails-bound application (lifecycle only; API stays on HTTP).
type App struct {
	ctx  context.Context
	boot *appbootstrap.App
	cfg  *config.Config
}

func NewApp(cfg *config.Config, boot *appbootstrap.App) *App {
	return &App{cfg: cfg, boot: boot}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.boot != nil {
		_ = a.boot.Close()
	}
}
