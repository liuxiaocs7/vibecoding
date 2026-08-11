# Vibecoding

Single-binary, local-first AI Auto-Dev board. Manage projects and issues on a Kanban, generate Dev Specs with an LLM, then let VibeBot create a git branch, patch code, run tests, and commit for review.

## Requirements

- Go 1.22+
- Node.js 18+ (to build the web UI)
- `git` on PATH
- An OpenAI-compatible API key (or `GEMINI_API_KEY`)

## Quick start

```bash
# Build frontend + Go binary
make build

# Run (listens on 127.0.0.1:8090, opens browser with --open)
./vibecoding --open
```

Data is stored under `~/.vibecoding/` (SQLite + settings).

### Development

```bash
# Terminal 1 — Go API
make backend && ./vibecoding

# Terminal 2 — Vite (proxies /api → :8090)
make dev
```

Open http://localhost:3000

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `127.0.0.1:8090` | Listen address |
| `--data-dir` | `~/.vibecoding` | SQLite / config directory |
| `--open` | false | Open browser after start |
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
# → dist/release/vibecoding-darwin-arm64
# → dist/release/vibecoding-darwin-amd64
# → dist/release/vibecoding-linux-amd64
```

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
