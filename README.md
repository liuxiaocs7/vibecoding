# Vibecoding

Single-binary, local-first AI Auto-Dev board. Manage projects and issues on a Kanban, generate Dev Specs with an LLM, then let VibeBot create a git branch, patch code, run tests, and commit for review.

Two run modes:

| Mode | What it is | When to use |
|------|------------|-------------|
| **Desktop** | Wails v2 window + OS WebView (no system browser required) | Local GUI on macOS / Windows / Linux |
| **Server** | HTTP only (`127.0.0.1:8090`), optional `--open` | Docker, CI, headless hosts |

## Requirements

### Common

- Go 1.21+ (macOS 15+ needs **Go 1.23.3+**; this repo uses Go 1.25.x)
- Node.js 15+ (18+ recommended) to build the web UI
- `git` on PATH
- An OpenAI-compatible API key (or `GEMINI_API_KEY`)

### Desktop (Wails v2)

Install the Wails CLI and verify the toolchain:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
# ensure $(go env GOPATH)/bin is on PATH
wails doctor
```

Platform notes (see [Wails installation](https://wails.io/docs/gettingstarted/installation)):

| OS | Develop | Run |
|----|---------|-----|
| **macOS** | Xcode Command Line Tools (`xcode-select --install`) | System **WKWebView** (built-in) |
| **Windows** 10/11 | Go + Node; WebView2 present (check with `wails doctor`) | [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (usually preinstalled) |
| **Linux** | `gcc` + GTK3 + WebKit2GTK **dev** packages (`wails doctor` prints distro commands) | GTK3 + WebKit2GTK **runtime** (see below) |

Supported platforms: Windows 10/11 AMD64/ARM64; macOS 10.15+ AMD64 (dev), 11.0+ ARM64; Linux AMD64/ARM64.

**Linux runtime examples** (end users need runtime libs, not `-dev`):

| Distro | Install |
|--------|---------|
| Debian 12 / Ubuntu 22.04+ | `apt install libgtk-3-0 libwebkit2gtk-4.1-0` |
| Debian 11 / Ubuntu 20.04 | `apt install libgtk-3-0 libwebkit2gtk-4.0-37` |
| Fedora 40+ | `dnf install gtk3 webkit2gtk4.1` |
| Arch / Manjaro | `pacman -S gtk3 webkit2gtk-4.1` |

Desktop Linux builds default to **WebKit2GTK ABI 4.1** (`webkit2_41`). Older distros (ABI 4.0) should use **Server** mode or build with `-tags webkit2_40`. See [Linux distro support](https://wails.io/docs/guides/linux-distro-support/).

Optional: [UPX](https://upx.github.io/) (compress), [NSIS](https://wails.io/docs/guides/windows-installer/) (Windows installer).

### Server / Docker

No WebView. Build with `-tags server` and `CGO_ENABLED=0`:

```bash
go build -tags server -o vibecoding ./cmd/vibecoding
# or: make build-server
```

A modern browser is only needed if you use `--open` or open the URL yourself.

## Quick start

### Desktop (default)

```bash
# Build frontend + Wails desktop binary (needs CGO + OS WebView deps)
make build
# equivalent: CGO_ENABLED=1 go build -tags "desktop,production" -o vibecoding .

# Run native window
./vibecoding
# or: make run
```

### Server (HTTP)

```bash
make build-server
./vibecoding --open
# or: make run-server
```

Data is stored under `~/.vibecoding/` (SQLite + settings).

### Development

```bash
# Option A — Wails live reload (desktop)
make doctor    # once
make dev-desktop

# Option B — Vite + HTTP server
# Terminal 1 — Go API (server tag)
make backend && ./vibecoding

# Terminal 2 — Vite (proxies /api → :8090)
make dev
```

Open http://localhost:3000 when using Option B.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `127.0.0.1:8090` | Listen address |
| `--data-dir` | `~/.vibecoding` | SQLite / config directory |
| `--open` | false | Open browser after start (server mode) |
| `--log-level` | `info` | Log level: `debug` / `info` / `warn` / `error` |
| `--log-format` | `text` | Log format: `text` / `json` |
| `--log-output` | `stdout` | `stdout` / `stderr` / `discard` / path to file |

Logging uses [`github.com/ymhhh/go-common/logger`](https://github.com/ymhhh/go-common/tree/main/logger) (startup, HTTP `/api/*`, LLM chat, Auto-Dev phases).

## Typical workflow

1. Open **LLM settings** and save the full chat-completions URL + API Key (stored only on the local server).
2. Create a **Project** and add a local git repository path (use **Validate path**).
3. Create an **Issue** in Requirements, chat with the AI, then **Extract Dev Spec**.
4. **Accept Spec → Backlog**, then **Start Auto-Dev**.
5. Watch live logs (SSE). On success the issue moves to **In Review**.
6. **Approve & Merge** to merge the feature branch into the repo default branch locally.

## Release builds

```bash
make release
# → dist/release/vibecoding-server-darwin-arm64
# → dist/release/vibecoding-server-darwin-amd64
# → dist/release/vibecoding-server-linux-amd64
# → dist/release/vibecoding-server-windows-amd64.exe
# → dist/release/vibecoding-desktop-<host-os>-<arch>  (built for the machine running make)
```

Desktop packages should be produced on each target OS (or CI matrix). Linux desktop uses `-tags webkit2_41` by default. Desktop Go builds must include Wails' `production` (or `dev`) tag, e.g. `-tags "desktop,production"`. On macOS, link with `-framework UniformTypeIdentifiers` (set automatically by `make build`).
## Docker

Mount host repositories into the container so Auto-Dev can edit them:

```bash
docker build -t vibecoding .
docker run --rm -p 8090:8090 \
  -v "$HOME/.vibecoding:/data" \
  -v "$HOME/Codes:/Codes" \
  vibecoding --addr 0.0.0.0:8090 --data-dir /data
```

Then configure repo paths as `/Codes/...` inside the app.

## Security notes

- Default bind is localhost-only.
- API keys never persist in the browser; they are stored in the local SQLite DB.
- Auto-Dev only writes inside configured repository paths.

## License

[MIT](./LICENSE) © 2026 Henry Huang
