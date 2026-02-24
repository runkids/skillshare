# Build arguments (must be before first FROM for cross-stage visibility)
ARG NODE_VERSION=24
ARG PNPM_VERSION=10.28.0
ARG GO_VERSION=1.25.5

# Stage 1: Build frontend assets (for playground UI)
FROM node:${NODE_VERSION}-bookworm-slim AS ui-builder
ARG PNPM_VERSION
RUN npm install -g pnpm@${PNPM_VERSION}
WORKDIR /ui
COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY ui/ ./
RUN pnpm run build

# Stage 2: Go sandbox with pre-built frontend
FROM golang:${GO_VERSION}-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash git ca-certificates curl jq sudo procps \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Install Go development tools
RUN go install github.com/air-verse/air@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest

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
