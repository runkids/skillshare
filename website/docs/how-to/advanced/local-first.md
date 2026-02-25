---
sidebar_position: 10
---

# Centralized vs Local-First

Skill management tools generally follow one of two architectural approaches: **centralized platforms** or **local-first**. Neither is universally better — each comes with trade-offs. This page walks through both so you can decide which fits your workflow.

:::tip This is not a feature comparison
For feature-level differences (install flow, config format, etc.), see [Comparing Skill Management Approaches](/docs/understand/philosophy/comparison). This page focuses on **architectural trade-offs** — where your data lives, how discovery works, and what you control.
:::

## Two Approaches

### Centralized Platform

A centralized platform hosts a unified registry where skills are published, searched, and ranked. Install activity is aggregated into community metrics like download counts and trending rankings.

**Strengths**:
- Built-in discovery — browse, search, and compare skills in one place
- Community signals — download counts and trending help surface popular skills
- Low friction — no setup required for discovery; just search and install

**Considerations**:
- Install activity is tracked by the platform
- Ranking and counting rules are managed by the platform operator

### Local-First (skillshare)

skillshare keeps all state on your machine. Skills are installed via `git clone` and managed through a local config file. Nothing is sent to a remote server.

**Strengths**:
- Zero telemetry — no install tracking, no data sent anywhere
- Full ownership — your skills live in your own filesystem
- Works offline after initial install
- Single binary, no runtime dependency

**Considerations**:
- No built-in community metrics (download counts, trending)
- Discovery requires setting up or connecting to a hub

## Discovery

Local-first doesn't mean no discovery. skillshare provides three discovery channels:

| Channel | How it works |
|---------|-------------|
| **GitHub search** | `skillshare search <query>` — searches public GitHub repos directly |
| **Public hub** | `skillshare search --hub` — queries the built-in [community hub](https://github.com/runkids/skillshare-hub) |
| **Custom hub** | `skillshare search --hub <url>` — queries any hub you or your organization maintains |

### What Is a Hub?

A hub is a static JSON file (`skillshare-hub.json`) that lists skills with their name, description, source, and tags. It can live anywhere — a Git repo, an HTTP server, or a local filesystem:

```bash
# Build an index from your installed skills
skillshare hub index

# Search an organization's internal hub
skillshare search --hub https://internal.corp/skills/hub.json

# Search a local index file
skillshare search --hub ./skillshare-hub.json
```

Hubs are independent — anyone can create one, and users can connect to multiple hubs simultaneously. This makes them well-suited for organizations that need to maintain private skill catalogs alongside public ones.

See the [Hub Index Guide](/docs/how-to/sharing/hub-index) for a detailed walkthrough.

### Self-Hosted Metrics

skillshare itself doesn't track installs, but if you host a hub on your own server, you can add whatever analytics layer makes sense for you:

1. Host `skillshare-hub.json` on your server
2. Add request logging or a lightweight analytics endpoint
3. Track search hits, install referrals, or any metric you care about

This lets skill authors or organizations measure adoption on their own terms.

## Choosing the Right Approach

**A centralized platform may be a better fit if:**
- You want built-in community metrics and trending out of the box
- You prefer a single browsing destination for discovering skills
- You only use one AI CLI and don't need cross-tool sync

**Local-first may be a better fit if:**
- You use multiple AI CLIs and want unified management
- You prefer that install activity stays on your machine
- You need offline operation or work in restricted network environments
- You're an organization that needs control over which skills are available and discoverable

---

## See Also

- [Comparing Skill Management Approaches](/docs/understand/philosophy/comparison) — Feature-level comparison
- [Hub Index Guide](/docs/how-to/sharing/hub-index) — Building and using skill hubs
- [hub command](/docs/reference/commands/hub) — Hub command reference
- [Security Guide](./security.md) — Skill security scanning
