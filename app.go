//go:build desktop

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

// SaveTextFile opens a native save dialog and writes contents to the chosen path.
// An empty return path means the user cancelled.
func (a *App) SaveTextFile(defaultFilename, contents string) (string, error) {
	if a == nil || a.ctx == nil {
		return "", fmt.Errorf("desktop app is not ready")
	}
	name := strings.TrimSpace(defaultFilename)
	if name == "" {
		name = "dev-spec.md"
	}
	name = filepath.Base(name)
	opts := runtime.SaveDialogOptions{
		Title:                "Export Dev Spec",
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: "Markdown (*.md)", Pattern: "*.md"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	}
	if home, err := os.UserHomeDir(); err == nil {
		downloads := filepath.Join(home, "Downloads")
		if st, err := os.Stat(downloads); err == nil && st.IsDir() {
			opts.DefaultDirectory = downloads
		} else {
			opts.DefaultDirectory = home
		}
	}
	path, err := runtime.SaveFileDialog(a.ctx, opts)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
