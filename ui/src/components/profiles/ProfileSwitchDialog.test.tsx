import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { api, ApiError, type ProfileApplyResult, type ProfilesResponse } from '../../api/client';
import { invalidate } from '../../lib/queryEvents';
import ProfileSwitchDialog from './ProfileSwitchDialog';
import ProfileBanners from './ProfileBanners';

const fixture = vi.hoisted(() => ({ configDir: '/fixture/.skillshare' }));

vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { profiles: { setActive: vi.fn() } } }));
vi.mock('../../lib/queryEvents', () => ({ invalidate: vi.fn().mockResolvedValue([]) }));
vi.mock('../../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: true, projectRoot: '/fixture' }) }));
vi.mock('../../hooks/useSharedQueries', () => ({ useOverviewQuery: () => ({ data: { configDir: fixture.configDir } }), useSkillsQuery: () => ({ data: { resources: [{ name: 'a' }, { name: 'b' }] } }) }));
vi.mock('../../i18n', () => ({ useT: () => (key: string, params?: object) => key + (params ? JSON.stringify(params) : '') }));
vi.mock('../DialogShell', () => ({ default: ({ open, children }: { open: boolean; children: React.ReactNode }) => open ? <div role="dialog">{children}</div> : null }));
vi.mock('../CopyButton', () => ({ default: ({ value }: { value: string }) => <code>{value}</code> }));
vi.mock('../SyncPreviewModal', () => ({ default: ({ open, kind }: { open: boolean; kind: string }) => open ? <div data-testid="sync">{kind}</div> : null }));

const data: ProfilesResponse = {
  active: 'old', state: 'active', pending: false,
  profiles: ['work', 'other'].map((name) => ({
    name, description: '', skills: { include: ['a'], exclude: [] },
    selected: 1, unmatched: ['missing*'], usable: true,
  })),
};
const result: ProfileApplyResult = {
  profile: { previous: 'old', active: 'work', pending: false }, complete: true,
  targets: [], blocked: [], symlink_targets: [], snapshot: [],
};
const entry = { target: 'claude', entry: 'edited', step: 'prune', class: 'modified_copy', reason: 'copy was edited or could not be verified' };
const blocked = () => new ApiError(409, 'blocked', { code: 'profile_blocked', params: { blocked: JSON.stringify([entry]) } });
const setActive = vi.mocked(api.profiles.setActive);
beforeEach(() => { vi.clearAllMocks(); setActive.mockReset(); fixture.configDir = '/fixture/.skillshare'; });
function mount(candidate: string | null = 'work') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = (name: string | null, profiles = data) => <QueryClientProvider client={client}><ProfileSwitchDialog candidate={name} profiles={profiles} onClose={vi.fn()} /></QueryClientProvider>;
  return { ...render(view(candidate)), view };
}

it.each(['.skillshare', 'skillshare'])('requires snapshot-first consent in the %s project folder and sends only reviewed flags', async (dir) => {
  fixture.configDir = `/fixture/${dir}`;
  setActive.mockRejectedValueOnce(blocked()).mockResolvedValue(result);
  mount();
  const checkbox = await screen.findByRole('checkbox', { name: /profiles.discardProject/ });
  await waitFor(() => expect(checkbox).toBeEnabled());
  expect(checkbox.closest('label')).toHaveTextContent(`/fixture/${dir}/backups`);
  expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeDisabled();
  fireEvent.click(checkbox);
  await waitFor(() => expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeEnabled());
  expect(setActive).toHaveBeenLastCalledWith({ name: 'work', dryRun: true, discardEdits: true });
  fireEvent.click(screen.getByRole('button', { name: 'profiles.switch' }));
  await waitFor(() => expect(setActive).toHaveBeenLastCalledWith({ name: 'work', discardEdits: true }));
});

