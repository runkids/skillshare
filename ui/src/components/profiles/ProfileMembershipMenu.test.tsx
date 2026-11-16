import { fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { api, ApiError, type Skill } from '../../api/client';
import { definition, mountProfiles, profileFixtures } from '../../test/profileFixtures';

const skills: Skill[] = ['team__review', 'local__alpha'].map((flatName) => ({
  name: flatName.split('__').pop()!, flatName, relPath: flatName.replaceAll('__', '/'),
  sourcePath: '/fixture/source', kind: 'skill', isInRepo: false,
}));
function resources(kind: 'skill' | 'agent' = 'skill') {
  vi.spyOn(api, 'listSkills').mockResolvedValue({ resources: skills.map((s) => ({ ...s, kind })) });
  vi.spyOn(api, 'getSyncMatrix').mockResolvedValue({ entries: [] });
  vi.spyOn(api, 'listTrash').mockResolvedValue({ items: [], totalSize: 0 } as Awaited<ReturnType<typeof api.listTrash>>);
}
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it.each([
  ['list', false, 'Add to profile…', 'add'],
  ['cards', true, 'Remove from profile…', 'remove'],
] as const)('sends exact selected flat names from %s, project=%s', async (view, project, action, operation) => {
  const f = profileFixtures(project);
  resources();
  localStorage.setItem('skillshare:skills-view', view);
  await mountProfiles('/skills');
  fireEvent.click(await screen.findByRole('checkbox', { name: 'review' }));
  fireEvent.click(screen.getByRole('checkbox', { name: 'alpha' }));
  fireEvent.click(screen.getByRole('button', { name: action }));
  expect(f.assign).not.toHaveBeenCalled();
  fireEvent.click(await screen.findByRole('menuitem', { name: 'work' }));
  await waitFor(() => expect(f.assign).toHaveBeenCalledExactlyOnceWith('work', { [operation]: ['team__review', 'local__alpha'] }));
  expect(f.sync).not.toHaveBeenCalled();
});

it('keeps the selected skill after a glob refusal and retries the same flat-name batch', async () => {
  const f = profileFixtures(false, [definition(), definition('future', { usable: false, reason: 'unknown profile key "future"' })]);
  f.assign.mockRejectedValueOnce(new ApiError(400, 'profile "work": skill "team__review" is blocked by exclude pattern "team__*"'));
  resources();
  localStorage.setItem('skillshare:skills-view', 'list');
  await mountProfiles('/skills');
  fireEvent.click(await screen.findByRole('checkbox', { name: 'review' }));
  fireEvent.click(screen.getByRole('button', { name: 'Add to profile…' }));
  const unavailable = await screen.findByRole('menuitem', { name: /future/ });
  fireEvent.click(unavailable);
  expect(f.assign).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('menuitem', { name: 'work' }));
  await waitFor(() => expect(f.assign).toHaveBeenCalledExactlyOnceWith('work', { add: ['team__review'] }));
  expect(await screen.findByText(/team__review.*team__\*/)).toBeInTheDocument();
  expect(screen.getByRole('checkbox', { name: 'review' })).toBeChecked();
  fireEvent.click(screen.getByRole('button', { name: 'Add to profile…' }));
  fireEvent.click(await screen.findByRole('menuitem', { name: 'work' }));
  await waitFor(() => expect(f.assign).toHaveBeenCalledTimes(2));
  expect(f.assign.mock.calls[1]).toEqual(f.assign.mock.calls[0]);
  expect(f.sync).not.toHaveBeenCalled();
});

it('never offers profile membership writes for selected agents', async () => {
  const f = profileFixtures();
  resources('agent');
  localStorage.setItem('skillshare:skills-view', 'list');
  await mountProfiles('/agents');
  fireEvent.click(await screen.findByRole('checkbox', { name: 'review' }));
  expect(screen.queryByRole('button', { name: 'Add to profile…' })).toBeNull();
  expect(screen.queryByRole('button', { name: 'Remove from profile…' })).toBeNull();
  expect(f.assign).not.toHaveBeenCalled();
});
