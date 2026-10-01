import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type GitStatus } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import GitSyncPage from './GitSyncPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return { ...actual, api: { ...actual.api, gitStatus: vi.fn(), gitBranches: vi.fn(), gitDiscard: vi.fn() } };
});

const status: GitStatus = {
  gitInstalled: true, isRepo: true, hasRemote: false, branch: 'main', isDirty: true,
  files: [' M .metadata.json', '?? new-skill/'], sourceDir: '/source', scope: 'skills',
  scopeMismatch: false, headHash: 'abc123', ahead: 0, behind: 0, nestedRepos: [], configTracked: false,
};

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <I18nProvider><ToastProvider><GitSyncPage /></ToastProvider></I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.setItem('skillshare:locale', 'en');
  vi.mocked(api.gitStatus).mockResolvedValue(status);
  vi.mocked(api.gitBranches).mockResolvedValue({ current: 'main', local: ['main'], remote: [], isDirty: true, dirtyFiles: status.files });
  vi.mocked(api.gitDiscard).mockResolvedValue({ success: true, message: 'changes discarded' });
});

describe('discard changes', () => {
  it('requires confirmation, supports cancel, and refreshes after discarding', async () => {
    const client = mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Discard changes' }));
    const dialog = screen.getByRole('dialog', { name: 'Discard all changes?' });
    expect(within(dialog).getByText(/untracked files and folders will be deleted/)).toBeTruthy();
    expect(api.gitDiscard).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(api.gitDiscard).not.toHaveBeenCalled();

    const invalidate = vi.spyOn(client, 'invalidateQueries');
    vi.mocked(api.gitDiscard).mockImplementation(async () => {
      vi.mocked(api.gitStatus).mockResolvedValue({ ...status, isDirty: false, files: [] });
      return { success: true, message: 'changes discarded' };
    });
    fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }));
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Discard changes' }));
    await waitFor(() => expect(api.gitDiscard).toHaveBeenCalledWith({ dryRun: false }));
    expect(await screen.findByText('Changes discarded')).toBeTruthy();
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Discard changes' })).toBeNull());
    expect(invalidate).toHaveBeenCalledWith();
  });

  it('previews without opening the destructive confirmation when dry run is on', async () => {
    mount();
    const button = await screen.findByRole('button', { name: 'Discard changes' });
    fireEvent.click(screen.getByRole('switch', { name: 'Dry run' }));
    fireEvent.click(button);
    await waitFor(() => expect(api.gitDiscard).toHaveBeenCalledWith({ dryRun: true }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(await screen.findByText(/Dry run: would restore/)).toBeTruthy();
  });

  it('shows failure and leaves the changes available to retry', async () => {
    vi.mocked(api.gitDiscard).mockRejectedValue(new Error('permission denied'));
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Discard changes' }));
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Discard changes' }));
    expect(await screen.findByText('permission denied')).toBeTruthy();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect((screen.getByRole('button', { name: 'Discard changes' }) as HTMLButtonElement).disabled).toBe(false);
  });

  it('disables discard before the first commit', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, headHash: undefined });
    mount();
    const button = await screen.findByRole('button', { name: 'Discard changes' });
    expect((button as HTMLButtonElement).disabled).toBe(true);
    expect(api.gitDiscard).not.toHaveBeenCalled();
  });
});
