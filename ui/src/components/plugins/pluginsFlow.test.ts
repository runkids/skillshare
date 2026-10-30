import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { pluginsApi } from '../../api/plugins';
import { ApiError } from '../../api/client';
import type { PluginInventory, PluginPlan, PluginUpdateEvent } from '../../api/plugins';
import { changedRuns, failureText, importPackageNames, importRequest, piPackageTest, reviewChanges, syncTodo, updatesFound, updatedTargets, updatesLeft, updatesPending, usePluginsFlow, versionMoves } from './pluginsFlow';

vi.mock('../../api/plugins', async (importOriginal) => ({ ...await importOriginal<typeof import('../../api/plugins')>(), pluginsApi: { preview: vi.fn(), apply: vi.fn(), update: vi.fn(), stopUpdate: vi.fn() } }));

const t = (key: string) => key;
const plan = (revision: string, ...changes: Partial<PluginPlan['changes'][number]>[]): PluginPlan =>
  ({ revision, blocked: false, changes: changes.map((c) => ({ name: 'demo', target: 'codex', id: 'demo@market', action: 'selection', ...c })) });
const ok = { result: { results: [] }, failure: '' };
/** The server's stream for a batch: each plugin starts, then ends with the given result. */
const streams = (...ends: Omit<Extract<PluginUpdateEvent, { event: 'result' }>, 'event'>[]) =>
  vi.mocked(pluginsApi.update).mockImplementation(async (_items, on) => {
    for (const end of ends) { on({ event: 'start', name: end.name }); on({ event: 'result', ...end }); }
    on({ event: 'done', failure: '' });
  });

function flow() {
  const refresh = vi.fn();
  const onPreviewed = vi.fn();
  const hook = renderHook(() => usePluginsFlow({ t, refresh, onPreviewed }));
  return { hook, refresh, onPreviewed, now: () => hook.result.current };
}

