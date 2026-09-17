import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { mcpApi, type MCPPlan } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPSyncAll from './MCPSyncAll';

vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { sync: vi.fn(), syncExtras: vi.fn() } }));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { preview: vi.fn(), configure: vi.fn() } }));

beforeEach(() => { vi.clearAllMocks(); localStorage.clear(); });

const change = { target: 'claude', path: '/claude.json', name: 'docs', action: 'add' };
const plan = { revision: 'before', sourcePath: '/config.yaml', blocked: false, changes: [change] };

async function syncAll(...fresh: MCPPlan[]) {
  const user = userEvent.setup();
  vi.mocked(mcpApi.preview).mockResolvedValueOnce(plan);
  for (const next of fresh) vi.mocked(mcpApi.preview).mockResolvedValueOnce(next);
  vi.mocked(mcpApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
  vi.mocked(api.sync).mockResolvedValue({ results: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] });
  vi.mocked(api.syncExtras).mockResolvedValue({ extras: [] });
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSyncAll /></I18nProvider></QueryClientProvider>);
  await user.click(screen.getByRole('button', { name: 'Sync all resources' }));
  const dialog = await screen.findByRole('dialog', { name: 'Sync all resources' });
  await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Sync all resources' })).toBeEnabled());
  await user.click(within(dialog).getByRole('button', { name: 'Sync all resources' }));
}

it('rejects a changed MCP preview before applying any resource', async () => {
  await syncAll({ ...plan, changes: [{ ...change, action: 'update' }] });
  await screen.findByText('Settings changed. Preview again before syncing.');
  expect(api.sync).toHaveBeenCalledTimes(1);
  expect(api.sync).toHaveBeenCalledWith({ dryRun: true });
  expect(api.syncExtras).toHaveBeenCalledTimes(1);
  expect(mcpApi.configure).not.toHaveBeenCalled();
});

it('applies MCP with the fresh revision when only the revision changed', async () => {
  await syncAll({ ...plan, revision: 'mid', changes: [change, { ...change, name: 'other', action: 'unchanged' }] }, { ...plan, revision: 'after' });
  await waitFor(() => expect(mcpApi.configure).toHaveBeenCalledWith({}, 'after', true));
});

it('stops before MCP when its changes differ after syncing resources', async () => {
  await syncAll(plan, { ...plan, revision: 'after', changes: [{ ...change, action: 'remove' }] });
  await screen.findByText('Settings changed. Preview again before syncing.');
  expect(api.sync).toHaveBeenCalledWith({});
  expect(mcpApi.configure).not.toHaveBeenCalled();
});
