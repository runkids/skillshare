import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { mcpApi, type MCPPlan } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPRestoreDialog from './MCPRestoreDialog';

vi.mock('../../api/mcp', async (load) => ({
  ...(await load<typeof import('../../api/mcp')>()),
  mcpApi: { previewRestore: vi.fn(), restore: vi.fn() },
}));

const backups = [
  { id: '1790748600000000000-first', target: 'claude', path: '/.claude.json' },
  { id: '1790748500000000000-second', target: 'pi', path: '/work/app/.pi/mcp.json' },
];
const plan = (revision: string, names: string[]): MCPPlan => ({
  revision,
  sourcePath: '/config.yaml',
  blocked: false,
  changes: names.map((name) => ({ target: 'pi', path: backups[1].path, name, action: 'restore' })),
});

beforeEach(() => vi.resetAllMocks());

it('restores only the selected backup with its completed preview revision', async () => {
  const user = userEvent.setup();
  const restored = vi.fn();
  let resolveSecond!: (value: MCPPlan) => void;
  vi.mocked(mcpApi.previewRestore)
    .mockResolvedValueOnce(plan('first', ['docs']))
    .mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSecond = resolve;
      }),
    );
  vi.mocked(mcpApi.restore).mockResolvedValue({ applied: [], backupIds: [] });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <I18nProvider>
        <MCPRestoreDialog backups={backups} onClose={vi.fn()} onRestored={restored} />
      </I18nProvider>
    </QueryClientProvider>,
  );

  const restore = screen.getByRole('button', { name: 'Restore this file' });
  await waitFor(() => expect(restore).toBeEnabled());
  await user.click(screen.getAllByRole('radio')[1]);
  await user.click(restore);
  expect(mcpApi.restore).not.toHaveBeenCalled();
  expect(mcpApi.previewRestore).toHaveBeenLastCalledWith(backups[1].id);

  await act(async () => resolveSecond(plan('second', ['context'])));
  await waitFor(() => expect(restore).toBeEnabled());
  await user.click(restore);
  await waitFor(() => expect(restored).toHaveBeenCalledOnce());
  expect(mcpApi.restore).toHaveBeenCalledExactlyOnceWith(backups[1].id, 'second');
});
