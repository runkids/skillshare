---
slug: e2e-testing-for-ai-agents
title: "How We Let AI Agents Develop and Verify Our CLI — E2E Runbooks + Docker Sandbox"
authors: [runkids]
tags: [testing, ai-agents, devcontainer, e2e]
---

CLIs are the lowest-friction interface for AI agents. Text in, text out, no GUI to wrestle with. But there's a less obvious advantage: **CLIs are also the best thing for agents to _develop and verify_.**

We build [skillshare](https://github.com/runkids/skillshare) — a CLI that manages AI skills across 50+ tools (Claude Code, Cursor, OpenCode, and more). Along the way, we built a testing infrastructure that lets AI agents go from spec to verified PR with minimal human intervention. Here's how.

<!-- truncate -->

## The Testing Pyramid for CLI Tools

```
        E2E (Docker Sandbox)
       ─────────────────────────
      Integration (testutil.Sandbox)
     ─────────────────────────────────
    Unit Tests (go test)
```

Each layer solves a different confidence problem:

- **Unit tests** — Is the logic correct?
- **Integration tests** — Does the CLI command behave correctly?
- **E2E tests** — Does it actually work in a clean environment?

The key insight: **every layer is something an AI agent can run autonomously**.

## Layer 1: Unit Tests

Nothing special here — standard `go test` for pure functions. AI agents can write and run these instantly.

```bash
go test ./internal/sync/...
go test ./internal/audit/...
```

## Layer 2: Integration Tests with `testutil.Sandbox`

This is where it gets interesting. Our `testutil.Sandbox` creates a **fully isolated fake HOME directory** for each test — complete with `.claude/`, `.cursor/`, and 50+ other AI CLI target directories.

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

An AI agent writes a feature, adds an integration test, runs it, and gets immediate pass/fail feedback. No human needed.

## Layer 3: E2E with Docker Sandbox

Integration tests run on the host, which may have a different environment than real users. The top layer is a **Docker-based devcontainer** — a clean Debian environment that simulates a real user's first experience.

```bash
make devc          # Start devcontainer + enter shell (one step)
make devc-reset    # Full reset if something breaks
```

Inside the devcontainer, the AI agent can run any command in complete isolation — install skills from Git repos, sync to targets, run the full CLI — without any risk to the host environment.

## The Secret Sauce: E2E Runbooks

Here's what ties it all together. We maintain **E2E runbooks** in `ai_docs/tests/` — human-readable, AI-executable test scripts:

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

An AI agent reads the runbook and knows exactly:
- What to do
- What to check
- What success looks like

No ambiguity. No hallucination about expected behavior. **The runbook is the spec.**

## The Glue: Built-in Skills

The runbooks and devcontainer don't work by magic — the AI agent needs to know _how_ to use them. That's where **built-in skills** come in.

skillshare ships project-level skills in `.skillshare/skills/` that teach AI agents the entire workflow:

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

This is the meta part: **a skill management tool that uses its own skill system to teach AI agents how to develop and test itself.** The skills are the documentation, the workflow guide, and the guardrails — all in one.

When we update the testing workflow, we update the skills. The next time an AI agent picks up a task, it automatically gets the latest instructions. No stale docs, no out-of-date READMEs.

## The Full Loop

Here's the complete development cycle an AI agent executes autonomously:

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

The human's job becomes:

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

## Takeaways

If you're building a CLI and want AI agents to help develop it:

1. **Invest in test isolation** — give agents a sandbox they can't break
2. **Write E2E runbooks** — explicit verification steps, not vague expectations
3. **Ship built-in skills** — teach agents your workflow, not just your API
4. **Use containers** — a clean environment is cheaper than debugging "works on my machine"
5. **Keep output parseable** — structured output + JSON flags let agents verify programmatically

AI agents are fast, confident, and wrong in new ways. We don't assume they write perfect code — we assume they can fix their own mistakes, **as long as the feedback loop is fast enough.** Runbooks, sandboxes, and built-in skills are that feedback loop.
