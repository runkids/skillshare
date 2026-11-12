// Runs skillshare on a machine: short reads as one exec, longer work as a detached job that is
// polled until it writes its exit code. No Herdr imports: main.js passes Herdr's exec and file
// reads in, and the tests pass real sh and fs.
import { START, START_SCRIPT, ALIVE, SWEEP, JOB_MINUTES, lastLine } from "./model.js";

// createRunner({ exec, readText }):
//   exec(script, args, on) runs `sh -c script` on machine `on` (null: this computer) and resolves
//     to { code, stdout, stderr }; it must not reject. Herdr stops an exec after 30 s.
//   readText(path, on) resolves to the file's text, or null when it can't be read.
// The returned runner:
//   sh(script, args, on): one exec of a script of ours.
//   run(args, { on, cwd }): one exec of `skillshare args` in cwd (home when null). Short reads only.
//   job(args, { on, cwd, script, sleep }): `skillshare args`, or with `script` a script of ours
//     given args, run detached. Resolves when it ends to { code, stdout, stderr }; a job that
//     dies without an exit code, or runs past JOB_MINUTES, resolves with code 1 and the reason.
//     `sleep(ms)` waits between polls (a Herdr view passes cx.sleep).
// The first job on each machine also removes job folders that a closed panel never collected.
export function createRunner({ exec, readText, pollMs = 500, aliveEvery = 10, budgetMs = JOB_MINUTES * 60 * 1000 }) {
  const swept = new Set();
  const read = (path, on) => Promise.resolve().then(() => readText(path, on)).catch(() => null);
  const sh = (script, args = [], on = null) => exec(script, args, on);
  const run = (args, { on = null, cwd = null } = {}) =>
    sh('cd "$1" 2>/dev/null || cd; shift; exec skillshare "$@"', [cwd ?? "", ...args], on);

  async function job(args, { on = null, cwd = null, script = null, sleep }) {
    if (!swept.has(on ?? "")) { swept.add(on ?? ""); sh(SWEEP, [], on); }
    const s = script ? await sh(START_SCRIPT, [cwd ?? "", script, ...args], on) : await sh(START, [cwd ?? "", ...args], on);
    const d = s.stdout.trim();
    if (s.code !== 0 || !d) return { code: s.code || 1, stdout: "", stderr: s.stderr || "Could not start skillshare" };
    const done = async (result) => { sh('rm -rf "$1"', [d], on); return result; };
    const polls = Math.ceil(budgetMs / pollMs);
    for (let i = 1; i <= polls; i++) {
      await sleep(pollMs);
      const code = await read(`${d}/code`, on);
      if (code !== null && code.trim() !== "") {
        const [stdout, stderr] = await Promise.all([read(`${d}/out`, on), read(`${d}/err`, on)]);
        return done({ code: Number(code.trim()), stdout: stdout ?? "", stderr: stderr ?? "" });
      }
      // A job that was killed never writes its code, so check now and then that it still runs.
      if (i % aliveEvery === 0 && (await sh(ALIVE, [d], on)).code === 1) {
        const err = await read(`${d}/err`, on);
        return done({ code: 1, stdout: "", stderr: lastLine(err) || "skillshare stopped before it finished" });
      }
    }
    return { code: 1, stdout: "", stderr: `skillshare took longer than ${JOB_MINUTES} minutes` };
  }

  return { sh, run, job };
}
