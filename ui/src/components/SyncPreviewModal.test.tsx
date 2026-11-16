import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import { invalidate } from '../lib/queryEvents';
import SyncPreviewModal from './SyncPreviewModal';
import ProfileBanners from './profiles/ProfileBanners';

const state = vi.hoisted(() => ({ diffLoading: false, targetsLoading: false, error: null as Error | null, refetch: vi.fn() }));
vi.mock('../api/client', async (load) => ({ ...await load<typeof import('../api/client')>(), api: { sync: vi.fn() } }));
vi.mock('../lib/queryEvents', () => ({ invalidate: vi.fn().mockResolvedValue([]) }));
vi.mock('../hooks/useSharedQueries', () => ({
  useDiffQuery: () => ({ data: { diffs: [{ target: 'claude', items: [] }] }, isFetching: state.diffLoading, error: state.error, refetch: state.refetch }),
  useSyncedTargetsQuery: () => ({ data: { targets: [{ name: 'claude', agentPath: '/agents' }] }, isPending: state.targetsLoading }),
}));
vi.mock('../i18n', () => ({ useT: () => (key: string) => key, useI18n: () => ({ locale: 'en' }), plural: (key: string) => key }));

const response = { results: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] };
const sync = vi.mocked(api.sync);
beforeEach(() => {
  vi.clearAllMocks();
  sync.mockReset();
  sync.mockResolvedValue(response);
  state.diffLoading = false;
  state.targetsLoading = false;
  state.error = null;
});
function mount(content: React.ReactNode) {
  return render(<QueryClientProvider client={new QueryClient()}><MemoryRouter>{content}</MemoryRouter></QueryClientProvider>);
}
function pending() {
  mount(<ProfileBanners data={{ active: 'work', state: 'active', pending: true, profiles: [] }} onChoose={vi.fn()} />);
  fireEvent.click(screen.getByRole('button', { name: 'profiles.syncNow' }));
}

it.each([undefined, 'skill', 'agent'] as const)('keeps ordinary %s no-change previews read-only', (kind) => {
  mount(<SyncPreviewModal open kind={kind} onClose={vi.fn()} />);
  expect(screen.getByRole('button', { name: 'syncPreview.closeButton' })).toBeEnabled();
  expect(screen.queryByRole('button', { name: 'syncPreview.syncNowButton' })).toBeNull();
  expect(sync).not.toHaveBeenCalled();
});

it.each(['diff', 'targets', 'error'])('never permits pending settlement while %s blocks the preview', (blocked) => {
  state.diffLoading = blocked === 'diff';
  state.targetsLoading = blocked === 'targets';
  state.error = blocked === 'error' ? new Error('preview unavailable') : null;
  pending();
  expect(screen.queryByRole('button', { name: 'syncPreview.syncNowButton' })).toBeNull();
  expect(sync).not.toHaveBeenCalled();
});

it('settles a pending profile with no target changes only after an explicit skills sync', async () => {
  pending();
  expect(sync).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'syncPreview.syncNowButton' }));
  await waitFor(() => expect(invalidate).toHaveBeenCalledWith(expect.anything(), 'synced'));
  expect(sync).toHaveBeenCalledExactlyOnceWith({ dryRun: false, kind: 'skill' });
  expect(screen.getByRole('button', { name: 'syncPreview.closeButton' })).toBeEnabled();
});

it('prevents a second pending settlement while the first explicit request is in flight', async () => {
  let resolve!: (value: typeof response) => void;
  sync.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
  pending();
  const apply = screen.getByRole('button', { name: 'syncPreview.syncNowButton' });
  fireEvent.click(apply);
  expect(apply).toBeDisabled();
  fireEvent.click(apply);
  expect(sync).toHaveBeenCalledExactlyOnceWith({ dryRun: false, kind: 'skill' });
  await act(async () => resolve(response));
});

it.each([
  { profileComplete: false, warnings: ['kept modified alpha'] },
  { failed: [{ target: 'claude', part: 'skill' as const, error: 'permission denied', message: 'permission denied' }], warnings: ['permission denied'] },
  { profile: { name: 'work', state: 'active' as const, pending: true } },
])('does not settle the caller after an incomplete HTTP 200 result: %j', async (partial) => {
  const complete = vi.fn();
  sync.mockResolvedValueOnce({ ...response, ...partial }).mockResolvedValueOnce({ ...response, profileComplete: true });
  mount(<SyncPreviewModal open kind="skill" profilePending onClose={vi.fn()} onComplete={complete} />);
  fireEvent.click(screen.getByRole('button', { name: 'syncPreview.syncNowButton' }));
  expect(await screen.findByText('syncPreview.incomplete')).toBeInTheDocument();
  expect(screen.queryByText('syncPreview.completed')).toBeNull();
  expect(complete).not.toHaveBeenCalled();
  for (const warning of partial.warnings ?? []) expect(screen.getByText(warning)).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'syncPreview.retryButton' }));
  fireEvent.click(screen.getByRole('button', { name: 'syncPreview.syncNowButton' }));
  await waitFor(() => expect(complete).toHaveBeenCalledExactlyOnceWith({ ...response, profileComplete: true }));
  expect(sync).toHaveBeenCalledTimes(2);
});
