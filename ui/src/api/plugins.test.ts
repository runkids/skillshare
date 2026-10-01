import { describe, expect, it, vi } from 'vitest';
import { apiFetch } from './client';
import { pluginShareCommand, pluginsApi } from './plugins';

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
