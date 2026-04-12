# Native Rules And Hooks Families Design

Date: 2026-04-11
Status: Draft for review

## Summary

This design extends the earlier `v0.19.0` managed-resources alignment work with
an explicit native capability model for `rules` and `hooks`.

The key decision is:

- keep Skillshare's broad built-in target registry for `skills`
- keep narrower per-resource capability subsets for richer resource families
- model managed `rules` and `hooks` the same way `agents` are already modeled

In practice, that means:

- `supported targets` remains the source of truth for target names, aliases, and
  skill paths
- managed `rules` and managed `hooks` gain their own explicit capability matrix
- CLI, server, and UI read from the same family definitions instead of
  hardcoding partial support in multiple places
- the initial family split is explicit:
  - managed `rules`: `claude`, `codex`, `gemini`, `pi`
  - managed `hooks`: `claude`, `codex`, `gemini`

This follows the maintainer's existing architecture rather than introducing a
new one.

## Relationship To Existing Specs

This document builds on:

- [v0.19.0-Aligned Managed Resources Refactor](./2026-04-11-v019-managed-resources-alignment-design.md)
- [Hooks And Rules Managed Resources Design](./2026-04-11-hooks-rules-managed-resources-design.md)

Those documents establish:

- `/resources` as the primary managed inventory surface
- managed-only `rules` and `hooks` inventory in `/resources`
- shared managed-resource orchestration for sync and collect

This document adds the missing piece:

- how native `rules` and `hooks` capability should be represented across the
  many targets Skillshare knows about
- how CLI authoring and editing should work without pretending every skill
  target supports every managed resource family

## Problem

The current branch has three forms of drift:

### 1. Supported-target drift

Skillshare's docs and `targets.yaml` define a broad registry of built-in
targets. That registry primarily answers:

- what target names exist
- what aliases resolve to them
- where their skill directories live

It does **not** mean every target supports:

- `agents`
- native instruction files
- native hooks

The codebase already recognizes this distinction for `agents`, but `rules` and
`hooks` do not yet have a comparable explicit capability model.

### 2. Implicit family drift

Managed `rules` and `hooks` currently infer target support from a mix of:

- target name matching
- path heuristics
- branch-local assumptions about shared directories such as `.agents`

That works for the currently implemented cases, but it is not explicit enough to
scale cleanly to Gemini, Pi, or other target families. It also makes UI and CLI
validation harder to keep in sync.

### 3. CLI parity drift

Managed `rules` and `hooks` already have server CRUD APIs and UI flows, but the
CLI still exposes them mostly through:

- `skillshare sync --resources rules,hooks`
- `skillshare collect --resources rules,hooks`

There is no first-class managed-resource authoring surface in the CLI that
matches the fact that these resources are now first-class in the UI.

## Maintainer Precedent

The strongest evidence for the correct direction is the existing `agents`
architecture.

The codebase already separates:

- a broad built-in target registry for `skills`
- a narrower capability subset for `agents`

In [internal/config/targets.go](../../../internal/config/targets.go), target
specs have required `Skills` paths and optional `Agents` paths.

That produces two different registries:

- `DefaultTargets` / `ProjectTargets` for all skill-capable targets
- `DefaultAgentTargets` / `ProjectAgentTargets` for the narrower
  agent-capable subset

`syncAgentsGlobal` explicitly syncs only to `agent-capable targets` in
[cmd/skillshare/sync_agents.go](../../../cmd/skillshare/sync_agents.go).

Tests also codify that some built-in targets should be excluded from the agent
subset even though they are supported skill targets:

- `copilot`
- `codex`
- `windsurf`

See [internal/config/targets_test.go](../../../internal/config/targets_test.go).

This is the architectural precedent this design follows:

- broad registry for target identity
- narrower capability subset per richer resource family

## Decision

Adopt **native capability families** for managed `rules` and `hooks`.

This is "option 2" from design discussion:

- keep the supported-targets registry broad
- introduce an explicit managed capability registry for `rules` and `hooks`
- map concrete target names into native families only when they actually share
  the same file formats and runtime semantics

This design does **not** attempt to give every supported skill target generic
rules/hooks support.

## Goals

- Preserve the maintainer's existing `skills` vs `agents` architectural split.
- Add managed `rules` and `hooks` using the same pattern.
- Make native capability support explicit, testable, and shared across CLI,
  server, and UI.
- Add first-class CLI authoring flows for managed `rules` and `hooks`.
- Drive hook/rule editor options from native family definitions instead of
  frontend-only hardcoded assumptions.
