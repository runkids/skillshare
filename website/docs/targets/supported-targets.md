---
sidebar_position: 2
---

# Supported Targets

Complete list of AI CLIs that skillshare supports out of the box.

## Overview

Skillshare supports **44+ AI CLI tools**. When you run `skillshare init`, it automatically detects and configures any installed tools.

---

## Built-in Targets

These are auto-detected during `skillshare init`:

```text
┌────────────────────────────────────────────────────────────────────────────┐
│                           SUPPORTED TARGETS                                │
│                                                                            │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────┐ ┌─────────┐ ┌─────────┐ │
│  │  AdaL   │ │ Agents  │ │   Amp   │ │Antigravity│ │ Augment │ │   Bob   │ │
│  └─────────┘ └─────────┘ └─────────┘ └───────────┘ └─────────┘ └─────────┘ │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌─────────┐ ┌─────────┐              │
│  │ Claude  │ │  Cline  │ │CodeBuddy │ │  Codex  │ │Cmd Code │              │
│  └─────────┘ └─────────┘ └──────────┘ └─────────┘ └─────────┘              │
│  ┌──────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐              │
│  │ Continue │ │ Copilot │ │  Crush  │ │ Cursor  │ │  Droid  │              │
│  └──────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐              │
│  │ Gemini  │ │  Goose  │ │  iFlow  │ │  Junie  │ │ Kilocode │              │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └──────────┘              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐               │
│  │  Kimi   │ │  Kiro   │ │  Kode   │ │  Letta  │ │ MCPJam  │               │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘               │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐ ┌─────────┐ ┌──────────┐           │
│  │   Mux   │ │ Neovate  │ │OpenClaude │ │OpenClaw │ │ OpenCode │           │
│  └─────────┘ └──────────┘ └───────────┘ └─────────┘ └──────────┘           │
│  ┌───────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐             │
│  │ OpenHands │ │   Pi    │ │  Pochi  │ │  Qoder  │ │  Qwen   │             │
│  └───────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘             │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐              │
│  │   Roo   │ │  Trae   │ │ Trae CN │ │  Vibe   │ │ Windsurf │              │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └──────────┘              │
│  ┌──────────┐                                                              │
│  │ Zencoder │                                                              │
│  └──────────┘                                                              │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Target Paths

| Target | Skills Path |
|--------|-------------|
| adal | `~/.adal/skills` |
| agents | `~/.agents/skills` |
| amp | `~/.amp/skills` |
| antigravity | `~/.antigravity/skills` |
| augment | `~/.augment/skills` |
| bob | `~/.bob/skills` |
| claude | `~/.claude/skills` |
| cline | `~/.cline/skills` |
| codebuddy | `~/.codebuddy/skills` |
| codex | `~/.codex/skills` |
| cmdcode | `~/.cmdcode/skills` |
| continue | `~/.continue/skills` |
| copilot | `~/.github-copilot/skills` |
| crush | `~/.crush/skills` |
| cursor | `~/.cursor/skills` |
| droid | `~/.droid/skills` |
| gemini | `~/.gemini/skills` |
| goose | `~/.goose/skills` |
| iflow | `~/.iflow/skills` |
| junie | `~/.junie/skills` |
| kilocode | `~/.kilocode/skills` |
| kimi | `~/.kimi/skills` |
| kiro | `~/.kiro/skills` |
| kode | `~/.kode/skills` |
| letta | `~/.letta/skills` |
| mcpjam | `~/.mcpjam/skills` |
| mux | `~/.mux/skills` |
| neovate | `~/.neovate/skills` |
| openclaude | `~/.openclaude/skills` |
| openclaw | `~/.openclaw/skills` |
| opencode | `~/.opencode/skills` |
| openhands | `~/.openhands/skills` |
| pi | `~/.pi/skills` |
| pochi | `~/.pochi/skills` |
| qoder | `~/.qoder/skills` |
| qwen | `~/.qwen/skills` |
| roo | `~/.roo/skills` |
| trae | `~/.trae/skills` |
| trae-cn | `~/.trae-cn/skills` |
| vibe | `~/.vibe/skills` |
| windsurf | `~/.windsurf/skills` |
| zencoder | `~/.zencoder/skills` |

---

## Check Target Path

For any target, run:

```bash
skillshare target <name>
```

---

## Custom Targets

Don't see your AI CLI? Add it manually:

```bash
skillshare target add myapp ~/.myapp/skills
```

See [Adding Custom Targets](./adding-custom-targets) for details.

---

## Related

- [Adding Custom Targets](./adding-custom-targets) — Add unsupported tools
- [Configuration](./configuration) — Config file reference
- [Commands: target](/docs/commands/target) — Target command
