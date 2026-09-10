.PHONY: dev dev-server dev-web build run test clean release reset-db

# Développement : le serveur Go d'un côté, Vite de l'autre, avec son proxy vers
# l'API. L'interface n'est pas embarquée dans ce mode, elle est rechargée à chaud.
dev:
	@echo "Starting Go backend & Vite frontend in development mode..."
	@(go run ./cmd/server & cd web && npm run dev)

dev-server:
	go run ./cmd/server

dev-web:
	cd web && npm run dev

# Build : l'interface est compilée dans internal/webui/dist, d'où go:embed la
# prend. Le binaire produit ne dépend d'aucun fichier voisin.
build:
	@echo "Building interface..."
	cd web && npm run build
	@touch internal/webui/dist/.gitkeep
	@echo "Building binary with the interface embedded..."
	go build -o bin/taskflow ./cmd/server
	@echo "Done: bin/taskflow"

run: build
	./bin/taskflow

test:
	go test ./internal/...
	cd web && npm test && npx tsc --noEmit -p tsconfig.app.json && npx oxlint src

# Release : un fichier par plateforme, sans dépendance système. SQLite est en Go
# pur (modernc.org/sqlite), donc rien n'oblige à compiler sur la cible.
release: 
	@echo "Building interface..."
	cd web && npm run build
	@touch internal/webui/dist/.gitkeep
	@mkdir -p dist
	@for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		out=dist/taskflow-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then out=$$out.exe; fi; \
		echo "  $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" -o $$out ./cmd/server || exit 1; \
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

