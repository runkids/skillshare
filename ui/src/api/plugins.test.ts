import { describe, expect, it, vi } from 'vitest';
import { apiFetch } from './client';
import { pluginShareCommand, pluginsApi, syncAction } from './plugins';

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
