# Contributing to skillshare

Thanks for your interest in contributing! This guide helps you get started.

## How to Contribute

### 1. Open an Issue First

Before writing code, [open an issue](https://github.com/runkids/skillshare/issues/new) to describe what you'd like to change and why. This helps us:

- Align on the scope and approach
- Avoid duplicate effort
- Discuss alternative solutions early

Even small changes benefit from a quick issue — it gives context for reviewers and future contributors.

### 2. Submit a Draft PR

If you'd like to propose an implementation, open a [draft pull request](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests#draft-pull-requests) and link it to the issue. Draft PRs let us:

- Collaborate on the approach before investing time in polish
- Catch design mismatches early
- Provide incremental feedback

> **Note:** Due to the nature of this project, most PRs won't be merged directly — but every contribution is valuable. Your draft PR serves as a concrete reference that shapes the final implementation.

### 3. Include Tests

PRs with test coverage are much easier to review and merge. skillshare has both unit and integration tests:

- **Unit tests**: alongside source files (`*_test.go`)
- **Integration tests**: `tests/integration/` using `testutil.Sandbox`

Run the full check before submitting:

```bash
make check          # format + lint + unit + integration tests
```

## Development Setup

### Option A: Dev Containers (Recommended)

Open in [Dev Containers](https://containers.dev/) — Go toolchain, Node.js, pnpm, and demo content are pre-configured.

### Option B: Local

Requirements: Go 1.23+

```bash
git clone https://github.com/runkids/skillshare.git
cd skillshare
make build          # build binary
make check          # format + lint + test
```

### Useful Commands

```bash
make build          # build binary → bin/skillshare
make test           # unit + integration tests
make test-unit      # unit tests only
make lint           # go vet
make fmt            # gofmt
make check          # fmt + lint + test (must pass before PR)
make ui-dev         # Go API server + Vite HMR for Web UI
```

## PR Checklist

- [ ] Linked to an issue
- [ ] Tests included and passing (`make check`)
- [ ] No unrelated changes in the diff
- [ ] Commit messages explain "why", not just "what"

## Questions?

Not sure where to start? Browse [open issues](https://github.com/runkids/skillshare/issues) or open a new one to discuss your idea.
