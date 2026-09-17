import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Skill, TrashedSkill } from '../api/client';
import { api } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import TrashPage from './TrashPage';

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listTrash: vi.fn(),
      restoreTrash: vi.fn(),
      emptyTrash: vi.fn(),
    },
  };
});

function renderTrashPage(kind: Skill['kind']) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <ToastProvider>
          <TrashPage kind={kind} />
        </ToastProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
}

describe('TrashPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listTrash).mockResolvedValue({
      totalSize: 20,
      items: [trashItem('alpha', 'skill'), trashItem('beta', 'agent')],
    });
  });

  it('lists only items of the tab kind', async () => {
    renderTrashPage('skill');

    expect(await screen.findByText('alpha')).toBeInTheDocument();
    expect(screen.queryByText('beta')).not.toBeInTheDocument();
  });

  it('restores without asking because restoring is reversible', async () => {
    vi.mocked(api.restoreTrash).mockResolvedValue({ success: true });
    const user = userEvent.setup();
    renderTrashPage('agent');

    await user.click(await screen.findByRole('button', { name: /restore/i }));

    expect(api.restoreTrash).toHaveBeenCalledWith('beta', 'agent');
  });

  it('empties only the trash of the tab kind after confirmation', async () => {
    vi.mocked(api.emptyTrash).mockResolvedValue({ success: true, removed: 1 });
    const user = userEvent.setup();
    renderTrashPage('agent');

    await user.click(await screen.findByRole('button', { name: /empty trash/i }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: /empty trash/i }));

    await waitFor(() => expect(api.emptyTrash).toHaveBeenCalledWith('agent'));
  });
});

function trashItem(name: string, kind: Skill['kind']): TrashedSkill {
  return {
    name,
    kind,
    timestamp: `2026-06-03_10-00-0${name.length}`,
    date: new Date().toISOString(),
    size: 10,
    path: `/trash/${name}`,
  };
}
