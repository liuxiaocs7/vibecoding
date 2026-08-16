# Vibecoding

**English** | [简体中文](./README.zh-CN.md)

Local-first AI Auto-Dev board: manage projects and issues on a Kanban, generate Dev Specs with an LLM, then let VibeBot create a git branch, patch code, run tests, and commit for review.

## Which mode should I use?

There are **two separate binaries / build targets**. They share the same data directory and `/api` backend, but start differently:

| Mode | Command to build | What you get | Use when |
|------|------------------|--------------|----------|
| **Desktop** | `make build` | Native window (Wails + OS WebView). No system browser needed. | Everyday local GUI on macOS / Windows / Linux |
| **Server** | `make build-server` | HTTP process only (default `http://127.0.0.1:8090`). Optional `--open` opens the system browser. | Headless / Docker / CI, or callers that only need the HTTP API (e.g. future OpenAPI clients) |

Both modes write to `~/.vibecoding/` by default (SQLite + settings).  
**Important:** `make build` and `make build-server` both output a file named `./vibecoding`. Rebuilding one mode overwrites the other; rebuild when you switch.

---

## Requirements

### Always needed

- Go 1.21+ (macOS 15+ needs **Go 1.23.3+**; this repo uses Go 1.25.x)
- Node.js 15+ (18+ recommended) to build the web UI
- `git` on PATH
- An OpenAI-compatible API key (configured in the app, or via env such as `GEMINI_API_KEY` where supported)

### Extra for Desktop only

Install the Wails CLI once, then check the machine:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
# ensure $(go env GOPATH)/bin is on your PATH
make doctor   # same as: wails doctor
```

| OS | Develop | Run |
|----|---------|-----|
| **macOS** | Xcode Command Line Tools (`xcode-select --install`) | Built-in **WKWebView** |
| **Windows** 10/11 | Go + Node; WebView2 (see `wails doctor`) | [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (usually already installed) |
| **Linux** | `gcc` + GTK3 + WebKit2GTK **dev** packages (`wails doctor` prints the exact apt/dnf/pacman lines) | GTK3 + WebKit2GTK **runtime** (examples below) |

Supported: Windows 10/11 AMD64/ARM64; macOS 10.15+ AMD64 (dev) / 11.0+ ARM64; Linux AMD64/ARM64.

Linux runtime packages (end users — not `-dev`):

| Distro | Install |
|--------|---------|
| Debian 12 / Ubuntu 22.04+ | `apt install libgtk-3-0 libwebkit2gtk-4.1-0` |
| Debian 11 / Ubuntu 20.04 | `apt install libgtk-3-0 libwebkit2gtk-4.0-37` |
| Fedora 40+ | `dnf install gtk3 webkit2gtk4.1` |
| Arch / Manjaro | `pacman -S gtk3 webkit2gtk-4.1` |

Desktop Linux builds default to WebKit2GTK ABI **4.1** (`webkit2_41`). On older ABI 4.0 distros, prefer **Server** mode, or build desktop with `-tags webkit2_40`. See [Wails Linux distro support](https://wails.io/docs/guides/linux-distro-support/).

Optional: [UPX](https://upx.github.io/), [NSIS](https://wails.io/docs/guides/windows-installer/) (Windows installer).

### Extra for Server only

No WebView / CGO. A browser is optional — only if you pass `--open` or open the URL yourself. API-only use needs no browser.

---

## Quick start

### 1) Desktop (GUI)

```bash
make doctor          # once: verify Wails / WebView toolchain
make build           # frontend + desktop binary → ./vibecoding
./vibecoding         # opens a native window
# or: make run
```

Equivalent without Make:

```bash
cd web && npm install && npm run build && cd ..
# macOS also needs: export CGO_LDFLAGS="-framework UniformTypeIdentifiers"
CGO_ENABLED=1 go build -tags "desktop,production" -o vibecoding .
./vibecoding
```

### 2) Server (HTTP / API)

```bash
make build-server    # frontend + server binary → ./vibecoding
./vibecoding         # listen on http://127.0.0.1:8090 (no browser)
./vibecoding --open  # same, then open the system browser
# or: make run-server
```

Then either:

- Open `http://127.0.0.1:8090` in a browser for the Kanban UI, or
- Call `http://127.0.0.1:8090/api/...` from scripts / future OpenAPI clients (no UI required)

