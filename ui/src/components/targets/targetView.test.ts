import { describe, expect, it } from 'vitest';
import type { SyncMatrixEntry, Target } from '../../api/client';
import { joinList, targetHealth, targetSkipHints, targetWarning, togglePatterns } from './targetView';

const target = (over: Partial<Target>): Target => ({
  name: 'claude', path: '~/.claude/skills', mode: 'merge', targetNaming: 'flat', status: 'merged',
  linkedCount: 0, localCount: 0, include: [], exclude: [], expectedSkillCount: 0, skillsEnabled: true, ...over,
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

  it('ignores the skills folder of a target with skills off, counting only its agents', () => {
    expect(targetHealth(target({ skillsEnabled: false, status: 'not exist', expectedSkillCount: 9, agentExpectedCount: 1 }))).toEqual({ state: 'pending', pending: 1 });
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

describe('joinList', () => {
  it('spaces the Chinese conjunction so it does not run into Latin names', () => {
    expect([joinList(['MCP', 'GEMINI.md'], 'zh-TW'), joinList(['MCP', '指令檔'], 'zh-TW'), joinList(['MCP', 'GEMINI.md'], 'en')])
      .toEqual(['MCP 與 GEMINI.md', 'MCP 與指令檔', 'MCP and GEMINI.md']);
  });
});

it('excludes an ambiguous physical candidate without changing other filters', () => {
  expect(togglePatterns(row('_repo__skills__codex__fix', 'skipped'), ['*fix'], ['old'])).toEqual({ include: ['*fix'], exclude: ['old', '_repo__skills__codex__fix'] });
});
it('keeps diagnostic paths while localizing selection and naming warnings separately', () => {
  expect(targetWarning('Target "codex": skipped ambiguous codex variant _repo/skills/codex/fix; use include/exclude or a target override to select one')).toEqual({ key: 'sync.skipped.ambiguousVariant', path: '_repo/skills/codex/fix' });
  expect(targetWarning("Target 'codex': skipped broken because SKILL.md name could not be read: error")).toEqual({ key: 'sync.skipped.hintNaming', path: 'broken' });
});

describe('targetSkipHints', () => {
 it('keeps ambiguity separate from naming and collisions', () => {
  expect(targetSkipHints(target({ skippedSkillCount: 5, namingSkippedSkillCount: 1, selectionSkippedSkillCount: 2, collisionCount: 1 }))).toEqual(['sync.skipped.hintMixed', 'sync.skipped.hintSelection']);
  expect(targetSkipHints(target({ targetNaming: 'standard', skippedSkillCount: 2, selectionSkippedSkillCount: 2 }))).toEqual(['sync.skipped.hintSelection']);
 });
});
