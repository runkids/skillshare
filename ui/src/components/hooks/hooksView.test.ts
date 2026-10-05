import { describe, expect, it } from 'vitest';
import { hookAccounts, keyLabel, agentOfKey, boundAgents, bindingToDraft, checkBinding, draftToBinding, eventsToRows, hookAgentOf, hookCount, hookMessage, newRow, ownerRoot, rootPlan, rowsToEvents, scopeBackups, scopeChanges, scopeEntries, scopePaths, scopePlan, scopeUnmanaged, switchMode, syncState, writes, isCodeAgent, hookLabel, hookNote } from './hooksView';

const claudeEvents = {
  PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: './check.sh', timeout: 30, statusMessage: 'checking' }] }],
};

describe('Git command binding editor', () => {
  const binding = {
    commands: { 'tool.check': { events: ['pre-commit', 'pre-push'], command: '{files}/check.sh', parallel: false } },
    files: { 'check.sh': '#!/bin/sh\nprintf "%s\\n" "$@"\n' },
  };
  it('round-trips commands and helper contents through raw YAML', () => {
    const draft = bindingToDraft('git', binding);
    expect(draft.mode).toBe('native');
    expect(draft.native).toContain('commands:');
    expect(draft.rows).toEqual([]);
    expect(checkBinding('git', draft)).toMatchObject({ nativeError: false, empty: false });
    expect(draftToBinding('git', draft)).toEqual(binding);
  });
  it('rejects malformed YAML, duplicate keys, a sequence and empty commands', () => {
    for (const native of ['commands: [', 'commands: {}\ncommands: {}', '- command', 'commands: {}']) {
      const check = checkBinding('git', { ...bindingToDraft('git', binding), native });
      expect(check.nativeError || check.empty).toBe(true);
    }
  });
  it('reports inactive ahead of synced while preserving conflict priority', () => {
    const plan = { revision: '', fingerprint: '', sourcePath: '', blocked: false, changes: [{ target: 'git', path: '/file', name: 'check', action: 'inactive' }] };
    expect(syncState(plan, 'check', 'git')).toBe('inactive');
    plan.changes.push({ target: 'git', path: '/file', name: 'check', action: 'conflict' });
    expect(syncState(plan, 'check', 'git')).toBe('conflict');
  });
  it('selects linked-worktree backups by their declared root', () => {
    const data = { source: { projects: { '/linked': {} } }, backups: [{ path: '/main/.git/config', root: '/linked' }] };
    expect(scopeBackups(data, '/linked')).toEqual(data.backups);
    expect(scopeBackups(data)).toEqual([]);
  });
});

describe('command events editor', () => {
  it('round-trips a matcher group and keeps fields it has no input for', () => {
    const rows = eventsToRows(claudeEvents)!;
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({ event: 'PreToolUse', matcher: 'Bash', command: './check.sh', timeout: '30' });
    expect(rowsToEvents(rows)).toEqual(claudeEvents);
  });

  it('keeps the timeout key of the Agent instead of translating units', () => {
    const copilot = { preToolUse: [{ type: 'command', bash: './x.sh', timeoutSec: 5 }] };
    const rows = eventsToRows(copilot)!;
    expect(rows[0]).toMatchObject({ shape: 'flat', commandKey: 'bash', timeoutKey: 'timeoutSec' });
    expect(rowsToEvents(rows)).toEqual(copilot);
  });

  it('sends edited fields back in the native shape', () => {
    const rows = eventsToRows(claudeEvents)!;
    rows[0].command = './other.sh';
    rows[0].timeout = '';
    expect(rowsToEvents(rows)).toEqual({ PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: './other.sh', statusMessage: 'checking' }] }] });
  });

  it('refuses shapes it cannot put back, so they stay in the native editor', () => {
    expect(eventsToRows({ Stop: [{ hooks: [{ type: 'command', command: 'a' }, { type: 'command', command: 'b' }] }] })).toBeNull();
    expect(eventsToRows({ Stop: [{ hooks: [{ type: 'prompt', prompt: 'x' }] }] })).toBeNull();
    expect(bindingToDraft('claude', { events: { Stop: [{ hooks: [{ type: 'prompt', prompt: 'x' }] }] } }).mode).toBe('native');
  });

  it('carries content across the two editors', () => {
    const simple = bindingToDraft('claude', { events: claudeEvents });
    const native = switchMode(simple, 'native');
    expect(JSON.parse(native.native)).toEqual(claudeEvents);
    expect(draftToBinding('claude', switchMode(native, 'simple'))).toEqual({ events: claudeEvents });
  });

  it('will not switch to the simple editor from invalid native text', () => {
    const native = { ...bindingToDraft('claude', undefined), mode: 'native' as const, native: '{' };
    expect(switchMode(native, 'simple').mode).toBe('native');
    expect(checkBinding('claude', native).nativeError).toBe(true);
  });
});