- Preserve existing compatibility behavior already codified in this branch,
  including current `.agents` / `universal` Codex-family compatibility.

## Non-Goals

- No attempt to make all 56+ skill targets support native `rules` or `hooks`.
- No fake cross-tool hook abstraction that erases native semantics.
- No replacement of Pi extensions with a generic hook layer.
- No broad redesign of the `skills` or `agents` command model.
- No removal of existing `sync --resources` or `collect --resources` flows.

## Design Overview

Skillshare should model four different layers:

1. `target registry`
2. `resource family registry`
3. `target -> family resolution`
4. `family-specific adapters and validators`

### 1. Target Registry

The current target registry remains the source of truth for:

- canonical target names
- aliases
- skill paths
- optional agent paths

This remains the responsibility of `targets.yaml` plus the existing config
helpers.

### 2. Managed Capability Registry

Introduce a new explicit registry for native managed-resource support.

This registry answers:

- which families support managed `rules`
- which families support managed `hooks`
- which target names resolve to those families
- which files or config surfaces each family owns
- which events, handler types, and fields are valid for each family

The capability registry should live in managed-resource code, not in frontend
components and not in ad hoc path heuristics.

A plausible home is:

- `internal/resources/managed/capabilities.go`

with supporting family-specific files if needed.

### 3. Target-To-Family Resolution

All rule/hook preview, sync, diff, collect, UI forms, and CLI authoring should
resolve through the same functions.

Resolution must prefer:

1. explicit target-name mapping
2. alias-aware matching
3. validated compatibility fallbacks

It should not rely on loose path guessing as the primary contract.

### 4. Family-Specific Adapters

Each native family continues to own its own:

- compile logic
- collect/import logic
- validation rules
- preview root behavior

The new registry coordinates them. It does not flatten them into a generic
format.

## Capability Matrix

### Skills

`skills` remain broad and continue to use the existing supported-targets
registry.

### Agents

`agents` remain the current narrower subset defined by optional agent paths.

### Rules

Initial native managed `rules` families:

- `claude`
- `codex`
- `gemini`
- `pi`

### Hooks

Initial native managed `hooks` families:

- `claude`
- `codex`
- `gemini`

`pi` does **not** join the managed hooks family set. Pi's native extension
system is closer to a programmable runtime/plugin surface than a
`settings.json` hook registry and should not be misrepresented as equivalent.

## Family Definitions

### Claude Rules Family

Primary managed surfaces:

- project root `CLAUDE.md`
- `./.claude/rules/**`
- global `~/.claude/CLAUDE.md`
- global `~/.claude/rules/**`

Managed rule semantics remain the current Claude semantics.

### Codex Rules Family

Primary managed surfaces:

- project root `AGENTS.md`
- global `~/.codex/AGENTS.md`

Compatibility behavior to preserve:

- the current branch treats the shared `.agents` / `universal` target as a
  Codex-family managed rule destination
- existing tests already codify this behavior

That compatibility remains supported, but as an explicit mapping rule rather
than an accidental side effect of path guessing.

### Gemini Rules Family

Primary managed surfaces:

- project root `GEMINI.md`
- `./.gemini/rules/**`
- global `~/.gemini/GEMINI.md`
- global `~/.gemini/rules/**`

This aligns with Gemini CLI's documented `GEMINI.md` context model.

### Pi Rules Family

Pi's native instruction surfaces are not a `rules/` directory. They are:

- `AGENTS.md`
- nested `AGENTS.md` files in the project tree
- `.pi/SYSTEM.md`
- `.pi/APPEND_SYSTEM.md`
- global equivalents under `~/.pi/agent/`

Therefore the Pi rules family should be modeled as **managed instruction files**
rather than "markdown fragments under a rules directory".

Initial managed Pi rule support should cover the documented instruction
surfaces:

- `pi/AGENTS.md`
- `pi/**/AGENTS.md` when explicitly targeted
- `pi/SYSTEM.md`
- `pi/APPEND_SYSTEM.md`

This family requires its own compile adapter and path validator because its
native surfaces differ from Claude/Codex/Gemini.

### Claude Hooks Family

Native surface:

- `settings.json` under Claude's native config root

Supported handler types remain:

- `command`
- `http`
- `prompt`
- `agent`

Handler fields remain Claude-native:

- command/url/prompt
- timeout string
- status message

### Codex Hooks Family

Native surfaces:

- `.codex/config.toml`
- `.codex/hooks.json`

Semantics to preserve:

- feature flag in `config.toml`
- hook definitions in `hooks.json`
- supported events only:
  - `SessionStart`
  - `PreToolUse`
  - `PostToolUse`
  - `UserPromptSubmit`
  - `Stop`
