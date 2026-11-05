import { test } from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createRunner } from "../skillshare/mods/skillshare/jobs.js";
import { SWEEP } from "../skillshare/mods/skillshare/model.js";

// A runner over real sh with a fake `skillshare` (body). Like Herdr, each exec's process group
// is ended when the exec returns. `scripts` records every script the runner executed.
function fakeRunner(t, body, opts = {}) {
  const root = mkdtempSync(join(tmpdir(), "ss-jobs-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeFileSync(join(root, "skillshare"), `#!/bin/sh\n${body}\n`, { mode: 0o755 });
  const env = { ...process.env, PATH: `${root}:${process.env.PATH}`, TMPDIR: root };
  const scripts = [];
  const exec = (script, args, on) => new Promise((resolve) => {
    scripts.push({ script, on });
    const sh = spawn("sh", ["-c", script, "test", ...args], { env, detached: true });
    let stdout = "", stderr = "";
    sh.stdout.on("data", (b) => { stdout += b; });
    sh.stderr.on("data", (b) => { stderr += b; });
    sh.on("exit", (code) => {
      try { process.kill(-sh.pid, "SIGKILL"); } catch { /* group already gone */ }
      resolve({ code: code ?? 1, stdout, stderr });
    });
  });
  const readText = (path) => readFile(path, "utf8").catch(() => null);
  return { runner: createRunner({ exec, readText, pollMs: 20, aliveEvery: 2, ...opts }), scripts };
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

test("a job resolves with skillshare's exit code and output", async (t) => {
  const { runner } = fakeRunner(t, 'echo "$@"; echo oops >&2; exit 3');
  assert.deepEqual(await runner.job(["update", "--all"], { sleep }), { code: 3, stdout: "update --all\n", stderr: "oops\n" });
});

test("a job that dies without an exit code reports that it stopped", async (t) => {
  const { runner } = fakeRunner(t, "kill -9 -$(ps -o pgid= -p $$ | tr -d ' ')");
  const r = await runner.job(["sync"], { sleep });
  assert.deepEqual([r.code, r.stderr], [1, "skillshare stopped before it finished"]);
});

test("a job past the time budget gives up with the reason", async (t) => {
  const { runner } = fakeRunner(t, "sleep 2", { budgetMs: 100, aliveEvery: 1000 });
  const r = await runner.job(["check"], { sleep });
  assert.deepEqual([r.code, r.stderr], [1, "skillshare took longer than 15 minutes"]);
});

test("the first job on each machine sweeps old job folders once", async (t) => {
  const { runner, scripts } = fakeRunner(t, "true");
  await runner.job(["a"], { sleep });
  await runner.job(["b"], { sleep });
  await runner.job(["c"], { on: "other", sleep });
  assert.deepEqual(scripts.filter((s) => s.script === SWEEP).map((s) => s.on), [null, "other"]);
});
