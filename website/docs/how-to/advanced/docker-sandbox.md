---
sidebar_position: 8
---

# Docker: Test, Develop, and Deploy

Use Docker for repeatable testing, frontend development without Go, production deployment, and automated skill validation in CI.

## Mode Selection Diagram

```mermaid
flowchart TD
A["Need Docker"] --> B{"Primary goal"}
B --> C["Regression checks"]
B --> D["Remote-source validation"]
B --> E["Command exploration"]
B --> F["Frontend development"]
B --> G["Deployment / CI"]
B --> H["VS Code / Codespaces"]

C --> C1["Offline test sandbox"]
C1 --> C2["make test-docker"]

D --> D1["Online test sandbox"]
D1 --> D2["make test-docker-online"]

E --> E1["Persistent playground"]
E1 --> E2["make playground"]

F --> F1["Dev profile"]
F1 --> F2["dev-docker + ui-dev"]

G --> G1{"Production or CI?"}
G1 --> G2["docker-build"]
G1 --> G3["docker/ci/Dockerfile"]

H --> H1["Devcontainer"]
H1 --> H2["Reopen in Container"]
```

Command mapping:

| Command | `mise` | `make` |
|---|---|---|
| Test (offline) | `mise run test:docker` | `make test-docker` |
| Test (online) | `mise run test:docker:online` | `make test-docker-online` |
| **Playground** (start + shell) | **`mise run playground`** | **`make playground`** |
| Playground (stop) | `mise run playground:down` | `make playground-down` |
| Sandbox (advanced) | — | `./scripts/sandbox.sh <up\|down\|shell\|reset\|status\|logs\|bare>` |
| Dev API server | `mise run dev:docker` | `make dev-docker` |
| Dev stop | `mise run dev:docker:down` | `make dev-docker-down` |
| Docker build | `mise run docker:build` | `make docker-build` |
| Docker multiarch | `mise run docker:build:multiarch` | `make docker-build-multiarch` |

## What You Can Use It For

| Mode | Best for | Network | Lifecycle |
|------|----------|---------|-----------|
| Offline test sandbox | Stable regression checks (`build + unit + integration`) | Disabled | One-shot |
| Online test sandbox | Optional remote install/update checks | Enabled | One-shot |
| Interactive playground | Manual command exploration and demos | Enabled | Persistent |
| Dev profile | Go API server in Docker + Vite HMR on host | Enabled | Persistent |
| Devcontainer | VS Code / Codespaces one-click dev environment | Enabled | Persistent |
| Production image | Lightweight deployment (`docker/production/`) | Enabled | Persistent |
| CI image | Skill validation in pipelines (`docker/ci/`) | Enabled | One-shot |

---

## Common Scenarios

### 1. Verify local install/update logic deterministically

Use this when you are changing `install` / `update` behavior and want a CI-like local gate.

```bash
mise run test:docker
make test-docker
```

This validates local-path and `file://` workflows in isolation.

### 2. Run optional remote-source checks

Use this for GitHub/remote-source validation that depends on network access.

```bash
make test-docker-online
```

### 3. Open a dedicated playground and explore all commands {#playground}

One command to start and enter the playground:

```bash
make playground
mise run playground
```

Inside the playground, `skillshare` and `ss` are ready. Both global mode and project mode are pre-initialized:

```bash
skillshare --help
ss status
skillshare list
```

### Project Mode in the Playground

The playground automatically sets up a demo project at `~/demo-project` with a sample skill and a `claude` target. You can start exploring project mode right away:

```bash
cd ~/demo-project
skillshare status        # auto-detects project mode
skillshare list
skillshare sync --dry-run
```

To launch the web dashboard, use the built-in aliases:

```bash
skillshare-ui            # global mode dashboard → http://localhost:19420
skillshare-ui-p          # project mode dashboard (~/demo-project) → http://localhost:19420
```

Then open `http://localhost:19420` on your host machine (port is mapped via Docker Compose).

### GitHub Token (for Search)

The playground automatically picks up your GitHub token from the host for `skillshare search`. It checks in order: `$GITHUB_TOKEN` → `$GH_TOKEN` → `gh auth token`. No extra setup needed if you're already authenticated on the host.

```bash
# If not detected, set it before starting the playground:
export GITHUB_TOKEN=ghp_your_token_here
make playground
```

When finished:

```bash
make playground-down
```

---

## Use Cases by Role

### Individual developers

