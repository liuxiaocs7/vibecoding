//go:build desktop

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

func sanitizeExportFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "dev-spec.md"
	}
	name = filepath.Base(name)
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			b.WriteRune(r)
			lastDash = false
		}
	}
	out := strings.Trim(b.String(), "-. ")
	if out == "" {
		out = "dev-spec.md"
	}
	if !strings.HasSuffix(strings.ToLower(out), ".md") {
		out += ".md"
	}
	return out
}

func downloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	return home, nil
}

func uniquePath(dir, name string) string {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return path
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, time.Now().Unix(), ext))
}

// SaveTextFileToDownloads writes contents into ~/Downloads (no dialog).
// Preferred for the "导出下载" button: WKWebView save dialogs often open behind
// the Issue modal and look like a no-op when cancelled.
func (a *App) SaveTextFileToDownloads(defaultFilename, contents string) (string, error) {
	if a == nil {
		return "", fmt.Errorf("desktop app is not ready")
	}
	dir, err := downloadsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := sanitizeExportFilename(defaultFilename)
	path := uniquePath(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// SaveTextFile opens a native save dialog and writes contents to the chosen path.
// An empty return path means the user cancelled.
func (a *App) SaveTextFile(defaultFilename, contents string) (string, error) {
	if a == nil || a.ctx == nil {
		return "", fmt.Errorf("desktop app is not ready")
	}
	name := sanitizeExportFilename(defaultFilename)
	opts := wailsruntime.SaveDialogOptions{
		Title:                "Export Dev Spec",
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Markdown (*.md)", Pattern: "*.md"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	}
	if dir, err := downloadsDir(); err == nil {
		opts.DefaultDirectory = dir
	}
	// Bring the app forward so the sheet is not stuck behind the Issue modal.
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	path, err := wailsruntime.SaveFileDialog(a.ctx, opts)
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

// RevealInFinder shows path in the system file manager (Finder / Explorer / file manager).
func (a *App) RevealInFinder(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("empty path")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,", path).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

// WindowToggleMaximise toggles the native desktop window maximize state.
func (a *App) WindowToggleMaximise() {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.WindowToggleMaximise(a.ctx)
}

// WindowIsMaximised reports whether the desktop window is currently maximised.
func (a *App) WindowIsMaximised() bool {
	if a == nil || a.ctx == nil {
		return false
	}
	return wailsruntime.WindowIsMaximised(a.ctx)
}
