import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { api, ApiError } from '../api/client';
import { definition, mountProfiles, profileFixtures } from '../test/profileFixtures';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it.each([false, true])('creates an explicitly empty inactive definition from the empty page, project=%s', async (project) => {
  const f = profileFixtures(project, []);
  await mountProfiles();
  fireEvent.click(await screen.findByRole('button', { name: 'Create profile' }));
  const dialog = await screen.findByRole('dialog', { name: 'Create profile' });
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'frontend' } });
  expect(f.create).not.toHaveBeenCalled();
  expect(f.sync).not.toHaveBeenCalled();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Create profile' }));
  await waitFor(() => expect(f.create).toHaveBeenCalledExactlyOnceWith({ name: 'frontend', description: '', skills: { include: [], exclude: [] } }));
  expect(f.sync).not.toHaveBeenCalled();
});

it('opens only raw repair actions for an unusable definition', async () => {
  const f = profileFixtures(false, [definition(), definition('future', { usable: false, reason: 'unknown profile key "future"' })]);
  await mountProfiles();
  expect(await screen.findByRole('link', { name: 'work' })).toHaveAttribute('href', '/profiles/work');
  fireEvent.click(screen.getByRole('link', { name: 'future' }));
  expect(await screen.findByRole('button', { name: 'Rename profile' })).toBeEnabled();
  expect(screen.getByRole('button', { name: 'Delete profile' })).toBeEnabled();
  expect(screen.queryByRole('button', { name: 'Save profile' })).toBeNull();
  expect(screen.queryByLabelText('Description')).toBeNull();
  expect(screen.queryByLabelText('Include patterns')).toBeNull();
  expect(screen.queryByRole('region', { name: 'Selected skills' })).toBeNull();
  expect(f.preview).not.toHaveBeenCalled();
  expect(f.replace).not.toHaveBeenCalled();
  expect(f.remove).not.toHaveBeenCalled();
});

it.each([false, true])('renames an unusable definition without semantic replacement, active=%s', async (active) => {
  const f = profileFixtures(false, [definition('future', { usable: false, reason: 'unknown profile key "future"' })]);
  if (active) Object.assign(f.catalog, { active: 'future', state: 'unavailable', pending: true });
  await mountProfiles('/profiles/future');
  const remove = await screen.findByRole('button', { name: 'Delete profile' });
  if (active) expect(remove).toBeDisabled();
  else expect(remove).toBeEnabled();
  fireEvent.click(screen.getByRole('button', { name: 'Rename profile' }));
  const dialog = await screen.findByRole('dialog', { name: 'Rename profile' });
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'future-renamed' } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Rename profile' }));
  await waitFor(() => expect(f.replace).toHaveBeenCalledExactlyOnceWith('future', { name: 'future-renamed' }));
  expect(f.remove).not.toHaveBeenCalled();
  expect(f.preview).not.toHaveBeenCalled();
  expect(f.sync).not.toHaveBeenCalled();
});

it('keeps the inactive definition when a concurrent active-delete refusal arrives', async () => {
  const f = profileFixtures();
  f.remove.mockRejectedValue(new ApiError(400, 'cannot delete active profile "work"'));
  await mountProfiles('/profiles/work');
  fireEvent.click(await screen.findByRole('button', { name: 'Delete profile' }));
  const dialog = await screen.findByRole('dialog');
  expect(f.remove).not.toHaveBeenCalled();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Delete profile' }));
  await waitFor(() => expect(f.remove).toHaveBeenCalledExactlyOnceWith('work'));
  expect(await screen.findByText('cannot delete active profile "work"')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Save profile' })).toBeInTheDocument();
  expect(f.sync).not.toHaveBeenCalled();
});

it.each(['escape', 'backdrop'])('preserves an in-flight create through %s dismissal and keeps a failed draft retryable', async (dismissal) => {
  const f = profileFixtures(false, []);
  let reject!: (error: Error) => void;
  f.create.mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; }));
  await mountProfiles();
  fireEvent.click(await screen.findByRole('button', { name: 'Create profile' }));
  const dialog = await screen.findByRole('dialog', { name: 'Create profile' });
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'frontend' } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Create profile' }));
  await waitFor(() => expect(f.create).toHaveBeenCalledTimes(1));
  if (dismissal === 'escape') fireEvent.keyDown(document, { key: 'Escape' });
  else fireEvent.mouseDown(dialog);
  expect(screen.getByRole('dialog', { name: 'Create profile' })).toBe(dialog);
  expect(within(dialog).getByLabelText('Name')).toHaveValue('frontend');
  await act(async () => { reject(new Error('disk full')); });
  expect(await within(dialog).findByText('disk full')).toBeInTheDocument();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Create profile' }));
  await waitFor(() => expect(f.create).toHaveBeenCalledTimes(2));
  expect(f.create.mock.calls[1]).toEqual(f.create.mock.calls[0]);
});

