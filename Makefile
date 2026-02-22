.PHONY: help build build-meta run test test-unit test-int test-docker test-docker-online test-redteam test-redteam-signal test-redteam-rules-signal playground playground-down dev-docker dev-docker-down docker-build docker-build-multiarch lint fmt fmt-check check install clean ui-install ui-build ui-dev build-all

help:
	@echo "Common tasks:"
	@echo "  make build          # build binary"
	@echo "  make run            # run local binary help"
	@echo "  make test           # unit + integration tests"
	@echo "  make test-unit      # unit tests only"
	@echo "  make test-int       # integration tests only"
	@echo "  make test-docker    # docker offline sandbox (build + unit + integration)"
	@echo "  make test-docker-online  # docker online install/update tests"
	@echo "  make test-redteam   # red team supply-chain security tests"
	@echo "  make test-redteam-signal  # verify red team test fails on intentional mutation"
	@echo "  make test-redteam-rules-signal  # verify red team test fails when critical builtin rules are disabled"
	@echo "  make playground     # start playground + enter shell (one step)"
	@echo "  make playground-down  # stop and remove playground"
	@echo "  make lint           # go vet"
	@echo "  make fmt            # format Go files"
	@echo "  make check          # fmt-check + lint + test"
	@echo "  make ui-dev         # Go API server + Vite dev server (requires local Go)"
	@echo "  make dev-docker     # Go API in Docker + auto-rebuild (pair with: cd ui && pnpm run dev)"
	@echo "  make dev-docker-down  # stop dev Docker container"
	@echo "  make build-all      # ui-build + build"
	@echo "  make clean          # remove build artifacts"
	@echo ""
	@echo "Advanced (run scripts directly):"
	@echo "  ./scripts/sandbox.sh <up|down|shell|reset|status|logs|bare>"
	@echo "  ./scripts/test.sh --cover"
	@echo "  ./scripts/test_install.sh"
	@echo "  docker compose -f docker-compose.sandbox.yml --profile dev up -d  # start without watch"

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

test-docker:
	./scripts/test_docker.sh

test-docker-online:
	./scripts/test_docker_online.sh

test-redteam: build
	./scripts/red_team_test.sh

test-redteam-signal:
	./scripts/test_redteam_signal.sh

test-redteam-rules-signal:
	./scripts/test_redteam_rules_signal.sh

playground:
	./scripts/sandbox_playground_up.sh
	./scripts/sandbox_playground_shell.sh

playground-down:
	./scripts/sandbox_playground_down.sh

dev-docker:
	docker compose -f docker-compose.sandbox.yml --profile dev watch

dev-docker-down:
	docker compose -f docker-compose.sandbox.yml --profile dev down

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
	air & \
	cd ui && pnpm run dev

build-all: ui-build build

clean:
	rm -rf bin coverage.out
