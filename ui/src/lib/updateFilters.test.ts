import { describe, expect, it } from 'vitest';
import type { Skill } from '../api/client';
import {
  compareUrl,
  foldsAway,
  hasUpstreamPreview,
  matchesFilters,
  NO_FILTERS,
  repeatedLabels,
  sourcePage,
  type FilterUnit,
} from './updateFilters';

const skill = (relPath: string, extra: Partial<Skill> = {}): Skill => ({
  name: relPath.split('/').pop()!,
  kind: 'skill',
  flatName: relPath.replace(/\//g, '__'),
  relPath,
  sourcePath: `/src/${relPath}`,
  isInRepo: false,
  type: 'github-subdir',
  ...extra,
});

const unit = (relPath: string, extra: Partial<Skill> = {}, source?: string): FilterUnit => {
  const s = skill(relPath, extra);
  return { name: relPath, label: s.name, isRepo: false, items: [s], source: source ?? s.source };
};

const repo = (name: string): FilterUnit => ({
  name,
  label: name.slice(1),
  isRepo: true,
  items: [skill(`${name}/a`, { isInRepo: true, repoUrl: 'https://github.com/acme/team.git' })],
  source: 'https://github.com/acme/team.git',
});

describe('matchesFilters', () => {
  it('finds a unit by a glob over its name', () => {
    expect(
      matchesFilters(unit('writing/pr-review'), { ...NO_FILTERS, query: 'pr-*' }, 'up-to-date'),
    ).toBe(true);
  });

  it('finds a unit by its source when the search is plain text', () => {
    const u = unit('a', {}, 'github.com/dataops/skills/a');
    expect(matchesFilters(u, { ...NO_FILTERS, query: 'dataops' }, 'up-to-date')).toBe(true);
  });

  it('counts a behind repo and an available update as the same Status group', () => {
    const f = { ...NO_FILTERS, status: 'updates' as const };
    expect([
      matchesFilters(repo('_team'), f, 'behind'),
      matchesFilters(unit('a'), f, 'update-available'),
    ]).toEqual([true, true]);
  });

  it('ignores the filter a chip counts for', () => {
    const f = { ...NO_FILTERS, source: 'tracked' as const };
    expect(matchesFilters(unit('a'), f, 'up-to-date', 'source')).toBe(true);
  });

  it('places a tracked repo in a folder of its own name', () => {
    expect(matchesFilters(repo('_team'), { ...NO_FILTERS, folder: '_team' }, 'behind')).toBe(true);
  });
});

describe('foldsAway', () => {
  it.each([
    ['an up-to-date row with nothing searched', NO_FILTERS, 'up-to-date', false, true],
    ['a row with an update', NO_FILTERS, 'update-available', false, false],
    ['a row while searching', { ...NO_FILTERS, query: 'pdf' }, 'up-to-date', false, false],
    [
      'a row under a status filter',
      { ...NO_FILTERS, status: 'current' as const },
      'up-to-date',
      false,
      false,
    ],
    ['a row an update run reports on', NO_FILTERS, 'up-to-date', true, false],
  ])('%s', (_, filters, status, inRun, expected) => {
    expect(foldsAway(filters, status, inRun)).toBe(expected);
  });
});

describe('hasUpstreamPreview', () => {
  it.each([
    [
      'a skill installed from git',
      unit('a', { repoUrl: 'https://github.com/acme/skills.git' }),
      true,
    ],
    ['a skill installed from a local path', unit('a', { type: 'local', source: '/src/a' }), false],
    ['a tracked repo', repo('_team'), true],
  ])('%s', (_, u, expected) => {
    expect(hasUpstreamPreview(u)).toBe(expected);
  });
});

describe('repeatedLabels', () => {
  it('names labels that appear in more than one folder', () => {
    expect([
      ...repeatedLabels([unit('frontend/code-review'), unit('backend/code-review'), unit('go')]),
    ]).toEqual(['code-review']);
  });
});

describe('sourcePage', () => {
  it.each([
    [
      unit('a', { branch: 'dev' }, 'github.com/acme/skills/frontend/a'),
      'https://github.com/acme/skills/tree/dev/frontend/a',
    ],
    [
      unit('a', {}, 'https://github.com/acme/skills/tree/main/a'),
      'https://github.com/acme/skills/tree/main/a',
    ],
    [unit('a', {}, 'git@github.com:acme/a.git'), 'https://github.com/acme/a'],
    [repo('_team'), 'https://github.com/acme/team'],
    [unit('a', {}, 'gitlab.com/acme/skills/a'), 'https://gitlab.com/acme/skills/-/tree/HEAD/a'],
    [
      unit('a', {}, 'https://gitlab.com/acme/skills/-/tree/main/a'),
      'https://gitlab.com/acme/skills/-/tree/main/a',
    ],
    [
      unit(
        'a',
        { repoUrl: 'https://gitlab.com/acme/team/skills.git' },
        'https://gitlab.com/acme/team/skills.git//tools/a',
      ),
      'https://gitlab.com/acme/team/skills/-/tree/HEAD/tools/a',
    ],
    [
      unit('a', { branch: 'main' }, 'bitbucket.org/acme/skills/a'),
      'https://bitbucket.org/acme/skills/src/main/a',
    ],
    [unit('a', {}, 'git.example.com/acme/skills/a'), null],
    [unit('a', { type: 'local' }, '/src/a'), null],
  ])('%#', (u, expected) => {
    expect(sourcePage(u)?.url ?? null).toBe(expected);
  });

  it('names the host the link opens', () => {
    expect(sourcePage(unit('a', {}, 'gitlab.com/acme/skills/a'))?.host).toBe('GitLab');
  });

  it('links the history of a skill in a repo whose owner is named tree', () => {
    expect(
      sourcePage(unit('a', { branch: 'main' }, 'github.com/tree/skills/a'), 'commits')?.url,
    ).toBe('https://github.com/tree/skills/commits/main/a');
  });

  it('links the history of a GitLab skill', () => {
    expect(sourcePage(unit('a', {}, 'gitlab.com/acme/skills/a'), 'commits')?.url).toBe(
      'https://gitlab.com/acme/skills/-/commits/HEAD/a',
    );
  });
});

describe('compareUrl', () => {
  it.each([
    ['https://github.com/acme/team.git', 'https://github.com/acme/team/compare/a1...b2'],
    ['https://gitlab.com/acme/team.git', 'https://gitlab.com/acme/team/-/compare/a1...b2'],
    ['https://bitbucket.org/acme/team.git', 'https://bitbucket.org/acme/team'],
  ])('%s', (url, expected) => {
    const r = { ...repo('_team'), source: url };
    r.items = [{ ...r.items[0], repoUrl: url }];
    expect(compareUrl(r, 'a1', 'b2')).toBe(expected);
  });
});
