import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { mcpApi } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPRemoveDialog from './MCPRemoveDialog';

vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { preview: vi.fn(), configure: vi.fn() } }));

it('removes from source without syncing when an Agent entry conflicts', async () => {
  const user = userEvent.setup();
  const saved = vi.fn();
  vi.mocked(mcpApi.preview).mockResolvedValue({ revision: 'reviewed', sourcePath: '/config.yaml', blocked: true, changes: [{ target: 'cursor', path: '/.cursor/mcp.json', name: 'docs', action: 'conflict', message: 'Agent configuration changed; import it or explicitly replace this entry' }] });
  vi.mocked(mcpApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPRemoveDialog name="docs" onClose={vi.fn()} onSaved={saved} /></I18nProvider></QueryClientProvider>);
  expect(await screen.findByText('Agent config was changed outside skillshare')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Remove and sync' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: 'Remove from source only' }));
  await waitFor(() => expect(saved).toHaveBeenCalled());
  expect(mcpApi.configure).toHaveBeenCalledWith({ name: 'docs', remove: true }, 'reviewed', false);
});
