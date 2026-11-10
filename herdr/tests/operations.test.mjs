import { test } from "node:test";
import assert from "node:assert/strict";
import { operate } from "../skillshare/mods/skillshare/operations.js";
import { CONFLICTS, PREVIEW, RESOLVE, ABORT, METADATA_STAGES, METADATA_WRITE } from "../skillshare/mods/skillshare/model.js";

// A fake machine. `jobs` maps skillshare args (joined by spaces) to results, or to a list of
// results for repeated runs; `scripts` does the same for our scripts, keyed by the script.
// `calls` records what ran, in order.
function machine({ jobs = {}, scripts = {} } = {}) {
  const calls = [];
  const ok = { code: 0, stdout: "", stderr: "" };
  const take = (table, key) => {
    const v = table[key];
    return { ...ok, ...(Array.isArray(v) ? v.shift() : v) };
  };
  const io = {
    job: async (args) => { calls.push(args.join(" ")); return take(jobs, args.join(" ")); },
    run: async (args) => { calls.push(args.join(" ")); return take(jobs, args.join(" ")); },
    sh: async (script, args) => { calls.push(script); return take(scripts, script); },
    step: () => {},
  };
  return { io, calls };
}
const CONFLICT = { code: 1, stderr: "Pull stopped: both changed the same files" };
const UNMERGED = 'cd "$1" && git -c core.quotePath=false diff --name-only --diff-filter=U';

test("an update the audit blocks names the skill", async () => {
  const { io } = machine({ jobs: { "update figlens --json -g": { code: 1, stdout: JSON.stringify({ items: [{ name: "figlens", status: "security_blocked" }] }) } } });
  const r = await operate({ op: "update", updates: [{ kind: "skills", scope: "global", name: "figlens", label: "figlens" }] }, io);
  assert.deepEqual(r, { kind: "error", text: "Audit blocked figlens. Ask an agent to audit it before updating." });
});

test("an uninstall that fails says why", async () => {
  const error = "1 agent(s) failed to uninstall: tutor: permission denied";
  const { io } = machine({ jobs: { "uninstall agents tutor --json -g": { code: 1, stdout: JSON.stringify({ removed: [], failed: ["tutor"], error }) } } });
  const r = await operate({ op: "uninstall", item: { kind: "agents", scope: "global", rel: "tutor", label: "tutor" } }, io);
  assert.deepEqual(r, { kind: "error", text: error });
});

test("a project uninstall asks for JSON", async () => {
  const { io, calls } = machine({ jobs: { "uninstall a -p --force --json": { stdout: JSON.stringify({ removed: ["a"], failed: [] }) } } });
  const r = await operate({ op: "uninstall", project: "/p", item: { kind: "skills", scope: "project", rel: "a", label: "a" } }, io);
  assert.deepEqual([r.kind, calls], ["done", ["uninstall a -p --force --json"]]);
});

test("a failed sync of one scope leaves the next unsynced", async () => {
  const { io, calls } = machine({ jobs: { "sync --all --json -p": { code: 1, stderr: "hook conflict" } } });
  await operate({ op: "sync", scopes: ["project", "global"], project: "/p" }, io);
  assert.deepEqual(calls, ["sync --all --json -p"]);
});

test("a pull both sides changed undoes the merge and asks about each file", async () => {
  const { io, calls } = machine({
    jobs: { pull: CONFLICT },
    scripts: { [CONFLICTS]: { stdout: "skills/a/SKILL.md\n" }, [PREVIEW]: { stdout: "@@local skills/a/SKILL.md\nmine\n@@remote skills/a/SKILL.md\ntheirs\n" } },
  });
  const r = await operate({ op: "pull", repo: "/src" }, io);
  assert.deepEqual([r.kind, r.conflict.files, r.conflict.push, calls.includes(ABORT)], ["warn", ["skills/a/SKILL.md"], false, true]);
});

test("a pull where only skill records conflict merges them and syncs", async () => {
  const stages = '@@stage 1\n{"entries":{}}\n@@stage 2\n{"entries":{"a":{"source":"x"}}}\n@@stage 3\n{"entries":{"b":{"source":"y"}}}\n';
  const { io, calls } = machine({
    jobs: { pull: CONFLICT },
    scripts: { [CONFLICTS]: { stdout: ".metadata.json\n" }, [UNMERGED]: { stdout: ".metadata.json\n" }, [METADATA_STAGES]: { stdout: stages } },
  });
  const r = await operate({ op: "pull", repo: "/src" }, io);
  assert.deepEqual([r.kind, calls.filter((c) => c === METADATA_WRITE || c === RESOLVE || c.startsWith("sync"))], ["done", [METADATA_WRITE, RESOLVE, "sync --all --json -g"]]);
});

test("resolving after the remote moved asks again instead of guessing", async () => {
  const { io, calls } = machine({
    scripts: { [CONFLICTS]: { stdout: "skills/a/SKILL.md\nskills/b/SKILL.md\n" }, [PREVIEW]: { stdout: "" } },
  });
  const r = await operate({ op: "resolve", repo: "/src", push: true, choices: { "skills/a/SKILL.md": "local" } }, io);
  assert.deepEqual([r.kind, r.conflict?.files, calls.includes(RESOLVE)], ["warn", ["skills/a/SKILL.md", "skills/b/SKILL.md"], false]);
});
