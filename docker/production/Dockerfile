# Production Dockerfile — lightweight single-binary image
# Build: docker build -f docker/production/Dockerfile -t skillshare .

# Stage 1: Build Go binary
FROM golang:1.25.5-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /skillshare ./cmd/skillshare

# Stage 2: Minimal runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash git ca-certificates curl tini && \
    rm -rf /var/lib/apt/lists/*
RUN useradd -r -u 10001 -s /usr/sbin/nologin skillshare && \
    mkdir -p /home/skillshare/.config/skillshare && \
    chown -R 10001:10001 /home/skillshare
COPY --from=go-builder /skillshare /usr/local/bin/skillshare
COPY docker/production/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER 10001
ENV HOME=/home/skillshare
WORKDIR /home/skillshare
EXPOSE 19420

ENTRYPOINT ["tini", "--", "/entrypoint.sh"]
CMD ["skillshare", "ui", "-g", "--host", "0.0.0.0", "--no-open"]
HEALTHCHECK --interval=10s --timeout=5s --retries=3 --start-period=10s \
  CMD curl -sf http://localhost:19420/api/health || exit 1
