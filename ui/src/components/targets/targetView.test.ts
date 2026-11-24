import { describe, expect, it } from 'vitest';
import type { SyncMatrixEntry, Target } from '../../api/client';
import {
  joinList,
  patternMatches,
  previewChange,
  previewReason,
  rowAction,
  searchEntries,
  targetHealth,
  targetSkipHints,
  targetWarning,
  togglePatterns,
} from './targetView';

const target = (over: Partial<Target>): Target => ({
  name: 'claude',
  path: '~/.claude/skills',
  mode: 'merge',
  targetNaming: 'flat',
  status: 'merged',
  linkedCount: 0,
  localCount: 0,
  include: [],
  exclude: [],
  expectedSkillCount: 0,
  skillsEnabled: true,
  ...over,
});
const row = (
  skill: string,
  status: SyncMatrixEntry['status'],
  kind?: 'agent',
): SyncMatrixEntry => ({ skill, target: 'claude', status, reason: '', kind });

describe('targetHealth', () => {
  it('counts skills and agents the next sync would add', () => {
    expect(
      targetHealth(
        target({
          linkedCount: 45,
          expectedSkillCount: 48,
          agentLinkedCount: 1,
          agentExpectedCount: 2,
        }),
      ),
    ).toEqual({ state: 'pending', pending: 4 });
  });

  it('reports a folder that has not been created yet separately from a problem', () => {
    expect([
      targetHealth(target({ status: 'not exist' })).state,
      targetHealth(target({ status: 'conflict' })).state,
    ]).toEqual(['missing', 'problem']);
  });

  it('reads "has files" as a problem in no mode: nothing linked yet for merge, files to move for symlink', () => {
    const states = [{}, { expectedSkillCount: 2 }, { mode: 'symlink' }].map(
      (over) => targetHealth(target({ status: 'has files', ...over })).state,
    );
    expect(states).toEqual(['synced', 'pending', 'migrate']);
  });

  it('ignores the skills folder of a target with skills off, counting only its agents', () => {
    expect(
      targetHealth(
        target({
          skillsEnabled: false,
          status: 'not exist',
          expectedSkillCount: 9,
          agentExpectedCount: 1,
        }),
      ),
    ).toEqual({ state: 'pending', pending: 1 });
  });
});

describe('previewReason', () => {
  const t = ((key: string, vars?: object) =>
    vars ? `${key} ${JSON.stringify(vars)}` : key) as never;
  it('credits a synced row to the include filter only when one is set', () => {
    expect([true, false].map((f) => previewReason(t, row('a', 'synced'), f))).toEqual([
      'targetDetail.reason.included',
      'targetDetail.reason.noFilter',
    ]);
  });
  it('names the exclude pattern that skipped a row', () => {
    expect(previewReason(t, { ...row('a', 'excluded'), reason: 'old-*' }, true)).toBe(
      'targetDetail.reason.excluded {"pattern":"old-*"}',
    );
  });
});

describe('searchEntries', () => {
  it('matches part of a name in any case, and an agent by its name without the extension', () => {
    const rows = [
      row('frontend/React-Doctor', 'synced'),
      row('react.md', 'synced', 'agent'),
      row('archify', 'excluded'),
    ];
    expect(searchEntries(rows, ' react ').map((e) => e.skill)).toEqual([
      'frontend/React-Doctor',
      'react.md',
    ]);
    expect(searchEntries(rows, '.md')).toEqual([]);
  });
});

describe('togglePatterns', () => {
  it('round-trips a row that only syncs because of its own include', () => {
    const on = togglePatterns(row('scratch', 'not_included'), ['core-*'], []);
    expect(on).toEqual({ include: ['core-*', 'scratch'], exclude: [] });
    expect(togglePatterns(row('scratch', 'synced'), on!.include, on!.exclude)).toEqual({
      include: ['core-*'],
      exclude: [],
    });
  });

  it('excludes an agent by its name without the extension', () => {
    expect(togglePatterns(row('ux-critic.md', 'synced', 'agent'), [], [])).toEqual({
      include: [],
      exclude: ['ux-critic'],
    });
  });

  it('cannot re-include a row a wildcard excludes', () => {
    expect(togglePatterns(row('core-deprecated', 'excluded'), [], ['*-deprecated'])).toBeNull();
  });
});

