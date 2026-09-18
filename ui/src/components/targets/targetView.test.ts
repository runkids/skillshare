import { describe, expect, it } from 'vitest';
import type { SyncMatrixEntry, Target } from '../../api/client';
import { targetHealth, togglePatterns } from './targetView';

const target = (over: Partial<Target>): Target => ({
  name: 'claude', path: '~/.claude/skills', mode: 'merge', targetNaming: 'flat', status: 'merged',
  linkedCount: 0, localCount: 0, include: [], exclude: [], expectedSkillCount: 0, ...over,
});
const row = (skill: string, status: SyncMatrixEntry['status'], kind?: 'agent'): SyncMatrixEntry => ({ skill, target: 'claude', status, reason: '', kind });

describe('targetHealth', () => {
  it('counts skills and agents the next sync would add', () => {
    expect(targetHealth(target({ linkedCount: 45, expectedSkillCount: 48, agentLinkedCount: 1, agentExpectedCount: 2 }))).toEqual({ state: 'pending', pending: 4 });
  });

  it('reports a folder that has not been created yet separately from a problem', () => {
    expect([targetHealth(target({ status: 'not exist' })).state, targetHealth(target({ status: 'conflict' })).state]).toEqual(['missing', 'problem']);
  });

  it('reads "has files" as a problem in no mode: nothing linked yet for merge, files to move for symlink', () => {
    const states = [{}, { expectedSkillCount: 2 }, { mode: 'symlink' }].map((over) => targetHealth(target({ status: 'has files', ...over })).state);
    expect(states).toEqual(['synced', 'pending', 'migrate']);
  });
});

describe('togglePatterns', () => {
  it('round-trips a row that only syncs because of its own include', () => {
    const on = togglePatterns(row('scratch', 'not_included'), ['core-*'], []);
    expect(on).toEqual({ include: ['core-*', 'scratch'], exclude: [] });
    expect(togglePatterns(row('scratch', 'synced'), on!.include, on!.exclude)).toEqual({ include: ['core-*'], exclude: [] });
  });

  it('excludes an agent by its name without the extension', () => {
    expect(togglePatterns(row('ux-critic.md', 'synced', 'agent'), [], [])).toEqual({ include: [], exclude: ['ux-critic'] });
  });

  it('cannot re-include a row a wildcard excludes', () => {
    expect(togglePatterns(row('core-deprecated', 'excluded'), [], ['*-deprecated'])).toBeNull();
  });
});
