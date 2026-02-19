.PHONY: help build build-meta run test test-unit test-int test-cover test-install test-docker test-docker-online sandbox-up sandbox-bare sandbox-shell sandbox-down sandbox-reset sandbox-status dev-docker-up dev-docker-down dev-docker-watch docker-build docker-build-multiarch lint fmt fmt-check check install clean ui-install ui-build ui-dev build-all

help:
	@echo "Common tasks:"
	@echo "  make build                   # build binary"
	@echo "  make run                     # run local binary help"
	@echo "  make test                    # unit + integration tests"
	@echo "  make test-unit               # unit tests only"
	@echo "  make test-int                # integration tests only"
	@echo "  make test-cover              # tests with coverage"
	@echo "  make test-install            # install.sh sandbox tests"
	@echo "  make test-docker             # docker offline sandbox (build + unit + integration)"
	@echo "  make test-docker-online      # optional docker online install/update tests"
	@echo "  make sandbox-up              # start persistent docker playground"
	@echo "  make sandbox-bare            # start playground without auto-init"
	@echo "  make sandbox-shell           # enter docker playground shell"
	@echo "  make sandbox-down            # stop and remove docker playground"
	@echo "  make sandbox-reset           # stop + remove playground volume (full reset)"
	@echo "  make sandbox-status          # show playground container status"
	@echo "  make dev-docker-up           # Go API server in Docker (pair with: cd ui && pnpm run dev)"
	@echo "  make dev-docker-down         # stop dev profile container"
	@echo "  make dev-docker-watch        # Go API server with auto-rebuild on Go file changes"
	@echo "  make docker-build            # build production Docker image"
	@echo "  make docker-build-multiarch  # build multi-arch production image"
	@echo "  make lint                    # go vet"
	@echo "  make fmt                     # format Go files"
	@echo "  make fmt-check               # verify formatting only"
	@echo "  make check                   # fmt-check + lint + test"
	@echo "  make ui-install              # install frontend dependencies"
	@echo "  make ui-build                # build frontend"
	@echo "  make ui-dev                  # Go API server + Vite dev server (requires local Go)"
	@echo "  make build-all               # ui-build + build"
	@echo "  make clean                   # remove build artifacts"

build:
	mkdir -p bin && go build -o bin/skillshare ./cmd/skillshare

build-meta:
	./scripts/build.sh

run: build
	./bin/skillshare --help

test:
	./scripts/test.sh

test-unit:
	./scripts/test.sh --unit

test-int:
	./scripts/test.sh --int

test-cover:
	./scripts/test.sh --cover

test-install:
	./scripts/test_install.sh

test-docker:
	./scripts/test_docker.sh

test-docker-online:
	./scripts/test_docker_online.sh

sandbox-up:
	./scripts/sandbox_playground_up.sh

sandbox-bare:
	./scripts/sandbox_playground_up.sh --bare

sandbox-shell:
	./scripts/sandbox_playground_shell.sh

sandbox-down:
	./scripts/sandbox_playground_down.sh

sandbox-reset:
	./scripts/sandbox_playground_down.sh --volumes

sandbox-status:
	docker compose -f docker-compose.sandbox.yml --profile playground ps

dev-docker-up:
	docker compose -f docker-compose.sandbox.yml --profile dev up -d sandbox-dev

dev-docker-down:
	docker compose -f docker-compose.sandbox.yml --profile dev down

dev-docker-watch:
	docker compose -f docker-compose.sandbox.yml --profile dev watch

docker-build:
	docker build -f docker/production/Dockerfile -t skillshare .

docker-build-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 -f docker/production/Dockerfile -t skillshare .

lint:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal ./tests

fmt-check:
	test -z "$$(gofmt -l ./cmd ./internal ./tests)"

check: fmt-check lint test

install:
	go install ./cmd/skillshare

ui-install:
	cd ui && pnpm install

ui-build: ui-install
	cd ui && pnpm run build

ui-dev:
	@trap 'kill 0' EXIT; \
	go run ./cmd/skillshare ui --no-open --host $${SKILLSHARE_UI_HOST:-localhost} & \
	cd ui && pnpm run dev

build-all: ui-build build

clean:
	rm -rf bin coverage.out
