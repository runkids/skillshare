import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { definition, mountProfiles, profileFixtures } from '../../test/profileFixtures';

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it.each([false, true])('keeps a failed dirty pass-through edit for an exact retry, project=%s', async (project) => {
  const f = profileFixtures(project, [definition('work', { skills: null })]);
  f.replace.mockRejectedValueOnce(new Error('disk full'));
  await mountProfiles('/profiles/work');
  const description = await screen.findByLabelText('Description');
  const save = screen.getByRole('button', { name: 'Save profile' });
  expect(save).toBeDisabled();
  fireEvent.change(description, { target: { value: 'New focus' } });
  fireEvent.click(save);
  await waitFor(() => expect(f.replace).toHaveBeenCalledExactlyOnceWith('work', { description: 'New focus' }));
  expect(await screen.findByText('disk full')).toBeInTheDocument();
  expect(description).toHaveValue('New focus');
  expect(save).toBeEnabled();
  expect(f.sync).not.toHaveBeenCalled();
  fireEvent.click(save);
  await waitFor(() => expect(f.replace).toHaveBeenCalledTimes(2));
  expect(f.replace.mock.calls[1]).toEqual(f.replace.mock.calls[0]);
  await waitFor(() => expect(save).toBeDisabled());
});

it('writes explicit empty selection when pass-through is deliberately restricted', async () => {
  const f = profileFixtures(false, [definition('work', { skills: null })]);
  await mountProfiles('/profiles/work');
  fireEvent.click(await screen.findByRole('checkbox', { name: 'Select specific skills' }));
  fireEvent.click(screen.getByRole('button', { name: 'Save profile' }));
  await waitFor(() => expect(f.replace).toHaveBeenCalledExactlyOnceWith('work', { description: 'Focus', skills: { include: [], exclude: [] } }));
  expect(f.sync).not.toHaveBeenCalled();
});

it('debounces previews and refuses to replace the current selected list with a stale response', async () => {
  const f = profileFixtures();
  await mountProfiles('/profiles/work');
  const include = await screen.findByLabelText('Include patterns');
  await waitFor(() => expect(f.preview).toHaveBeenCalled());
  f.preview.mockClear();
  let resolveOld!: (value: { selected: string[]; unmatched: string[] }) => void;
  let resolveNew!: typeof resolveOld;
  f.preview.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve; }))
    .mockImplementationOnce(() => new Promise((resolve) => { resolveNew = resolve; }));
  vi.useFakeTimers();
  fireEvent.change(include, { target: { value: 'beta' } });
  fireEvent.keyDown(include, { key: 'Enter' });
  await act(async () => { await vi.advanceTimersByTimeAsync(249); });
  expect(f.preview).not.toHaveBeenCalled();
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  expect(f.preview).toHaveBeenCalledExactlyOnceWith({ skills: { include: ['alpha', 'beta'], exclude: [] } });
  fireEvent.change(include, { target: { value: 'gamma' } });
  fireEvent.keyDown(include, { key: 'Enter' });
  await act(async () => { await vi.advanceTimersByTimeAsync(250); });
  expect(f.preview).toHaveBeenLastCalledWith({ skills: { include: ['alpha', 'beta', 'gamma'], exclude: [] } });
  await act(async () => { resolveNew({ selected: ['current-result'], unmatched: ['missing-*'] }); });
  await act(async () => { resolveOld({ selected: ['stale-result'], unmatched: [] }); });
  const selected = screen.getByRole('region', { name: 'Selected skills' });
  expect(within(selected).getByText('current-result')).toBeInTheDocument();
  expect(within(selected).queryByText('stale-result')).toBeNull();
  vi.useRealTimers();
  fireEvent.click(screen.getByRole('button', { name: 'Save profile' }));
  await waitFor(() => expect(f.replace).toHaveBeenCalledExactlyOnceWith('work', {
    description: 'Focus', skills: { include: ['alpha', 'beta', 'gamma'], exclude: [] },
  }));
});

it.each([
  ['none', 'invalid profile name "none": use lowercase letters, digits, underscores or hyphens (1–64 characters); none is reserved', 'Use 1–64 lowercase letters, digits, underscores or hyphens; none is reserved.'],
  ['focus', 'profile "focus" already exists', 'Profile focus already exists.'],
])('keeps a refused rename to %s for an exact retry without replacing selection or activating', async (candidate, message, diagnostic) => {
  const f = profileFixtures();
  await mountProfiles('/profiles/work');
  fireEvent.click(await screen.findByRole('button', { name: 'Rename profile' }));
  const dialog = await screen.findByRole('dialog', { name: 'Rename profile' });
  f.replace.mockRejectedValueOnce(new Error(message));
  fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: candidate } });
  expect(f.replace).not.toHaveBeenCalled();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Rename profile' }));
  await waitFor(() => expect(f.replace).toHaveBeenCalledExactlyOnceWith('work', { name: candidate }));
  expect(await within(dialog).findByText(diagnostic)).toBeInTheDocument();
  expect(within(dialog).getByLabelText('Name')).toHaveValue(candidate);
  fireEvent.click(within(dialog).getByRole('button', { name: 'Rename profile' }));
  await waitFor(() => expect(f.replace).toHaveBeenCalledTimes(2));
  expect(f.replace.mock.calls[1]).toEqual(f.replace.mock.calls[0]);
  expect(f.sync).not.toHaveBeenCalled();
});

it('refuses active deletion and applies an active edit only after an explicit skills sync', async () => {
  const f = profileFixtures(true);
  Object.assign(f.catalog, { active: 'work', state: 'active' });
  f.replace.mockResolvedValue({ changed: true, needsSync: true, skills: null });
  await mountProfiles('/profiles/work');
  expect(await screen.findByRole('button', { name: 'Delete profile' })).toBeDisabled();
  fireEvent.click(screen.getByRole('button', { name: 'Delete profile' }));
  expect(f.remove).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Active focus' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save profile' }));
  await waitFor(() => expect(f.replace).toHaveBeenCalledExactlyOnceWith('work', {
    description: 'Active focus', skills: { include: ['alpha'], exclude: [] },
  }));
  expect(f.sync).not.toHaveBeenCalled();
  fireEvent.click(await screen.findByRole('button', { name: 'Sync now' }));
  const dialog = await screen.findByRole('dialog');
  expect(f.sync).not.toHaveBeenCalled();
  fireEvent.click(await within(dialog).findByRole('button', { name: 'Sync Now' }));
  await waitFor(() => expect(f.sync).toHaveBeenCalledExactlyOnceWith({ dryRun: false, kind: 'skill' }));
});
