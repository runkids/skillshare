import { expect, it } from 'vitest';
import { ApiError, type ProfilesResponse } from '../../api/client';
import type { TranslationParams } from '../../i18n';
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

const translated = (key: string, params?: TranslationParams) => JSON.stringify({ key, params });
it('preserves the complete path when translating the profile target-root diagnostic', () => {
  const path = '/fixture/技能 folder';
  expect(profileReason(`target root ${path} is an unmanaged link; kept`, translated)).toBe(translated('profiles.targetRoot', { path }));
});

it.each(['refresh', 'prune', 'convert'])('localizes profile retention warnings during %s without dropping their identifiers', (step) => {
  for (const [kind, key, suffix] of [['modified', 'profiles.keptModified', '; use collect or --discard-edits'], ['unmanaged', 'profiles.keptUnmanaged', '']]) {
    const warning = `custom: kept ${kind} team__技能 during ${step}: copy was edited or could not be verified${suffix}`;
    expect(profileReason(warning, translated)).toBe(translated(key, {
      target: 'custom', entry: 'team__技能', step: translated(`profiles.step.${step}`), reason: translated('profiles.edited'),
    }));
  }
});

it('keeps external or unrecognized diagnostics intact inside and outside known wrappers', () => {
  for (const reason of ['EACCES: /private', 'target root failed: EIO', 'custom: kept modified a during future: EIO; use collect or --discard-edits', 'custom: kept modified a during prune: EIO']) {
    expect(profileReason(reason, translated)).toBe(reason);
  }
  expect(profileReason('custom: kept unmanaged a during prune: external EIO: detail', translated)).toBe(translated('profiles.keptUnmanaged', {
    target: 'custom', entry: 'a', step: translated('profiles.step.prune'), reason: 'external EIO: detail',
  }));
});
