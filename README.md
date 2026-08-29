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
2. Optionally open **Coding Executor** in the same settings: default is built-in LLM (VibeBot); or pick Claude Code / Cursor / Codex / a custom CLI if installed and logged in on this machine.
3. Create a **Project** and add a local git repository path (**Validate path**).
4. Create an **Issue**, chat with the AI, then **Extract Dev Spec**. Large issues can be **split into sub-requirements**, each with its own spec. Chat can update every sub-spec at once, or a single sub-spec.
5. **Accept Spec → Backlog**, then **Start Auto-Dev**. Split issues are implemented in order, with one commit per sub-requirement. Coding runs in isolated git worktrees under `{data-dir}/worktrees/{issueID}/{repoID}/` (default `~/.vibecoding/worktrees/...`) so your main checkout and dirty files stay untouched.
6. Watch live logs (SSE), including agent CLI output when that executor is selected. On success the issue moves to **In Review** with a **real** file tree + unified diff and quality-gate results. Failed tests may auto-heal up to `maxHeal` rounds (default 2); still failing returns the issue to Backlog and keeps the worktree for inspection. Rework can target the whole issue or one sub-requirement.
7. **Approve & Merge** merges the feature branch into the repo default branch locally (refuses if that branch is checked out and dirty), then removes the worktree. You can also open the worktree in Cursor / VS Code from the review UI.

Design details: [docs/autodev-isolation-executor-review.md](./docs/autodev-isolation-executor-review.md).

### Coding executors (optional CLI)

| Preset | Typical binary | Notes |
|--------|----------------|-------|
| Built-in LLM | (app settings) | Default. Uses your OpenAI-compatible key. |
| Claude | `claude` | Install/login via Anthropic Claude Code. App does not install or log in for you. |
| Cursor | `cursor-agent` or `agent` | Cursor CLI agent. Do **not** pass Cursor’s `--worktree` — Auto-Dev already uses its own worktree as cwd. |
| Codex | `codex` | OpenAI Codex CLI. |
| Custom | your command | Must pass `{prompt}` in args and/or enable stdin. |

Unattended permission flags (YOLO / skip-permissions) are only used because **cwd is the isolated worktree**, not your primary working tree. Vibecoding does not forward its LLM API key into these CLIs; they use their own auth (`ANTHROPIC_API_KEY`, Cursor login, etc.).

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
- Auto-Dev writes inside isolated worktrees under `{data-dir}/worktrees/...` and commits on feature branches; it does not checkout your primary working tree. Agent CLIs may skip interactive permission prompts only in that worktree cwd.
- Merging refuses when the default branch is checked out and dirty — commit or stash first.

## License

[MIT](./LICENSE) © 2026 Henry Huang
