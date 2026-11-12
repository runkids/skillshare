import { test } from "node:test";
import { execFileSync } from "node:child_process";
import assert from "node:assert/strict";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, unlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const source = join(root, "skillshare"), standalone = join(root, "skillshare-mod");
// Herdr checks create ignored editor files; compare only files Git would publish.
const files = (path) => execFileSync("git", ["ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", `${path}/`], { cwd: join(root, ".."), encoding: "utf8" })
  .split("\0").filter((name) => name && existsSync(join(root, "..", name))).map((name) => name.slice(`${path}/`.length)).sort();

test("the package generator handles an unstaged canonical asset deletion", (t) => {
  const dir = mkdtempSync(join(tmpdir(), "ss-herdr-package-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const put = (name, content) => { mkdirSync(join(dir, name, ".."), { recursive: true }); writeFileSync(join(dir, name), content); };
  put("herdr/skillshare/mods/skillshare/main.js", "panel");
  put("herdr/skillshare/mods/skillshare/old.svg", "old asset");
  put("herdr/skillshare/panels.png", "preview");
  put("scripts/herdr/package-mod.py", "");
  copyFileSync(join(root, "../scripts/herdr/package-mod.py"), join(dir, "scripts/herdr/package-mod.py"));
  const run = (...args) => execFileSync("python3", ["scripts/herdr/package-mod.py", ...args], { cwd: dir });
  execFileSync("git", ["init", "-q"], { cwd: dir });
  run();
  execFileSync("git", ["add", "."], { cwd: dir });
  unlinkSync(join(dir, "herdr/skillshare/mods/skillshare/old.svg"));
  run();
  assert.ok(!existsSync(join(dir, "herdr/skillshare-mod/mods/skillshare/old.svg")));
  run("--check");
});

test("the standalone mod is a complete gallery package without a layout", () => {
  assert.ok(existsSync(join(standalone, "RECIPE.md")), "standalone mod recipe is missing");
  assert.ok(!existsSync(join(standalone, "layouts")), "a mod share folder must not include layouts");
  assert.deepEqual(readFileSync(join(standalone, "preview.png")), readFileSync(join(source, "panels.png")));
  const path = "mods/skillshare", expected = files(`herdr/skillshare/${path}`), actual = files(`herdr/skillshare-mod/${path}`);
  assert.ok(!actual.includes("gpui-kit.d.ts"), "local editor files must not ship");
  assert.ok(!actual.includes("jsconfig.json"), "local editor files must not ship");
  assert.deepEqual(actual, expected);
  for (const name of expected) {
    assert.deepEqual(readFileSync(join(standalone, path, name)), readFileSync(join(source, path, name)), `${name}: regenerate with python3 scripts/herdr/package-mod.py`);
  }
});