| Scenario | What to use | What it replaces |
|----------|-------------|-----------------|
| Try skillshare without installing Go/Node | `docker run ghcr.io/runkids/skillshare` | Install Go + Node + pnpm, then build from source |
| Run full test suite before opening a PR | `make test-docker` | Depend on local toolchain (Go version mismatch = flaky results) |
| Frontend work without Go installed | `make dev-docker` + `cd ui && pnpm run dev` | Must install Go 1.25+ locally to run the API server |
| Demo skillshare to a colleague | `make playground` → Web UI on `:19420` | Walk them through a full local install |
| Verify Linux behavior on Apple Silicon | `make docker-build` | Push to CI and wait |

### Teams and open-source contributors

| Scenario | What to use | What it solves |
|----------|-------------|---------------|
| New contributor onboarding | `make playground` — one command, ready to go | No more "install Go, set PATH, clone, build" setup guide |
| Automated skill quality gate in CI | `docker run ghcr.io/.../skillshare-ci audit /skills` | Previously required installing Go + building from source in every workflow |
| "Works on my machine" across contributors | Docker pins Go 1.25.5 + all dependencies | Different local Go versions causing test flakes |
| PR reviewer reproducing an issue | `./scripts/test_docker.sh --cmd "go test -run TestXxx ..."` | Must clone + full local setup to reproduce |

### Enterprise and self-hosted deployment

| Scenario | What to use | Value |
|----------|-------------|-------|
| Internal skill management dashboard | Production image + volume mount for skills | One container, no Go/Node on the server |
| Kubernetes deployment | Production image (healthcheck + graceful shutdown + non-root) | Ready for readiness/liveness probes, passes PodSecurityPolicy |
| Automated skill PR review | CI image + `skillshare audit` in GitHub Actions | Block unsafe skills from merging — one line in your workflow |
| Container security compliance | `read_only` + `cap_drop: ALL` + `no-new-privileges` | Passes CIS Docker Benchmark, Trivy, and Aqua scans |
| ARM servers (AWS Graviton) for cost savings | `make docker-build-multiarch` | Native arm64 image, no emulation overhead |

### Quick examples

**Self-hosted dashboard with persistent skills:**

```bash
docker run -d \
  -p 19420:19420 \
  -v skillshare-data:/home/skillshare/.config/skillshare \
  ghcr.io/runkids/skillshare
```

**CI skill audit in GitHub Actions:**

```yaml
- name: Audit skills
  run: |
    docker run --rm \
      -v ${{ github.workspace }}/skills:/skills \
      ghcr.io/runkids/skillshare-ci audit /skills
```

**Kubernetes deployment (minimal):**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: skillshare
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: skillshare
          image: ghcr.io/runkids/skillshare:latest
          ports:
            - containerPort: 19420
          livenessProbe:
            httpGet:
              path: /api/health
              port: 19420
          readinessProbe:
            httpGet:
              path: /api/health
              port: 19420
          securityContext:
            runAsNonRoot: true
            readOnlyRootFilesystem: true
```

---

## Dev Profile

Two ways to develop the frontend with Vite HMR:

**With Go installed locally** (single command):

```bash
make ui-dev              # starts Go API server + Vite dev server together
# Open http://localhost:5173
```

**Without Go** (Go API runs in Docker, auto-rebuilds on Go changes):

```bash
# Terminal 1
make dev-docker          # Go API in Docker + Compose Watch (localhost:19420)

# Terminal 2
cd ui && pnpm run dev    # Vite dev server (localhost:5173, proxies /api → :19420)