- `command` handlers only
- timeout must be numeric seconds
- `UserPromptSubmit` and `Stop` require empty matcher

Compatibility behavior to preserve:

- current branch maps `universal` / `.agents` shared targets into Codex-family
  preview roots

### Gemini Hooks Family

Native surface:

- `settings.json` under Gemini config

Supported hook model, based on Gemini CLI docs:

- `command` hooks only
- event groups:
  - `SessionStart`
  - `SessionEnd`
  - `BeforeAgent`
  - `AfterAgent`
  - `BeforeModel`
  - `AfterModel`
  - `BeforeToolSelection`
  - `BeforeTool`
  - `AfterTool`
  - `PreCompress`
  - `Notification`
- hook fields:
  - `name`
  - `description`
  - `command`
  - `timeout` in milliseconds
- group fields:
  - `matcher`
  - `sequential`

Matcher semantics differ by event type:

- tool events use regex matching
- lifecycle events use exact string matching
- empty or `*` means all

This is materially different from Claude and Codex. It must be modeled
explicitly rather than stuffed into the current generic hook form.

### Pi Hooks

No managed Pi hooks family is introduced.

Pi's native automation surface is extensions under:

- `~/.pi/agent/extensions/`
- `.pi/extensions/`

That is a separate product surface and should remain out of scope for managed
hooks. The spec should describe this clearly in docs and UI copy so users do not
assume missing support is accidental.

## Target Mapping Rules

The capability registry must distinguish:

- `target identity`
- `native family`

### Initial explicit mappings

The initial implementation should preserve currently verified mappings:

- `claude`
- `claude-code` -> `claude`
- `codex`
- `gemini`
- `gemini-cli` -> `gemini`
- `pi`

The initial implementation should **not** map `omp` / `oh-my-pi` into the Pi
family until native rule and hook parity is explicitly verified. Shared skills
paths or branding similarity are not sufficient evidence.

### Shared-target compatibility mappings

The branch currently codifies `.agents` / `universal` compatibility as
Codex-family output for managed rules and hooks. This should remain supported.

The design should therefore explicitly map:

- `universal` -> Codex family for managed rules
- `universal` -> Codex family for managed hooks

This is a compatibility rule, not a blanket statement that all `.agents`-style
tools are Codex-compatible.

### Out-of-scope mappings

Targets should **not** be mapped into a managed family solely because they share
a skills directory path.

Examples:

- `windsurf`
- `warp`
- `witsy`
- `replit`
- `purecode`
- `omp`
- `xcode-claude`
- `xcode-codex`

If a target later proves it shares native managed semantics with an existing
family, that mapping can be added deliberately.

## Exhaustive Target Coverage

The spec should provide **100% classification coverage** for the current built-in
target registry in `internal/config/targets.yaml`.

That means every canonical built-in target must be in exactly one of these
states:

- mapped to a managed `rules` and `hooks` family
- mapped to a managed `rules` family only
- supported for `skills`, but not mapped to managed `rules` or `hooks` yet

It does **not** mean every skill target gets native managed hooks/rules support.
That would misrepresent tools that do not expose comparable native surfaces.

Aliases inherit the disposition of their canonical target unless the spec says
otherwise.

This appendix is exhaustive for the current 56 canonical built-in targets and
must be updated whenever `targets.yaml` changes.

### Managed Rules And Hooks

Claude family:

- `claude`

Codex family:

- `codex`
- `universal`

Gemini family:

- `gemini`

### Managed Rules Only

Pi family:

- `pi`

### Skills-Only In Initial Pass

- `adal`
- `amp`
- `antigravity`
- `astrbot`
- `augment`
- `bob`
- `cline`
- `codebuddy`
- `comate`
- `commandcode`
- `continue`
- `cortex`
- `copilot`
- `crush`
- `cursor`
- `deepagents`
- `droid`
- `firebender`
- `goose`
- `hermes`
- `iflow`
- `junie`
- `kilocode`
- `kimi`
- `kiro`
- `kode`
- `letta`
- `lingma`
- `mcpjam`
- `mux`
- `neovate`
- `omp`
- `openclaw`
- `opencode`
- `openhands`
- `pochi`
- `purecode`
- `qoder`
- `qwen`
- `roo`
- `trae`
- `trae-cn`
- `vibe`
- `verdent`
- `warp`
- `windsurf`
- `witsy`
- `xcode-claude`
- `xcode-codex`
- `zencoder`
- `replit`

### Explicit Non-Inferences

