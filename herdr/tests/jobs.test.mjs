import { test } from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtempSync, writeFileSync, rmSync, symlinkSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createRunner, withShell } from "../skillshare/mods/skillshare/jobs.js";
import { SWEEP } from "../skillshare/mods/skillshare/model.js";

// A runner over real sh with a fake `skillshare` (body). Like Herdr, each exec's process group
// is ended when the exec returns. `scripts` records every script the runner executed.
function fakeRunner(t, body, opts = {}) {
  const root = mkdtempSync(join(tmpdir(), "ss-jobs-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeFileSync(join(root, "skillshare"), `#!/bin/sh\n${body}\n`, { mode: 0o755 });
  if (opts.cygpath) writeFileSync(join(root, "cygpath"), `#!/bin/sh\n${opts.cygpath(root)}\n`, { mode: 0o755 });
  const env = { ...process.env, PATH: `${root}:${process.env.PATH}`, TMPDIR: root };
  const scripts = [], reads = [];
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
  const readText = (path) => { reads.push(path); return readFile(path, "utf8").catch(() => null); };
  return { runner: createRunner({ exec, readText, pollMs: 20, aliveEvery: 2, ...opts }), scripts, reads, root };
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

test("where cygpath exists, the job's files are read at the path it prints", async (t) => {
  // Git for Windows' sh: Herdr reads files natively, so it needs C:/... rather than /tmp/...
  const alias = mkdtempSync(join(tmpdir(), "ss-alias-"));
  t.after(() => rmSync(alias, { recursive: true, force: true }));
  const { runner, reads, root } = fakeRunner(t, "echo ok", { cygpath: (r) => `printf %s "$2" | sed "s|^${r}|${alias}/root|"` });
  symlinkSync(root, join(alias, "root"));
  const r = await runner.job(["list"], { sleep });
  assert.deepEqual([r.stdout, reads.every((p) => p.startsWith(`${alias}/root/`))], ["ok\n", true]);
});

// A Herdr exec on a Windows machine: sh can't start, git is on PATH.
function windowsProgram(calls, gitPath = "C:/Program Files/Git/ucrt64/libexec/git-core\r\n") {
  return async (command, args, on) => {
    calls.push([command, on]);
    if (command === "sh") throw new Error("cannot start sh: program not found");
    if (command === "git") return { code: 0, stdout: gitPath, stderr: "" };
    return { code: 0, stdout: args.join(" "), stderr: "" };
  };
}

test("without sh, a script runs in Git for Windows' bin/sh.exe", async () => {
  const calls = [];
  const r = await withShell(windowsProgram(calls))(["-c", "echo hi"], "win");
  assert.deepEqual([calls.at(-1), r.stdout], [["C:/Program Files/Git/bin/sh.exe", "win"], "-c echo hi"]);
});

test("where sh starts, git is never asked", async () => {
  const calls = [];
  const shell = withShell(async (command, args, on) => { calls.push(command); return { code: 0, stdout: "", stderr: "" }; });
  for (const on of [null, null, "linux"]) await shell(["-c", ":"], on);
  assert.deepEqual(calls, ["sh", "sh", "sh"]);
});

test("each machine looks for its shell once", async () => {
  const calls = [];
  const shell = withShell(windowsProgram(calls));
  for (const on of ["win", "win", null]) await shell(["-c", ":"], on);
  assert.deepEqual(calls.filter(([c]) => c === "git").map(([, on]) => on), ["win", null]);
});

test("without sh or Git for Windows, the reason sh could not start stands", async () => {
  const shell = withShell(windowsProgram([], "/usr/lib/git-core\n"));
  await assert.rejects(shell(["-c", ":"]), /cannot start sh/);
});
