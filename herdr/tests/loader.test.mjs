import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { findProject, loadResources, loadMachine } from "../skillshare/mods/skillshare/loader.js";
import { SNAPSHOT, GIT_STATUS } from "../skillshare/mods/skillshare/model.js";

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
