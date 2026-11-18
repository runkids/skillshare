import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { auditApi } from './audit';
import { resourcesApi } from './resources';

const fetch = vi.fn<typeof globalThis.fetch>();

// The kind unions are closed today; a cast stands in for a value that needs escaping.
const odd = 'a&b' as 'skill';

describe('?kind= query', () => {
  beforeEach(() => {
    fetch.mockReset().mockImplementation(async () => Response.json({}));
    vi.stubGlobal('fetch', fetch);
  });
  afterEach(() => vi.unstubAllGlobals());

  it('is encoded on resource requests', async () => {
    await resourcesApi.getResource('demo', odd);

    expect(fetch.mock.calls[0][0]).toBe('/api/resources/demo?kind=a%26b');
  });

  it('is encoded on audit requests', async () => {
    await auditApi.auditAll(odd as unknown as 'skills');

    expect(fetch.mock.calls[0][0]).toBe('/api/audit?kind=a%26b');
  });

  it('is left off without a kind', async () => {
    await resourcesApi.listSkills();
    await auditApi.auditAll();

    expect(fetch.mock.calls.map(([path]) => path)).toEqual(['/api/resources', '/api/audit']);
  });
});
