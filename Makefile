.PHONY: all web sync-web build backend run dev doctor dev-desktop clean release

BINARY := vibecoding

all: build

# Check Wails / OS WebView toolchain (requires wails CLI on PATH).
doctor:
	wails doctor

# Desktop live reload via Wails (requires desktop entry; see wails.json).
dev-desktop:
	wails dev

# Build the React SPA into web/dist, then sync into cmd/vibecoding/dist for embed.
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

# Build the single binary (frontend must be built first for embedding).
build: web
	go build -o $(BINARY) ./cmd/vibecoding

# Build backend only (uses whatever is currently in cmd/vibecoding/dist).
backend: sync-web
	go build -o $(BINARY) ./cmd/vibecoding

# Run the server (rebuilds binary first).
run: build
	./$(BINARY) --open

# Frontend Vite dev server proxied to a separately-run backend on :8090.
dev:
	cd web && npm run dev

clean:
	rm -f $(BINARY)
	rm -rf web/dist
	rm -rf dist/release
	printf '%s\n' '<!doctype html><title>Vibecoding</title><p>Run make web</p>' > cmd/vibecoding/dist/index.html

# Cross-compile release artifacts.
release: web
	mkdir -p dist/release
	GOOS=darwin GOARCH=arm64 go build -o dist/release/vibecoding-darwin-arm64 ./cmd/vibecoding
	GOOS=darwin GOARCH=amd64 go build -o dist/release/vibecoding-darwin-amd64 ./cmd/vibecoding
	GOOS=linux GOARCH=amd64 go build -o dist/release/vibecoding-linux-amd64 ./cmd/vibecoding
	cp README.md dist/release/ 2>/dev/null || true
	@echo "Release binaries in dist/release/"
