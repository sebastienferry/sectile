.DEFAULT_GOAL := help
EXE := $(if $(filter windows,$(shell go env GOOS)),.exe,)
# electron/runtime.cjs resolves the agent under this name, packaged or not.
DESKTOP_AGENT := desktop/bin/sectile-agent$(EXE)
.PHONY: help build-all build-server build-agent build-app build-app-package build-release \
        all build server server-build agent agent-build binary-build desktop desktop-build desktop-package build-desktop build-desktop-package release \
        web-deps desktop-deps start serve run test clean reset-db

# Node dependencies are reinstalled as soon as a lockfile moves, so a build never
# starts with a package missing from node_modules. The stamp keeps repeat builds
# free: npm only runs again when package.json or package-lock.json is newer.
web-deps: web/node_modules/.install-stamp
desktop-deps: desktop/node_modules/.install-stamp

web/node_modules/.install-stamp: web/package.json web/package-lock.json
	cd web && npm ci
	@touch $@

desktop/node_modules/.install-stamp: desktop/package.json desktop/package-lock.json
	cd desktop && npm ci
	cd desktop && node node_modules/electron/install.js
	@touch $@

# Print every documented target (the default goal).
help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# The server embeds the compiled web UI.
build-all: build-server build-agent build-app ## Build server, agent and desktop app
	@echo "Built server, local agent and desktop app."

# Compatibility aliases for existing scripts.
all build: build-all
server server-build: build-server
agent agent-build: build-agent
binary-build: build-server build-agent
desktop desktop-build build-desktop: build-app
desktop-package build-desktop-package: build-app-package
release: build-release

# The server embeds the UI; the agent builds independently of Node dependencies.
build-server: web-deps ## Build the web UI and the server binary
	cd web && npm run build
	@touch internal/webui/dist/.gitkeep
	@mkdir -p bin
	go build -o bin/server$(EXE).new ./cmd/server
	mv -f bin/server$(EXE).new bin/server$(EXE)

build-agent: ## Build the agent binary
	@mkdir -p bin
	go build -o bin/agent$(EXE).new ./cmd/agent
	mv -f bin/agent$(EXE).new bin/agent$(EXE)

# The agent writes its own path into the MCP registration native clients read,
# so it needs a stable one: `go run` would leave a build-cache path that stops
# resolving as soon as the cache is pruned. Building first keeps the target as
# convenient without that cost.
start: build-agent ## Build and run the agent (ARGS=...)
	./bin/agent$(EXE) $(ARGS)

serve: ## Run the server from source (ARGS=...)
	go run ./cmd/server $(ARGS)

# Same idea as serve and start: build what the app loads, then run it.
run: build-agent desktop-deps ## Run the desktop app from source (Vite build + Electron)
	@mkdir -p desktop/bin
	cp bin/agent$(EXE) $(DESKTOP_AGENT).new
	mv -f $(DESKTOP_AGENT).new $(DESKTOP_AGENT)
	cd desktop && npm run build
	cd desktop && npm start

test: web-deps ## Run Go and web test suites
	go test ./...
	cd web && npm test && npx tsc --noEmit -p tsconfig.app.json && npx oxlint src

# Both components cross-compile with pure Go dependencies.
build-release: web-deps ## Cross-compile every binary into dist/
	@echo "Building interface..."
	cd web && npm run build
	@touch internal/webui/dist/.gitkeep
	@mkdir -p dist
	@for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		for component in server agent; do \
			out=dist/$$component-$$os-$$arch; \
			if [ "$$os" = "windows" ]; then out=$$out.exe; fi; \
			echo "  $$component $$os/$$arch"; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" -o $$out ./cmd/$$component || exit 1; \
		done; \
	done
	@echo "Binaries in dist/:"
	@ls -lh dist/ | tail -n +2 | awk '{print "  " $$9 " (" $$5 ")"}'

# Repartir d'une base vide. Les trois fichiers comptent : supprimer tasks.db en
# laissant tasks.db-wal fait revenir les données au démarrage suivant, SQLite
# rejouant son journal.
reset-db: ## Delete local and user-data SQLite databases
	rm -f tasks.db tasks.db-wal tasks.db-shm
	rm -f "$${HOME}/Library/Application Support/taskflow/tasks.db" \
	      "$${HOME}/Library/Application Support/taskflow/tasks.db-wal" \
	      "$${HOME}/Library/Application Support/taskflow/tasks.db-shm"
	rm -f "$${HOME}/Library/Application Support/taskacao/tasks.db" \
	      "$${HOME}/Library/Application Support/taskacao/tasks.db-wal" \
	      "$${HOME}/Library/Application Support/taskacao/tasks.db-shm"
	@echo "Bases supprimées : répertoire courant et dossiers de données."

clean: ## Remove build artifacts
	rm -rf bin/ dist/ web/dist
	rm -rf internal/webui/dist
	@mkdir -p internal/webui/dist && touch internal/webui/dist/.gitkeep

build-app: build-agent desktop-deps ## Build the desktop app without packaging
	mkdir -p desktop/bin
	cp bin/agent$(EXE) $(DESKTOP_AGENT).new
	mv -f $(DESKTOP_AGENT).new $(DESKTOP_AGENT)
	cd desktop && npm run build

build-app-package: build-app ## Package the desktop app
	cd desktop && npm run package
