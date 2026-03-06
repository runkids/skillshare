---
slug: e2e-testing-for-ai-agents
title: "What I Learned Building a Testing Infrastructure for AI-Driven CLI Development"
authors: [runkids]
tags: [testing, ai-agents, devcontainer, e2e]
---

Over the past few months building [skillshare](https://github.com/runkids/skillshare), I've been experimenting with letting AI agents (Claude Code, mainly) handle more and more of the development cycle — writing features, fixing bugs, even running verification.

Along the way I learned some things about what makes AI agents productive (and what doesn't). Sharing here in case it's useful to others building CLI tools.

<!-- truncate -->

## The Problem I Kept Hitting

AI agents write code fast. But "fast" doesn't mean "correct." I kept finding myself manually verifying things the agent said were done — re-running commands, checking files, eyeballing output. The bottleneck wasn't code generation. **It was verification.**

So I started building infrastructure to let the agent verify its own work.

## Three Layers of Testing

```
        E2E (Docker Sandbox)
       ─────────────────────────
      Integration (testutil.Sandbox)
     ─────────────────────────────────
    Unit Tests (go test)
```

Nothing revolutionary here. The key was making **every layer runnable by the agent itself** — no human in the loop for verification.

### Layer 1: Unit Tests

Standard `go test` for pure functions. AI agents can write and run these instantly.

```bash
go test ./internal/sync/...
go test ./internal/audit/...
```

### Layer 2: Integration Tests with `testutil.Sandbox`

This is where things started clicking. `testutil.Sandbox` creates a **fully isolated fake HOME directory** for each test — complete with `.claude/`, `.cursor/`, and 50+ other AI CLI target directories.

```go
sb := testutil.NewSandbox(t)
defer sb.Cleanup()

// Create a skill in the source directory
sb.CreateSkill("my-skill", map[string]string{
    "SKILL.md": "---\nname: my-skill\n---\n# Content",
})

// Run the CLI and verify
result := sb.RunCLI("sync")
result.AssertSuccess()
```

Why this matters for AI agents:

- **No side effects** — each test gets a fresh environment, agents can't break the host
- **Fast feedback** — `go test -run TestXxx` takes seconds
- **Self-contained** — no external dependencies, no network, no cleanup needed

The agent writes a feature, adds an integration test, runs it, gets immediate pass/fail. No human needed.

### Layer 3: E2E with Docker Sandbox

Integration tests run on the host, which may have a different environment than real users. So the top layer is a **Docker-based devcontainer** — a clean Debian environment that simulates a real user's first experience.

```bash
make devc          # Start devcontainer + enter shell (one step)
make devc-reset    # Full reset if something breaks
```

Inside the devcontainer, the AI agent can run any command in complete isolation — install skills from Git repos, sync to targets, run the full CLI — without any risk to the host environment.

## The Thing I Didn't Expect: Runbooks

I started writing **E2E runbooks** — step-by-step test scripts in `ai_docs/tests/` — mostly for my own documentation. Turns out they're exactly what AI agents need:

```markdown
## Test: Install a tracked skill

### Setup
1. Start with a clean environment

### Steps
1. Run: `skillshare install runkids/skillshare`
2. Verify: skill directory exists in `~/.config/skillshare/skills/_runkids__skillshare/`
3. Verify: SKILL.md contains expected frontmatter
4. Run: `skillshare list`
5. Verify: output shows the installed skill with [tracked] badge
6. Run: `skillshare sync`
7. Verify: symlinks exist in all enabled targets

### Expected Result
- Skill is installed, tracked, and synced to all targets
```

Explicit steps + expected results = no ambiguity. The agent stops guessing and just follows the script. **The runbook became the spec.**

## Eating Our Own Dog Food

The part I'm most happy about. The runbooks and devcontainer don't work by magic — the AI agent needs to know _how_ to use them. That's where **built-in skills** come in.

skillshare uses its own skill system to teach AI agents the entire workflow:

```
.skillshare/skills/
  devcontainer/SKILL.md    # How to enter, run commands, use ssenv isolation
  cli-e2e-test/SKILL.md    # How to pick a runbook, execute it, report results
```

The **devcontainer skill** teaches the agent:
- All CLI execution must happen inside the container (`docker exec`, never on host)
- Use `ssenv` for HOME isolation — each test gets a clean `~/.config/skillshare/`
- Zero-rebuild workflow — edit code on host, `docker exec` picks up changes instantly
- Common mistakes to avoid (wrong arch, forgetting `cd /workspace`, etc.)

The **cli-e2e-test skill** orchestrates the full E2E flow:
- Phase 0: Check devcontainer is running
- Phase 1: Detect which runbooks are relevant to recent code changes
- Phase 2: Let the agent pick or auto-generate a test
- Phase 3: Execute steps with `ssenv` isolation, verify each assertion
- Phase 4: Report results, clean up, and run a retrospective

A skill management tool that uses its own skill system to teach AI agents how to develop and test itself. When I improve the testing workflow, I update the skill. Next time the agent picks up a task, it gets the updated instructions. No stale READMEs.

## The Full Loop

Here's the development cycle the agent runs autonomously:

```
Spec / Issue
    |
    v
Write code
    |
    v
Run unit tests (go test ./internal/...)
    |
    v
Run integration tests (go test ./tests/integration/ -run TestXxx)
    |
    v
Enter devcontainer (make devc)
    |
    v
Execute E2E runbook
    |
    v
All green? -> Open PR
```

My job becomes:

1. **Write the spec** — what should the feature do?
2. **Write the runbook** — how do we verify it works?
3. **Review the PR** — does the implementation make sense?

## Why CLIs Make This Possible

This workflow works _because_ skillshare is a CLI:

- **Text in, text out** — AI agents can read command output and verify assertions naturally
- **Deterministic behavior** — same input produces same output, no UI flakiness
- **Container-friendly** — CLIs run perfectly in Docker, no display server or browser needed
- **Composable** — agents can chain commands, pipe output, and script complex scenarios

GUIs need screenshot comparison, browser automation, and flaky selectors. CLIs need `assert output contains "success"`. The verification surface is fundamentally simpler.

## What I'd Tell Someone Starting Out

1. **Invest in test isolation early** — agents need a sandbox they can't break
2. **Write runbooks** — explicit verification beats vague expectations
3. **Teach agents your workflow via skills/context files** — not just your API
4. **Use containers** — cheaper than debugging environment differences
5. **Add `--json` flags** — structured output lets agents verify programmatically

## Honest Takeaway

AI agents are fast, confident, and wrong in new ways. I don't assume they write perfect code — I assume they can fix their own mistakes, **as long as the feedback loop is fast enough.** Runbooks, sandboxes, and built-in skills are that feedback loop.

Still learning and iterating on this. If you're building CLI tools with AI agents, I'd love to hear what's working for you.