it('requires explicit allowEmpty and keeps unmatched patterns visible', async () => {
  setActive.mockRejectedValueOnce(new ApiError(409, 'empty', { code: 'profile_empty' })).mockResolvedValue(result);
  mount();
  expect(await screen.findByRole('checkbox', { name: 'profiles.allowEmpty' })).not.toBeChecked();
  await waitFor(() => expect(screen.getByRole('checkbox')).toBeEnabled());
  expect(screen.getByText(/missing\*/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeDisabled();
  fireEvent.click(screen.getByRole('checkbox'));
  await waitFor(() => expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeEnabled());
  fireEvent.click(screen.getByRole('button', { name: 'profiles.switch' }));
  await waitFor(() => expect(setActive).toHaveBeenLastCalledWith({ name: 'work', allowEmpty: true }));
});

it('ignores an older preview after candidate changes and resets consent', async () => {
  let resolve!: (value: ProfileApplyResult) => void;
  setActive.mockRejectedValueOnce(blocked()).mockImplementationOnce(() => new Promise((r) => { resolve = r; })).mockRejectedValueOnce(blocked());
  const { rerender, view } = mount();
  const checkbox = await screen.findByRole('checkbox');
  await waitFor(() => expect(checkbox).toBeEnabled());
  fireEvent.click(checkbox);
  await waitFor(() => expect(setActive).toHaveBeenCalledTimes(2));
  rerender(view('other'));
  expect(await screen.findByRole('checkbox')).not.toBeChecked();
  expect(setActive).toHaveBeenLastCalledWith({ name: 'other', dryRun: true });
  await act(async () => resolve({ ...result, targets: [{ name: 'stale-target', state: 'applied', added: ['stale'], removed: [], refreshed: [], converted: [], kept_modified: [] }] }));
  expect(screen.queryByText('stale-target')).toBeNull();
  expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeDisabled();
});

it.each(['failed', 'incomplete'] as const)('retains %s targets and snapshots through cache refresh', async (state) => {
  const partial: ProfileApplyResult = {
    ...result, complete: false, profile: { ...result.profile, pending: true },
    targets: [{
      name: 'claude', state, added: [], refreshed: ['a'], removed: [], converted: [], kept_modified: [],
      error: 'EACCES', prune_error: 'prune EIO',
    }],
    snapshot: [{
      target: 'claude', path: '/snapshot', restore: 'skillshare restore claude --from stamp -p --force',
      retention: 'Existing backup retention applies; restore before cleanup removes this snapshot.',
    }],
  };
  setActive.mockResolvedValueOnce(result).mockResolvedValueOnce(partial);
  const { rerender, view } = mount();
  await waitFor(() => expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeEnabled());
  fireEvent.click(screen.getByRole('button', { name: 'profiles.switch' }));
  expect(await screen.findByText('profiles.incomplete')).toBeInTheDocument();
  expect(screen.getByText('EACCES')).toBeInTheDocument();
  expect(screen.getByText('prune EIO')).toBeInTheDocument();
  expect(screen.getByText(/skillshare restore claude/)).toBeInTheDocument();
  expect(invalidate).toHaveBeenCalledWith(expect.anything(), 'profileSwitched');
  rerender(view('work', { ...data, active: 'work', pending: true }));
  expect(screen.getByText('profiles.incomplete')).toBeInTheDocument();
  expect(screen.getByText('EACCES')).toBeInTheDocument();
  expect(screen.getByRole('dialog')).toBeInTheDocument();
});

it('never authorizes unknown blocked classes or malformed refusal details', async () => {
  setActive.mockRejectedValue(new ApiError(409, 'blocked', { code: 'profile_blocked', params: { blocked: JSON.stringify([{ ...entry, class: 'unmanaged' }]) } }));
  mount();
  await screen.findByText(/edited/);
  expect(screen.queryByRole('checkbox')).toBeNull();
  expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeDisabled();
});

it('requires fresh consent when the real apply discovers a newly edited copy', async () => {
  setActive.mockRejectedValueOnce(blocked()).mockResolvedValueOnce(result).mockRejectedValue(blocked());
  mount();
  const checkbox = await screen.findByRole('checkbox');
  await waitFor(() => expect(checkbox).toBeEnabled());
  fireEvent.click(checkbox);
  await waitFor(() => expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeEnabled());
  fireEvent.click(screen.getByRole('button', { name: 'profiles.switch' }));
  await waitFor(() => expect(screen.getByRole('checkbox')).not.toBeChecked());
  expect(screen.getByRole('button', { name: 'profiles.switch' })).toBeDisabled();
  expect(setActive.mock.calls.filter(([body]) => !body.dryRun)).toEqual([[{ name: 'work', discardEdits: true }]]);
});

it('offers both unavailable repairs and opens the shared skills-only sync preview when pending', async () => {
  const choose = vi.fn();
  const { rerender } = render(<ProfileBanners data={{ ...data, state: 'unavailable', active: 'lost' }} onChoose={choose} />);
  fireEvent.click(screen.getByRole('button', { name: 'profiles.all' }));
  expect(choose).toHaveBeenCalledWith(null);
  fireEvent.click(screen.getByRole('button', { name: 'profiles.choose' }));
  fireEvent.click(screen.getByRole('combobox', { name: 'profiles.choose' }));
  fireEvent.mouseDown(screen.getByRole('option', { name: /work/ }));
  expect(choose).toHaveBeenLastCalledWith('work');
  rerender(<ProfileBanners data={{ ...data, pending: true }} onChoose={choose} />);
  fireEvent.click(screen.getByRole('button', { name: 'profiles.syncNow' }));
  expect(screen.getByTestId('sync')).toHaveTextContent('skill');
});
