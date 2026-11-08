import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { apiFetch } from './client';
import { mcpCheckApi, MCPToolsAcceptanceError } from './mcpCheck';
import { BASE } from './http';

vi.mock('./client', () => ({ apiFetch: vi.fn() }));
beforeEach(() => { vi.resetAllMocks(); vi.stubGlobal('fetch', vi.fn()); });
afterEach(() => vi.unstubAllGlobals());

it('keeps static checks static and distinguishes omitted and empty live roots', async () => {
  await mcpCheckApi.run();
  await mcpCheckApi.review();
  await mcpCheckApi.review('');
  await mcpCheckApi.review('/project a');
  expect(vi.mocked(apiFetch).mock.calls.map(([path]) => path)).toEqual([
    '/mcp/check', '/mcp/check?live=1', '/mcp/check?live=1&root=', '/mcp/check?live=1&root=%2Fproject+a',
  ]);
});

it('posts only the exact reviewed scope and revision and preserves a false outcome', async () => {
  vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ accepted: false, report: { toolsAcceptance: 'error', servers: [], summary: { errors: 1, warnings: 0 } } })));
  const result = await mcpCheckApi.accept({ name: 'docs', root: '/one', revision: 'reviewed' });
  const [path, init] = vi.mocked(fetch).mock.calls[0];
  expect(path).toBe(BASE + '/mcp/check');
  expect(init?.method).toBe('POST');
  expect(JSON.parse(String(init?.body))).toEqual({ name: 'docs', root: '/one', revision: 'reviewed' });
  expect(result.accepted).toBe(false);
});

it.each([422, 409])('preserves the typed refusal report without converting HTTP %s to success', async (status) => {
  const report = { toolsAcceptance: status === 409 ? 'stale' : 'error', servers: [{ name: 'docs', project: '/one', ok: false, findings: [{ level: 'error', check: 'live', target: '', message: 'live probe failed: tool inventory contains duplicate names' }], contract: { status: 'error' } }], summary: { errors: 1, warnings: 0 } };
  vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ accepted: false, report, error: 'inventory was not accepted; review the check report' }), { status }));
  const request = mcpCheckApi.accept({ name: 'docs', root: '/one', revision: 'reviewed' });
  await expect(request).rejects.toBeInstanceOf(MCPToolsAcceptanceError);
  await expect(request).rejects.toMatchObject({ status, report });
});

it('preserves a valid non-success response that has no check report', async () => {
  vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ error: 'invalid MCP check request' }), { status: 400 }));
  await expect(mcpCheckApi.accept({ name: 'docs', root: '', revision: 'reviewed' })).rejects.toMatchObject({ status: 400, report: null });
});

it('does not expose raw invalid acceptance responses or assume an uncertain request failed to persist', async () => {
  vi.mocked(fetch).mockResolvedValueOnce(new Response('<private invalid fixture response>', { status: 502 })).mockRejectedValueOnce(new TypeError('inert network failure'));
  const approval = { name: 'docs', root: '', revision: 'reviewed' };
  await expect(mcpCheckApi.accept(approval)).rejects.toMatchObject({ status: 502, message: 'invalid MCP acceptance response', report: null });
  await expect(mcpCheckApi.accept(approval)).rejects.toMatchObject({ status: 0, message: 'MCP acceptance outcome is unknown; probe again before accepting', report: null });
});
