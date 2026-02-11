---
sidebar_position: 1
---

# Targets

Targets are AI CLI skill directories that skillshare syncs to.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        TARGETS                              │
│                                                             │
│   Source ─────────────► sync ─────────────► Targets         │
│                                                             │
│                         ┌───────────────────────────────┐   │
│                         │  claude    ~/.claude/skills   │   │
│                         │  cursor    ~/.cursor/skills   │   │
│                         │  codex     ~/.codex/skills    │   │
│                         │  gemini    ~/.gemini/skills   │   │
│                         │  ...       44+ supported      │   │
│                         └───────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Quick Links

| Topic | Description |
|-------|-------------|
| [Supported Targets](./supported-targets) | Complete list of 44+ supported AI CLIs |
| [Adding Custom Targets](./adding-custom-targets) | Add any tool with a skills directory |
| [Configuration](./configuration) | Config file reference |

---

## Common Operations

### List targets

```bash
skillshare target list
```

### Show target details

```bash
skillshare target claude
```

### Change sync mode

```bash
skillshare target claude --mode symlink
skillshare sync
```

### Add custom target

```bash
skillshare target add myapp ~/.myapp/skills
skillshare sync
```

### Remove target

```bash
skillshare target remove claude
```

---

## Auto-Detection

When running `skillshare init`, installed AI CLIs are automatically detected and added as targets.

Only paths that exist are added. See [Supported Targets](./supported-targets) for the full list of checked paths.

---

## Related

- [Source & Targets](/docs/concepts/source-and-targets) — Core concepts
- [Sync Modes](/docs/concepts/sync-modes) — Merge vs symlink
- [Commands: target](/docs/commands/target) — Target command details
