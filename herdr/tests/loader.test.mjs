import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { findProject, loadResources, loadMachine, loadLatestRelease, loadPreview, loadFile } from "../skillshare/mods/skillshare/loader.js";
import { GIT_STATUS, READ_SKILL, READ_FILE, LATEST_RELEASE } from "../skillshare/mods/skillshare/model.js";

// A fake machine answering by command: `jobs` and `runs` keyed by skillshare args joined by
// spaces, `scripts` by script. `seen` records each call's options.
function machine({ jobs = {}, runs = {}, scripts = {} } = {}) {
  const seen = [];
  const ok = { code: 0, stdout: "", stderr: "" };
  return {
    seen,
    io: {
      job: async (args, opt) => { seen.push({ args, ...opt }); return { ...ok, ...jobs[args.join(" ")] }; },
      run: async (args) => ({ ...ok, ...runs[args.join(" ")] }),
      sh: async (script) => ({ ...ok, ...scripts[script] }),
    },
  };
}
const resources = (flag) => Object.fromEntries([["list --all", []], ["diff", {}], ["mcp list", {}], ["plugin list", {}], ["hooks list", {}]].map(([cmd, value]) => [`${cmd} --json ${flag}`, { stdout: JSON.stringify(value) }]));
const emptyStatus = { stdout: "{}" };