describe('plugins flow', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(pluginsApi.preview).mockResolvedValue(plan('reviewed'));
    vi.mocked(pluginsApi.apply).mockResolvedValue(ok);
  });

  it('writes nothing on preview, and applies the reviewed request with the revision the preview returned', async () => {
    const { now, refresh, onPreviewed } = flow();
    await act(() => now().preview({ action: 'sync' }, 'sync'));
    expect(now().review?.plan.revision).toBe('reviewed');
    expect(onPreviewed).toHaveBeenCalledTimes(1);
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    expect(refresh).not.toHaveBeenCalled();

    await act(() => now().apply());
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'sync' }, 'reviewed');
    expect(now().review).toBeNull();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('does not apply without a review', async () => {
    const { now } = flow();
    await act(() => now().apply());
    expect(pluginsApi.apply).not.toHaveBeenCalled();
  });

  it('names the control that started a preview while it runs, and is idle after', async () => {
    let finish!: (p: PluginPlan) => void;
    vi.mocked(pluginsApi.preview).mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const { now } = flow();
    act(() => now().begin({ action: 'check' }, 'check'));
    expect([now().busy, now().working]).toEqual([true, 'check']);
    await act(async () => finish(plan('r')));
    expect([now().busy, now().working]).toEqual([false, '']);
  });

  it('keeps a failed preview as the failure, opens no review and rethrows for the dialog that asked', async () => {
    vi.mocked(pluginsApi.preview).mockRejectedValue(new Error('source not found'));
    const { now, onPreviewed } = flow();
    await act(async () => { await expect(now().preview({ action: 'add' })).rejects.toThrow('source not found'); });
    expect(now().failure).toBe('source not found');
    expect(now().review).toBeNull();
    expect(onPreviewed).not.toHaveBeenCalled();
    expect(now().busy).toBe(false);
  });

  it('localizes a keyed import preview failure without accepting the import', async () => {
    const error = new ApiError(400, 'Native plugin does not match the declaration recipe', { code: 'plugins.error.importRecipeMismatch' });
    vi.mocked(pluginsApi.preview).mockRejectedValueOnce(error);
    const { now } = flow();
    await act(async () => { await expect(now().preview({ action: 'import', from: 'pi', plugin: 'npm:wrong' })).rejects.toBe(error); });
    expect(now().failure).toBe('plugins.error.importRecipeMismatch');
    expect(now().review).toBeNull();
    expect(pluginsApi.apply).not.toHaveBeenCalled();
  });

  it('localizes a batch import preview failure and still applies the next reviewed import', async () => {
    vi.mocked(pluginsApi.preview).mockRejectedValueOnce(new ApiError(400, 'Remote recipe has no plugin identity', { code: 'plugins.error.importRecipeIdentityRequired' }));
    const { now } = flow();
    await act(() => now().importMany([
      { action: 'import', from: 'pi', plugin: 'npm:unknown' },
      { action: 'import', from: 'pi', plugin: 'npm:known' },
    ]));
    expect(now().failure).toBe('npm:unknown: plugins.error.importRecipeIdentityRequired');
    expect(pluginsApi.apply).toHaveBeenCalledExactlyOnceWith({ action: 'import', from: 'pi', plugin: 'npm:known' }, 'reviewed');
  });

  it('keeps the review open and refreshes when apply is refused', async () => {
    vi.mocked(pluginsApi.apply).mockRejectedValue(new Error('preview is stale'));
    const { now, refresh } = flow();
    await act(() => now().preview({ action: 'sync' }));
    await act(() => now().apply());
    expect(now().failure).toBe('preview is stale');
    expect(now().review).not.toBeNull();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('shows the outcomes of an apply and its failure, and clears both with the next preview', async () => {
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'Native authentication required' }] }, failure: 'One target failed' });
    const { now } = flow();
    await act(() => now().preview({ action: 'sync' }));
    await act(() => now().apply());
    expect(now().failure).toBe('Native authentication required');
    expect(now().outcomes).toHaveLength(1);
    await act(() => now().preview({ action: 'sync' }));
    expect([now().failure, now().outcomes]).toEqual(['', []]);
  });

  it('remembers what a check found, opens no review, and takes an Agent off once its batch update is applied', async () => {
    const observed = { id: '/owned/driver', source: 'https://fixture.invalid/driver.git', sourceRef: 'main', plugin: 'driver', entry: 'entry.js', version: '1.1.0', commit: 'abc', digest: 'reviewed-digest' };
    const found = { name: 'driver', target: 'pi', binding: observed };
    vi.mocked(pluginsApi.preview).mockResolvedValueOnce(plan('r', { ...found, action: 'update-available' }));
    streams({ name: 'driver', plan: plan('r2', { ...found, action: 'update' }), result: { results: [{ name: 'driver', target: 'pi', status: 'installed' }] }, failure: '', stale: false });
    const { now, onPreviewed } = flow();
    await act(() => now().check());
    expect(now().updates).toEqual({ driver: { pi: observed } });
    expect(now().review).toBeNull();
    expect(onPreviewed).not.toHaveBeenCalled();
    await act(() => now().updateMany([{ name: 'driver', targets: ['pi'] }]));
    expect(vi.mocked(pluginsApi.update).mock.calls[0][0]).toEqual([{ name: 'driver', targets: ['pi'], seen: { pi: observed } }]);
    expect(now().updates).toEqual({ driver: {} });
    expect(now().batch).toEqual([{ name: 'driver', targets: ['pi'], status: 'done', versions: { moves: [{ target: 'pi', from: undefined, to: '1.1.0', fromCommit: undefined, toCommit: 'abc' }], shared: true } }]);
  });

  it('lists what is still to update only where the Agent can be updated from here, and drops an Agent an update took off', () => {
    const inventory = { packages: { tail: { bindings: { pi: { id: 'npm:tail' }, omp: { id: 'tail' } } }, guard: { bindings: { omp: { id: 'guard' } } }, driver: { bindings: {} } }, hosts: [], targetDefinitions: [{ target: 'pi', label: 'Pi', project: false, operations: ['check', 'update'] }, { target: 'omp', label: 'OMP', project: false, operations: ['check'] }] };
    const updates = { tail: { pi: { version: '2.1.0' }, omp: { version: '2.1.0' } }, guard: { omp: { version: '1.0.0' } }, driver: {} };
    expect(updatesPending(updates, inventory)).toEqual([{ name: 'tail', targets: ['pi'] }]);
  });

  it('puts a retried plugin back to waiting where it is, and keeps going after a plugin fails in a batch', async () => {
    const u = plan('u', { action: 'update' });
    streams(
      { name: 'demo', plan: u, result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'timeout' }] }, failure: 'demo: timeout', stale: false },
      { name: 'other', plan: u, ...ok, stale: false },
    );
    const { now, refresh } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }, { name: 'other', targets: ['codex'] }]));
    expect(now().batch).toEqual([{ name: 'demo', targets: ['codex'], status: 'failed', note: 'timeout', failed: ['codex'] }, { name: 'other', targets: ['codex'], status: 'done', note: undefined }]);
    expect(refresh).toHaveBeenCalledTimes(1);
    streams({ name: 'demo', plan: u, ...ok, stale: false });
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }]));
    expect(now().batch).toEqual([{ name: 'demo', targets: ['codex'], status: 'done', note: undefined, failed: undefined }, { name: 'other', targets: ['codex'], status: 'done', note: undefined }]);
  });

  it('counts the Agents that updated in a plugin where another was skipped', () => {
    expect(updatedTargets({ name: 'tail', targets: ['pi', 'codex'], status: 'skipped', failed: ['codex'] })).toEqual(['pi']);
  });

  it('shares one version move only when every Agent makes the same one', () => {
    const inventory = { targetDefinitions: [], hosts: [], packages: { tail: { bindings: { pi: { id: 'a', version: '1.0.0' }, codex: { id: 'b', version: '1.0.0' }, claude: { id: 'c', version: '0.9.0' } } } } };
    expect(versionMoves(inventory, 'tail', [{ target: 'pi', version: '1.1.0' }, { target: 'codex', version: '1.1.0' }]).shared).toBe(true);
    expect(versionMoves(inventory, 'tail', [{ target: 'pi', version: '1.1.0' }, { target: 'claude', version: '1.1.0' }]).moves.map((m) => m.from)).toEqual(['1.0.0', '0.9.0']);
    expect(versionMoves(inventory, 'tail', [{ target: 'pi', version: '1.1.0' }, { target: 'claude', version: '1.1.0' }]).shared).toBe(false);
  });

  it('tells whether a check came back, so a check the user asked for can open its result', async () => {
    const { now } = flow();
    await act(async () => expect(await now().check()).toBe(true));
    vi.mocked(pluginsApi.preview).mockRejectedValueOnce(new Error('offline'));
    await act(async () => expect(await now().check()).toBe(false));
  });

  it('drops a pending update for an Agent unticked since the check', () => {
    const inventory = { hosts: [], targetDefinitions: [{ target: 'pi', label: 'Pi', project: false, operations: ['update'] }, { target: 'codex', label: 'Codex', project: false, operations: ['update'] }], packages: { tail: { bindings: { pi: { id: 'npm:tail' }, codex: { id: 'tail@m', sync: false } } } } };
    expect(updatesPending({ tail: { pi: { version: '2' }, codex: { version: '2' } } }, inventory)).toEqual([{ name: 'tail', targets: ['pi'] }]);
  });

  it('drops a pending update whose plugin or Agent binding is gone', () => {
    const inventory = { packages: { kept: { bindings: { pi: { id: 'npm:kept' } } } }, hosts: [], targetDefinitions: [{ target: 'pi', label: 'Pi', project: false, operations: ['update'] }, { target: 'codex', label: 'Codex', project: false, operations: ['update'] }] };
    const updates = { kept: { pi: { version: '2' }, codex: { version: '2' } }, removed: { pi: { version: '2' } } };
    expect(updatesPending(updates, inventory)).toEqual([{ name: 'kept', targets: ['pi'] }]);
  });

  it('keeps a plugin whose update came back skipped on the list, as not updated', async () => {
    const found = { name: 'driver', target: 'pi', binding: { id: 'npm:driver', version: '1.1.0' } };
    vi.mocked(pluginsApi.preview).mockResolvedValueOnce(plan('r', { ...found, action: 'update-available' }));
    streams({ name: 'driver', plan: plan('u', { ...found, action: 'update' }), result: { results: [{ name: 'driver', target: 'pi', status: 'skipped', message: 'marketplace gone' }] }, failure: '', stale: false });
    const { now } = flow();
    await act(() => now().check());
    await act(() => now().updateMany([{ name: 'driver', targets: ['pi'] }]));
    expect(now().updates.driver.pi).toBeDefined();
    expect(now().batch?.[0]).toMatchObject({ status: 'skipped', note: 'marketplace gone' });
  });

  it('keeps retained-import guidance in the completed batch', async () => {
    const note = { name: 'demo', target: 'codex', guidance: true, messageKey: 'plugins.note.importRecipeChanged' };
    streams({ name: 'demo', plan: plan('u', { ...note, action: 'noop' }), result: { results: [{ ...note, status: 'unchanged' }] }, failure: '', stale: false });
    const { now } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }]));
    expect(now().batch?.[0]).toMatchObject({ status: 'done', note: 'plugins.note.importRecipeChanged' });
  });


  it('drops a plugin whose source moved since the check and checks again, showing why that check failed', async () => {
    vi.mocked(pluginsApi.preview)
      .mockResolvedValueOnce(plan('r', { name: 'driver', target: 'pi', action: 'update-available', binding: { id: 'npm:driver', version: '1.1.0' } }))
      .mockRejectedValueOnce(new Error('offline'));
    streams({ name: 'driver', plan: plan('u'), result: null, failure: 'source changed since the check; check again', stale: true });
    const { now } = flow();
    await act(() => now().check());
    await act(() => now().updateMany([{ name: 'driver', targets: ['pi'] }]));
    expect(now().batch?.[0]).toMatchObject({ status: 'skipped', stale: true });
    expect([now().updates.driver, now().failure]).toEqual([undefined, 'offline']);
  });

  it('fails a plugin a broken stream never finished, with why and its Agents to retry', async () => {
    vi.mocked(pluginsApi.update).mockImplementation(async (_items, on) => { on({ event: 'start', name: 'demo' }); throw new Error('stream closed'); });
    const { now } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }, { name: 'other', targets: ['codex'] }]));
    expect(now().batch?.map(({ status, note, failed }) => ({ status, note, failed }))).toEqual([{ status: 'failed', note: 'stream closed', failed: ['codex'] }, { status: 'failed', note: 'stream closed', failed: ['codex'] }]);
  });

  it('skips a plugin a stopped batch never started', async () => {
    streams({ name: 'demo', plan: plan('u', { action: 'update' }), ...ok, stale: false });
    const { now } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }, { name: 'other', targets: ['codex'] }]));
    expect(now().batch?.map((i) => i.status)).toEqual(['done', 'skipped']);
  });

  it('keeps the failed Agents when a retry of them fails again, so the next retry still sends only those', async () => {
    const u = plan('u', { action: 'update' }, { action: 'update', target: 'pi' });
    const codexFails = { result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'timeout' }] }, failure: 'demo: timeout' };
    streams({ name: 'demo', plan: u, result: { results: [...codexFails.result.results, { name: 'demo', target: 'pi', status: 'installed' }] }, failure: 'demo: timeout', stale: false });
    const { now } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex', 'pi'] }]));
    streams({ name: 'demo', plan: u, ...codexFails, stale: false });
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }]));
    expect(now().batch?.[0]).toMatchObject({ status: 'failed', targets: ['codex', 'pi'], failed: ['codex'] });
  });

  it('names the Agents that failed when the others updated, so a retry sends only those', async () => {
    streams({ name: 'demo', plan: plan('u', { action: 'update' }, { action: 'update', target: 'pi' }), result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'timeout' }, { name: 'demo', target: 'pi', status: 'installed' }] }, failure: 'demo: timeout', stale: false });
    const { now } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex', 'pi'] }]));
    expect(now().batch?.[0]).toMatchObject({ status: 'failed', failed: ['codex'] });
  });

  it('previews and applies a tick in one step, with that preview\'s revision and no review', async () => {
    const { now, refresh } = flow();
    await act(() => now().selectTarget('demo', 'codex', false));
    expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'disable', name: 'demo', targets: ['codex'] });
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'disable', name: 'demo', targets: ['codex'] }, 'reviewed');
    expect(now().review).toBeNull();
    expect(refresh).toHaveBeenCalledTimes(1);
    await act(() => now().selectTarget('demo', 'codex', true));
    expect(pluginsApi.preview).toHaveBeenLastCalledWith({ action: 'enable', name: 'demo', targets: ['codex'] });
  });

  it('does not apply a tick whose preview failed', async () => {
    vi.mocked(pluginsApi.preview).mockRejectedValue(new Error('blocked'));
    const { now, refresh } = flow();
    await act(() => now().selectTarget('demo', 'codex', true));
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    expect(now().failure).toBe('blocked');
    expect(refresh).not.toHaveBeenCalled();
  });
});

