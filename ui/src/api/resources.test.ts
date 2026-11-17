import { afterEach, describe, expect, it, vi } from 'vitest';
import { resourcesApi } from './resources';

describe('source links API', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('posts the source-link request and returns its result', async () => {
    const result = { path: '/source/_team', target: '/work/team', kind: 'symlink', warning: '' };
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(result)));
    vi.stubGlobal('fetch', fetch);
    const body = { path: '/work/team', name: '_team', enable: true };
    expect(await resourcesApi.createSourceLink(body)).toEqual(result);
    expect(fetch).toHaveBeenCalledWith('/api/source-links', expect.objectContaining({ method: 'POST', body: JSON.stringify(body) }));
  });

  it('preserves the HTTP 400 guard message', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'target is missing' }), { status: 400 })));
    await expect(resourcesApi.createSourceLink({ path: '/missing' })).rejects.toMatchObject({ status: 400, message: 'target is missing' });
  });

  it('deletes the encoded first-level link name', async () => {
    const result = { success: true, name: '_my team' };
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(result)));
    vi.stubGlobal('fetch', fetch);
    expect(await resourcesApi.removeSourceLink('_my team')).toEqual(result);
    expect(fetch).toHaveBeenCalledWith('/api/source-links/_my%20team', expect.objectContaining({ method: 'DELETE' }));
  });
});


describe('repo removal intent', () => {
  afterEach(() => vi.unstubAllGlobals());

  it.each<[boolean, boolean, string]>([
    [false, false, ''], [true, false, '?force=true'],
    [false, true, '?recordOnly=true'], [true, true, '?force=true&recordOnly=true'],
  ])('encodes force=%s and recordOnly=%s', async (force, recordOnly, query) => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ success: true })));
    vi.stubGlobal('fetch', fetch);
    await resourcesApi.deleteRepo('org/_team', force, recordOnly);
    expect(fetch).toHaveBeenCalledWith(`/api/repos/org%2F_team${query}`, expect.objectContaining({ method: 'DELETE' }));
  });
});
