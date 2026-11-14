// Optional Playwright proof against a task-owned dashboard and synthetic HOME.
// NODE_PATH must expose playwright; no browser or package dependency is bundled.
const { chromium } = require('playwright');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');

(async () => {
  const home = process.env.PLUGIN_FIXTURE_HOME;
  const binary = process.env.PLUGIN_FIXTURE_BINARY;
  const out = process.env.PLUGIN_SCREENSHOTS;
  assert(home && binary && out, 'Set synthetic PLUGIN_FIXTURE_HOME, PLUGIN_FIXTURE_BINARY and PLUGIN_SCREENSHOTS');
  const source = path.join(home, 'source');
  const command = path.join(home, '.local/bin/demo-one');
  const script = path.join(source, 'bin/one');
  const env = { ...process.env, HOME: home, SKILLSHARE_CONFIG: path.join(home, '.config/skillshare/config.yaml'),
    XDG_STATE_HOME: path.join(home, '.local/state'), XDG_CACHE_HOME: path.join(home, '.cache'),
    CODEX_HOME: '', CLAUDE_CONFIG_DIR: '', PI_CODING_AGENT_DIR: '', OMP_CONFIG_DIR: '' };
  const cli = (...args) => execFileSync(binary, [...args, '--json', '-g'], { env, stdio: ['ignore', 'pipe', 'pipe'] });
  fs.mkdirSync(out, { recursive: true });
  cli('plugin', 'remove', 'demo');
  fs.rmSync(path.join(source, '.cursor-plugin'), { recursive: true, force: true });
  fs.writeFileSync(script, '#!/bin/sh\nprintf original\n');
  cli('plugin', 'add', source);
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    const sent = [];
    page.on('request', (r) => { if (r.url().endsWith('/api/plugins/update/stream')) sent.push(r.postDataJSON()); });
    const open = async () => {
      await page.goto(process.env.PLUGIN_DASHBOARD_URL || 'http://127.0.0.1:9104/plugins');
      await page.getByRole('button', { name: 'Update 1', exact: true }).click();
    };
    const capture = (name) => page.screenshot({ path: path.join(out, `${name}.png`), fullPage: true, animations: 'disabled' });
    fs.writeFileSync(script, '#!/bin/sh\nprintf first-update\n');
    await open();
    const dialog = page.getByRole('dialog');
    assert.match(await dialog.innerText(), /Commands/);
    assert.doesNotMatch(await dialog.innerText(), /1 Agent/);
    await capture('dashboard-command-only');
    await dialog.getByRole('button', { name: 'Update 1 plugin', exact: true }).click();
    await dialog.getByText('Updated 1 plugin', { exact: true }).waitFor();
    assert.deepEqual(sent[0].items[0].targets, []);
    assert.equal(sent[0].items[0].commands, true);
    assert.equal(execFileSync(command, { encoding: 'utf8' }), 'first-update');
    await capture('dashboard-command-updated');

    fs.mkdirSync(path.join(source, '.cursor-plugin'), { recursive: true });
    fs.writeFileSync(path.join(source, '.cursor-plugin/plugin.json'), '{"name":"demo","version":"1.0.0"}');
    cli('plugin', 'add', source, '--name', 'demo', '--target', 'cursor');
    fs.writeFileSync(script, '#!/bin/sh\nprintf second-update\n');
    await open();
    assert.match(await dialog.innerText(), /Cursor/);
    assert.match(await dialog.innerText(), /Commands/);
    const owned = fs.readlinkSync(command);
    fs.unlinkSync(command);
    fs.writeFileSync(command, 'operator');
    await dialog.getByRole('button', { name: 'Update 1 plugin', exact: true }).click();
    await dialog.getByRole('button', { name: 'Retry', exact: true }).waitFor();
    assert.match(await dialog.innerText(), /Cursor/);
    assert.equal(fs.readFileSync(command, 'utf8'), 'operator');
    await capture('dashboard-mixed-failure');
    fs.unlinkSync(command);
    fs.symlinkSync(owned, command);
    await dialog.getByRole('button', { name: 'Retry', exact: true }).click();
    await dialog.getByText('Updated 1 plugin', { exact: true }).waitFor();
    assert.deepEqual(sent.at(-1).items[0].targets, []);
    assert.equal(sent.at(-1).items[0].commands, true);
    assert.equal(execFileSync(command, { encoding: 'utf8' }), 'second-update');
    await capture('dashboard-retry');
    fs.writeFileSync(path.join(out, 'dashboard-requests.json'), JSON.stringify(sent, null, 2));
    console.log('PASS command-only selection, native success/command failure, operator preservation, command-only retry');
  } finally {
    await browser.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
