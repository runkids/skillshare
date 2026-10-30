import { describe, expect, it, vi } from 'vitest';
import { apiFetch } from './client';
import { type PluginBinding, bindingStatus, pluginShareCommand, pluginsApi, syncAction } from './plugins';

vi.mock('./client', () => ({ apiFetch: vi.fn() }));

describe('pluginsApi.list', () => {
  it('reads a null installed list as empty', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      packages: {},
      hosts: [{ target: 'opencode', version: '1.18.32', status: 'blocked', error: 'both opencode.json and opencode.jsonc exist', installed: null }],
    });
    const inv = await pluginsApi.list();
    expect(inv.hosts[0].installed).toEqual([]);
  });
});

describe('pluginShareCommand', () => {
  it('adds the plugin from its source with what picked it there', () => {
    expect(pluginShareCommand('mine', { source: 'https://github.com/owner/market.git', plugin: 'demo', sourceRef: 'v1.2' }))
      .toBe('skillshare plugin add https://github.com/owner/market.git --plugin demo --name mine --source-ref v1.2 -g');
  });
  it('quotes values the shell would split', () => {
    expect(pluginShareCommand('demo', { source: 'https://example.com/a b.git' })).toBe("skillshare plugin add 'https://example.com/a b.git' -g");
  });
  it('offers nothing for a local directory', () => {
    expect(pluginShareCommand('demo', { source: '/Users/me/plugins/demo' })).toBe('');
  });
});

describe('syncAction', () => {
  const host = (managedMarketplaces?: string[]) => ({ target: 'claude' as const, version: '1', status: 'ready', installed: [], managedMarketplaces });
  const excluded = { id: 'demo@skillshare-demo-0123', source: 'https://example.com/demo', sync: false };
  it('still has work for an excluded plugin whose Skillshare marketplace is left', () => {
    expect(syncAction(excluded, host(['skillshare-demo-0123']))).toBe('uninstall');
  });
  it('leaves an excluded plugin alone when its marketplace is gone or not Skillshare\'s', () => {
    expect(syncAction(excluded, host([]))).toBe('');
  });
  it('never claims an imported plugin\'s marketplace', () => {
    expect(syncAction({ id: 'demo@team', sync: false }, host(['team']))).toBe('');
  });
});


describe('host-local bindings', () => {
  const host = { target: 'omp', version: '1', status: 'ready', installed: [] };
  it('keeps an unbound import visible without proposing an install', () => {
    const binding = { id: '', imported: true };
    expect(syncAction(binding, host)).toBe('');
    expect(bindingStatus(binding, host)).toBe('plugins.importRequired');
    expect(bindingStatus(binding)).toBe('plugins.importRequired');
  });
  it('shows cleanup even if the shared target was removed', () => {
    const binding = { id: 'demo@old', cleanup: true };
    expect(syncAction(binding, host)).toBe('remove');
    expect(bindingStatus(binding, host)).toBe('plugins.cleanup');
  });
  it('does not propose commands through a changed native account', () => {
    const binding = { id: 'demo@old', cleanup: true, contextChanged: true };
    expect(syncAction(binding, host)).toBe('');
    expect(bindingStatus(binding, host)).toBe('plugins.accountChanged');
  });
  it('shows a retained local import when the shared recipe has changed', () => {
    const binding = { id: 'demo@local', imported: true, importRetained: true };
    const installed = { ...host, installed: [{ id: binding.id, enabled: true }] };
    expect(bindingStatus(binding)).toBe('plugins.importRetained');
    expect(bindingStatus(binding, installed)).toBe('plugins.importRetained');
    expect(bindingStatus({ ...binding, importRetained: false }, installed)).toBe('');
    expect(bindingStatus({ ...binding, cleanup: true }, installed)).toBe('plugins.cleanup');
  });
});


describe('shared ref reconciliation', () => {
  const binding: PluginBinding = { id: '/local/old', source: 'owner/repo', refChanged: true };
  const ready = { target: 'pi', version: '1', status: 'ready', installed: [{ id: binding.id, enabled: true }] };
  it.each([
    [{}, ready, 'update'], [{}, { ...ready, installed: [] }, 'install'],
    [{ sync: false }, ready, 'uninstall'], [{ sync: false }, { ...ready, installed: [] }, ''],
    [{ cleanup: true }, ready, 'remove'], [{ contextChanged: true }, ready, ''],
    [{ imported: true, id: '' }, ready, ''], [{ pending: 'remove' }, ready, 'update'],
    [{}, undefined, ''], [{}, { ...ready, status: 'missing', installed: [] }, 'install'],
    [{}, { ...ready, error: 'unavailable' }, ''],
  ] as [Partial<PluginBinding>, typeof ready | undefined, string][])('keeps reconciliation precedence: %o / %o', (changes, host, action) => {
    expect(syncAction({ ...binding, ...changes }, host)).toBe(action);
  });
});
