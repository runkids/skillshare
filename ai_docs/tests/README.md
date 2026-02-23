# AI Testing Guide (Devcontainer + ssenv)

This guide is for AI agents running tests/verification in this repository.

## Non-negotiable rules

1. Always run tests inside devcontainer.
2. Always use an isolated `ssenv` environment for runtime verification.
3. Never rely on host `HOME` for CLI behavior checks.
4. End each run with explicit cleanup (`ssenv delete <name> --force`).

## Required bootstrap

From repository root:

```bash
docker compose -f .devcontainer/docker-compose.yml up -d
docker exec -it skillshare_devcontainer-skillshare-devcontainer-1 bash
```

Inside container:

```bash
/workspace/.devcontainer/ensure-skillshare-linux-binary.sh
sshelp
```

## Standard AI workflow

Use this pattern for every verification run:

```bash
ENV_NAME="ai-test-$(date +%Y%m%d-%H%M%S)"
ssenv create "$ENV_NAME"
ssenv enter "$ENV_NAME" -- ss init --no-copy --no-targets --no-git --no-skill
ssenv enter "$ENV_NAME" -- ss doctor
ssenv delete "$ENV_NAME" --force
```

Notes:

- For interactive debugging, use `ssuse <env>` or `ssnew <env>`, then `exit` when done.
- For deterministic automation, prefer `ssenv enter <env> -- <command>` one-liners.
- If `ssenv delete <active-env> --force` is called from an eval-switched shell, it auto-returns to `/workspace`.

## Test command policy

Run in devcontainer unless there is a documented exception.

```bash
go build -o bin/skillshare ./cmd/skillshare
SKILLSHARE_TEST_BINARY="$PWD/bin/skillshare" go test ./tests/integration -count=1
go test ./...
```

## Quick commands reference

- `sshelp`: show shortcuts and usage.
- `ssls`: list isolated environments.
- `ssnew <name>`: create + enter isolated shell.
- `ssuse <name>`: enter existing isolated shell.
- `ssback`: leave isolated context helper.
- `ssenv shortcuts`: print the same shortcuts reference.

## Related runbook

- `ai_docs/tests/first_use_cli_e2e_runbook.md`
