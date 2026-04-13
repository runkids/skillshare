# Contributing to skillshare

Thanks for your interest in contributing! This guide helps you get started.

## Start with an Issue

The best way to contribute is to [open an issue](https://github.com/runkids/skillshare/issues/new). Issues are where ideas, feature requests, and design discussions happen. This helps us:

- Align on whether the change fits the project direction
- Agree on scope and approach before any code is written
- Avoid investing time in work that may not be merged

**Please open an issue before writing code**, even if you're confident in the approach. Skipping this step is the most common reason contributions can't be accepted.

## Pull Requests

### What PRs are good for

PRs work best for **small, focused changes**:

- Bug fixes with a clear reproduction
- Typo and documentation corrections
- Small improvements (a few files, under ~200 lines of meaningful change)

These can go straight to a PR (still link to an issue if one exists).

### Feature Ideas

For new features or large changes:

1. **Open an issue** to describe the idea and the problem it solves
2. **Submit a proposal** — copy [`proposals/TEMPLATE.md`](proposals/TEMPLATE.md), fill it in, and open a PR to `proposals/`

Approved proposals will be added to the roadmap. Implementation is handled by the maintainer to ensure consistency with the project's architecture and codebase conventions.

> **Note:** Due to the nature of this project, most feature PRs won't be merged directly — but every contribution is valuable. Your PR serves as a concrete reference that shapes the final implementation.

### PR Checklist

- [ ] Linked to an issue (required for features, recommended for bug fixes)
- [ ] Tests included and passing (`make check`)
- [ ] No unrelated changes in the diff
- [ ] Commit messages explain "why", not just "what"
- [ ] Scope is focused — one concern per PR

## Development Setup

All development and testing should be done inside the **devcontainer**. This ensures a consistent environment (Go toolchain, Node.js, pnpm, and demo content are pre-configured).

```bash
git clone https://github.com/runkids/skillshare.git
cd skillshare
make devc            # start devcontainer + enter shell (one step)
```

Once inside the devcontainer:

```bash
make build           # build binary → bin/skillshare
make test            # unit + integration tests
make check           # fmt + lint + test (must pass before PR)
make ui-dev          # Go API server + Vite HMR for Web UI
```

Other devcontainer commands:

```bash
make devc-down       # stop devcontainer
make devc-restart    # restart devcontainer
make devc-reset      # full reset (remove volumes)
make devc-status     # show devcontainer status
```

## Questions?

Not sure where to start? Browse [open issues](https://github.com/runkids/skillshare/issues) or open a new one to discuss your idea.
