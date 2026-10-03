import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import FileBackups from './FileBackups';

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, listFileBackups: vi.fn(), getFileBackupVersions: vi.fn() } };
});

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.listFileBackups).mockResolvedValue({ files: [{ path: '/other/AGENTS.md', versions: 1, latest: '2026-10-03T00:00:00Z' }] });
  vi.mocked(api.getFileBackupVersions).mockResolvedValue({ path: '/other/AGENTS.md', current: { exists: true }, versions: [] });
});

function renderBackups(path = '/backup?tab=files') {
  return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <I18nProvider><ToastProvider><MemoryRouter initialEntries={[path]}><FileBackups /></MemoryRouter></ToastProvider></I18nProvider>
  </QueryClientProvider>);
}

it('shows no history when the requested note has no backups instead of selecting another file', async () => {
  renderBackups('/backup?tab=files&path=%2Fmemory%2Fnew.md');
  expect(await screen.findByText('No file backups yet')).toBeInTheDocument();
  expect(api.getFileBackupVersions).not.toHaveBeenCalled();
  expect(screen.queryByText('/other/AGENTS.md')).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /Restore/ })).not.toBeInTheDocument();
});

it('selects the first file when no path was requested', async () => {
  renderBackups();
  expect(await screen.findByRole('listitem')).toHaveAttribute('aria-current', 'true');
  expect(api.getFileBackupVersions).toHaveBeenCalledWith('/other/AGENTS.md');
});
