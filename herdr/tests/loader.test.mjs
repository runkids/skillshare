import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { findProject, loadResources, loadMachine, loadLatestRelease, loadPreview, loadFile } from "../skillshare/mods/skillshare/loader.js";
import { SNAPSHOT, GIT_STATUS, READ_SKILL, READ_FILE, LATEST_RELEASE } from "../skillshare/mods/skillshare/model.js";

// A fake machine answering by command: `jobs` and `runs` keyed by skillshare args joined by
// spaces (a snapshot by its scope flag), `scripts` by script. `seen` records each call's options.
function machine({ jobs = {}, runs = {}, scripts = {} } = {}) {
  const seen = [];
  const ok = { code: 0, stdout: "", stderr: "" };
  return {
    seen,
    io: {
      job: async (args, opt) => { seen.push({ args, ...opt }); return { ...ok, ...(opt.script === SNAPSHOT ? jobs[`snapshot ${args[0]}`] : jobs[args.join(" ")]) }; },
      run: async (args) => ({ ...ok, ...runs[args.join(" ")] }),
      sh: async (script) => ({ ...ok, ...scripts[script] }),
    },
  };
}
const snapshot = (list) => ({ stdout: `@@list 0\n${JSON.stringify(list)}\n@@diff 0\n{}\n@@mcp 0\n[]\n@@plugins 0\n[]\n@@hooks 0\n[]\n` });

test("the first folder skillshare treats as a project wins", (t) => {
  const root = mkdtempSync(join(tmpdir(), "ss-proj-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const plain = join(root, "plain"), proj = join(root, "proj");
  mkdirSync(plain); mkdirSync(join(proj, ".skillshare"), { recursive: true });
  writeFileSync(join(proj, ".skillshare", "config.yaml"), "");
  const sh = async (script, args) => ({ stdout: spawnSync("sh", ["-c", script, "test", ...args], { encoding: "utf8" }).stdout });
  return findProject({ sh }, null, [plain, proj]).then((p) => assert.equal(p, proj));
});

test("a project's snapshot runs in the project folder", async () => {
  const { io, seen } = machine({ jobs: { "snapshot -p": snapshot([]), "snapshot -g": snapshot([]) } });
  await loadResources(io, "mini", "/work/app", false);
  assert.deepEqual(seen.map((s) => [s.args[0], s.cwd]).sort(), [["-g", null], ["-p", "/work/app"]]);
});

test("a scope whose list did not parse carries the CLI's error", async () => {
  const { io } = machine({ jobs: { "snapshot -g": { code: 1, stdout: "", stderr: "config not found" } } });
  assert.equal((await loadResources(io, null, null, false)).error, "config not found");
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
  const { io } = machine({ jobs: { "snapshot -p": snapshot([]), "snapshot -g": snapshot([]) }, runs: { "status --json -g": names("claude", "codex"), "status --json -p": names("cursor") } });
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
