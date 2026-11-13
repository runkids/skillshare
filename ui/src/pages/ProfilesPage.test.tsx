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

it('keeps an unusable definition from being overwritten through the list', async () => {
  const f = profileFixtures(false, [definition(), definition('future', { usable: false, reason: 'unknown profile key "future"' })]);
  await mountProfiles();
  expect(await screen.findByRole('link', { name: 'work' })).toHaveAttribute('href', '/profiles/work');
  expect(screen.queryByRole('link', { name: 'future' })).toBeNull();
  expect(f.replace).not.toHaveBeenCalled();
  expect(f.remove).not.toHaveBeenCalled();
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
