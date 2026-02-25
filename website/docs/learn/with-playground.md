---
sidebar_position: 6
---

# Try skillshare in the Playground

> A pre-configured Docker sandbox with demo skills, audit rules, and a project — ready to explore in seconds.

## Prerequisites

- Docker and Docker Compose installed
- Clone the skillshare repo: `git clone https://github.com/runkids/skillshare.git`

## Start the Playground

```bash
cd skillshare
make playground
```

This single command:

1. Builds the sandbox Docker image (Go toolchain included)
2. Compiles the `skillshare` binary inside the container
3. Initializes global mode with all targets auto-detected
4. Creates demo skills (clean, warning, critical) across categories
5. Sets up a demo project with project-level skills and custom audit rules
6. Drops you into an interactive shell — ready to explore

## What's Inside

### Demo Skills (Global)

| Skill | Category | Audit Findings |
|-------|----------|----------------|
| `audit-demo-clean` | root | None (clean baseline) |
| `deploy-checklist` | `devops/` | None |
| `audit-demo-ci-release` | `security/` | HIGH + MEDIUM (sudo, external URLs) |
| `audit-demo-debug-exfil` | `security/` | CRITICAL (credential exfiltration) |
| `audit-demo-external-link` | `security/` | LOW (external URLs) |
| `audit-demo-dangling-link` | `security/` | LOW (broken local links) |

### Demo Project (`~/demo-project`)

A pre-configured `.skillshare/` project with:
- `hello-world` — clean project skill
- `demos/audit-demo-release` — release helper with audit warnings
- `guides/code-review` — nested code review guide
- Custom `audit-rules.yaml` with a TODO/FIXME policy rule

### Custom Audit Rules

Both global and project level `audit-rules.yaml` are pre-configured so you can see how rule customization works — enable/disable rules, add custom patterns, set allowlists.

## Things to Try

```bash
# Check what's installed
skillshare status
skillshare list

# Run a security audit — see findings across severity levels
skillshare audit

# Try project mode
cd ~/demo-project
skillshare status          # auto-detects project mode
skillshare audit           # project-level scan with custom rules

# Launch the web dashboard (port 19420)
skillshare-ui              # global mode
skillshare-ui-p            # project mode

# Explore nested skills
ls ~/.config/skillshare/skills/security/
ls ~/.config/skillshare/skills/devops/
```

## Bare Mode

Start with a clean slate — no auto-init, no demo content:

```bash
./scripts/sandbox_playground_up.sh --bare
./scripts/sandbox_playground_shell.sh
```

Useful for testing `skillshare init` from scratch.

## Stop the Playground

```bash
make playground-down
```

Data persists in a Docker volume (`playground-home`). Next `make playground` picks up where you left off.

## Architecture

The playground runs in a **read-only** Docker container with security hardening:

- `read_only: true` — filesystem is immutable except for designated volumes
- `cap_drop: ALL` — no Linux capabilities
- `no-new-privileges` — prevents privilege escalation
- Writable volumes: `/sandbox-home` (persistent), `/tmp` (tmpfs, 256 MB)
- Port `19420` forwarded for the web dashboard

The workspace is mounted read-only from the host repo — you can edit code on your machine and rebuild inside the container.

## What's Next?

- [Getting started →](/docs/getting-started)
- [Security audit guide →](/docs/how-to/advanced/security)
- [Docker sandbox guide →](/docs/how-to/advanced/docker-sandbox)
