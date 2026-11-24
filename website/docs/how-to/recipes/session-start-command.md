---
sidebar_position: 9
---

# Recipe: One project command at session start

> Run one read-only project script when a Claude Code, Codex, Pi or OpenCode session starts, and give its output to the model, from one Entry in the global config.

## Scenario

A project has a small script that prints a short status when there is something to act on, and prints nothing otherwise. You want each Agent you use in that project to run it when a session starts and put its output in the session's context.

Skillshare does not translate events between Agents. Each Agent gets its own native binding in the same Entry: command hooks for Claude Code and Codex, an extension for Pi, and a plugin for OpenCode.

## Solution

The script in this example is `scripts/status.mjs` in the project. It must exit 0 and print to stdout only what the model should read. It runs with the project root as its working directory.

Declare the Entry under `hooks.projects` in the global config. The key is the project's absolute root, and it must be the Git repository root: the Codex and OpenCode bindings resolve paths from the repository root, and with a subfolder key such as a monorepo package, neither added the status in a test run. Command bindings do not expand `{files}` (only Git bindings do), so each command changes to the project root itself:

```yaml
hooks:
  projects:
    /path/to/project:
      entries:
        session-status:
          description: Project status at session start
          bindings:
            claude:
              events:
                SessionStart:
                  - hooks:
                      - type: command
                        command: 'cd "$CLAUDE_PROJECT_DIR" && node scripts/status.mjs'
                        timeout: 30
            codex:
              events:
                SessionStart:
                  - hooks:
                      - type: command
                        command: 'cd "$(git rev-parse --show-toplevel)" && node scripts/status.mjs'
                        timeout: 30
            pi:
              code: |
                import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

                export default function (pi: ExtensionAPI) {
                  pi.on("session_start", async (_event, ctx) => {
                    const result = await pi.exec("node", ["scripts/status.mjs"], { cwd: ctx.cwd, timeout: 30_000 });
                    const text = result.stdout.trim();
                    if (result.code === 0 && text) {
                      pi.sendMessage(
                        { customType: "session-status", content: text, display: true },
                        { deliverAs: "nextTurn" },
                      );
                    }
                  });
                }
            opencode:
              code: |
                import { execFile } from "node:child_process";
                import { promisify } from "node:util";
                import type { Plugin } from "@opencode-ai/plugin";

                const run = promisify(execFile);

                export const SessionStatus: Plugin = async ({ worktree }) => {
                  const status = new Map<string, Promise<string>>();
                  return {
                    "experimental.chat.system.transform": async ({ sessionID }, output) => {
                      const id = sessionID ?? "";
                      if (!status.has(id)) {
                        status.set(id, run("node", ["scripts/status.mjs"], { cwd: worktree, timeout: 30_000 }).then((r) => r.stdout.trim(), () => ""));
                      }
                      const text = await status.get(id);
                      if (text) output.system.push(text);
                    },
                  };
                };
```

The `opencode` binding above is for OpenCode v1. Its `experimental.chat.system.transform` hook is experimental, so a later OpenCode release may change it. With OpenCode v2 installed, Skillshare still accepts this `code` and writes it into a v2 package, but the code targets the v1 API: use the v2 binding in [Variations](#variations) instead.

Preview, then write the files:

```bash
skillshare hooks sync -g --dry-run
skillshare hooks sync -g
```

With OpenCode v1, the preview lists one file per Agent: `.claude/settings.json`, `.codex/hooks.json`, `.pi/extensions/skillshare-session-status.ts` and `.opencode/plugins/skillshare-session-status.ts`. Each Agent then needs the trust step in the table below before it runs the hook.

## What each Agent does

These results come from runs on Linux with Claude Code 2.1.278, Codex 0.155.1, Pi 1.6.0 and OpenCode 1.19.6 and 2.4.1. Each run started a session from the project root and again from a subfolder, then read the request the Agent sent to the model. Whether each Agent's interface also shows the text was not checked. The runs used each Agent's per-run trust override, so they did not exercise the trust steps; that column comes from Pi's extension documentation, the [Codex trust](/docs/reference/commands/hooks#codex-trust) reference and Skillshare's real-Agent hook tests.

| Agent | Start the session from | Where the output is in the request to the model | Trust before it runs |
|---|---|---|---|
| Claude Code | The project root. Claude Code reads `.claude/settings.json` only from the folder it starts in. | A system message that begins `SessionStart:startup hook success:` | Accept the workspace trust prompt for the folder |
| Codex | The project root or any subfolder. Codex reads `.codex/hooks.json` from the repository root, but runs the command in the folder it started in, so the command changes to the repository root first. | A developer message | Trust the project in Codex, then review the new hook in `/hooks` ([`--trust`](/docs/reference/commands/hooks#codex-trust) does not cover `hooks.projects`) |
| Pi | The project root. Pi reads `.pi/extensions` only from the folder it starts in. | A message sent with your first prompt; `display: true` also shows it in the session | Trust the project when Pi asks, or pass `--approve` for one run |
| OpenCode | The project root or any subfolder. `worktree` (v1) and `ctx.location.project.directory` (v2) are the repository root either way. | The system prompt | None was asked for in `opencode run` |

Claude Code and Codex run `SessionStart` when a session starts. Without a matcher, Claude Code also runs it on resume, `/clear` and compaction. Pi runs `session_start` on startup, reload, `/new`, resume and fork. The OpenCode plugins run the script before the session's first model request and keep the result for the rest of that session.

## Verification

```bash
skillshare hooks sync -g --dry-run
```

Each Agent's file shows as `unchanged`. Then start a session from the project root and ask the Agent what the session status said. When the script printed nothing, no message is added.

## Variations

**OpenCode v2.** Skillshare checks `opencode --version` to choose between v1 and v2. The plugin above uses the v1 API. For v2, write a plugin against the v2 API; this one is a complete `package` in place of `code`. Replace the `opencode` binding above with:

```yaml
            opencode:
              package:
                package.json: |
                  {
                    "name": "session-status",
                    "version": "1.0.0",
                    "type": "module",
                    "private": true,
                    "exports": { "./server": "./index.ts" },
                    "dependencies": { "@opencode/plugin": "^2.0.0" }
                  }
                index.ts: |
                  import { execFile } from "node:child_process";
                  import { promisify } from "node:util";
                  import { Plugin } from "@opencode/plugin";

                  const run = promisify(execFile);
                  const status = new Map<string, Promise<string>>();

                  export default Plugin.define({
                    id: "session-status",
                    setup: async (ctx) => {
                      const root = ctx.location.project.directory;
                      await ctx.session.hook("context", async (event) => {
                        if (!status.has(event.sessionID)) {
                          status.set(event.sessionID, run("node", ["scripts/status.mjs"], { cwd: root, timeout: 30_000 }).then((r) => r.stdout.trim(), () => ""));
                        }
                        const text = await status.get(event.sessionID);
                        if (text) event.system.push({ type: "text", text });
                      });
                    },
                  });
```

The sync warns that you must provide the declared dependencies yourself. In the run above, OpenCode 2.4.1 loaded the package without a separate install of `@opencode/plugin`. See the [hooks reference](/docs/reference/commands/hooks) for where v2 packages are written and registered.

**Project mode.** In a project that has its own `.skillshare/config.yaml`, save the part under `session-status:` (`description` and `bindings`) as `entry.yaml` and run `skillshare hooks add session-status --file entry.yaml --sync -p` in the project instead. `hooks.projects` does not manage a project that has its own config.
