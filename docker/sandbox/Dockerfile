# Stage 1: Build frontend assets (for playground UI)
FROM node:22-bookworm-slim AS ui-builder
RUN npm install -g pnpm@latest
WORKDIR /ui
COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY ui/ ./
RUN pnpm run build

# Stage 2: Go sandbox with pre-built frontend
FROM golang:1.25.5-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends bash git ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

# Copy pre-built frontend for playground UI cache
COPY --from=ui-builder /ui/dist /ui-dist

COPY docker/sandbox/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download
RUN mkdir -p /go/build-cache /go/pkg/mod && chmod -R 0777 /go/build-cache /go/pkg/mod

ENTRYPOINT ["/entrypoint.sh"]
CMD ["bash", "-c", "./scripts/test.sh"]