describe('binding validation', () => {
  it('needs an event and a command, and a positive timeout', () => {
    const draft = bindingToDraft('claude', undefined);
    draft.rows = [{ ...newRow('claude'), event: 'Stop', command: 'x', timeout: '0' }];
    expect(checkBinding('claude', draft).rowErrors).toBe(1);
    draft.rows[0].timeout = '10';
    expect(checkBinding('claude', draft).rowErrors).toBe(0);
  });

  it.each([
    ['a matcher', { matcher: 'Bash' }, '"matcher": "Bash"'],
    ['a timeout', { timeout: '30' }, '"timeout": 30'],
  ])('flags an extra row with only %s instead of dropping it', (_, filled, kept) => {
    const draft = bindingToDraft('claude', { events: claudeEvents });
    draft.rows = [...draft.rows, { ...newRow('claude'), ...filled }];
    expect(checkBinding('claude', draft).rowErrors).toBe(1);
    expect(switchMode(draft, 'native').native).toContain(kept);
  });

  it('ignores an extra row left completely empty', () => {
    const draft = bindingToDraft('claude', { events: claudeEvents });
    draft.rows = [...draft.rows, newRow('claude')];
    expect(checkBinding('claude', draft).rowErrors).toBe(0);
    expect(draftToBinding('claude', draft)).toEqual({ events: claudeEvents });
  });

  it('rejects a script file name that could leave its folder', () => {
    const draft = bindingToDraft('claude', { events: claudeEvents, files: { '../x.sh': '' } });
    expect(checkBinding('claude', draft).fileErrors).toBe(true);
  });

  it('keeps code Agents on their native code, unchanged', () => {
    const code = 'export default function plugin() {\n  return {};\n}\n';
    const draft = bindingToDraft('opencode', { code });
    expect(draftToBinding('opencode', draft)).toEqual({ code });
    expect(checkBinding('opencode', bindingToDraft('opencode', undefined)).codeMissing).toBe(true);
  });
});

describe('syncState', () => {
  const plan = { revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [
    { target: 'claude', path: '/a', name: 'a', action: 'unchanged' },
    { target: 'codex', path: '/b', name: 'a', action: 'add' },
    { target: 'gemini', path: '/c', name: 'a', action: 'conflict' },
  ] };
  it('reports each Agent on its own', () => {
    expect(syncState(plan, 'a', 'claude')).toBe('synced');
    expect(syncState(plan, 'a', 'codex')).toBe('pending');
    expect(syncState(plan, 'a', 'gemini')).toBe('conflict');
    expect(syncState(plan, 'a', 'qwen')).toBe('none');
  });
});

