# Vibecoding

[English](./README.md) | **简体中文**

本地优先的 AI 自动开发看板：在看板上管理项目与需求（Issue），用 LLM 生成开发规格（Dev Spec），随后由 VibeBot 自动创建 git 分支、修改代码、运行测试并提交，供人工评审。

## 我该用哪种模式？

项目提供**两种独立的构建产物**。它们共用同一份数据目录和 `/api` 后端，只是启动方式不同：

| 模式 | 构建命令 | 得到什么 | 适用场景 |
|------|----------|----------|----------|
| **桌面版 Desktop** | `make build` | 原生窗口（Wails + 系统 WebView），无需系统浏览器 | macOS / Windows / Linux 上的日常本地图形界面 |
| **服务版 Server** | `make build-server` | 仅 HTTP 进程（默认 `http://127.0.0.1:8090`），可选 `--open` 打开系统浏览器 | 无界面 / Docker / CI，或只需要 HTTP API 的调用方（例如未来的 OpenAPI 客户端） |

两种模式默认都把数据写入 `~/.vibecoding/`（SQLite + 配置）。
**注意：** `make build` 与 `make build-server` 都会输出名为 `./vibecoding` 的文件，重新构建其中一种会覆盖另一种；切换模式时请重新构建。

---

## 环境要求

### 通用（两种模式都需要）

- Go 1.21+（macOS 15+ 需 **Go 1.23.3+**；本仓库使用 Go 1.25.x）
- Node.js 15+（推荐 18+），用于构建前端 UI
- `git` 在 PATH 中
- 一个兼容 OpenAI 的 API Key（在应用内配置，或在支持处通过 `GEMINI_API_KEY` 等环境变量提供）

### 仅桌面版额外需要

先安装一次 Wails CLI，然后检查本机环境：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
# 确保 $(go env GOPATH)/bin 在你的 PATH 中
make doctor   # 等同于：wails doctor
```

| 操作系统 | 开发依赖 | 运行依赖 |
|----------|----------|----------|
| **macOS** | Xcode 命令行工具（`xcode-select --install`） | 系统自带 **WKWebView** |
| **Windows** 10/11 | Go + Node；WebView2（用 `wails doctor` 检查） | [WebView2 运行时](https://developer.microsoft.com/en-us/microsoft-edge/webview2/)（通常已预装） |
| **Linux** | `gcc` + GTK3 + WebKit2GTK **开发包**（`wails doctor` 会打印对应 apt/dnf/pacman 命令） | GTK3 + WebKit2GTK **运行时**（示例见下） |

支持平台：Windows 10/11 AMD64/ARM64；macOS 10.15+ AMD64（开发）/ 11.0+ ARM64；Linux AMD64/ARM64。

Linux 运行时依赖（终端用户，非 `-dev`）：

| 发行版 | 安装命令 |
|--------|----------|
| Debian 12 / Ubuntu 22.04+ | `apt install libgtk-3-0 libwebkit2gtk-4.1-0` |
| Debian 11 / Ubuntu 20.04 | `apt install libgtk-3-0 libwebkit2gtk-4.0-37` |
| Fedora 40+ | `dnf install gtk3 webkit2gtk4.1` |
| Arch / Manjaro | `pacman -S gtk3 webkit2gtk-4.1` |

桌面版 Linux 构建默认使用 WebKit2GTK ABI **4.1**（`webkit2_41`）。在较旧的 ABI 4.0 发行版上，建议改用**服务版**，或用 `-tags webkit2_40` 构建桌面版。参见 [Wails Linux 发行版支持](https://wails.io/docs/guides/linux-distro-support/)。

可选：[UPX](https://upx.github.io/)、[NSIS](https://wails.io/docs/guides/windows-installer/)（Windows 安装包）。

### 仅服务版额外说明

无需 WebView / CGO。浏览器是可选的——仅当你使用 `--open` 或自己打开 URL 时才需要。纯 API 调用无需浏览器。

---

## 快速开始

### 1）桌面版（图形界面）

```bash
make doctor          # 一次性：校验 Wails / WebView 工具链
make build           # 构建前端 + 桌面二进制 → ./vibecoding
./vibecoding         # 打开原生窗口
# 或：make run
```

不使用 Make 的等价命令：

```bash
cd web && npm install && npm run build && cd ..
# macOS 还需要：export CGO_LDFLAGS="-framework UniformTypeIdentifiers"
CGO_ENABLED=1 go build -tags "desktop,production" -o vibecoding .
./vibecoding
```

### 2）服务版（HTTP / API）

```bash
make build-server    # 构建前端 + 服务二进制 → ./vibecoding
./vibecoding         # 监听 http://127.0.0.1:8090（不打开浏览器）
./vibecoding --open  # 同上，并打开系统浏览器
# 或：make run-server
```

之后可以选择：

- 在浏览器打开 `http://127.0.0.1:8090` 使用看板 UI，或
- 从脚本 / 未来的 OpenAPI 客户端调用 `http://127.0.0.1:8090/api/...`（无需界面）

