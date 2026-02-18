---
sidebar_position: 2
---

# Supported Targets

Complete list of AI CLIs that skillshare supports out of the box.

## Overview

Skillshare supports **48+ AI CLI tools**. When you run `skillshare init`, it automatically detects and configures any installed tools.

---

## Built-in Targets

These are auto-detected during `skillshare init`:

```text
┌────────────────────────────────────────────────────────────────────────────┐
│                              SUPPORTED TARGETS                             │
│                                                                            │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────┐ ┌─────────┐           │
│  │   AdaL  │ │  Agents │ │   Amp   │ │ Antigravity │ │ Augment │           │
│  └─────────┘ └─────────┘ └─────────┘ └─────────────┘ └─────────┘           │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────┐ ┌─────────┐             │
│  │   Bob   │ │  Claude │ │  Cline  │ │ CodeBuddy │ │  COMATE │             │
│  └─────────┘ └─────────┘ └─────────┘ └───────────┘ └─────────┘             │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐ ┌─────────┐             │
│  │  Codex  │ │ Cmd Code │ │ Continue │ │ Copilot │ │  Cortex │             │
│  └─────────┘ └──────────┘ └──────────┘ └─────────┘ └─────────┘             │
│  ┌───────┐ ┌────────┐ ┌─────────────┐ ┌───────┐ ┌────────┐ ┌───────┐       │
│  │ Crush │ │ Cursor │ │ Deep Agents │ │ Droid │ │ Gemini │ │ Goose │       │
│  └───────┘ └────────┘ └─────────────┘ └───────┘ └────────┘ └───────┘       │
│  ┌───────┐ ┌───────┐ ┌──────────┐ ┌────────┐ ┌────────┐ ┌────────┐         │
│  │ iFlow │ │ Junie │ │ Kilocode │ │  Kimi  │ │  Kiro  │ │  Kode  │         │
│  └───────┘ └───────┘ └──────────┘ └────────┘ └────────┘ └────────┘         │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌────────────┐            │
│  │  Letta  │ │  MCPJam │ │   Mux   │ │ Neovate │ │ OpenClaude │            │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └────────────┘            │
│  ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌─────────┐ ┌─────────┐           │
│  │ OpenClaw │ │ OpenCode │ │ OpenHands │ │    Pi   │ │  Pochi  │           │
│  └──────────┘ └──────────┘ └───────────┘ └─────────┘ └─────────┘           │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │  Qoder  │ │   Qwen  │ │   Roo   │ │   Trae  │ │ Trae CN │ │   Vibe  │   │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘   │
│  ┌──────────┐ ┌──────────────┐ ┌─────────────┐ ┌──────────┐                │
│  │ Windsurf │ │ Xcode Claude │ │ Xcode Codex │ │ Zencoder │                │
│  └──────────┘ └──────────────┘ └─────────────┘ └──────────┘                │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Target Paths

| Target | Global Path | Project Path |
|--------|-------------|--------------|
| adal | `~/.adal/skills` | `.adal/skills` |
| agents | `~/.config/agents/skills` | `.agents/skills` |
| amp | `~/.config/agents/skills` | `.agents/skills` |
| antigravity | `~/.gemini/antigravity/skills` | `.agent/skills` |
| augment | `~/.augment/skills` | `.augment/skills` |
| bob | `~/.bob/skills` | `.bob/skills` |
| claude | `~/.claude/skills` | `.claude/skills` |
| cline | `~/.cline/skills` | `.cline/skills` |
| codebuddy | `~/.codebuddy/skills` | `.codebuddy/skills` |
| comate | `~/.comate/skills` | `.comate/skills` |
| codex | `~/.codex/skills` | `.agents/skills` |
| commandcode | `~/.commandcode/skills` | `.commandcode/skills` |
| continue | `~/.continue/skills` | `.continue/skills` |
| cortex | `~/.snowflake/cortex/skills` | `.cortex/skills` |
| copilot | `~/.copilot/skills` | `.github/skills` |
| crush | `~/.config/crush/skills` | `.crush/skills` |
| cursor | `~/.cursor/skills` | `.cursor/skills` |
| deepagents | `~/.deepagents/agent/skills` | `.deepagents/skills` |
| droid | `~/.factory/skills` | `.factory/skills` |
| gemini | `~/.gemini/skills` | `.gemini/skills` |
| goose | `~/.config/goose/skills` | `.goose/skills` |
| iflow | `~/.iflow/skills` | `.iflow/skills` |
| junie | `~/.junie/skills` | `.junie/skills` |
| kilocode | `~/.kilocode/skills` | `.kilocode/skills` |
| kimi | `~/.kimi/skills` | `.agents/skills` |
| kiro | `~/.kiro/skills` | `.kiro/skills` |
| kode | `~/.kode/skills` | `.kode/skills` |
| letta | `~/.letta/skills` | `.skills` |
| mcpjam | `~/.mcpjam/skills` | `.mcpjam/skills` |
| mux | `~/.mux/skills` | `.mux/skills` |
| neovate | `~/.neovate/skills` | `.neovate/skills` |
| openclaude | `~/.openclaude/skills` | — |
| openclaw | `~/.openclaw/skills` | `skills` |
| opencode | `~/.config/opencode/skills` | `.opencode/skills` |
| openhands | `~/.openhands/skills` | `.openhands/skills` |
| pi | `~/.pi/agent/skills` | `.pi/skills` |
| pochi | `~/.pochi/skills` | `.pochi/skills` |
| qoder | `~/.qoder/skills` | `.qoder/skills` |
| qwen | `~/.qwen/skills` | `.qwen/skills` |
| replit | — | `.agents/skills` |
| roo | `~/.roo/skills` | `.roo/skills` |
| trae | `~/.trae/skills` | `.trae/skills` |
| trae-cn | `~/.trae-cn/skills` | `.trae/skills` |
| vibe | `~/.vibe/skills` | `.vibe/skills` |
| windsurf | `~/.codeium/windsurf/skills` | `.windsurf/skills` |
| xcode-claude | `~/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig/skills` | `.claude/skills` |
| xcode-codex | `~/Library/Developer/Xcode/CodingAssistant/codex/skills` | `.codex/skills` |
| zencoder | `~/.zencoder/skills` | `.zencoder/skills` |

## Aliases

Some targets have alternative names for backward compatibility or convenience:

| Alias | Resolves To | Notes |
|-------|-------------|-------|
| `claude-code` | `claude` | Legacy name |
| `command-code` | `commandcode` | Hyphenated variant |
| `github-copilot` | `copilot` | Full product name |
| `opencode-ai` | `opencode` | With suffix |
| `trae-cn` | `traecn` | Hyphenated variant |

You can use either the alias or the canonical name in all commands:

```bash
skillshare target add claude           # canonical
skillshare target add claude-code      # alias — same result
```

Aliases are resolved automatically. The canonical name is used in config files and status output.

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

See [Adding Custom Targets](./adding-custom-targets.md) for details.

---

## Related

- [Adding Custom Targets](./adding-custom-targets.md) — Add unsupported tools
- [Configuration](./configuration.md) — Config file reference
- [Commands: target](/docs/commands/target) — Target command
