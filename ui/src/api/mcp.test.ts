import { afterEach, expect, it, vi } from 'vitest';
import { mcpApi } from './mcp';

afterEach(() => vi.unstubAllGlobals());

it('previews the mutation and saves with its revision', async () => {
  const fetch = vi
    .fn<typeof globalThis.fetch>()
    .mockResolvedValueOnce(Response.json({ revision: 'rev-1' }))
    .mockResolvedValueOnce(Response.json({ applied: [] }));
  vi.stubGlobal('fetch', fetch);
  const mutation = { name: 'docs', server: { url: 'https://docs.example/mcp' } };
  await mcpApi.save(mutation);
  expect(
    fetch.mock.calls.map(([url, init]) => [
      url,
      init?.method,
      typeof init?.body === 'string' ? JSON.parse(init.body) : init?.body,
    ]),
  ).toEqual([
    ['/api/mcp/preview', 'POST', { mutation }],
    ['/api/mcp', 'POST', { mutation, revision: 'rev-1', sync: false }],
  ]);
});
