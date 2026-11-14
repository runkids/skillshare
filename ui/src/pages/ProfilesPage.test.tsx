import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ApiError } from '../api/client';
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