test("the first folder skillshare treats as a project wins", (t) => {
  const root = mkdtempSync(join(tmpdir(), "ss-proj-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const plain = join(root, "plain"), proj = join(root, "proj");
  mkdirSync(plain); mkdirSync(join(proj, ".skillshare"), { recursive: true });
  writeFileSync(join(proj, ".skillshare", "config.yaml"), "");
  const sh = async (script, args) => ({ stdout: spawnSync("sh", ["-c", script, "test", ...args], { encoding: "utf8" }).stdout });
  return findProject({ sh }, null, [plain, proj]).then((p) => assert.equal(p, proj));
});

test("resources and source publish before a slow plugin read, and failures stay local", async () => {
  let finish;
  const slow = new Promise((resolve) => { finish = resolve; });
  const { io } = machine({ runs: { "status --json -g": { stdout: '{"source":{"path":"/src"}}' } }, scripts: { [GIT_STATUS]: { stdout: "norepo\n" } } });
  io.job = async (args) => {
    if (args[0] === "plugin") return slow;
    if (args[0] === "mcp") throw new Error("MCP unavailable");
    return { code: 0, stdout: args[0] === "list" ? "[]" : "{}", stderr: "" };
  };
  const updates = [];
  const read = loadResources(io, null, null, false, (value) => updates.push(value));
  // Drain only immediate reads: the plugins promise is deliberately still unresolved.
  for (let i = 0; i < 20; i++) await Promise.resolve();
  const partial = updates.at(-1);
  assert.ok(partial, "publish partial results without waiting for plugins");
  assert.deepEqual(partial.raw.global.list, []);
  assert.deepEqual(partial.raw.global.hooks, {});
  assert.equal(partial.git.repo, false);
  assert.deepEqual(partial.raw.global.pending, ["plugins"]);
  assert.match(partial.raw.global.errors.mcp, /MCP unavailable/);
  finish({ code: 0, stdout: '{"packages":{}}', stderr: "" });
  const result = await read;
  assert.deepEqual(result.raw.global.pending, []);
  assert.deepEqual(partial.raw.global.pending, ["plugins"], "published snapshots do not mutate");
});

test("refresh keeps cached resources on failure and retries them on the next read", async () => {
  const previous = { global: { list: [{ name: "cached" }], plugins: { packages: { cached: {} } } } };
  const { io } = machine({ jobs: { ...resources("-g"), "plugin list --json -g": { code: 1, stderr: "offline" } }, runs: { "status --json -g": emptyStatus } });
  const updates = [];
  const failed = await loadResources(io, null, null, false, (value) => updates.push(value), previous);
  assert.deepEqual(updates[0].raw.global.list, previous.global.list);
  assert.deepEqual(failed.raw.global.plugins, previous.global.plugins);
  assert.equal(failed.raw.global.errors.plugins, "offline");
  assert.deepEqual(previous.global, { list: [{ name: "cached" }], plugins: { packages: { cached: {} } } });
  const good = machine({ jobs: resources("-g"), runs: { "status --json -g": emptyStatus } });
  const retried = await loadResources(good.io, null, null, false, () => {}, failed.raw);
  assert.deepEqual(retried.raw.global.errors, {});
  assert.deepEqual(retried.raw.global.plugins, {});
});

test("global resources publish while the project's inventory is still reading", async () => {
  let finish;
  const slow = new Promise((resolve) => { finish = resolve; });
  const { io } = machine({ jobs: { ...resources("-g"), ...resources("-p") }, runs: { "status --json -g": emptyStatus, "status --json -p": emptyStatus } });
  const job = io.job;
  io.job = (args, opt) => args.join(" ") === "plugin list --json -p" ? slow : job(args, opt);
  const updates = [];
  const read = loadResources(io, "mini", "/work/app", false, (value) => updates.push(value));
  for (let i = 0; i < 20; i++) await Promise.resolve();
  const partial = updates.at(-1);
  assert.deepEqual(partial.raw.global.pending, []);
  assert.deepEqual(partial.raw.project.pending, ["plugins"]);
  finish({ code: 0, stdout: "{}", stderr: "" });
  await read;
});

test("each project resource runs in the project folder with its scope flag", async () => {
  const { io, seen } = machine({ jobs: { ...resources("-p"), ...resources("-g") } });
  await loadResources(io, "mini", "/work/app", false);
  assert.equal(seen.length, 10);
  for (const call of seen) {
    assert.equal(call.cwd, call.args.at(-1) === "-p" ? "/work/app" : null);
    assert.equal(call.on, "mini");
  }
});

test("a scope whose list did not parse carries the CLI's error", async () => {
  const { io } = machine({ jobs: { ...resources("-g"), "list --all --json -g": { code: 1, stdout: "", stderr: "config not found" } }, runs: { "status --json -g": emptyStatus } });
  assert.equal((await loadResources(io, null, null, false)).error, "global list: config not found");
});

test("a machine card counts targets that take skills and the source's git state", async () => {
  const { io } = machine({
    jobs: { "doctor --json -g": { stdout: JSON.stringify({ version: { current: "0.28.4", latest: "0.28.5", update_available: true } }) } },
    runs: { "status --json -g": { stdout: JSON.stringify({ source: { path: "/src" }, targets: [{ name: "claude" }, { name: "codex", skills_enabled: false }] }) } },
    scripts: { [GIT_STATUS]: { stdout: "top /src\n## main...origin/main [behind 2]\n" } },
  });
  const m = await loadMachine(io, "mini");
  assert.deepEqual([m.label, m.version, m.update, m.targets, m.git.behind], ["mini", "0.28.4", true, 1, 2]);
});

test("each scope's target names come from its own status", async () => {
  const names = (...n) => ({ stdout: JSON.stringify({ targets: [...n.map((name) => ({ name })), { name: "off", skills_enabled: false }], agents: { targets: [{ name: n[0] }] } }) });
  const { io } = machine({ jobs: { ...resources("-p"), ...resources("-g") }, runs: { "status --json -g": names("claude", "codex"), "status --json -p": names("cursor") } });
  const { raw } = await loadResources(io, null, "/work/app", false);
  assert.deepEqual([raw.global.targets, raw.project.targets], [["claude", "codex", "off"], ["cursor", "off"]]);
  assert.deepEqual([raw.global.skillTargets, raw.global.agentTargets, raw.project.agentTargets], [["claude", "codex"], ["claude"], ["cursor"]]);
});

test("a skill's preview has its files and its document without the frontmatter", async () => {
  const { io } = machine({ runs: { "status --json -g": { stdout: JSON.stringify({ source: { path: "/src" } }) } }, scripts: { [READ_SKILL]: { stdout: "SKILL.md\nrefs/a.md\n@@\n---\nname: x\n---\nBody\n" } } });
  const p = await loadPreview(io, null, null, { kind: "skills", scope: "global", rel: "x" });
  assert.deepEqual([p.files, p.dir, p.meta, p.body], [["SKILL.md", "refs/a.md"], "/src/x", [{ key: "name", value: "x" }], "Body\n"]);
});

test("a preview says so when the source or the skill cannot be read", async () => {
  const item = { kind: "skills", scope: "global", rel: "x" };
  assert.match((await loadPreview(machine().io, null, null, item)).error, /where this source is/);
  const { io } = machine({ runs: { "status --json -g": { stdout: JSON.stringify({ source: { path: "/src" } }) } }, scripts: { [READ_SKILL]: { code: 3 } } });
  assert.equal((await loadPreview(io, null, null, item)).error, "/src/x could not be read.");
});

test("an agent's preview reads its file from the agents source", async () => {
  const { io } = machine({ runs: { "status --json -g": { stdout: JSON.stringify({ agents: { source: "/agents" } }) } }, scripts: { [READ_FILE]: { stdout: "Review code.\n" } } });
  const p = await loadPreview(io, null, null, { kind: "agents", scope: "global", rel: "team/reviewer", label: "reviewer" });
  assert.deepEqual([p.files, p.dir, p.body], [["reviewer.md"], null, "Review code.\n"]);
});

test("an MCP preview never carries a secret's value", async () => {
  const yaml = "servers:\n  github:\n    command: npx\n    env:\n      GITHUB_TOKEN: ghp_secret\n";
  const { io } = machine({ scripts: { [READ_FILE]: { stdout: yaml } } });
  const p = await loadPreview(io, null, null, { kind: "mcp", scope: "global", name: "github" }, "/src/mcp.yaml");
  assert.ok(p.server && !JSON.stringify(p).includes("ghp_secret"));
});

test("another file of a skill is Markdown only when its name says so", async () => {
  const { io } = machine({ scripts: { [READ_FILE]: { stdout: "---\na: 1\n---\ntext" } } });
  assert.deepEqual(await loadFile(io, null, "/src/x", "notes.md"), { meta: [{ key: "a", value: "1" }], body: "text" });
  assert.deepEqual(await loadFile(io, null, "/src/x", "run.sh"), { meta: [], body: "---\na: 1\n---\ntext", plain: true });
});

test("a skill with more files than the list holds still offers SKILL.md", async () => {
  const { io } = machine({ runs: { "status --json -g": { stdout: JSON.stringify({ source: { path: "/src" } }) } }, scripts: { [READ_SKILL]: { stdout: "assets/a.png\n@@\nBody\n" } } });
  assert.deepEqual((await loadPreview(io, null, null, { kind: "skills", scope: "global", rel: "x" })).files, ["SKILL.md", "assets/a.png"]);
});

test("the latest release is the tag GitHub's link lands on, or none when it can't be reached", async () => {
  const up = machine({ scripts: { [LATEST_RELEASE]: { stdout: "https://github.com/runkids/skillshare/releases/tag/v0.31.0" } } });
  const down = machine({ scripts: { [LATEST_RELEASE]: { code: 6, stdout: "" } } });
  assert.deepEqual([await loadLatestRelease(up.io), await loadLatestRelease(down.io)], ["v0.31.0", null]);
});
