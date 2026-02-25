---
sidebar_position: 1
---

# Why Local-First

> skillshare is a single binary with no runtime dependencies. Here's why.

## The Decision

skillshare ships as a single Go binary. No Node.js, no Python, no package manager, no daemon. Install it, run it, done.

This wasn't the path of least resistance — it was a deliberate choice driven by three principles.

## Principle 1: Zero Dependency Chain

Every dependency is an attack surface and a maintenance burden.

If skillshare required Node.js, you'd need to manage Node versions, deal with `node_modules`, handle platform-specific native modules, and trust the entire npm supply chain. For a tool that manages AI skills — which are themselves untrusted content — adding an untrusted dependency chain is unacceptable.

Go compiles to a static binary. The dependency chain ends at compile time. What you download is what you run.

## Principle 2: Works Everywhere the Same Way

skillshare runs on:
- macOS (Intel and Apple Silicon)
- Linux (amd64 and arm64)
- Windows (amd64)
- Docker containers (no special setup)
- CI/CD pipelines (no language runtime needed)
- Dev containers and Codespaces

A single binary means identical behavior across all platforms. No "works on my machine" debugging. No CI environment drift.

## Principle 3: Offline by Default

skillshare's core operations — `sync`, `list`, `status`, `backup`, `restore` — work without network access. Only operations that explicitly need a remote (`install`, `search`, `check`, `update`, `push`, `pull`) require connectivity.

This matters for:
- **Air-gapped environments**: Defense, healthcare, and financial institutions often restrict network access
- **Unreliable connections**: Trains, planes, conference WiFi
- **Speed**: Local operations complete in milliseconds, not seconds

## Why Not a Package Manager Plugin?

We considered shipping as an npm package, a Homebrew formula (which we now support as an additional channel), or a pip package. Each had the same problem: they add a runtime dependency that skillshare's users may not have or want.

A developer using Cursor on Windows shouldn't need to install Homebrew. A CI pipeline running Alpine Linux shouldn't need Node.js. The tool should adapt to the user's environment, not the other way around.

## The Trade-Off

The single-binary approach has costs:

- **Build complexity**: Cross-compilation for 6+ targets, CGO disabled
- **Update mechanism**: No `npm update` — skillshare has its own `upgrade` command
- **UI delivery**: The web dashboard can't bundle with the binary (too large), so it's downloaded at runtime and cached

We accept these trade-offs because they keep the user experience simple: download, run, done.

## Related

- [Security-First Design](/docs/understand/philosophy/security-first)
- [Comparison with other tools](/docs/understand/philosophy/comparison)