it('does not close a newer draft when an unmounted create finishes', async () => {
  const f = profileFixtures(false, []);
  let resolve!: (result: { changed: boolean; needsSync: boolean; skills: null }) => void;
  f.create.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
  const first = await mountProfiles();
  fireEvent.click(await screen.findByRole('button', { name: 'Create profile' }));
  let dialog = await screen.findByRole('dialog', { name: 'Create profile' });
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'first' } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Create profile' }));
  await waitFor(() => expect(f.create).toHaveBeenCalledTimes(1));
  first.unmount();
  await mountProfiles();
  fireEvent.click(await screen.findByRole('button', { name: 'Create profile' }));
  dialog = await screen.findByRole('dialog', { name: 'Create profile' });
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'new-draft' } });
  await act(async () => { resolve({ changed: true, needsSync: false, skills: null }); });
  expect(within(screen.getByRole('dialog', { name: 'Create profile' })).getByLabelText('Name')).toHaveValue('new-draft');
  expect(f.create).toHaveBeenCalledTimes(1);
  expect(window.location.pathname).toBe('/profiles');
});

it('releases an interrupted create after Back changes the route without unmounting its draft', async () => {
  const f = profileFixtures();
  let resolve!: (result: { changed: boolean; needsSync: boolean; skills: null }) => void;
  f.create.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
  await mountProfiles();
  fireEvent.click(await screen.findByRole('button', { name: 'Create profile' }));
  const dialog = await screen.findByRole('dialog', { name: 'Create profile' });
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'frontend' } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Create profile' }));
  await waitFor(() => expect(f.create).toHaveBeenCalledTimes(1));
  await act(async () => {
    window.history.replaceState({ key: 'interrupted-create-return' }, '', '/profiles/work');
    window.dispatchEvent(new PopStateEvent('popstate'));
  });
  await act(async () => { resolve({ changed: true, needsSync: false, skills: null }); });
  const draft = within(screen.getByRole('dialog', { name: 'Create profile' })).getByLabelText('Name');
  expect(draft).toHaveValue('frontend');
  expect(draft).toBeEnabled();
  expect(window.location.pathname).toBe('/profiles/work');
  fireEvent.keyDown(document, { key: 'Escape' });
  expect(screen.queryByRole('dialog', { name: 'Create profile' })).toBeNull();
});

it.each(['rename', 'delete'])('finishes %s after its catalog refresh unmounts the old editor', async (action) => {
  const f = profileFixtures();
  await mountProfiles('/profiles/work');
  await screen.findByRole('button', { name: 'Rename profile' });
  let resolveDiff!: (value: Awaited<ReturnType<typeof api.diff>>) => void;
  vi.mocked(api.diff).mockImplementationOnce(() => new Promise((done) => { resolveDiff = done; }));
  f.list.mockResolvedValue({ ...f.catalog, profiles: action === 'rename' ? [definition('renamed')] : [] });
  fireEvent.click(screen.getByRole('button', { name: action === 'rename' ? 'Rename profile' : 'Delete profile' }));
  const dialog = await screen.findByRole('dialog');
  if (action === 'rename') fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'renamed' } });
  fireEvent.click(within(dialog).getByRole('button', { name: action === 'rename' ? 'Rename profile' : 'Delete profile' }));
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Save profile' })).toBeNull());
  await waitFor(() => expect(resolveDiff).toBeTypeOf('function'));
  await act(async () => { resolveDiff({ diffs: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] }); });
  await waitFor(() => expect(window.location.pathname).toBe(action === 'rename' ? '/profiles/renamed' : '/profiles'));
  if (action === 'rename') expect(f.replace).toHaveBeenCalledExactlyOnceWith('work', { name: 'renamed' });
  else expect(f.remove).toHaveBeenCalledExactlyOnceWith('work');
});
