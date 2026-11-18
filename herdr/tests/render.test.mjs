import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { compileFunction } from "node:vm";
import { BUSY } from "../skillshare/mods/skillshare/channel.js";
import { buildItems, previewTargets } from "../skillshare/mods/skillshare/model.js";

// Render the actual view against a tiny UI tree, without starting Herdr or any CLI.
function element() {
  const node = { nodes: [] };
  const el = new Proxy(node, { get(target, key) {
    if (key in target) return target[key];
    return (...args) => {
      if (key === "child") node.nodes.push(args[0]);
      else if (key === "children") node.nodes.push(...args[0]);
      else if (key === "when") { if (args[0]) args[1](el); }
      else if (key === "id") node.id = args[0];
      else if (key === "spin") node.spinning = true;
      else if (key === "on_click") node.click = args[0];
      return el;
    };
  } });
  return el;
}
const flatten = (node) => typeof node === "object" && node ? [node, ...node.nodes.flatMap(flatten)] : [];
const label = (node) => node.nodes.map((child) => typeof child === "object" ? label(child) : child).join(" ");

async function panel(dialogProps = {}, overrides = {}) {
  const shared = new Map();
  const url = new URL("../skillshare/mods/skillshare/main.js", import.meta.url);
  const source = await readFile(url, "utf8"), bindings = {};
  const imports = [...source.matchAll(/import\s*\{([^}]+)\}\s*from\s*"([^"]+)";/g)];
  for (const [, names, specifier] of imports) {
    let exports;
    if (specifier === "gpui-kit") exports = { View: class {}, div: element, image: element, svg: element };
    else if (specifier === "gpui-base") exports = { v_flex: element, h_flex: element, InputState: {}, Input: { new: element }, TextView: {}, HoverCard: {} };
    else if (specifier === "herdr") {
      exports = Object.fromEntries(["exec", "hc", "run", "spaces", "agents", "focused", "machines", "setShared", "theme", "layout", "openOverlay", "closeOverlay", "nextCommand", "readTextFile", "knownAgents", "props"].map((key) => [key, () => {}]));
      exports.shared = (key) => shared.get(key);
      exports.settings = () => ({});
      exports.theme = () => ({});
      exports.props = () => dialogProps;
    } else exports = await import(new URL(specifier, url));
    for (const name of names.split(",").map((name) => name.trim()).filter(Boolean)) {
      const [key, alias = key] = name.split(/\s+as\s+/);
      bindings[alias] = exports[key];
    }
  }
  const body = source.replace(/import\s*\{[^}]+\}\s*from\s*"[^"]+";/g, "").replace("export default class", "return class");
  Object.assign(bindings, overrides);
  const Panel = compileFunction(body, Object.keys(bindings))(...Object.values(bindings));
  const view = new Panel();
  view.pending = [];
  view.machines = { mini: { on: "mini", label: "mini", version: "0.30.4", latest: "0.30.5", update: true, targets: 4, git: { repo: true, upstream: "origin/main", files: [], ahead: 0, behind: 0 } } };
  Object.assign(view, { raw: {}, items: [], ctx: { on: "mini" }, project: null, scope: "global", kind: "plugins", query: "", checks: {}, loadedAt: 0 });
  return { view, shared };
}

test("partial overview and library show available data without zero plugins or a sync verdict", async () => {
  const { view } = await panel();
  view.raw = { global: { list: [], diff: { targets: [{ name: "codex", items: [] }] }, pending: ["plugins"], errors: {} } };
  const overview = view.renderOverview({}, view.ctx), text = label(overview);
  assert.match(text, /Still reading plugins/);
  assert.doesNotMatch(text, /Everything is in sync/);
  assert.match(label(flatten(overview).find((node) => node.id === "have-plugins")), /…/);
  assert.match(text, /unconfirmed/);
  const library = view.renderLibrary({}, view.ctx);
  assert.match(label(library), /Plugins …/);
  assert.match(label(library), /inventory is not confirmed/);
  assert.doesNotMatch(label(library), /No plugins here yet/);
});

test("cached positive drift is unconfirmed while its refresh is pending or failed", async () => {
  const { view } = await panel();
  for (const state of [{ pending: ["diff"], errors: {} }, { pending: [], errors: { diff: "offline" } }]) {
    view.raw = { global: { list: [], diff: { targets: [{ name: "codex", items: [{ name: "old", is_sync: true }] }] }, ...state } };
    const text = label(view.renderOverview({}, view.ctx));
    assert.doesNotMatch(text, /target needs a sync|to sync/);
    assert.match(text, /unconfirmed/);
  }
});

test("Overview keeps previously actionable rows visible during pending or failed refreshes", async () => {
  const { view } = await panel();
  view.loadedAt = 1;
  for (const state of [{ pending: ["diff"], errors: {} }, { pending: [], errors: { diff: "offline" } }]) {
    view.raw = { global: { list: [{ name: "cached-skill" }], diff: { targets: [{ name: "codex", items: [{ name: "cached-skill", kind: "skill", action: "modify", is_sync: true }] }] }, ...state } };
    view.rebuild();
    const text = label(view.renderOverview({}, view.ctx));
    assert.match(text, /cached-skill/);
    assert.match(text, /sync status unknown/);
    assert.doesNotMatch(text, /WAITING TO SYNC/);
  }
});

