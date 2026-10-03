// Tries to take Pi's settings lock the way Pi 0.99.2 does (settings-manager.js:
// lockfile.lockSync(path, { realpath: false })) and releases it at once. Prints
// "acquired" or "locked". Used by TestPiNativeLockHoldsAgainstPi.
//
//   PI_ROOT=/opt/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent \
//     node scripts/pi/lock-probe.mjs <settings.json>
import { createRequire } from "node:module";
import { join } from "node:path";

const [file] = process.argv.slice(2);
const piRoot = process.env.PI_ROOT;
if (!file || !piRoot) {
  console.error("usage: PI_ROOT=... node lock-probe.mjs <settings.json>");
  process.exit(2);
}
const lockfile = createRequire(join(piRoot, "package.json"))("proper-lockfile");
try {
  const release = lockfile.lockSync(file, { realpath: false });
  release();
  console.log("acquired");
} catch (e) {
  if (e.code !== "ELOCKED") throw e;
  console.log("locked");
}
