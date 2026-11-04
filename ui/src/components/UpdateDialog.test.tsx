import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import UpdateDialog from './UpdateDialog';
import { api } from '../api/client';

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      getVersionCheck: vi.fn().mockResolvedValue({
        cliVersion: '0.21.12',
        cliLatest: '0.21.13',
        cliUpdateAvailable: true,
        skillVersion: '0.21.12',
        skillLatest: '0.21.13',
        skillUpdateAvailable: true,
      }),
      upgradeApp: vi.fn().mockResolvedValue({}),
      restartApp: vi.fn().mockResolvedValue({ ok: true, restarting: true }),
      health: vi.fn(() => new Promise(() => {})),
    },
  };
});

describe('UpdateDialog', () => {
  it('keeps the UI the upgrade just downloaded when restarting', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <I18nProvider>
          <UpdateDialog />
        </I18nProvider>
      </QueryClientProvider>,
    );

    await userEvent.click(await screen.findByRole('button', { name: /Update now/ }));

    await waitFor(() => expect(api.restartApp).toHaveBeenCalledWith({ clearCache: false }));
  });
});
