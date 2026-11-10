import { test } from "node:test";
import assert from "node:assert/strict";
import { createChannel, BUSY, EVERY } from "../skillshare/mods/skillshare/channel.js";

// A channel over an in-memory map, standing in for Herdr's shared values. One map shared by
// several channels is the panel and a dialog.
function channels(n = 1, start = 0) {
  const store = new Map();
  const clock = { t: start };
  const make = () => createChannel({ get: (k) => store.get(k), set: (k, v) => store.set(k, v), now: () => clock.t });
  return { store, clock, list: Array.from({ length: n }, make) };
}

test("the panel gets each request made after it listened, once", () => {
  const { list: [panel, dialog] } = channels(2);
  dialog.request({ op: "old" });
  const next = panel.listen();
  dialog.request({ op: "pull" });
  assert.deepEqual([next()?.op, next()], ["pull", null]);
});

test("a dialog cannot claim a machine the panel is working on", () => {
  const { list: [panel, dialog] } = channels(2);
  panel.claim("mini", "Syncing global…");
  assert.equal(dialog.claim("mini", "Installing…"), false);
});

test("other machines stay free while one is busy", () => {
  const { list: [panel, dialog] } = channels(2);
  panel.claim("mini", "Syncing global…");
  assert.equal(dialog.claim(null, "Installing…"), true);
});

test("a released machine can be claimed again", () => {
  const { list: [panel, dialog] } = channels(2);
  panel.claim("mini", "Syncing global…");
  panel.release("mini");
  assert.equal(dialog.claim("mini", "Installing…"), true);
});

test("busy text lists what every machine is doing", () => {
  const { list: [panel] } = channels();
  panel.claim(null, "Pulling…");
  panel.claim("mini", "Updating figlens…");
  assert.equal(panel.busyText(), "Pulling… · Updating figlens…");
});

test("a lock whose holder went away stops blocking after a minute", () => {
  const { clock, list: [dialog] } = channels();
  dialog.claim("mini", "Installing figlens…");
  clock.t += 61 * 1000;
  assert.equal(dialog.busyAt("mini"), false);
});

test("a holder that keeps touching its lock keeps it through a long job", () => {
  const { clock, list: [dialog, panel] } = channels(2);
  dialog.claim("mini", "Installing figlens…");
  for (let i = 0; i < 60; i++) { clock.t += 30 * 1000; dialog.touch("mini"); }
  assert.equal(panel.claim("mini", "Syncing global…"), false);
});

test("a busy value left by an older panel reads as free", () => {
  const { store, list: [panel] } = channels();
  store.set(BUSY, "Syncing global…");
  assert.equal(panel.claim(null, "Installing…"), true);
});

test("a machine's own work reads back by machine", () => {
  const { list: [panel] } = channels();
  panel.claim("mini", "Syncing global…");
  assert.deepEqual([panel.textAt("mini"), panel.textAt(null)], ["Syncing global…", null]);
});

test("a lock on every machine waits for all work and then holds each machine", () => {
  const { list: [ch] } = channels();
  ch.claim("mini", "Syncing…");
  const early = ch.claim(EVERY, "Updating the panel…");
  ch.release("mini");
  const taken = ch.claim(EVERY, "Updating the panel…");
  assert.deepEqual([early, taken, ch.busyAt(null), ch.claim("mini", "Syncing…"), ch.textAt("mini")], [false, true, true, false, "Updating the panel…"]);
});
