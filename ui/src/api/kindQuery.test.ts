import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetch } from './http';
import { auditApi } from './audit';
import { resourcesApi } from './resources';

vi.mock('./http', async (load) => ({
  ...(await load<typeof import('./http')>()),
  apiFetch: vi.fn(),
}));

// The kind unions are closed today; a cast stands in for a value that needs escaping.
const odd = 'a&b' as 'skill';

describe('?kind= query', () => {
  beforeEach(() => vi.mocked(apiFetch).mockReset());

  it('is encoded on resource requests', async () => {
    await resourcesApi.getResource('demo', odd);

    expect(vi.mocked(apiFetch).mock.calls[0][0]).toBe('/resources/demo?kind=a%26b');
  });

  it('is encoded on audit requests', async () => {
    await auditApi.auditAll(odd as unknown as 'skills');

    expect(vi.mocked(apiFetch).mock.calls[0][0]).toBe('/audit?kind=a%26b');
  });

  it('is left off without a kind', async () => {
    await resourcesApi.listSkills();
    await auditApi.auditAll();

    expect(vi.mocked(apiFetch).mock.calls.map(([path]) => path)).toEqual(['/resources', '/audit']);
  });
});
