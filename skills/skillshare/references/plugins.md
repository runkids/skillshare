# Plugins

Plugins are complete native packages, not standalone skills. Keep their components
together. Install targets: `claude`, `codex`, `cursor`, `antigravity` (`agy` alias),
`antigravity-cli`, `copilot`, `pi`, and `opencode`. Grok supports native import/removal,
with trust/install handled in Grok. Kimi, Hermes, and Devin are discovery-only.
Read `targetDefinitions` from JSON output instead of assuming every format supports
all operations. `targetInfo` reports per-target components, version, and problems.

## Inspect before changing

```bash
skillshare plugin list --json -g
skillshare plugin discover ./plugin-directory --json
skillshare plugin add ./plugin-directory --plugin demo --target claude --dry-run --json -g
```

`add` accepts a local folder, owner/repo, or HTTPS Git URL. For a multi-plugin
marketplace, select a named candidate with `--plugin`. Use `--name` to bind
different native distributions under one logical package, never infer equivalence
from display names. External catalog sources are not auto-converted. Multiple local catalogs are merged.
Safe internal relative symlinks are preserved; escaping, absolute, broken, cyclic,
and `.git` links are rejected. A malformed manifest does not hide other formats.

Use `--source-ref TAG_OR_COMMIT` with discover/add/update for a remote Git source.
Bindings retain `source_ref` and the reviewed `commit`; `--revision` is a separate
preview token. Use `--entry dist/plugin.js` with discover/add for an explicit
OpenCode entry. Never build or execute package code just to discover it.

## Apply and adopt

```bash
skillshare plugin add ./plugin-directory --plugin demo --target claude --no-tui -g
skillshare plugin import demo@team --from claude --no-tui -g
skillshare plugin inspect demo --json -g
```

Add installs; import records an existing native installation without reinstalling
or changing native enabled state. Pass `--revision ID` when applying a specific
preview. A stale preview is rejected. Review native trust/authentication problems
in the native client; never add native auto-accept flags to bypass them.

## Sync selection

```bash
skillshare plugin disable demo --target codex --no-tui -g
skillshare sync plugins demo --dry-run --json -g
skillshare sync plugins demo --no-tui -g
skillshare plugin enable demo --target codex --no-tui -g
```

Enable/disable only select targets for synchronization. Deselecting a managed
binding removes its installation on the next sync, but keeps the definition.
This is not native enable/disable. `sync --all` does not include plugins.

## Update and remove

```bash
skillshare plugin check demo --json -g
skillshare plugin update demo --target claude --dry-run --json -g
skillshare plugin update demo --target claude --no-tui -g
skillshare plugin remove demo --dry-run --json -g
```

Claude supports native updates; Codex does not. Cursor/Antigravity replace managed
local copies; Pi/OpenCode refresh reviewed source snapshots. Imported Pi/OpenCode v1 packages must be updated natively. OpenCode v2 global
imports may use native update; project imports may not. Copilot source updates
require known native enabled state; Antigravity CLI and Grok update natively. Project mode supports Claude, Antigravity, Pi,
and OpenCode, never falling back to global scope.

## Additional formats and scopes

- Cursor: `.cursor-plugin/plugin.json` or Agent Plugins root manifest; copy to
  `~/.cursor/plugins/local/`. Local imports must be allowed; marketplace copies can
  take precedence. No CLI required. Reload and verify in Cursor.
- Antigravity: root `plugin.json` with explicit `name`; copy to
  `~/.gemini/config/plugins/`, or project `.agents/plugins/` (existing `_agents/plugins/`
  is supported). This targets desktop/workspace discovery, not the separate
  standalone agy CLI plugin store. Use `antigravity-cli` for that store; `agy`
  remains the desktop alias. No Gemini CLI adapter is provided.
- Pi: `package.json` with a `pi` resource manifest or `pi-package` conventions. Native install/remove; read-only
  settings inventory honors `PI_CODING_AGENT_DIR`. Project trust must be completed
  in Pi; do not bypass it with automatic approval flags.
- OpenCode: SDK dependency, `.opencode/plugins/` convention, or explicit `--entry`,
  with an existing JS/TS entry.
  Preserve the whole tree and register its file URL in the native JSON/JSONC config.
  Version 1 uses `plugin`, version 2 uses `plugins`. Runtime dependencies must already
  be available. The CLI version selects the schema; do not guess from docs alone.
- Local directory plugins cannot import unowned folders or marketplace installs.
  Importing Pi/OpenCode entries with filters/options is blocked to avoid losing them.
  Supply `--name` when a native package source is not a valid logical name.

```bash
skillshare plugin add ./agy-plugin --target agy --dry-run --json -p
skillshare plugin add ./pi-package --target pi --no-tui -g
skillshare plugin add ./opencode-package --target opencode --no-tui -g
```

On a partial failure, inspect each target outcome and retry with `sync plugins`.
Do not delete native caches or rewrite native installed-plugin registries. Removal
retains shared marketplaces and snapshots. Installed does not mean loaded, logged
in, or hook-trusted. Interactive humans can use the bare `skillshare plugin` manager;
automation must provide explicit arguments and use `--json` or `--no-tui`.

## Native capability boundaries

- Copilot installs a reviewed snapshot. An imported binding without a source cannot
  be reinstalled automatically after removal; install natively, then sync again.
- Antigravity CLI accepts native and Claude manifests using its own CLI. Its native
  list does not expose enablement; never present unknown as disabled. Update in agy.
- Grok install/update requires native trust; never add `--trust` automatically.
- Kimi, Hermes, and Devin discovery does not authorize native installation,
  capability consent, or cloud changes. Explain the adapter limitation.
- Registration is not proof of resource loading. Verify inside the target Agent.
- Failed snapshot updates restore the previous managed snapshot; this is not a
  claim that every native cache side effect can be rolled back.
