---
name: skillshare-code-review
description: >-
  Review a skillshare change before it becomes a pull request, or review an
  open pull request. Use this skill whenever the user asks to: review a diff,
  branch, commit range or PR, check a change before opening a PR, run a
  pre-Codex review, or find what a change missed. It checks correctness, the
  repository's documented rules, the originating issue or spec, every consumer
  of a changed concept, and cleanup (reuse, simplification, efficiency, depth
  of fix), then verifies each finding against the code before reporting it.
  Read-only unless the user asks for fixes.
argument-hint: "[base ref | PR number | path] [fix]"
metadata:
  targets: [claude, universal]
---

Review one change in five independent passes, verify every finding, and report them by pass. $ARGUMENTS names the target (a base ref, a PR number, or a path) and may end with `fix`.

**Scope**: Read-only by default. With `fix`, apply findings as the end of Step 5 describes. Never commit, push, or comment on GitHub unless the user asks.

Before acting, run `python3 scripts/ai-context.py pull-requests`. Its Review Rounds section defines what a review must sweep before Codex sees the change; this skill is how that sweep runs.

## Step 1: Pin the diff

| $ARGUMENTS | Diff |
|---|---|
| empty | `git diff origin/main...HEAD` plus `git diff HEAD` (uncommitted work counts) |
| a ref | `git diff <ref>...HEAD` plus `git diff HEAD` |
| a PR number | check the PR out in a new scratch worktree (`git worktree add`, then `gh pr checkout <n>` inside it) and review there with `git diff origin/<base>...HEAD`, so Step 4 reads the PR's code, not the current branch |
| a path | that file or directory, reviewed whole |

Run `git fetch origin` first so the base is current. Stop here if the ref does not resolve or the diff is empty. In a shared checkout, list files changed by another session (`git status`) and leave them out of scope.

## Step 2: Gather context once

1. **Spec**: in order, issue references in commit messages or the PR body (`gh issue view <n>`), a path in $ARGUMENTS, or a matching file in `specs/`. If none exists, the Spec pass reports "no spec".
2. **Rules**: `AGENTS.md`, plus the topics for the touched areas. Run `python3 scripts/ai-context.py list` and pick by path: `cmd/` and `internal/` → `cli-development`; `ui/` or `website/src/` → `frontend`; docs, READMEs, `skills/` → `documentation`; `wiki/` → `ai-context`.
3. **Concepts**: name each flag, field, error, config key, API route, or behavior the diff adds or alters. The Consumer pass sweeps these.

## Step 3: Run five passes in parallel

Launch one subagent per pass in a single message, using your tool's default subagent model; do not name a model the tool may not have. If your tool has no subagents, run the passes yourself one after another. Give each the diff command, the context from Step 2, and its brief below. Each returns findings as `file:line`, a one-sentence claim, and a concrete scenario (input or state → wrong result). A finding without a scenario is not reported.

1. **Correctness**: bugs the change introduces. Wrong results, crashes, data loss, broken error paths, races, and security gaps: path or symlink escapes, secrets in output, bypassed audits or trust prompts. Ignore style.
2. **Rules**: breaches of `AGENTS.md` and the loaded topics. Cite the file and rule.
3. **Spec**: requirements missing or partial, behavior nobody asked for, and requirements implemented wrongly. Quote the spec line for each.
4. **Consumers**: for each concept from Step 2, grep every consumer and state dimension that the pull-requests topic's Review Rounds section lists, plus `skills/skillshare/references` and `ui/src/i18n/locales`. Report each consumer the diff did not update, and each state dimension the change crosses without a test. Translations count as consumers: for every changed file under `website/docs/`, check the same path under each `website/i18n/<locale>/docusaurus-plugin-content-docs/current/`, and for `README.md`, each translated README. Report each copy that exists but was not updated to match, with the English lines it is missing.
5. **Cleanup**: code that re-implements an existing helper (name it), complexity a simpler form would remove, wasted work on hot paths, and fixes applied as a special case where the shared mechanism should change. Labelled as judgement calls.

## Step 4: Verify

For every finding, read the cited code yourself and decide:

- **CONFIRMED**: the scenario happens; you can name the line where it goes wrong.
- **PLAUSIBLE**: the code allows it, but it depends on state you could not check. Say which.
- **Dropped**: the code prevents it, the scenario needs an unsupported workflow (see the pull-requests topic), or it duplicates another finding. Count drops; do not list them.

## Step 5: Report

Use the user's language. Keep the passes separate and do not rerank across them: a change can pass the spec and still break a rule.

```
## Correctness (n)
1. CONFIRMED path:line: claim. Scenario: ...
## Rules (n)
## Spec (n)   or: no spec
## Consumers (n)
## Cleanup (n)

Dropped: n. Not checked: <passes or areas skipped, and why>.
```

End with the single most severe confirmed finding per pass, or "clean".

With `fix`: apply the CONFIRMED findings that do not change intended behavior, add a failing test first for each correctness fix, rerun the narrowest check that proves it (in the devcontainer, as the `testing` topic describes), and report each finding as fixed, skipped (why), or no change needed.

## Rules

- Do not run `go test`, `make test`, `skillshare`, or frontend scripts on the host. Checks run in the devcontainer, as the `testing` topic describes.
- A finding is a claim until Step 4 confirms it. Never report an unverified finding as fact.
- Review the diff, not the whole package. Pre-existing problems go in one line under "Not checked" as follow-ups.
