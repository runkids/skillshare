---
slug: e2e-tests-now-powered-by-mdproof
title: "skillshare E2E Tests Are Now Fully Powered by mdproof"
authors: [runkids]
tags: [testing, e2e, mdproof, ai-agents]
---

A week ago I wrote about [building testing infrastructure for AI-driven CLI development](/blog/e2e-testing-for-ai-agents). The runbooks I described — Markdown files with step-by-step commands and expected results — turned out to be the most useful piece of that entire infrastructure.

So useful, in fact, that I extracted the runner into its own tool: [**mdproof**](https://github.com/runkids/mdproof).

As of today, all 22 of skillshare's E2E runbooks run through mdproof. The built-in runner is gone.

<!-- truncate -->

## From Informal Runbooks to a Real Test Runner

The original runbooks were just documentation. They looked like this:

```markdown
## Steps
1. Run: `skillshare install runkids/skillshare`
2. Verify: skill directory exists
3. Verify: SKILL.md contains expected frontmatter
```

A human (or an AI agent) would read the steps and manually execute them. That worked, but it was slow and error-prone. The agent had to interpret "verify: skill directory exists" and figure out the right `test -d` command on its own.

What I wanted was something that could **execute the commands and check the assertions automatically** — but still look like a readable Markdown document.

## What mdproof Does

mdproof treats Markdown files as executable test runbooks. Write your test as a document, run it as a real test:

````markdown
### Step 2: First init

```bash
ss init --no-copy --targets claude,cursor --mode merge --no-git --no-skill
```

Expected:
- exit_code: 0
- Initialized successfully
- regex: Config:.*\.config/skillshare/config\.yaml
````

That's an actual step from skillshare's [`first_use_cli_e2e_runbook.md`](https://github.com/runkids/skillshare/blob/main/ai_docs/tests/first_use_cli_e2e_runbook.md). mdproof parses it, runs the bash block, and checks each assertion. No test framework. No assertion library. The test IS the document.

```
$ mdproof ai_docs/tests/first_use_cli_e2e_runbook.md

 ✓ first_use_cli_e2e_runbook.md
 ──────────────────────────────────────────────────
 ✓  Step 0  Verify command entrypoint              12ms
 ✓  Step 1  Create isolated first-use HOME          3ms
 ✓  Step 2  First init                             84ms
 ✓  Step 3  Verify config was created               8ms
 ✓  Step 4  Add one demo skill to source            4ms
 ✓  Step 5  First sync                             62ms
 ✓  Step 6  Verify skill reached both targets       5ms
 ✓  Step 7  Per-target compatibility tuning        94ms
 ✓  Step 8  Run doctor                             31ms
 ──────────────────────────────────────────────────
 9/9 passed  303ms
```

## Four Assertion Types

Through building skillshare's test suite, four assertion patterns emerged naturally. mdproof supports all of them:

### Substring + Negation

The simplest form. Check if output contains (or doesn't contain) a string:

```markdown
Expected:
- Initialized successfully
- config_created=yes
- Not Exec format error
```

### Exit Code

Explicit exit code checking — essential for CLI tools:

```markdown
Expected:
- exit_code: 0
```

### Regex

Pattern matching with multiline mode (`(?m)`) enabled by default:

```markdown
Expected:
- regex: v(dev|\d+\.\d+)
- regex: Config:.*\.config/skillshare/config\.yaml
```

### jq (JSON assertions)

This is where it gets powerful. skillshare's `--json` flags produce structured output, and `jq:` assertions let you validate the structure precisely:

````markdown
### Step 2: status --json outputs pure JSON

```bash
OUTPUT=$(ss status --json)
echo "$OUTPUT" | jq -e ".skill_count >= 2" && echo "FIELD_CHECK=OK"
echo "$OUTPUT" | jq -e ".targets | length >= 1" && echo "TARGETS=OK"
FIRST=$(echo "$OUTPUT" | head -c1)
[ "$FIRST" = "{" ] && echo "PURE_JSON=OK"
```

Expected:
- exit_code: 0
- FIELD_CHECK=OK
- TARGETS=OK
- PURE_JSON=OK
````

This pattern — run `jq -e` inside the bash block, echo a sentinel, assert on the sentinel — emerged from skillshare's [JSON output purity runbook](https://github.com/runkids/skillshare/blob/main/ai_docs/tests/json_output_purity_runbook.md). It's ugly but reliable: if the `jq` expression fails, the sentinel never prints, and the assertion catches it.

## Lifecycle Hooks

Real E2E tests need setup and teardown. mdproof supports a `runbook.json` config that runs hooks automatically:

```json
{
  "build": "cd /workspace && make build && /workspace/.devcontainer/ensure-mdproof.sh",
  "setup": "ss init -g --no-copy --all-targets --no-git --no-skill --force",
  "teardown": "ss uninstall --all --force",
  "timeout": "5m"
}
```

- **build**: Runs once before all runbooks (compile the binary)
- **setup**: Runs before each runbook (clean slate)
- **teardown**: Runs after each runbook (cleanup)
- **timeout**: Global safety net

This means runbook files stay focused on the actual test logic. No boilerplate.

## JSON Reports for AI Agents

The whole point of extracting mdproof was to close the agent feedback loop. `--report json` outputs structured results that agents can parse directly:

```json
{
  "version": "1",
  "runbook": "first_use_cli_e2e_runbook.md",
  "duration_ms": 303,
  "summary": { "total": 9, "passed": 9, "failed": 0, "skipped": 0 },
  "steps": [
    {
      "step": { "number": 0, "title": "Verify command entrypoint", "command": "..." },
      "status": "passed",
      "exit_code": 0,
      "stdout": "skillshare version vdev (...)",
      "stderr": ""
    }
  ]
}
```

Agent writes code → agent runs `mdproof --report json` → agent reads the JSON → agent fixes failures → re-run. No human interprets terminal colors. No regex on test output. The agent gets exactly the data it needs.

## Why I Extracted It

The runbook runner started as ~200 lines inside skillshare's codebase. Over three days (March 9–11) it grew features that had nothing to do with skillshare:

- Persistent bash sessions (variables survive across steps)
- Typed assertion engine (exit_code, regex, jq, substring)
- Lifecycle hooks (build/setup/teardown)
- Selective step execution (`--steps 3,5` or `--from 4`)
- Retry and depends directives
- Container-only safety checks
- JSON and JUnit XML reporting

At that point it was clear: **this isn't a skillshare feature — it's a testing tool.** So I pulled it out into [runkids/mdproof](https://github.com/runkids/mdproof) and replaced the built-in runner with a single dependency.

The migration commit was one line of real logic: swap the executor.

## What skillshare's Test Suite Looks Like Now

22 runbooks covering the full CLI surface:

| Category | Runbooks | Example |
|----------|----------|---------|
| First-time user flow | 1 | `first_use_cli_e2e_runbook.md` |
| JSON output purity | 2 | `json_output_purity_runbook.md`, `extras_refactor_json_runbook.md` |
| Security audit | 1 | `audit_output_antigravity_runbook.md` |
| Install/update | 2 | `issue46_install_update_optimization_runbook.md` |
| Sync behavior | 3 | `symlinked_dir_sync_runbook.md`, `sync_extras_runbook.md` |
| URL parsing | 3 | `gitlab_subgroup_url_parse_runbook.md`, `azure_devops_url_parse_runbook.md` |
| Config features | 2 | `gitlab_hosts_config_runbook.md`, `registry_yaml_split_runbook.md` |
| Cleanup operations | 3 | `uninstall_sync_orphan_runbook.md`, `uninstall_all_glob_runbook.md` |
| Precision checks | 2 | `check_treehash_precision_runbook.md`, `update_prune_check_stale_runbook.md` |
| Other | 4 | target paths, gitignore perf, precommit hooks, discovery |

All executed via:

```bash
mdproof --report json ai_docs/tests/
```

One command. 22 runbooks. Structured results.

## The Loop

In the [previous post](/blog/e2e-testing-for-ai-agents) I described a development loop where the agent writes code, runs tests, and verifies its own work. mdproof makes the E2E layer of that loop concrete:

```
Agent writes code
    ↓
go test ./... (unit + integration)
    ↓
mdproof --report json ai_docs/tests/relevant_runbook.md
    ↓
Agent reads JSON → all passed? → PR
                 → failed? → fix and re-run
```

The runbooks haven't changed much since that post. What changed is that they're no longer documentation that an agent interprets — they're **executable specifications** that produce machine-readable results.

## Try It

If you're building a CLI tool and want executable Markdown tests:

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/mdproof/main/install.sh | sh
```

Write a `test.md`, add some steps with `Expected:` blocks, and run `mdproof test.md`. That's it.

If you use skillshare, mdproof is already available as a skill:

```bash
skillshare install runkids/mdproof
```

Your AI agent gets the full syntax reference and can start writing runbooks immediately.

---

[mdproof on GitHub](https://github.com/runkids/mdproof) · [skillshare's runbook suite](https://github.com/runkids/skillshare/tree/main/ai_docs/tests)
