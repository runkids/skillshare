import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { MCPPlan } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import { MCP_CHANGED, runSync } from '../sync/syncView';
import MCPSyncBox from './MCPSyncBox';

vi.mock('../sync/syncView', async (load) => ({
  ...(await load<typeof import('../sync/syncView')>()),
  runSync: vi.fn(),
}));

const own = {
  target: 'claude',
  path: '/work/app/.mcp.json',
  root: '/work/app',
  name: 'docs',
  action: 'add',
};
const other = {
  target: 'cursor',
  path: '/home/me/.cursor/mcp.json',
  name: 'notes',
  action: 'update',
};
const plan: MCPPlan = { revision: 'r1', sourcePath: '', blocked: false, changes: [own, other] };

// The project view passes only its own changes; the plan still covers every file.
const box = (p: MCPPlan = plan) =>
  render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient()}>
        <I18nProvider>
          <MCPSyncBox changes={[own]} roots={['/work', '/work/app']} plan={p} />
        </I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );

describe('MCP sync box', () => {
  beforeEach(() => {
    vi.mocked(runSync).mockReset();
  });

  it('confirms every pending change, then writes only MCP with the reviewed plan', async () => {
    const user = userEvent.setup();
    vi.mocked(runSync).mockResolvedValue({ resources: undefined, failures: [] });
    box();
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    expect(screen.getByText('Also writes 1 change outside this project.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Sync Now' }));
    await waitFor(() =>
      expect(runSync).toHaveBeenCalledWith({
        resources: null,
        extras: false,
        mcp: plan,
        force: false,
      }),
    );
    expect(await screen.findByText('The MCP config files are written.')).toBeInTheDocument();
  });

  it('says so when the plan moved before it was written', async () => {
    const user = userEvent.setup();
    vi.mocked(runSync).mockRejectedValue(new Error(MCP_CHANGED));
    box();
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    await user.click(screen.getByRole('button', { name: 'Sync Now' }));
    expect(await screen.findByText(/The MCP changes shifted during the sync/)).toBeInTheDocument();
  });

  it('sends a blocked plan to the Sync page instead of offering to write it', () => {
    box({ ...plan, blocked: true });
    expect(screen.queryByRole('button', { name: 'Sync MCP' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Review in Sync' })).toBeInTheDocument();
  });
});