The following should remain skills-only until explicitly verified, even though
their names or shared paths could tempt over-mapping:

- `.agents`-path targets other than `universal` and current Codex compatibility
- `omp` / `oh-my-pi`
- `xcode-claude`
- `xcode-codex`

## Family Decision Rule

Not every target needs its own hook or rule design.

The right default is:

- every target gets an explicit classification
- only verified native capability surfaces get mapped into managed families
- a new family is added only when an existing family cannot faithfully represent
  the target's native files, semantics, and validation rules

### Map To An Existing Family When

- the target is an alias of an existing canonical target
- the target uses the same native files and directories
- the target uses the same event model and field schema
- the target uses the same compile and collect semantics

Examples:

- `claude-code` -> Claude family
- `gemini-cli` -> Gemini family
- `universal` -> Codex compatibility family for current managed rules/hooks

### Add A New Family When

- the target has a native rules surface, but it is structurally different from
  existing families
- the target has a native hooks surface, but its event model or handler schema
  is materially different from existing families
- mapping it into an existing family would force fake fields, invalid options,
  or lossy compile behavior

Example:

- `pi` needs its own managed rules family because its instruction surfaces are
  `AGENTS.md`, `.pi/SYSTEM.md`, and `.pi/APPEND_SYSTEM.md`, which do not match
  Claude/Codex/Gemini rule layouts

### Keep Skills-Only When

- Skillshare knows the target's skills path, but native rules/hooks support has
  not been verified
- the target exposes no comparable native hooks/rules surface
- the only evidence is a shared directory path or similar branding

Examples:

- `omp`
- `xcode-claude`
- `xcode-codex`
- most `.agents`-path targets

## CLI Design

### Principle

CLI parity with skills does **not** mean copying the entire skills lifecycle.

Skills are installable packages and directories. Managed `rules` and `hooks`
are source records with native-family compile semantics.

So parity should mean:

- first-class CLI visibility
- first-class CLI creation
- first-class CLI inspection
- first-class CLI mutation
- first-class CLI deletion

not:

- repo search/install/update workflows identical to skills

### New command groups

Add first-class managed-resource command groups:

- `skillshare rules ...`
- `skillshare hooks ...`

These commands sit alongside the existing generic:

- `sync --resources ...`
- `collect --resources ...`

### Rules commands

Initial rules command surface:

- `skillshare rules list`
- `skillshare rules show <id>`
- `skillshare rules new`
- `skillshare rules update <id>`
- `skillshare rules delete <id>`
- `skillshare rules enable <id>`
- `skillshare rules disable <id>`
- `skillshare rules target add <id> <target>`
- `skillshare rules target remove <id> <target>`
- `skillshare rules target clear <id>`
- `skillshare rules diff`

Creation and update should support both:

- simple structured flags for common flows
- `--stdin` or `--file` for raw content-oriented workflows

### Hooks commands

Initial hooks command surface:

- `skillshare hooks list`
- `skillshare hooks show <id>`
- `skillshare hooks new`
- `skillshare hooks update <id>`
- `skillshare hooks delete <id>`
- `skillshare hooks enable <id>`
- `skillshare hooks disable <id>`
- `skillshare hooks target add <id> <target>`
- `skillshare hooks target remove <id> <target>`
- `skillshare hooks target clear <id>`
- `skillshare hooks diff`

Hook create/update must be family-aware. That means:

- Codex creation only offers Codex-supported events and `command` handlers.
- Gemini creation only offers Gemini-supported events and command-hook fields.
- Claude creation can offer the broader handler set.

### Keep existing sync and collect flows

Do **not** remove:

- `skillshare sync --resources rules,hooks`
- `skillshare collect --resources rules,hooks`

The new command groups are authoring and inspection surfaces, not replacements
for orchestration commands.

## Server And API Design

### Keep current CRUD endpoints

The current managed CRUD endpoints remain the base contract:

- `/api/managed/rules`
- `/api/managed/hooks`

### Add capability metadata endpoint

Add a read-only endpoint for UI and CLI metadata generation:

- `/api/managed/capabilities`

This endpoint should expose:

- supported rule families
- supported hook families
- family-to-target mappings
- rule path templates / allowed surfaces
- hook event lists
- hook handler type lists
- field constraints per family
- compatibility notes such as `pi hooks unsupported`

The goal is to eliminate duplicated frontend-only capability logic.

## UI Design

### Resources page stays managed-only

This design does not revisit the earlier decision:

- `/resources` remains managed inventory
- discovered import diagnostics remain outside primary managed inventory rows