describe('project isolation', () => {
  const on = { bindings: { codex: {} } };
  const data = {
    source: { entries: { g: on, off: { enabled: false, bindings: { codex: {} } } }, projects: { '/work/app': { entries: { p: on } } } },
    paths: { codex: '/home/me/.codex/hooks.json' },
    projectPaths: { '/work/app': { codex: '/work/app/.codex/hooks.json' } },
    plan: { revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [
      { target: 'codex', path: '/home/me/.codex/hooks.json', name: 'g', action: 'add' },
      { target: 'codex', path: '/work/app/.codex/hooks.json', name: 'p', root: '/work/app', action: 'conflict' },
    ] },
  };

  it('keeps global and project hooks, files and changes apart', () => {
    expect(Object.keys(scopeEntries(data))).toEqual(['g', 'off']);
    expect(Object.keys(scopeEntries(data, '/work/app'))).toEqual(['p']);
    expect(scopeChanges(data).map((c) => c.name)).toEqual(['g']);
    expect(scopeChanges(data, '/work/app').map((c) => c.name)).toEqual(['p']);
    expect(scopePaths(data).codex).toBe('/home/me/.codex/hooks.json');
    expect(scopePaths(data, '/work/app').codex).toBe('/work/app/.codex/hooks.json');
    expect(scopePaths(data, '/work/unknown')).toEqual({});
  });

  it('blocks a project only for its own conflicts and skips disabled hooks in the count', () => {
    expect(scopePlan(data)?.blocked).toBe(false);
    expect(scopePlan(data, '/work/app')?.blocked).toBe(true);
    expect([hookCount(data, 'codex'), hookCount(data, 'codex', '/work/app')]).toEqual([1, 1]);
  });

  it('assigns unmanaged hooks and backups, which carry no project, to the deepest root above their path', () => {
    const rooted = {
      ...data,
      source: { ...data.source, projects: { ...data.source.projects, '/work/app/sub': { entries: {} } } },
      unmanaged: [{ target: 'codex', path: '/home/me/.codex/hooks.json', names: ['a'] }, { target: 'codex', path: '/work/app/.codex/hooks.json', names: ['b'] }],
      backups: [{ id: '1', target: 'codex', path: '/work/app/sub/.codex/hooks.json' }, { id: '2', target: 'codex', path: '/home/me/.codex/hooks.json' }],
    };
    expect(ownerRoot(rooted, '/work/app/sub/.codex/hooks.json')).toBe('/work/app/sub');
    expect(ownerRoot(rooted, '/work/application/x')).toBeUndefined();
    expect(scopeUnmanaged(rooted).map((u) => u.names[0])).toEqual(['a']);
    expect(scopeUnmanaged(rooted, '/work/app').map((u) => u.names[0])).toEqual(['b']);
    expect(scopeBackups(rooted, '/work/app/sub').map((b) => b.id)).toEqual(['1']);
    expect(scopeBackups(rooted).map((b) => b.id)).toEqual(['2']);
  });

  it('narrows a plan to one root without changing its revision, and leaves the global plan alone', () => {
    const narrowed = rootPlan({ ...data.plan, blocked: true }, '/work/other');
    expect(narrowed.changes).toEqual([]);
    expect(narrowed.blocked).toBe(false);
    expect(narrowed.revision).toBe('r');
    expect(rootPlan(data.plan, '/work/app').blocked).toBe(true);
    expect(rootPlan(data.plan)).toBe(data.plan);
  });

  it('reads the Agent behind a project target and ignores tools without hooks', () => {
    expect([hookAgentOf('app@codex'), hookAgentOf('codex'), hookAgentOf('windsurf')]).toEqual(['codex', 'codex', undefined]);
    expect(hookAgentOf('omp-work', 'omp')).toBe('omp-work');
    expect(hookAgentOf('pi-work', 'pi')).toBe('pi-work');
    expect(hookAgentOf('unsupported', 'windsurf')).toBeUndefined();
  });
});

describe('hookMessage', () => {
  it('words the known domain notes with UI keys and passes any other note through', () => {
    const t = (key: string) => `[${key}]`;
    expect(hookMessage(t, 'saving the import takes over the existing registrations in place; sync leaves them as they are instead of adding duplicates')).toBe('[hooks.message.importUnmanaged]');
    expect(hookMessage(t, 'an identical hook exists that Skillshare does not manage; import it or explicitly replace it')).toBe('[hooks.message.identicalUnmanaged]');
    expect(hookMessage(t, 'hook guard: gemini does not document the event "Stopp"; check its spelling')).toBe('[hooks.message.unknownEvent]');
    expect(hookMessage(t, 'something new')).toBe('something new');
  });
});

describe('Oh My Pi hooks', () => {
  it('is a code Agent with its own label and native note', () => {
    expect(isCodeAgent('omp')).toBe(true);
    expect(hookLabel('omp')).toBe('Oh My Pi');
    expect(hookNote((k) => k, 'omp', 'server note')).toBe('hooks.native.omp');
  });
});

describe('account target keys', () => {
  it('resolves labels and places each account after its Agent', () => {
    const accounts = { 'codex-2': 'codex' };
    expect(keyLabel(accounts, 'codex-2')).toBe('codex-2 (Codex)');
    expect(agentOfKey(accounts, 'codex-2')).toBe('codex');
    expect(agentOfKey(accounts, 'claude')).toBe('claude');
    expect(hookAccounts([{ name: 'codex-2', agent: 'codex', kind: 'command' }, { name: 'codex', kind: 'command' }])).toEqual(accounts);
    expect(boundAgents({ bindings: { pi: { code: 'x' }, 'codex-2': { events: {} }, codex: { events: {} } } }, accounts)).toEqual(['codex', 'codex-2', 'pi']);
  });
});


describe('inactive Git sync decisions', () => {
  it('never counts a skipped inactive output as a pending write', () => {
    expect(writes({ action: 'inactive' })).toBe(false);
  });
  it('keeps additions pending and synced outputs inactive without pending writes', () => {
    const change = { target: 'git', path: '/file', name: 'check', action: 'add', inactiveReason: 'Git 2.39 cannot run config hooks' };
    const plan = { revision: '', fingerprint: '', sourcePath: '', blocked: false, changes: [change] };
    expect(writes(change)).toBe(true);
    expect(syncState(plan, 'check', 'git')).toBe('pending');
    change.action = 'unchanged';
    expect(writes(change)).toBe(false);
    expect(syncState(plan, 'check', 'git')).toBe('inactive');
  });
});
