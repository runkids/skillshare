import { describe, expect, it } from 'vitest';
import type { Skill } from '../api/client';
import { groupByFolder, groupBySource, limitGroups, descendantCount, nestSkills, sortSkills, withAncestors } from './resourceGrouping';

const skill = (relPath: string, extra: Partial<Skill> = {}): Skill => ({
  name: relPath.split('/').pop()!, kind: 'skill', flatName: relPath.replace(/\//g, '__'), relPath, sourcePath: '',
  isInRepo: false, ...extra,
});

describe('sortSkills', () => {
  it('puts skills without an install date last when sorting by newest', () => {
    const sorted = sortSkills([
      skill('undated'),
      skill('old', { installedAt: '2024-01-01T00:00:00Z' }),
      skill('new', { installedAt: '2025-01-01T00:00:00Z' }),
    ], 'newest');

    expect(sorted.map((s) => s.name)).toEqual(['new', 'old', 'undated']);
  });
});

describe('groupBySource', () => {
  it('orders tracked repos before local skills and keeps one group per repo', () => {
    const groups = groupBySource([
      skill('solo'),
      skill('_repo-b/x', { isInRepo: true, type: 'tracked' }),
      skill('_repo-a/y', { isInRepo: true, type: 'tracked' }),
      skill('_repo-a/z', { isInRepo: true, type: 'tracked' }),
    ]);

    expect(groups.map((g) => [g.key, g.items.length])).toEqual([['_repo-a', 2], ['_repo-b', 1], ['local', 1]]);
  });

  it('groups a repo installed with --into under its full repo path', () => {
    const repo = { isInRepo: true, type: 'tracked', repoPath: 'org/_repo' };
    const groups = groupBySource([skill('org/_repo/skills/x', repo), skill('org/_repo/skills/y', repo)]);

    expect(groups.map((g) => [g.key, g.items.length])).toEqual([['org/_repo', 2]]);
  });
});

describe('groupByFolder', () => {
  it('lists the root group first and keeps item order within a group', () => {
    const groups = groupByFolder([skill('work/b'), skill('root'), skill('work/a')]);

    expect(groups.map((g) => [g.key, g.items.map((s) => s.name)])).toEqual([['', ['root']], ['work', ['b', 'a']]]);
  });
});

describe('limitGroups', () => {
  it('cuts the last visible group and drops groups past the limit', () => {
    const groups = [{ items: [skill('a'), skill('b')] }, { items: [skill('c'), skill('d')] }, { items: [skill('e')] }];

    expect(limitGroups(groups, 3).map((g) => g.items.map((s) => s.name))).toEqual([['a', 'b'], ['c']]);
  });

  it('does not charge collapsed groups against the limit', () => {
    const groups = [{ key: 'x', items: [skill('a'), skill('b')] }, { key: 'y', items: [skill('c'), skill('d')] }];

    expect(limitGroups(groups, 1, (g) => g.key === 'x').map((g) => g.items.map((s) => s.name))).toEqual([['a', 'b'], ['c']]);
  });
});

describe('nestSkills', () => {
  const pack = [
    skill('visual-pack'),
    skill('visual-pack/skills/cover'),
    skill('visual-pack/skills/portrait'),
    skill('visual-pack/skills/portrait/skills/wardrobe'),
    skill('visual-pack-extra'),
  ];

  it('nests each skill under the nearest skill folder above it', () => {
    const { roots, children } = nestSkills(pack);

    expect([roots.map((s) => s.relPath), [...children].map(([k, v]) => [k, v.map((s) => s.name)])]).toEqual([
      ['visual-pack', 'visual-pack-extra'],
      [['visual-pack', ['cover', 'portrait']], ['visual-pack__skills__portrait', ['wardrobe']]],
    ]);
  });

  it('counts every descendant, not only direct children', () => {
    expect(descendantCount(pack[0], pack)).toBe(3);
  });

  it('brings back the packages that hold a match', () => {
    expect(withAncestors([pack[3]], pack).map((s) => s.name)).toEqual(['visual-pack', 'portrait', 'wardrobe']);
  });

  it('keeps a child top-level when its parent is not in the list', () => {
    expect(nestSkills(pack.slice(1)).roots.map((s) => s.name)).toEqual(['cover', 'portrait', 'visual-pack-extra']);
  });
});
