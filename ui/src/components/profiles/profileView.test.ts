import { expect, it } from 'vitest';
import { ApiError, type ProfilesResponse } from '../../api/client';
import { blockedEntries, profileOptions, profileVisible, profileGroups, profileReason, profileError } from './profileView';

const none: ProfilesResponse = { active: '', state: 'none', pending: false, profiles: [] };
it('hides an unused picker, keeps unavailable state repairable, and orders unusable choices', () => {
  expect(profileVisible(none)).toBe(false);
  expect(profileVisible({ ...none, active: 'lost', state: 'unavailable' })).toBe(true);
  const data = { ...none, profiles: ['z', 'a'].map((name) => ({ name, description: name, skills: null, selected: 2, unmatched: [], usable: name !== 'z', reason: 'future key' })) };
  expect(profileOptions(data, (key) => key).map((o) => [o.value, o.disabled])).toEqual([['', undefined], ['a', false], ['z', true]]);
});

it('reads only the typed blocked refusal envelope and never turns malformed details into consent', () => {
  const row = { target: 'claude', entry: 'a', step: 'prune', class: 'modified_copy', reason: 'edited' };
  expect(blockedEntries(new ApiError(409, 'x', { code: 'profile_blocked', params: { blocked: JSON.stringify([row]) } }))).toEqual([row]);
  for (const blocked of ['bad JSON', '{}', '[{}]']) {
    expect(blockedEntries(new ApiError(409, 'x', { code: 'profile_blocked', params: { blocked } }))).toEqual([]);
  }
  expect(blockedEntries(new Error('profile_blocked'))).toEqual([]);
});

it('uses sync row shapes without losing refreshed, converted or retained entries', () => {
  const groups = profileGroups([{ name: 'claude', state: 'incomplete', added: ['a'], removed: ['b'], refreshed: ['c'], converted: ['d'], kept_modified: [{ target: 'claude', entry: 'e', class: 'modified_copy', reason: 'edited', step: 'prune' }] }]);
  expect(groups[0].rows.map((r) => [r.name, r.icon, r.counts])).toEqual([
    ['a', 'add', true], ['c', 'update', true], ['b', 'remove', true], ['d', 'update', true], ['e', 'kept', false],
  ]);
});

it('localizes known display reasons while keeping unknown diagnostics verbatim', () => {
  const t = (key: string) => key;
  expect(profileReason('copy was edited or could not be verified', t)).toBe('profiles.edited');
  expect(profileReason('skills.include must be a list of strings', t)).toBe('profiles.listStrings');
  expect(profileReason('future diagnostic', t)).toBe('future diagnostic');
  expect(profileError(new ApiError(409, 'English lock message', { code: 'profile_locked' }), t)).toBe('profiles.locked');
  expect(profileError(new ApiError(409, 'snapshot EIO', { code: 'profile_refused' }), t)).toContain('snapshot EIO');
});
