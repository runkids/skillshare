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
      getUpdatePolicy: vi.fn().mockResolvedValue({ enabled: true, configured: true, envOverride: false }),
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
  it('allows an explicitly requested upgrade while automatic checks are disabled', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(api.getUpdatePolicy).mockResolvedValueOnce({ enabled: false, configured: false, envOverride: false });
    vi.mocked(api.upgradeApp).mockResolvedValueOnce({ devMode: true, cliUpdated: true, cliVersion: '9.9.9' });
    const onDismiss = vi.fn();
    render(
      <QueryClientProvider client={queryClient}><I18nProvider>
        <UpdateDialog onDismiss={onDismiss} manualData={{ autoCheckEnabled: false, cliDevMode: true, cliVersion: '1.0.0', cliLatest: '9.9.9', cliUpdateAvailable: true, skillVersion: '', skillUpdateAvailable: false }} />
      </I18nProvider></QueryClientProvider>,
    );
    await userEvent.click(await screen.findByRole('button', { name: /Update now/ }));
    await waitFor(() => expect(api.upgradeApp).toHaveBeenCalled());
    await waitFor(() => expect(onDismiss).toHaveBeenCalledOnce(), { timeout: 2000 });
  });

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
