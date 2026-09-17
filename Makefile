.PHONY: all web sync-web build build-server backend run run-server serve serve-bg doctor dev-desktop wails-build clean release

BINARY_DESKTOP := vibecoding
BINARY_SERVER := vibecoding-server

# Wails on modern macOS/Xcode needs UniformTypeIdentifiers at link time.
ifeq ($(shell go env GOOS),darwin)
export CGO_LDFLAGS += -framework UniformTypeIdentifiers
endif

all: build

# Check Wails / OS WebView toolchain (requires wails CLI on PATH).
doctor:
	wails doctor

# Desktop live reload via Wails.
dev-desktop:
	wails dev

# Packaged desktop app (.app on macOS) via Wails CLI.
# Root mains use //go:build desktop || bindings — Wails strips "desktop" during
# bindings generation and rebuilds with the "bindings" tag instead.
wails-build:
	wails build

# Build the React SPA into web/dist, then sync into cmd/vibecoding/dist for server embed.
web:
	cd web && npm install && npm run build
	$(MAKE) sync-web

sync-web:
	rm -rf cmd/vibecoding/dist
	mkdir -p cmd/vibecoding/dist
	if [ -d web/dist ]; then \
		cp -R web/dist/. cmd/vibecoding/dist/; \
	else \
		printf '%s\n' '<!doctype html><title>Vibecoding</title><p>Run make web</p>' > cmd/vibecoding/dist/index.html; \
	fi

# Desktop binary (Wails + CGO). Requires platform WebView deps; see README.
# Wails requires the `production` (or `dev`) tag in addition to our `desktop` entry tag.
build: web
	CGO_ENABLED=1 go build -tags "desktop,production" -o $(BINARY_DESKTOP) .

# Headless HTTP server (no WebView / no desktop window). Safe for Docker / CI.
# Output is ./vibecoding-server so it does not overwrite the desktop binary.
build-server: web
	CGO_ENABLED=0 go build -tags server -o $(BINARY_SERVER) ./cmd/vibecoding

# Rebuild server using whatever is already in cmd/vibecoding/dist (fast iterate).
backend: sync-web
	CGO_ENABLED=0 go build -tags server -o $(BINARY_SERVER) ./cmd/vibecoding

# Run desktop app (rebuilds first).
run: build
	./$(BINARY_DESKTOP)

# Browser-only: rebuild frontend + server, start HTTP, open system browser.
# Does NOT launch the Wails desktop window.
run-server: build-server
	./$(BINARY_SERVER) --open

# Same as run-server but skips a full npm web rebuild when dist already exists.
# Preferred daily entry for "backend + browser, no desktop app".
serve: backend
	./$(BINARY_SERVER) --open

# Start HTTP only (no auto-open). Open http://127.0.0.1:8090 yourself.
serve-bg: backend
	./$(BINARY_SERVER)

# Frontend Vite dev server proxied to a separately-run backend on :8090.
dev:
	cd web && npm run dev

clean:
	rm -f $(BINARY_DESKTOP) $(BINARY_SERVER)
	rm -rf web/dist
	rm -rf build/bin
	rm -rf dist/release
	printf '%s\n' '<!doctype html><title>Vibecoding</title><p>Run make web</p>' > cmd/vibecoding/dist/index.html

# Release artifacts. Desktop needs a native (or matching) OS; server is pure Go.
# Linux desktop defaults to WebKit2GTK ABI 4.1 (webkit2_41).
release: web
	mkdir -p dist/release
	@echo "Building server binaries (cross-compile OK)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -tags server -o dist/release/vibecoding-server-darwin-arm64 ./cmd/vibecoding
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -tags server -o dist/release/vibecoding-server-darwin-amd64 ./cmd/vibecoding
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags server -o dist/release/vibecoding-server-linux-amd64 ./cmd/vibecoding
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags server -o dist/release/vibecoding-server-windows-amd64.exe ./cmd/vibecoding
	@echo "Building desktop binary for host ($(shell go env GOOS)/$(shell go env GOARCH))..."
	CGO_ENABLED=1 go build -tags "desktop,production,webkit2_41" -o dist/release/vibecoding-desktop-$(shell go env GOOS)-$(shell go env GOARCH) .
	cp README.md dist/release/ 2>/dev/null || true
	@echo "Release binaries in dist/release/"
	@echo "Note: desktop cross-compile is not supported here; build desktop on each target OS/CI runner."