test("another scope's failure never overrides an empty successful selected scope", async () => {
  const raw = { global: { list: [], pending: [], errors: {} }, project: { pending: [], errors: { plugins: "offline" } } };
  const { view } = await panel({}, { findProject: async () => "/work/app", loadChecks: async () => ({}),
    loadResources: async (_io, _on, _project, _network, progress) => {
      const res = { raw, git: { repo: false }, error: "project plugins: offline" };
      progress(res);
      return res;
    } });
  const tasks = [], cx = { spawn: (fn) => tasks.push(fn(cx)), notify: () => {} };
  view.refresh(cx, false);
  await Promise.all(tasks);
  assert.match(label(view.renderOverview({}, view.ctx)), /Everything is in sync/);
  assert.match(label(view.renderLibrary({}, view.ctx)), /No plugins here yet/);
  assert.doesNotMatch(label(view.renderOverview({}, view.ctx)), /offline|could not be read/);
});

test("unknown plugin previews never claim synchronization in About or Targets", async () => {
  const { view } = await panel();
  const item = { kind: "plugins", state: "unknown", label: "cached", scope: "global", targets: ["codex"], src: "test" };
  for (const pane of ["main", "targets"]) {
    view.pane = pane;
    const text = label(view.renderPreview({}, { item, targets: [{ name: "codex", text: "sync status unknown", warn: false }] }, () => {}));
    assert.doesNotMatch(text, /in sync|In sync/);
    assert.match(text, /unknown|unconfirmed/);
  }
});

test("Source dialog preserves read errors instead of reporting a non-repository", async () => {
  const { view } = await panel({ role: "source", git: { error: "offline" } });
  const text = label(view.dialogBody({}));
  assert.match(text, /offline/);
  assert.doesNotMatch(text, /not in a git repository/);
});

test("pull spins only its own button while upgrade is disabled", async () => {
  const { view, shared } = await panel();
  shared.set(BUSY, { mini: { text: "Pulling on mini…", at: Date.now() } });
  view.pending = [{ op: "pullsync", on: "mini" }];
  const nodes = flatten(view.renderMachines({}));
  const pull = nodes.find((node) => node.id === "ps-mini"), update = nodes.find((node) => node.id === "up-mini");
  assert.ok(flatten(pull).some((node) => node.spinning === true));
  assert.ok(!flatten(update).some((node) => node.spinning === true));
  assert.equal("click" in update, false);
});

test("machine upgrade spins only its update button and disables pull without a spinner", async () => {
  const { view, shared } = await panel();
  shared.set(BUSY, { mini: { text: "Updating skillshare on mini…", at: Date.now() } });
  view.pending = [{ op: "upgrade", on: "mini" }];
  const nodes = flatten(view.renderMachines({}));
  const update = nodes.find((node) => node.id === "up-mini");
  const pull = nodes.find((node) => node.id === "ps-mini");
  assert.ok(flatten(update).some((node) => node.spinning === true), "update owns the spinner");
  assert.ok(!flatten(pull).some((node) => node.spinning === true), "pull must not imply it is running");
  assert.equal(label(pull), "Pull and sync");
  assert.equal("click" in update, false);
  assert.equal("click" in pull, false);
});

test("an extra whose source is missing needs a look on Overview and in Library, and never waits for a sync", async () => {
  const { view } = await panel();
  view.kind = "extras";
  view.raw = { global: { list: [], diff: { targets: [] }, targets: ["claude"], pending: [], errors: {},
    extras: [{ name: "gone", source_dir: "/src/gone", file_count: 0, targets: [{ path: "/home/me/.claude/rules", mode: "merge", status: "no source" }] }] } };
  view.rebuild();
  const overview = label(view.renderOverview({}, view.ctx));
  assert.match(overview, /NEED A LOOK/);
  assert.doesNotMatch(overview, /HOOKS NEED A LOOK|WAITING TO SYNC|Everything is in sync/);
  assert.match(overview, /1 extra needs a look/);
  assert.match(overview, /gone/);
  const library = label(view.renderLibrary({}, view.ctx));
  assert.match(library, /NEED A LOOK · 1/);
  assert.match(library, /source missing/);
  assert.doesNotMatch(library, /IN SYNC|NEEDS SYNC/);
});

test("the preview of an extra whose source is missing says so in every tab, with nothing to retry", async () => {
  const { view } = await panel({ role: "preview" });
  const [item] = buildItems("global", { extras: [{ name: "gone", source_dir: "/src/gone", source_exists: false, file_count: 0, targets: [{ path: "/home/me/.claude/rules", mode: "merge", status: "no source" }] }] });
  view.doc = { gone: "/src/gone is missing. Create it, or remove the extra." };
  view.pane = "main";
  const main = label(view.renderPreview({}, { item, targets: previewTargets(item) }, () => {}));
  assert.match(main, /Source missing/);
  assert.match(main, /The source is missing/);
  assert.match(main, /\/src\/gone is missing/);
  assert.doesNotMatch(main, /could not be read|Try again/);
  view.pane = "targets";
  const targets = label(view.renderPreview({}, { item, targets: previewTargets(item) }, () => {}));
  assert.match(targets, /Source missing, so a sync can't send it/);
  assert.doesNotMatch(targets, /In sync with/);
});

test("an extra's preview shows its frontmatter above the document, and highlights the file it shows", async () => {
  const { view } = await panel({ role: "preview" }, { TextView: { markdown: element },
    loadPreview: async () => ({ files: ["SKILL.md", "rules.md"], file: "rules.md", dir: "/src/rules", meta: [{ key: "paths", value: "src/**" }], body: "Body" }) });
  const item = { kind: "extras", extra: "folder", key: "global:extras:rules", label: "rules", scope: "global", state: "ok", rows: [], dir: "/src/rules" };
  const cx = { spawn: (fn) => fn(cx), notify: () => {} };
  await view.loadDoc(cx);
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(view.file, "rules.md");
  view.pane = "main";
  const text = label(view.renderPreview({}, { item, targets: [] }, () => {}));
  assert.match(text, /paths/);
  assert.match(text, /src\/\*\*/);
});
