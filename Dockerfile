# Server image: the compiled web interface embedded in a static Go binary, run
# as a non-root user on a distroless base. The agent is not part of this image;
# it runs on workstations and is distributed as a binary (see .gitlab-ci.yml).
#
#   docker build -t sectile-server .
#   docker run -p 8090:8090 -v sectile-data:/data sectile-server

# ── Web interface ─────────────────────────────────────────────────────────────
# Vite writes into ../internal/webui/dist (web/vite.config.ts), which is where
# go:embed picks the interface up in the next stage.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
# shared/ sits at the repository root and is imported by both the web app
# and the desktop app (web/tsconfig.app.json includes ../shared).
COPY shared/ ../shared/
COPY web/ ./
RUN mkdir -p ../internal/webui && npm run build

# ── Server binary ─────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS build
# The image pins GOTOOLCHAIN=local; go.mod may ask for a newer patch release
# than it ships, so let Go download the toolchain go.mod requires.
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# .dockerignore keeps the local dist out of the context; the freshly built one
# replaces it so the binary never embeds a stale or empty interface.
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN test -f internal/webui/dist/index.html
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/sectile-server ./cmd/server

# ── Runtime ───────────────────────────────────────────────────────────────────
# distroless/static carries CA certificates (tracker HTTPS calls) and nothing
# else. The server is pure Go (modernc sqlite), so no libc is needed.
FROM gcr.io/distroless/static-debian12:nonroot
# Everything the server writes lives under /data: the SQLite database and its
# -wal/-shm side files (DB_PATH), and the data directory os.UserConfigDir()
# resolves through XDG_CONFIG_HOME, where an optional .env can be mounted.
# Mount a persistent volume there.
#
# The key encrypting personal tracker credentials is generated there too, as
# secret.key. A volume snapshot therefore carries the database and its key
# together, which protects nothing on its own: set SECTILE_SECRET_KEY from the
# platform's secret store to keep them apart. The server starts either way and
# refuses only the credentials that would need the key.
ENV PORT=8090 \
    DB_PATH=/data/tasks.db \
    HOME=/data \
    XDG_CONFIG_HOME=/data/config
WORKDIR /data
COPY --from=build /out/sectile-server /usr/local/bin/sectile-server
EXPOSE 8090
VOLUME ["/data"]
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/sectile-server"]
