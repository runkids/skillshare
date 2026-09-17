import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { mcpApi } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPSyncAll from './MCPSyncAll';

vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { sync: vi.fn(), syncExtras: vi.fn() } }));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { preview: vi.fn(), configure: vi.fn() } }));

beforeEach(() => { vi.clearAllMocks(); localStorage.clear(); });

it('rejects a stale MCP preview before applying any resource', async () => {
  const user = userEvent.setup();
  const plan = { revision: 'before', sourcePath: '/config.yaml', blocked: false, changes: [] };
  vi.mocked(mcpApi.preview).mockResolvedValueOnce(plan).mockResolvedValueOnce({ ...plan, revision: 'after' });
  vi.mocked(api.sync).mockResolvedValue({ results: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] });
  vi.mocked(api.syncExtras).mockResolvedValue({ extras: [] });
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSyncAll /></I18nProvider></QueryClientProvider>);
  await user.click(screen.getByRole('button', { name: 'Sync all resources' }));
  const dialog = await screen.findByRole('dialog', { name: 'Sync all resources' });
  await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Sync all resources' })).toBeEnabled());
  await user.click(within(dialog).getByRole('button', { name: 'Sync all resources' }));
  await screen.findByText('Settings changed. Preview again before syncing.');
  expect(api.sync).toHaveBeenCalledTimes(1);
  expect(api.sync).toHaveBeenCalledWith({ dryRun: true });
  expect(api.syncExtras).toHaveBeenCalledTimes(1);
  expect(mcpApi.configure).not.toHaveBeenCalled();
});