# When done
make dev-docker-down
```

Both approaches give you instant HMR for `ui/` changes. The Docker variant pins the Go toolchain so backend behavior is consistent across contributors. When you edit Go files, Compose Watch detects the change, rebuilds the container, and restarts the API server automatically. Requires Docker Compose v2.22+.

**Note:** Go code changes require restarting the server when using `make ui-dev` (`Ctrl+C` and re-run). `make dev-docker` handles this automatically via Compose Watch.

---

## Devcontainer (VS Code / Codespaces)

Open the project in a ready-to-code container — no local Go, Node, or pnpm needed.

:::info Devcontainer vs Playground
Both use the same base image and demo content. The **playground** (`make playground`) is a terminal-only environment for exploring commands. The **devcontainer** adds VS Code integration (IntelliSense, debugger, extensions) for developing the skillshare codebase itself.
:::

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) running
- VS Code with the [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension installed

:::tip GitHub Codespaces
On GitHub, click **Code → Codespaces → New codespace**. The devcontainer config is picked up automatically — no local Docker or extension needed.
:::

### Getting started

1. Open the project folder in VS Code
2. Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on macOS) and select **Dev Containers: Reopen in Container**
3. Wait for the container to build (first time takes a few minutes, subsequent opens are fast)
4. Once ready, the setup script builds the binary and creates demo skills automatically

### What's included

The devcontainer reuses the same `docker/sandbox/Dockerfile` as the sandbox, so you get:

- Go 1.25 toolchain
- Node.js 24 + pnpm (via [devcontainer feature](https://github.com/devcontainers/features/tree/main/src/node)) — enables `make ui-dev` and `cd website && pnpm start` inside the container
- VS Code extensions: Go, Tailwind CSS, ESLint, Prettier
- Ports forwarded: `19420` (Web UI), `5173` (Vite HMR), `3000` (Docusaurus)
- Source code mounted at `/workspace`
- **Pre-configured demo environment** — same as the interactive playground:
  - Shortcut commands in PATH (`ss`, `ui`, `docs`)
  - Frontend dependencies pre-installed (`ui/` and `website/`)
  - Global demo skills (audit examples, deploy checklist)
  - Custom audit rules (global + project)
  - Demo project at `~/demo-project` with project-mode skills

### Quick start after container opens

```bash
ss status                 # global mode — already initialized
ss list                   # see demo skills (flat + nested)
ss audit                  # run audit with custom rules

cd ~/demo-project
ss status                 # auto-detects project mode
ss audit                  # project-level audit
ui -p                     # switch API to project mode → http://localhost:5173
```

### Frontend development

| Port | Service | Command |
|------|---------|---------|
| `5173` | Vite (React UI + HMR) | `ui` or `ui -p` |
| `19420` | Go API backend | started by `ui` / `ui -p` |
| `3000` | Docusaurus | `docs` |

```bash
ui                        # global mode: API + Vite → http://localhost:5173
ui -p                     # project mode: API + Vite → http://localhost:5173
ui stop                   # stop API + Vite
docs                      # documentation site → http://localhost:3000
docs stop                 # stop Docusaurus
```

`ui` starts both the Go API backend (port 19420, background) and Vite dev server (port 5173, HMR). Switching between `ui` and `ui -p` automatically restarts the API in the new mode. VS Code auto-forwards ports to your host browser.

### Token configuration

Tokens for private repo access (`GITHUB_TOKEN`, `GITLAB_TOKEN`, etc.) can come from multiple sources. They are checked in this order:

| Priority | Source | Setup |
|----------|--------|-------|
| 1 | `.devcontainer/.env` | Copy `.env.example` → `.env`, fill in values (gitignored) |
| 2 | Host env vars | Set in `~/.zshrc` — forwarded via `remoteEnv` in `devcontainer.json` |
| 3 | `gh auth login` | `GITHUB_TOKEN` auto-detected on container start (GitHub only) |

All sources are optional. You can also use `export` manually inside the container at any time.

Check current state:

```bash
credential-helper status
```

### Private repo testing

VS Code Dev Containers automatically forwards your host git credentials into the container. This means `git clone` of private repos may succeed even without explicit token env vars — the forwarded credential helper handles auth silently.

To disable **all** authentication (credential helper + token env vars) for testing:

```bash
eval "$(credential-helper --eval off)"    # disable everything
eval "$(credential-helper --eval on)"     # restore everything
credential-helper status                  # check current state
```

Without `--eval`, only the git credential helper is toggled (token env vars remain active).

### Running tests

```bash
make test          # unit + integration
make test-unit     # unit only
make lint          # go vet
```

---

## Production and CI Images

### Image comparison

Three Dockerfiles serve different purposes:

| | Production | CI | Sandbox |
|---|---|---|---|
| **Image** | `ghcr.io/runkids/skillshare` | `ghcr.io/runkids/skillshare-ci` | Local build only |
| **Dockerfile** | `docker/production/Dockerfile` | `docker/ci/Dockerfile` | `docker/sandbox/Dockerfile` |
| **Base** | `debian:bookworm-slim` | `debian:bookworm-slim` | `golang:1.25.5-bookworm` |
| **Includes** | git, curl, tini | git only | Go toolchain, gh, jq, air, delve, pre-built UI |
| **Non-root** | Yes (UID 10001) | No | No |
| **PID 1** | tini | default | default |
| **Healthcheck** | Yes (`/api/health`) | No | No |
| **Entrypoint** | `skillshare ui` (Web dashboard) | `skillshare` (direct CLI) | `entrypoint.sh` (test runner) |
| **Use case** | Self-hosted dashboard, Kubernetes | CI/CD skill validation | Development, testing, playground |
| **Published to GHCR** | Yes | Yes | No |
| **Multi-arch** | amd64 + arm64 | amd64 + arm64 | Host arch only |

**When to use which:**

- **Production** — Deploy the Web UI dashboard on a server or Kubernetes cluster
- **CI** — Run `audit`, `install --dry-run`, or other validation commands in GitHub Actions / GitLab CI
- **Sandbox** — Local development (`make test-docker`, `make playground`, `make dev-docker`)

### Production image

Build a lightweight production image with the embedded Web UI:

```bash
make docker-build                          # current platform only (fast, for local testing)
make docker-build-multiarch                # linux/amd64 + linux/arm64 (slow, for registry push)
```

`docker-build` only produces an image for your machine's architecture — an arm64 image from Apple Silicon won't run on x86 servers. Use `docker-build-multiarch` when pushing to a registry so any platform gets the right image automatically.

The production image uses `tini` as PID 1, runs as a non-root user (UID 10001), includes a healthcheck, and auto-initialises config on first run. Default command: `skillshare ui -g --host 0.0.0.0 --no-open`.

Published images are available on GHCR (pushed automatically on tag):

```bash
# Pull and run (auto-selects amd64 or arm64)
docker run -d -p 19420:19420 ghcr.io/runkids/skillshare

