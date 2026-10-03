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

it.each([
  [String.raw`C:\Users\test\memory\wiki\note.md`, String.raw`C:\Users\test\memory/wiki/note.md`],
  [String.raw`\\server\share\memory\wiki\note.md`, String.raw`\\server\share\memory/wiki/note.md`],
])('selects the native backup path %s for a mixed-separator Windows history link', async (native, requested) => {
  vi.mocked(api.listFileBackups).mockResolvedValue({ files: [{ path: native, versions: 1, latest: '2026-10-03T00:00:00Z' }] });
  renderBackups(`/backup?tab=files&path=${encodeURIComponent(requested)}`);
  expect(await screen.findByRole('listitem')).toHaveAttribute('aria-current', 'true');
  expect(api.getFileBackupVersions).toHaveBeenCalledWith(native);
  expect(screen.queryByText('No file backups yet')).not.toBeInTheDocument();
});

it('does not treat a literal POSIX backslash as a directory separator', async () => {
  vi.mocked(api.listFileBackups).mockResolvedValue({ files: [{ path: String.raw`/memory/wiki\note.md`, versions: 1, latest: '2026-10-03T00:00:00Z' }] });
  renderBackups('/backup?tab=files&path=%2Fmemory%2Fwiki%2Fnote.md');
  expect(await screen.findByText('No file backups yet')).toBeInTheDocument();
  expect(api.getFileBackupVersions).not.toHaveBeenCalled();
});
