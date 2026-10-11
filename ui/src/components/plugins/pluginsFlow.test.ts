import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { pluginsApi, type PluginInventory, type PluginPlan } from '../../api/plugins';
import { changedRuns, failureText, piPackageTest, reviewChanges, syncTodo, updatesFound, updatesLeft, updatesPending, usePluginsFlow } from './pluginsFlow';

vi.mock('../../api/plugins', async (importOriginal) => ({ ...await importOriginal<typeof import('../../api/plugins')>(), pluginsApi: { preview: vi.fn(), apply: vi.fn() } }));

const t = (key: string) => key;
const plan = (revision: string, ...changes: Partial<PluginPlan['changes'][number]>[]): PluginPlan =>
  ({ revision, blocked: false, changes: changes.map((c) => ({ name: 'demo', target: 'codex', id: 'demo@market', action: 'selection', ...c })) });
const ok = { result: { results: [] }, failure: '' };

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
    const found = { name: 'driver', target: 'pi', binding: { id: 'npm:driver', version: '1.1.0', commit: 'abc' } };
    vi.mocked(pluginsApi.preview)
      .mockResolvedValueOnce(plan('r', { ...found, action: 'update-available' }))
      .mockResolvedValueOnce(plan('r2', { ...found, action: 'update' }));
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'driver', target: 'pi', status: 'installed' }] }, failure: '' });
    const { now, onPreviewed } = flow();
    await act(() => now().check());
    expect(now().updates).toEqual({ driver: { pi: { version: '1.1.0', commit: 'abc' } } });
    expect(now().review).toBeNull();
    expect(onPreviewed).not.toHaveBeenCalled();
    await act(() => now().updateMany([{ name: 'driver', targets: ['pi'] }]));
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'update', name: 'driver', targets: ['pi'] }, 'r2');
    expect(now().updates).toEqual({ driver: {} });
    expect(now().batch).toEqual([{ name: 'driver', targets: ['pi'], status: 'done' }]);
  });

  it('lists what is still to update only where the Agent can be updated from here, and drops an Agent an update took off', () => {
    const inventory = { packages: { tail: { bindings: { pi: { id: 'npm:tail' }, omp: { id: 'tail' } } }, guard: { bindings: { omp: { id: 'guard' } } }, driver: { bindings: {} } }, hosts: [], targetDefinitions: [{ target: 'pi', label: 'Pi', project: false, operations: ['check', 'update'] }, { target: 'omp', label: 'OMP', project: false, operations: ['check'] }] };
    const updates = { tail: { pi: { version: '2.1.0' }, omp: { version: '2.1.0' } }, guard: { omp: { version: '1.0.0' } }, driver: {} };
    expect(updatesPending(updates, inventory)).toEqual([{ name: 'tail', targets: ['pi'] }]);
  });

  it('puts a retried plugin back to waiting where it is, and keeps going after a plugin fails in a batch', async () => {
    vi.mocked(pluginsApi.preview).mockResolvedValue(plan('u', { action: 'update' }));
    vi.mocked(pluginsApi.apply)
      .mockResolvedValueOnce({ result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'timeout' }] }, failure: 'demo: timeout' })
      .mockResolvedValueOnce(ok);
    const { now, refresh } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }, { name: 'other', targets: ['codex'] }]));
    expect(pluginsApi.apply).toHaveBeenCalledTimes(2);
    expect(now().batch).toEqual([{ name: 'demo', targets: ['codex'], status: 'failed', note: 'timeout', failed: ['codex'] }, { name: 'other', targets: ['codex'], status: 'done', note: undefined }]);
    expect(refresh).toHaveBeenCalledTimes(1);
    vi.mocked(pluginsApi.apply).mockResolvedValueOnce(ok);
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }]));
    expect(now().batch).toEqual([{ name: 'demo', targets: ['codex'], status: 'done', note: undefined, failed: undefined }, { name: 'other', targets: ['codex'], status: 'done', note: undefined }]);
  });

  it('drops a pending update whose plugin or Agent binding is gone', () => {
    const inventory = { packages: { kept: { bindings: { pi: { id: 'npm:kept' } } } }, hosts: [], targetDefinitions: [{ target: 'pi', label: 'Pi', project: false, operations: ['update'] }, { target: 'codex', label: 'Codex', project: false, operations: ['update'] }] };
    const updates = { kept: { pi: { version: '2' }, codex: { version: '2' } }, removed: { pi: { version: '2' } } };
    expect(updatesPending(updates, inventory)).toEqual([{ name: 'kept', targets: ['pi'] }]);
  });

  it('keeps a plugin whose update came back skipped on the list, as not updated', async () => {
    const found = { name: 'driver', target: 'pi', binding: { id: 'npm:driver', version: '1.1.0' } };
    vi.mocked(pluginsApi.preview).mockResolvedValueOnce(plan('r', { ...found, action: 'update-available' })).mockResolvedValueOnce(plan('u', { ...found, action: 'update' }));
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'driver', target: 'pi', status: 'skipped', message: 'marketplace gone' }] }, failure: '' });
    const { now } = flow();
    await act(() => now().check());
    await act(() => now().updateMany([{ name: 'driver', targets: ['pi'] }]));
    expect(now().updates.driver.pi).toBeDefined();
    expect(now().batch?.[0]).toMatchObject({ status: 'skipped', note: 'marketplace gone' });
  });

  it('applies nothing when the source moved past the version the check showed', async () => {
    vi.mocked(pluginsApi.preview)
      .mockResolvedValueOnce(plan('r', { name: 'driver', target: 'pi', action: 'update-available', binding: { id: 'npm:driver', version: '1.1.0' } }))
      .mockResolvedValueOnce(plan('u', { name: 'driver', target: 'pi', action: 'update', binding: { id: 'npm:driver', version: '1.2.0' } }));
    const { now } = flow();
    await act(() => now().check());
    await act(() => now().updateMany([{ name: 'driver', targets: ['pi'] }]));
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    expect(now().batch?.[0]).toMatchObject({ status: 'skipped', stale: true });
  });

  it('keeps the failed Agents when a retry of them fails again, so the next retry still sends only those', async () => {
    vi.mocked(pluginsApi.preview).mockResolvedValue(plan('u', { action: 'update' }, { action: 'update', target: 'pi' }));
    const codexFails = { result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'timeout' }] }, failure: 'demo: timeout' };
    vi.mocked(pluginsApi.apply).mockResolvedValueOnce({ result: { results: [...codexFails.result.results, { name: 'demo', target: 'pi', status: 'installed' }] }, failure: 'demo: timeout' }).mockResolvedValueOnce(codexFails);
    const { now } = flow();
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex', 'pi'] }]));
    await act(() => now().updateMany([{ name: 'demo', targets: ['codex'] }]));
    expect(now().batch?.[0]).toMatchObject({ status: 'failed', targets: ['codex', 'pi'], failed: ['codex'] });
  });

  it('names the Agents that failed when the others updated, so a retry sends only those', async () => {
    vi.mocked(pluginsApi.preview).mockResolvedValue(plan('u', { action: 'update' }, { action: 'update', target: 'pi' }));
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'timeout' }, { name: 'demo', target: 'pi', status: 'installed' }] }, failure: 'demo: timeout' });
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
  it('takes off an Agent that updated or was already there, and keeps one that failed', () => {
    const left = updatesLeft(pending, plan('r', { action: 'noop' }, { target: 'claude', action: 'update' }), { result: { results: [{ name: 'demo', target: 'codex', status: 'unchanged' }, { name: 'demo', target: 'claude', status: 'failed' }] }, failure: 'x' });
    expect(left).toEqual({ demo: { claude: { version: '1.1.0' } } });
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
