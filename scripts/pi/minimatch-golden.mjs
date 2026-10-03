// Golden data for the Go port of Pi's resource-filter glob matching (issue #342).
//
// Evaluates each (pattern, subject) pair with the minimatch that Pi itself
// imports, called the way package-manager.js matchesAnyPattern calls it (no
// options). The Go matcher in internal/plugin supports only a restricted
// grammar; this file proves that grammar agrees with Pi byte for byte.
//
// Run inside the devcontainer only (reads files, writes stdout):
//   PI_ROOT=/opt/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent \
//   node scripts/pi/minimatch-golden.mjs > internal/plugin/testdata/pi_minimatch_golden.json

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const piRoot = process.env.PI_ROOT;
if (!piRoot) throw new Error("PI_ROOT is required");
const mmRoot = join(piRoot, "node_modules", "minimatch");
const mmPkg = JSON.parse(readFileSync(join(mmRoot, "package.json"), "utf8"));
const entry = mmPkg.exports["."].import.default ?? mmPkg.exports["."].import;
const { minimatch } = await import(pathToFileURL(join(mmRoot, entry)).href);
const piVersion = JSON.parse(readFileSync(join(piRoot, "package.json"), "utf8")).version;

const patterns = [
  "extensions/a.ts", "a.ts", "*.ts", "*", "?.ts", "extensions/*.ts", "extensions/*", "*/a.ts", "ext*/a*.ts",
  "extensions/slow-*.ts", "extensions/?.ts", "/pkg/extensions/*.ts", "/pkg/*/a.ts", ".hidden.ts", "extensions/.h*",
  "*.js", "extensions/sub/index.ts", "extensions/*/index.ts", "a*", "*a", "*-lint.ts",
  // Non-ASCII: minimatch's '?' is one UTF-16 code unit, so an astral character needs two.
  "??.ts", "???.ts", "é*.ts", "中?.ts", "😀*.ts", "😀.ts", "extensions/??.ts", "a?.ts",
];
const subjects = [
  "extensions/a.ts", "a.ts", "extensions/b.js", "b.js", "extensions/slow-lint.ts", "slow-lint.ts",
  "extensions/.hidden.ts", ".hidden.ts", "extensions/sub/index.ts", "index.ts", "/pkg/extensions/a.ts",
  "/pkg/extensions/.hidden.ts", "/pkg/extensions/sub/index.ts", "extensions", "ab.ts", "x/a.ts",
  "é.ts", "中文.ts", "中.ts", "😀.ts", "a😀.ts", "extensions/😀.ts", "extensions/é.ts", "/pkg/extensions/😀.ts",
];
const cases = [];
for (const pattern of patterns) for (const subject of subjects) cases.push({ pattern, subject, match: minimatch(subject, pattern) });
console.log(JSON.stringify({ pi: piVersion, minimatch: mmPkg.version, cases }, null, 1));