### Family-driven create flows

`New Rule` and `New Hook` should become family-driven forms.

The form should first resolve or select:

- tool family

and then render only supported fields for that family.

### Hook editor behavior

Current generic hook UI should be replaced by family-aware rendering sourced
from the backend capability registry.

Examples:

- Codex: no `http`, `prompt`, or `agent` handlers; no invalid events; matcher
  disabled for `UserPromptSubmit` and `Stop`
- Gemini: command-only hooks, group-level `sequential`, hook `name`,
  `description`, timeout in milliseconds
- Claude: richer handler palette

### Rule editor behavior

Rule creation should also be family-aware:

- Claude/Codex/Gemini can continue to look mostly content-oriented
- Pi should offer explicit instruction surfaces such as `AGENTS.md`,
  `SYSTEM.md`, and `APPEND_SYSTEM.md`

### Unsupported-family messaging

When a target is supported for skills but not for managed hooks/rules, the UI
should say so plainly.

Example:

- Pi can appear in rules target pickers
- Pi should not appear in hooks target pickers
- Windsurf may appear as a supported skills target but should not imply managed
  rules/hooks support

## Implementation Architecture

### New capability layer

Introduce a managed capability layer above the existing low-level adapters.

Responsibilities:

- enumerate managed rule families
- enumerate managed hook families
- resolve target names to managed families
- surface family metadata to CLI and server
- define supported create/update schemas

Non-responsibilities:

- compiling files directly
- replacing family-specific adapters
- handling HTTP rendering

### Existing low-level packages remain

Keep existing packages as the source of family-specific compile behavior:

- `internal/resources/rules`
- `internal/resources/hooks`
- `internal/resources/adapters/*`

The new capability layer coordinates them.

### Current resolver replacement

The current branch has family inference embedded in functions such as:

- `resolveRuleTargetFamily`
- `resolveHookTargetFamily`
- `managedRulePathFamily`
- `managedHookPathFamily`

Those should be rewritten to delegate to the capability registry so path-based
fallbacks become explicit compatibility rules rather than hidden behavior.

## Migration And Compatibility

### Persisted records

Existing managed rule and hook records remain valid.

This design does not require record migration to add family support. Family is
derived from:

- record `tool`
- target mapping

### Existing compatibility routes and resources UI

No change to the earlier `/resources` design direction.

### Universal compatibility

Preserve current `.agents` / `universal` behavior for Codex-family managed
rules/hooks.

If the project later decides to narrow that behavior, it must happen in a
separate compatibility-review change with explicit migration messaging and test
updates.

## Documentation Changes

The current `supported targets` docs should not be overloaded to imply managed
resource support.

Add a separate docs surface such as:

- `Reference -> Managed Capabilities`

That page should distinguish:

- supported `skills` targets
- agent-capable targets
- rule-capable families
- hook-capable families

## Risks

### Over-generalization risk

If family definitions are too generic, the UI and CLI will again allow invalid
combinations that backend adapters later reject.

### Compatibility risk

If we tighten mapping too aggressively, we may break current `universal`
Codex-family behavior that branch tests already encode.

### Pi modeling risk

Pi's instruction and extension surfaces are different enough from
Claude/Codex/Gemini that forcing them into the same mental model will create UX
and code complexity. The design deliberately keeps Pi hooks out of managed hooks
to avoid that trap.

## Testing

### Backend

- capability registry resolves canonical names and aliases correctly
- unsupported targets remain unsupported for rules/hooks even when they are
  valid skill targets
- existing `universal` compatibility remains intact for Codex-family previews
- Gemini hooks compile, preview, and validate using Gemini-native event and
  field constraints
- Pi rules validate and compile only for supported Pi instruction surfaces

### CLI

- `skillshare rules list/new/show/update/delete` works in both global and
  project mode
- `skillshare hooks list/new/show/update/delete` works in both global and
  project mode
- invalid family-specific flag combinations fail early with clear errors

### UI

- create and edit flows derive options from `/api/managed/capabilities`
- hook editor shows only valid fields for the selected family
- Pi does not appear as a hook-capable family
- unsupported targets are clearly labeled rather than silently omitted where
  appropriate

## Recommendation

Proceed with native managed capability families.

This is the closest fit to the maintainer's existing design:

- broad target registry for `skills`
- narrower capability subsets for richer resource families
- explicit compatibility rules instead of hidden heuristics

It preserves existing branch behavior where compatibility is already codified,
adds the missing CLI surface for managed `rules` and `hooks`, and gives both the
CLI and UI a single source of truth for what each family actually supports.