# With persistent skill data
docker run -d -p 19420:19420 \
  -v skillshare-data:/home/skillshare/.config/skillshare \
  ghcr.io/runkids/skillshare
```

### CI image

A minimal image for validating skills in CI pipelines:

```bash
docker build -f docker/ci/Dockerfile -t skillshare-ci .
docker run --rm -v ./my-skills:/skills skillshare-ci audit /skills
```

The CI image's entrypoint is `skillshare` itself, so you pass subcommands directly:

```bash
# Audit with threshold
docker run --rm -v ./skills:/skills ghcr.io/runkids/skillshare-ci audit /skills --threshold HIGH

# Dry-run install to verify a repo
docker run --rm ghcr.io/runkids/skillshare-ci install org/repo --dry-run
```

### Sandbox image

The sandbox image is for local development and testing only (not published to GHCR). It includes the full Go toolchain, development tools (air, delve), GitHub CLI, and pre-built frontend assets.

Used by: `make test-docker`, `make test-docker-online`, `make playground`, `make dev-docker`.

See the [Playground](#playground) and [Dev Profile](#dev-profile) sections above for usage.

### Image tags and versioning

On tag push (`v*`), the `docker-publish` GitHub Actions workflow builds and pushes both production and CI images to GHCR with multi-arch support.

Each image is tagged with three patterns:

| Tag pattern | Example | Description |
|---|---|---|
| `v<major>.<minor>.<patch>` | `v0.16.1` | Exact version (immutable) |
| `<major>.<minor>` | `0.16` | Latest patch for this minor version (rolling) |
| `sha-<short>` | `sha-153464a` | Git commit SHA (immutable) |

:::tip
Use exact version tags (`v0.16.1`) in production for reproducibility. Use minor tags (`0.16`) to get patch updates automatically. Use `sha-` tags to pin to a specific commit.
:::

Browse published versions at [GitHub Packages](https://github.com/runkids/skillshare/pkgs/container/skillshare).

---

## Limits and Expectations

- **Playground and dev profile share port 19420** — run only one at a time. Stop the other first (`make playground-down` or `make dev-docker-down`).
- Offline sandbox cannot validate network-dependent features (for example remote `install` from GitHub).
- Playground uses container-local `HOME`, so it does not directly modify your real host home config.
- Go code changes are picked up automatically (`go build` runs inside the container from mounted source). **Frontend (`ui/`) changes** are picked up instantly when running `make ui-dev` (Vite HMR) inside the devcontainer. For the playground, run `make ui-build` on the host first since the playground container does not include Node.js.
- If you need custom experiments, pass commands directly:

```bash
./scripts/test_docker.sh --cmd "go test -v ./tests/integration/..."
./scripts/sandbox_playground_shell.sh "skillshare list"
```

---

## See Also

- [Getting Started](/docs/getting-started) — Standard setup
- [Commands Reference](/docs/reference/commands) — All commands
- [Troubleshooting](/docs/troubleshooting) — Common issues