describe('plugins flow rules', () => {
  const keyed = { status: 'failed', message: 'claude command failed', messageKey: 'plugins.error.commandFailed' };

  it('translates a keyed failure once, however many targets share it', () => {
    expect(failureText(t, { result: { results: [{ name: 'a', target: 'claude', ...keyed }, { name: 'b', target: 'claude', ...keyed }] }, failure: 'claude command failed\nclaude command failed' })).toBe('plugins.error.commandFailed');
  });
  it('keeps every distinct failure, keyed or not', () => {
    expect(failureText(t, { result: { results: [{ name: 'a', target: 'claude', ...keyed }, { name: 'b', target: 'codex', status: 'failed', message: 'Native authentication required' }] }, failure: 'joined' })).toBe('plugins.error.commandFailed Native authentication required');
  });
  it('shows the server\'s failure as sent when no outcome failed, and none when it reports none', () => {
    expect(failureText(t, { result: null, failure: 'preview is stale' })).toBe('preview is stale');
    expect(failureText(t, { result: { results: [{ name: 'a', target: 'claude', ...keyed }] }, failure: '' })).toBe('');
  });

  it('finds an update without a new version as version \'\'', () => {
    expect(updatesFound(plan('r', { action: 'update-available' }, { name: 'other', action: 'noop' }))).toEqual({ demo: { codex: { version: '', commit: undefined } } });
  });

  const pending = { demo: { codex: { version: '1.1.0' }, claude: { version: '1.1.0' } } };
  const updating = plan('r', { action: 'update' }, { target: 'claude', action: 'update' });
  it('takes off an Agent whose final operation succeeded and keeps failed or unfinished updates', () => {
    for (const action of ['noop', 'update', 'install']) {
      const status = action === 'noop' ? 'unchanged' : 'installed';
      const reviewed = plan('r', { action }, { target: 'claude', action: 'update' });
      const response = { result: { results: [{ name: 'demo', target: 'codex', status }, { name: 'demo', target: 'claude', status: 'failed' }] }, failure: 'native failure' };
      expect(updatesLeft(pending, reviewed, response)).toEqual({ demo: { claude: { version: '1.1.0' } } });
    }
    const replacing = plan('r', { action: 'remove' }, { action: 'install' });
    for (const tail of ['failed', 'not run', 'installed']) {
      const results = [{ name: 'demo', target: 'codex', status: 'removed' }];
      if (tail !== 'not run') results.push({ name: 'demo', target: 'codex', status: tail });
      const expected = tail === 'installed' ? { demo: { claude: { version: '1.1.0' } } } : pending;
      expect.soft(updatesLeft(pending, replacing, { result: { results }, failure: tail === 'installed' ? '' : 'native failure' }), tail).toEqual(expected);
    }
  });
  it('keeps an Agent whose update was skipped', () => {
    expect(updatesLeft(pending, updating, { result: { results: [{ name: 'demo', target: 'codex', status: 'skipped' }] }, failure: '' })).toEqual(pending);
  });
  it('keeps every Agent when the apply stopped before any ran', () => {
    expect(updatesLeft(pending, updating, { result: null, failure: 'preview is stale' })).toEqual(pending);
  });

  const inventory: PluginInventory = {
    targetDefinitions: [{ target: 'omo', label: 'omo', project: false, operations: ['add', 'sync'], npm: true }, { target: 'codex', label: 'Codex', project: false, operations: ['add', 'sync'] }],
    packages: {
      demo: { bindings: { codex: { id: 'demo@market', sync: false } } },
      driver: { bindings: { omo: { id: 'npm:@scope/driver' } } },
      // Skillshare installs this one from its source, even though only a Pi target has it.
      powers: { source: 'owner/powers', bindings: { omo: { id: 'local:/state/powers/content', source: 'owner/powers' } } },
      unbound: { bindings: {} },
    },
    hosts: [{ target: 'codex', version: '0.154', status: 'ready', installed: [{ id: 'demo@market', version: '2.0.0', enabled: true }] }],
  };

  it('lists what Sync would do when a tick differs from what the Agent has', () => {
    expect(syncTodo(inventory)).toEqual([{ name: 'demo', target: 'codex', word: 'uninstall' }]);
    expect(syncTodo(undefined)).toEqual([]);
  });
  it('calls a package Pi installs itself, bound only to Pi targets, a Pi package', () => {
    expect(['demo', 'driver', 'powers', 'unbound', 'gone'].filter(piPackageTest(inventory))).toEqual(['driver']);
  });
  it('splits a preview into what it changes and what it leaves alone, with the version each has', () => {
    const { active, idle } = reviewChanges(plan('r', { action: 'update' }, { action: 'noop' }, { name: 'driver', target: 'omo', action: 'noop', binding: { id: 'npm:@scope/driver', version: '3.0.0' } }), inventory);
    expect(active.map((c) => c.action)).toEqual(['update']);
    expect(idle).toEqual([{ name: 'demo', target: 'codex', version: '2.0.0' }, { name: 'driver', target: 'omo', version: '3.0.0' }]);
  });
  it('puts failed outcomes first, joins Agents that ended the same way and drops the unchanged', () => {
    const runs = changedRuns(t, [
      { name: 'kept', target: 'claude', status: 'unchanged' },
      { name: 'demo', target: 'claude', status: 'installed' },
      { name: 'demo', target: 'codex', status: 'installed' },
      { name: 'bad', target: 'codex', status: 'failed', message: 'Native authentication required' },
    ]);
    expect(runs.map((r) => [r.name, r.status, r.targets, r.note])).toEqual([['bad', 'failed', ['codex'], 'Native authentication required'], ['demo', 'installed', ['claude', 'codex'], '']]);
  });
});