Health check:

```bash
curl -s http://127.0.0.1:8090/api/health
```

### Flags (mostly Server; Desktop also accepts data-dir / logging)

| Flag | Default | Applies to | Description |
|------|---------|------------|-------------|
| `--addr` | `127.0.0.1:8090` | **Server** | HTTP listen address |
| `--data-dir` | `~/.vibecoding` | Both | SQLite / config directory |
| `--open` | `false` | **Server** | After start, open the system browser to `--addr` |
| `--log-level` | `info` | Both | `debug` / `info` / `warn` / `error` |
| `--log-format` | `text` | Both | `text` / `json` |
| `--log-output` | `stdout` | Both | `stdout` / `stderr` / `discard` / path to file |

Logging uses [`github.com/ymhhh/go-common/logger`](https://github.com/ymhhh/go-common/tree/main/logger).

---

## Development

### Desktop live reload (recommended for UI work)

```bash
make doctor       # once
make dev-desktop  # wails dev — Go + frontend hot reload in a native window
```

### Server + Vite (browser against local API)

```bash
# Terminal 1 — API (server binary)
make backend && ./vibecoding

# Terminal 2 — Vite on :3000, proxies /api → :8090
make dev
```

Open http://localhost:3000

---

## Typical product workflow

1. Open **LLM settings** and save the full chat-completions URL + API Key (stored only in local SQLite).
2. Create a **Project** and add a local git repository path (**Validate path**).
3. Create an **Issue**, chat with the AI, then **Extract Dev Spec**.
4. **Accept Spec → Backlog**, then **Start Auto-Dev**.
5. Watch live logs (SSE). On success the issue moves to **In Review**.
6. **Approve & Merge** to merge the feature branch into the repo default branch locally.

---

## Docker (Server mode only)

The image builds the **server** binary (`-tags server`, `CGO_ENABLED=0`). There is no desktop window inside the container.

```bash
docker build -t vibecoding .
docker run --rm -p 8090:8090 \
  -v "$HOME/.vibecoding:/data" \
  -v "$HOME/Codes:/Codes" \
  vibecoding --addr 0.0.0.0:8090 --data-dir /data
```

- UI / API from the host: `http://127.0.0.1:8090`
- Configure repo paths inside the app as `/Codes/...` (paths as seen **inside** the container)

---

## Release builds

```bash
make release
```

Produces under `dist/release/`:

| Artifact | Mode | Notes |
|----------|------|-------|
| `vibecoding-server-darwin-arm64` | Server | Cross-compile OK |
| `vibecoding-server-darwin-amd64` | Server | Cross-compile OK |
| `vibecoding-server-linux-amd64` | Server | Cross-compile OK |
| `vibecoding-server-windows-amd64.exe` | Server | Cross-compile OK |
| `vibecoding-desktop-<host-os>-<arch>` | Desktop | Built **only for the machine running `make release`** |

Desktop cannot be reliably cross-compiled here; build desktop on each target OS (or CI matrix). Linux desktop uses `-tags webkit2_41` by default. Desktop Go builds need Wails tags such as `desktop,production`; on macOS, `make build` / `make release` set `-framework UniformTypeIdentifiers` for you.

---

## Security notes

- Server default bind is localhost-only (`127.0.0.1`). Use `--addr 0.0.0.0:8090` only when you intend remote access (e.g. Docker port publish).
- API keys are stored in the local SQLite DB under `--data-dir`, not in the frontend.
- Auto-Dev only writes inside configured repository paths.

## License

[MIT](./LICENSE) © 2026 Henry Huang
