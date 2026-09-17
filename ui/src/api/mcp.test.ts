import { describe, expect, it, vi } from 'vitest';
import { apiFetch } from './client';
import { mcpApi } from './mcp';

vi.mock('./client', () => ({ apiFetch: vi.fn() }));

describe('mcpApi.save', () => {
  it('previews the mutation and saves with its revision', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({ revision: 'rev-1' }).mockResolvedValueOnce({ applied: [] });
    const mutation = { name: 'docs', server: { url: 'https://docs.example/mcp' } };
    await mcpApi.save(mutation);
    expect(vi.mocked(apiFetch).mock.calls.map(([path, init]) => [path, JSON.parse(String(init?.body))])).toEqual([
      ['/mcp/preview', { mutation }],
      ['/mcp', { mutation, revision: 'rev-1', sync: false }],
    ]);
  });
});
