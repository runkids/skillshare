import { afterEach, expect, it, vi } from 'vitest';
import { extrasApi } from './extras';

afterEach(() => vi.unstubAllGlobals());

function recordRequests() {
  const requests: [string, string, unknown][] = [];
  vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
    requests.push([
      init.method ?? 'GET',
      url,
      typeof init.body === 'string' ? JSON.parse(init.body) : init.body,
    ]);
    return Response.json({ success: true });
  });
  return requests;
}

it('distinguishes a bodyless POST from syncing with empty options', async () => {
  const requests = recordRequests();
  await extrasApi.openExtensionsDir();
  await extrasApi.syncExtras();
  expect(requests).toEqual([
    ['POST', '/api/extensions/open', undefined],
    ['POST', '/api/extras/sync', {}],
  ]);
});

it('preserves omitted and cleared settings and sends the target path on DELETE', async () => {
  const requests = recordRequests();
  await extrasApi.setExtraMode('my notes', 'out', 'copy');
  await extrasApi.setExtraMode('my notes', 'out', 'copy', false, '');
  await extrasApi.removeExtraTarget('my notes', 'out');
  expect(requests).toEqual([
    ['PATCH', '/api/extras/my%20notes/mode', { target: 'out', mode: 'copy' }],
    [
      'PATCH',
      '/api/extras/my%20notes/mode',
      { target: 'out', mode: 'copy', flatten: false, extension: '' },
    ],
    ['DELETE', '/api/extras/my%20notes/targets', { path: 'out' }],
  ]);
});
