.DEFAULT_GOAL := all
EXE := $(if $(filter windows,$(shell go env GOOS)),.exe,)
.PHONY: all server agent desktop start serve run binary-build dev dev-server dev-web build server-build agent-build test clean release reset-db

# Node dependencies are reinstalled as soon as a lockfile moves, so a build never
# starts with a package missing from node_modules. The stamp keeps repeat builds
# free: npm only runs again when package.json or package-lock.json is newer.
.PHONY: web-deps desktop-deps
web-deps: web/node_modules/.install-stamp
desktop-deps: desktop/node_modules/.install-stamp

web/node_modules/.install-stamp: web/package.json web/package-lock.json
	cd web && npm ci
	@touch $@

desktop/node_modules/.install-stamp: desktop/package.json desktop/package-lock.json
	cd desktop && npm ci
	cd desktop && node node_modules/electron/install.js
	@touch $@

# Development runs the Go API and Vite hot reload independently.
dev: web-deps
	@echo "Starting Go backend & Vite frontend in development mode..."
	@(go run ./cmd/server & cd web && npm run dev)

dev-server:
	go run ./cmd/server

dev-web: web-deps
	cd web && npm run dev

# The server embeds the compiled web UI.
all: server agent desktop
	@echo "Built server, local agent and desktop app."

# Compatibility aliases for existing scripts.
build: all
server-build: server
agent-build: agent

# The server embeds the UI; the agent builds independently of Node dependencies.
server: web-deps
	cd web && npm run build
	@touch internal/webui/dist/.gitkeep
	@mkdir -p bin
	go build -o bin/taskflow-server$(EXE).new ./cmd/server
	mv -f bin/taskflow-server$(EXE).new bin/taskflow-server$(EXE)

agent:
	@mkdir -p bin
	go build -o bin/taskflow-agent$(EXE).new ./cmd/agent
	mv -f bin/taskflow-agent$(EXE).new bin/taskflow-agent$(EXE)

binary-build: server agent

start:
	./bin/taskflow-agent$(EXE) $(ARGS)

serve:
	./bin/taskflow-server$(EXE) $(ARGS)

run: desktop-deps
	cd desktop && npm start

test: web-deps
	go test ./...
	cd web && npm test && npx tsc --noEmit -p tsconfig.app.json && npx oxlint src

# Both components cross-compile with pure Go dependencies.
release: web-deps
	@echo "Building interface..."
	cd web && npm run build
	@touch internal/webui/dist/.gitkeep
	@mkdir -p dist
	@for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		for component in server agent; do \
			out=dist/taskflow-$$component-$$os-$$arch; \
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
reset-db:
	rm -f tasks.db tasks.db-wal tasks.db-shm
	rm -f "$${HOME}/Library/Application Support/taskflow/tasks.db" \
	      "$${HOME}/Library/Application Support/taskflow/tasks.db-wal" \
	      "$${HOME}/Library/Application Support/taskflow/tasks.db-shm"
	rm -f "$${HOME}/Library/Application Support/taskacao/tasks.db" \
	      "$${HOME}/Library/Application Support/taskacao/tasks.db-wal" \
	      "$${HOME}/Library/Application Support/taskacao/tasks.db-shm"
	@echo "Bases supprimées : répertoire courant et dossiers de données."

clean:
	rm -rf bin/ dist/ web/dist
	rm -rf internal/webui/dist
	@mkdir -p internal/webui/dist && touch internal/webui/dist/.gitkeep

.PHONY: desktop-build desktop desktop-package
desktop-build: agent desktop-deps
	mkdir -p desktop/bin
	cp bin/taskflow-agent$(EXE) desktop/bin/taskflow-agent$(EXE).new
	mv -f desktop/bin/taskflow-agent$(EXE).new desktop/bin/taskflow-agent$(EXE)
	cd desktop && npm run build

desktop: desktop-package

desktop-package: desktop-build
	cd desktop && npm run package
