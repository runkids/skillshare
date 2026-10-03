import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, type GitStatus } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import GitSyncPage from './GitSyncPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return { ...actual, api: { ...actual.api, gitStatus: vi.fn(), gitBranches: vi.fn(), gitDiscard: vi.fn(), gitCommit: vi.fn(), pull: vi.fn(), push: vi.fn() } };
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
  vi.mocked(api.gitCommit).mockResolvedValue({ success: true, message: 'committed' });
  vi.mocked(api.pull).mockResolvedValue({ success: true, upToDate: false, commits: [], stats: { filesChanged: 1, insertions: 1, deletions: 0 }, syncResults: [] });
});

describe('updates from another computer', () => {
  const conflicts = {
    localHash: 'local-head', remoteHash: 'remote-head',
    files: [
      { path: 'shared.md', local: { deleted: false, noPreview: false, content: 'my edits' }, remote: { deleted: false, noPreview: false, content: 'other computer edits' } },
      { path: 'removed.md', local: { deleted: true, noPreview: false, content: '' }, remote: { deleted: false, noPreview: true, content: '' } },
    ],
  };

  it('compares conflicts and requires a choice for every file before applying', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, isDirty: false, files: [], ahead: 1, behind: 1 });
    vi.mocked(api.pull).mockRejectedValueOnce(new ApiError(409, 'conflict', { code: 'pull_conflict', params: conflicts }));
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Pull and merge' }));
    const dialog = await screen.findByRole('dialog', { name: 'Resolve pull conflicts' });
    expect(within(dialog).getByText('my edits')).toBeTruthy();
    expect(within(dialog).getByText('other computer edits')).toBeTruthy();
    expect(within(dialog).getByText('Deleted in this version')).toBeTruthy();
    const apply = within(dialog).getByRole('button', { name: 'Apply choices and pull' });
    expect((apply as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Keep local version of shared.md' }));
    expect((apply as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Keep remote version of removed.md' }));
    expect(api.pull).toHaveBeenCalledTimes(1);
    fireEvent.click(apply);
    await waitFor(() => expect(api.pull).toHaveBeenCalledWith({ resolution: {
      localHash: 'local-head', remoteHash: 'remote-head', choices: { 'shared.md': 'local', 'removed.md': 'remote' },
    }, dryRun: false }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(api.push).not.toHaveBeenCalled();
  });

  it('can cancel conflict review and reopen it without changing files', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, isDirty: false, files: [], ahead: 1, behind: 1 });
    vi.mocked(api.pull).mockRejectedValueOnce(new ApiError(409, 'conflict', { code: 'pull_conflict', params: conflicts }));
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Pull and merge' }));
    const dialog = await screen.findByRole('dialog', { name: 'Resolve pull conflicts' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(api.pull).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('button', { name: 'Force pull…' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Review conflicts' }));
    expect(screen.getByRole('dialog', { name: 'Resolve pull conflicts' })).toBeTruthy();
    expect(api.pull).toHaveBeenCalledTimes(1);
  });

  it('explains divergence and offers a merge before pushing', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, isDirty: false, files: [], ahead: 2, behind: 3 });
    mount();
    expect(await screen.findByText(/2 local commits and 3 remote commits/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Push 2 commits' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Pull and merge' }));
    await waitFor(() => expect(api.pull).toHaveBeenCalledWith({ force: false, dryRun: false }));
    expect(api.push).not.toHaveBeenCalled();
  });

  it('commits local changes before pulling when the remote has updates', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, behind: 1 });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Commit and pull' }));
    await waitFor(() => expect(api.pull).toHaveBeenCalledWith({ force: false, dryRun: false }));
    expect(api.gitCommit).toHaveBeenCalledWith({ message: undefined, dryRun: false });
    expect(vi.mocked(api.gitCommit).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(api.pull).mock.invocationCallOrder[0]);
    expect(api.push).not.toHaveBeenCalled();
  });

  it('syncs both ways: commits, pulls, then pushes', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true });
    vi.mocked(api.push).mockResolvedValue({ success: true, message: 'pushed successfully' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Sync both ways' }));
    fireEvent.click(within(await screen.findByRole('dialog', { name: 'Sync both ways?' })).getByRole('button', { name: 'Sync both ways' }));
    await waitFor(() => expect(api.push).toHaveBeenCalledWith({}));
    expect(api.pull).toHaveBeenCalledWith({ force: false, dryRun: false, alwaysSync: true });
    const order = [api.gitCommit, api.pull, api.push].map((fn) => vi.mocked(fn).mock.invocationCallOrder[0]);
    expect(order).toEqual([...order].sort((a, b) => a - b));
  });

  it('pushes to an empty remote, then syncs targets', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, isDirty: false, files: [] });
    vi.mocked(api.pull).mockRejectedValueOnce(new ApiError(400, 'the remote has no branches yet; push first', { code: 'remote_empty' }));
    vi.mocked(api.push).mockResolvedValue({ success: true, message: 'pushed successfully' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Sync both ways' }));
    fireEvent.click(within(await screen.findByRole('dialog', { name: 'Sync both ways?' })).getByRole('button', { name: 'Sync both ways' }));
    await waitFor(() => expect(api.pull).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.push).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(api.pull).mock.invocationCallOrder[1]);
    expect(await screen.findByText('Synced with the remote')).toBeTruthy();
  });

  it('stops before pushing when syncing both ways hits a conflict', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, isDirty: false, files: [] });
    vi.mocked(api.pull).mockRejectedValueOnce(new ApiError(409, 'conflict', { code: 'pull_conflict', params: conflicts }));
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Sync both ways' }));
    fireEvent.click(within(await screen.findByRole('dialog', { name: 'Sync both ways?' })).getByRole('button', { name: 'Sync both ways' }));
    expect(await screen.findByRole('dialog', { name: 'Resolve pull conflicts' })).toBeTruthy();
    expect(api.gitCommit).not.toHaveBeenCalled();
    expect(api.push).not.toHaveBeenCalled();
  });

  it('asks before syncing both ways, explaining each step, and does nothing when cancelled', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, remoteURL: 'git@github.com:me/skills.git', behind: 3 });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Sync both ways' }));
    const dialog = await screen.findByRole('dialog', { name: 'Sync both ways?' });
    for (const step of ['Commit 2 changed files', 'Pull and merge 3 remote commits', 'Sync targets for the skills scope', 'Push the merged result to me/skills']) {
      expect(within(dialog).getByText(step)).toBeTruthy();
    }
    expect(within(dialog).getByText(/nothing is pushed/)).toBeTruthy();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    for (const fn of [api.gitCommit, api.pull, api.push]) expect(fn).not.toHaveBeenCalled();
  });

  it('previews syncing both ways without changing anything', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true });
    mount();
    fireEvent.click(await screen.findByRole('switch', { name: 'Dry run' }));
    fireEvent.click(screen.getByRole('button', { name: 'Sync both ways' }));
    expect(await screen.findByText(/then push/)).toBeTruthy();
    for (const fn of [api.gitCommit, api.pull, api.push]) expect(fn).not.toHaveBeenCalled();
  });

  it('does not pull if the local commit fails or is only a preview', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, behind: 1 });
    vi.mocked(api.gitCommit).mockRejectedValueOnce(new Error('commit failed'));
    mount();
    const button = await screen.findByRole('button', { name: 'Commit and pull' });
    fireEvent.click(button);
    expect(await screen.findByText('commit failed')).toBeTruthy();
    expect(api.pull).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('switch', { name: 'Dry run' }));
    fireEvent.click(button);
    await waitFor(() => expect(api.gitCommit).toHaveBeenCalledWith({ message: undefined, dryRun: true }));
    expect(api.pull).not.toHaveBeenCalled();
  });

  it('previews a pull without trying to render missing commit details', async () => {
    vi.mocked(api.gitStatus).mockResolvedValue({ ...status, hasRemote: true, isDirty: false, files: [], ahead: 1, behind: 1 });
    vi.mocked(api.pull).mockResolvedValue({ success: true, dryRun: true, message: 'dry run: would pull and sync' } as Awaited<ReturnType<typeof api.pull>>);
    mount();
    const button = await screen.findByRole('button', { name: 'Pull and merge' });
    fireEvent.click(screen.getByRole('switch', { name: 'Dry run' }));
    fireEvent.click(button);
    await waitFor(() => expect(api.pull).toHaveBeenCalledWith({ force: false, dryRun: true }));
    expect(await screen.findByText('dry run: would pull and sync')).toBeTruthy();
  });
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