describe('patternMatches', () => {
  it('matches * within a name, as the server does', () => {
    expect(patternMatches('*-experimental', 'search-experimental')).toBe(true);
  });

  it('does not let * cross a slash', () => {
    expect(patternMatches('team*', 'team/ui')).toBe(false);
  });

  it('matches ? as exactly one character', () => {
    expect(['ab', 'abc'].map((n) => patternMatches('a?', n))).toEqual([true, false]);
  });

  it('matches a character class and its negation', () => {
    expect(
      ['api', 'cli'].map((n) => [patternMatches('[a-b]*', n), patternMatches('[^a]*', n)]),
    ).toEqual([
      [true, false],
      [false, true],
    ]);
  });

  it('takes dots and other regex characters literally', () => {
    expect(patternMatches('a.b', 'axb')).toBe(false);
  });

  it('matches nothing for a malformed pattern', () => {
    expect(patternMatches('[abc', '[abc')).toBe(false);
  });
});

describe('rowAction', () => {
  it('offers to exclude a synced row by name', () => {
    expect(rowAction(row('api', 'synced'), [], [])).toEqual({
      action: 'exclude',
      next: { include: [], exclude: ['api'] },
    });
  });

  it('offers to include a row no include matches', () => {
    expect(rowAction(row('api', 'not_included'), ['web-*'], [])?.action).toBe('include');
  });

  it('offers to drop an exclude that names the row', () => {
    expect(rowAction(row('api', 'excluded'), [], ['api'])?.action).toBe('unexclude');
  });

  it('offers nothing for a row a wildcard excludes', () => {
    expect(rowAction(row('api-x', 'excluded'), [], ['api-*'])).toBeNull();
  });
});

describe('previewChange', () => {
  it('counts the rows a rule change skipped and synced', () => {
    expect(
      previewChange(
        [row('a', 'synced'), row('b', 'excluded'), row('c', 'synced')],
        [row('a', 'excluded'), row('b', 'synced'), row('c', 'synced')],
      ),
    ).toEqual({ changed: ['a', 'b'], synced: 1, skipped: 1 });
  });

  it('ignores rows only one preview has', () => {
    expect(previewChange([], [row('a', 'synced')]).changed).toEqual([]);
  });
});

describe('joinList', () => {
  it('spaces the Chinese conjunction so it does not run into Latin names', () => {
    expect([
      joinList(['MCP', 'GEMINI.md'], 'zh-TW'),
      joinList(['MCP', '指令檔'], 'zh-TW'),
      joinList(['MCP', 'GEMINI.md'], 'en'),
    ]).toEqual(['MCP 與 GEMINI.md', 'MCP 與指令檔', 'MCP and GEMINI.md']);
  });
});

it('excludes an ambiguous physical candidate without changing other filters', () => {
  expect(togglePatterns(row('_repo__skills__codex__fix', 'skipped'), ['*fix'], ['old'])).toEqual({
    include: ['*fix'],
    exclude: ['old', '_repo__skills__codex__fix'],
  });
});
it('keeps diagnostic paths while localizing selection and naming warnings separately', () => {
  expect(
    targetWarning(
      'Target "codex": skipped ambiguous codex variant _repo/skills/codex/fix; use include/exclude or a target override to select one',
    ),
  ).toEqual({ key: 'sync.skipped.ambiguousVariant', path: '_repo/skills/codex/fix' });
  expect(
    targetWarning("Target 'codex': skipped broken because SKILL.md name could not be read: error"),
  ).toEqual({ key: 'sync.skipped.hintNaming', path: 'broken' });
});

describe('targetSkipHints', () => {
  it('keeps ambiguity separate from naming and collisions', () => {
    expect(
      targetSkipHints(
        target({
          skippedSkillCount: 5,
          namingSkippedSkillCount: 1,
          selectionSkippedSkillCount: 2,
          collisionCount: 1,
        }),
      ),
    ).toEqual(['sync.skipped.hintMixed', 'sync.skipped.hintSelection']);
    expect(
      targetSkipHints(
        target({ targetNaming: 'standard', skippedSkillCount: 2, selectionSkippedSkillCount: 2 }),
      ),
    ).toEqual(['sync.skipped.hintSelection']);
  });
});