it('keeps the chosen declaration name in single and batch import requests', () => {
  const inventory: PluginInventory = { hosts: [], packages: {
    bar: { bindings: { pi: { id: '', imported: true } } },
    other: { bindings: { pi: { id: '', imported: true } } },
    managed: { bindings: { pi: { id: 'npm:managed' } } },
  } };
  expect(importPackageNames(inventory, 'pi')).toEqual(['bar', 'other']);
  expect(importRequest('pi', 'npm:foo', 'bar')).toEqual({ action: 'import', from: 'pi', plugin: 'npm:foo', name: 'bar' });
  expect(importRequest('pi', 'npm:new', '')).toEqual({ action: 'import', from: 'pi', plugin: 'npm:new' });
  expect(importPackageNames(inventory, 'claude')).toEqual([]);
});

it('leaves the declaration name unset unless explicitly selected', () => {
 expect(importRequest('claude', 'a@m')).toEqual({action: 'import', from: 'claude', plugin: 'a@m'});
 expect(importRequest('claude', 'b@m')).toEqual({action: 'import', from: 'claude', plugin: 'b@m'});
});

it('keeps unchanged recipe guidance visible in preview and completed outcomes', () => {
 const note = {name: 'demo', target: 'codex', guidance: true, message: 'Shared recipe moved; plugin unimport then sync.', messageKey: 'plugins.note.importRecipeChanged'};
 const preview = reviewChanges(plan('r', {...note, action: 'noop'}));
 expect(preview.active).toHaveLength(1);
 expect(preview.idle).toHaveLength(0);
 expect(changedRuns(t, [{...note, status: 'unchanged'}])).toMatchObject([{name: 'demo', status: 'unchanged', note: 'plugins.note.importRecipeChanged'}]);
});

it.each([undefined, false])('folds an incidental noop message away without guidance: %s', (guidance) => {
 const note = {name: 'demo', target: 'omp', guidance, message: 'Removes this scope registration.'};
 const preview = reviewChanges(plan('r', {...note, action: 'noop'}));
 expect(preview.active).toEqual([]);
 expect(preview.idle).toEqual([{name: 'demo', target: 'omp'}]);
 expect(changedRuns(t, [{...note, status: 'unchanged'}])).toEqual([]);
});