健康检查：

```bash
curl -s http://127.0.0.1:8090/api/health
```

### 命令行参数（主要用于服务版；桌面版也接受 data-dir / 日志相关）

| 参数 | 默认值 | 适用模式 | 说明 |
|------|--------|----------|------|
| `--addr` | `127.0.0.1:8090` | **服务版** | HTTP 监听地址 |
| `--data-dir` | `~/.vibecoding` | 两者 | SQLite / 配置目录 |
| `--open` | `false` | **服务版** | 启动后打开系统浏览器访问 `--addr` |
| `--log-level` | `info` | 两者 | `debug` / `info` / `warn` / `error` |
| `--log-format` | `text` | 两者 | `text` / `json` |
| `--log-output` | `stdout` | 两者 | `stdout` / `stderr` / `discard` / 文件路径 |

日志使用 [`github.com/ymhhh/go-common/logger`](https://github.com/ymhhh/go-common/tree/main/logger)。

---

## 开发调试

### 桌面版热重载（做 UI 时推荐）

```bash
make doctor       # 一次性
make dev-desktop  # wails dev —— Go 与前端在原生窗口中热重载
```

### 服务版 + Vite（用浏览器对接本地 API）

```bash
# 终端 1 —— API（服务版二进制）
make backend && ./vibecoding

# 终端 2 —— Vite 跑在 :3000，代理 /api → :8090
make dev
```

打开 http://localhost:3000

---

## 典型使用流程

1. 打开 **LLM 设置**，保存完整的 chat-completions URL 与 API Key（仅存储在本地 SQLite）。
2. 创建**项目**，添加本地 git 仓库路径（用 **Validate path** 校验）。
3. 创建**需求（Issue）**，与 AI 对话，然后 **Extract Dev Spec** 提取开发规格。
4. **Accept Spec → Backlog**，随后 **Start Auto-Dev** 启动自动开发。
5. 观看实时日志（SSE）。成功后需求进入 **In Review**。
6. **Approve & Merge** 在本地把特性分支合并到仓库默认分支。

---

## Docker（仅服务版）

镜像构建的是**服务版**二进制（`-tags server`，`CGO_ENABLED=0`），容器内没有桌面窗口。

```bash
docker build -t vibecoding .
docker run --rm -p 8090:8090 \
  -v "$HOME/.vibecoding:/data" \
  -v "$HOME/Codes:/Codes" \
  vibecoding --addr 0.0.0.0:8090 --data-dir /data
```

- 宿主机访问 UI / API：`http://127.0.0.1:8090`
- 在应用内把仓库路径配置为 `/Codes/...`（即**容器内**看到的路径）

---

## 发布构建

```bash
make release
```

在 `dist/release/` 下生成：

| 产物 | 模式 | 说明 |
|------|------|------|
| `vibecoding-server-darwin-arm64` | 服务版 | 可交叉编译 |
| `vibecoding-server-darwin-amd64` | 服务版 | 可交叉编译 |
| `vibecoding-server-linux-amd64` | 服务版 | 可交叉编译 |
| `vibecoding-server-windows-amd64.exe` | 服务版 | 可交叉编译 |
| `vibecoding-desktop-<宿主os>-<架构>` | 桌面版 | **仅为运行 `make release` 的本机**构建 |

桌面版在此无法可靠交叉编译，请在每个目标操作系统（或 CI 矩阵）上分别构建。Linux 桌面版默认使用 `-tags webkit2_41`。桌面版 Go 构建需要 Wails 相关 tag，如 `desktop,production`；在 macOS 上，`make build` / `make release` 会自动加上 `-framework UniformTypeIdentifiers`。

---

## 安全说明

- 服务版默认仅绑定本机（`127.0.0.1`）。仅当确实需要远程访问（如 Docker 端口映射）时才使用 `--addr 0.0.0.0:8090`。
- API Key 存储在 `--data-dir` 下的本地 SQLite 中，不保存在前端。
- Auto-Dev 只会在已配置的仓库路径内写入。

## 许可证

[MIT](./LICENSE) © 2026 Henry Huang
