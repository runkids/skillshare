// Resolves a project's packages the way the pi CLI does and prints, as JSON, every
// extension, skill and prompt with whether it is enabled. Trust is passed in, so
// no trust decision is read or written, and resolving never imports a module.
// Used by TestPiProjectOverridesResolveInPi.
//
//   PI_ROOT=<pi package> node scripts/pi/resolve-probe.mjs <cwd> <agentDir> <trusted|untrusted>
import { pathToFileURL } from "node:url";
import { join } from "node:path";

const [cwd, agentDir, trust] = process.argv.slice(2);
const piRoot = process.env.PI_ROOT;
if (!cwd || !agentDir || !["trusted", "untrusted"].includes(trust) || !piRoot) {
  console.error("usage: PI_ROOT=... node resolve-probe.mjs <cwd> <agentDir> <trusted|untrusted>");
  process.exit(2);
}
// The bundle the CLI runs; its index only re-exports, so importing it runs nothing.
const { DefaultPackageManager, SettingsManager } = await import(pathToFileURL(join(piRoot, "dist", "bundle", "index.js")).href);
const settingsManager = SettingsManager.create(cwd, agentDir, { projectTrusted: trust === "trusted" });
const errors = settingsManager.drainErrors();
if (errors.length) {
  console.error(JSON.stringify(errors));
  process.exit(1);
}
const pm = new DefaultPackageManager({ cwd, agentDir, settingsManager });
const r = await pm.resolve(async () => "skip");
const list = (items) => Object.fromEntries(items.map((x) => [x.path, x.enabled]));
console.log(JSON.stringify({ extensions: list(r.extensions), skills: list(r.skills), prompts: list(r.prompts) }));
