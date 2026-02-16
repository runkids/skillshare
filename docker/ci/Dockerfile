# CI Dockerfile — minimal image for skill validation in pipelines
# Usage: docker run ghcr.io/<org>/skillshare-ci audit /path/to/skills
# Build: docker build -f docker/ci/Dockerfile -t skillshare-ci .

# Stage 1: Build Go binary
FROM golang:1.25.5-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /skillshare ./cmd/skillshare

# Stage 2: Minimal CI runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates && \
    rm -rf /var/lib/apt/lists/*
COPY --from=go-builder /skillshare /usr/local/bin/skillshare
ENTRYPOINT ["skillshare"]
