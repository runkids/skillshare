import { describe, expect, it } from 'vitest';
import type { Skill } from '../api/client';
import { folderOptions } from './folderOptions';
import { canMove, existingFolders, isValidFolderName, isValidIntoPath, linkPrefix, newFolderPath, canMoveFolder, planFolder, skillNameFromSource } from './moveFolders';

const paths = (resources: Skill[]) => existingFolders(resources).map((f) => f.path);

const skill = (relPath: string, extra: Partial<Skill> = {}): Skill => ({
  name: relPath.split('/').pop()!,
  kind: 'skill',
  flatName: relPath.replace(/\//g, '__'),
  relPath,
  sourcePath: `/src/${relPath}`,
  isInRepo: false,
  ...extra,
});

describe('existingFolders', () => {
  it('lists every ancestor folder, not only the first segment', () => {
    expect(paths([skill('a/b/c/demo'), skill('solo')])).toEqual(['a', 'a/b', 'a/b/c']);
  });

  it('skips folders with a _-prefixed segment', () => {
    expect(paths([skill('org/_scratch/demo'), skill('ok/demo')])).toEqual(['ok', 'org']);
  });

  it('skips folders the server would reject as --into', () => {
    expect(paths([skill('my folder/demo'), skill('v1.2/demo'), skill('ok/demo')])).toEqual(['ok']);
  });

  it('skips a tracked repo and everything under it', () => {
    const inRepo = skill('org/_team/skills/demo', { isInRepo: true, repoPath: 'org/_team' });
    expect(paths([inRepo, skill('org/own')])).toEqual(['org']);
  });

  it('skips a followed source link and everything under it', () => {
    const linked = skill('mine/sub/demo', { linkName: 'mine', linkTarget: '/elsewhere' });
    expect(paths([linked, skill('plain/demo')])).toEqual(['plain']);
  });

  it('skips a folder that is itself a skill', () => {
    expect(paths([skill('suite'), skill('suite/inner'), skill('grp/demo')])).toEqual(['grp']);
  });

  it('counts the skills under each folder, subfolders included', () => {
    const found = existingFolders([skill('a/one'), skill('a/b/two'), skill('a/b/three')]);
    expect(found).toEqual([{ path: 'a', count: 3 }, { path: 'a/b', count: 2 }]);
  });

  it('takes agent folders from agent file paths', () => {
    expect(paths([skill('team/reviewer.md', { kind: 'agent' })])).toEqual(['team']);
  });
});

describe('isValidIntoPath', () => {
  it('accepts nested names', () => {
    expect(isValidIntoPath('frontend/forms_v2')).toBe(true);
  });

  it.each(['', '/abs', 'a//b', 'a/../b', '.', '_hidden', 'has space', 'Agents', 'a'.repeat(65), `${'ab/'.repeat(86)}x`])(
    'rejects %j',
    (path) => expect(isValidIntoPath(path)).toBe(false),
  );
});

describe('newFolderPath', () => {
  it('puts the name inside its parent', () => {
    expect(newFolderPath('archive', 'office')).toBe('archive/office');
  });

  it('puts the name at the root when the parent is the root', () => {
    expect(newFolderPath('', 'office')).toBe('office');
  });

  it.each(['', '_office', '.office', 'a/b', 'has space', 'agents'])('rejects the name %j', (name) => {
    expect(newFolderPath('archive', name)).toBeNull();
    expect(isValidFolderName(name)).toBe(false);
  });

  it('rejects a path that grows past the limit', () => {
    expect(newFolderPath('a'.repeat(60) + '/' + 'b'.repeat(60) + '/' + 'c'.repeat(60) + '/' + 'd'.repeat(60), 'e'.repeat(30))).toBeNull();
  });
});

describe('linkPrefix', () => {
  it('replaces / with __ and ends with __', () => {
    expect(linkPrefix('archive/office')).toBe('archive__office__');
  });

  it('is empty at the root', () => {
    expect(linkPrefix('')).toBe('');
  });
});

describe('skillNameFromSource', () => {
  it.each([
    ['github.com/anthropics/skills/pdf-tools', 'pdf-tools'],
    ['anthropics/skills/pdf', 'pdf'],
    ['https://github.com/anthropics/skills/pdf.git', 'pdf'],
    ['~/Downloads/my-skill', 'my-skill'],
    ['/abs/path/my-skill/', 'my-skill'],
  ])('reads %s', (source, name) => expect(skillNameFromSource(source)).toBe(name));

  it.each(['anthropics/skills', 'https://github.com/anthropics/skills', 'git@github.com:a/b', 'github.com/o/r/tree/main/x'])(
    'finds no skill in %s',
    (source) => expect(skillNameFromSource(source)).toBeNull(),
  );
});

describe('folderOptions', () => {
  const t = ((key: string) => key) as Parameters<typeof folderOptions>[0];
  const folders = [{ path: 'a', count: 2 }, { path: 'a/b', count: 1 }, { path: 'ab', count: 1 }];

  it('blocks a disabled folder and the folders below it, not a sibling that shares the prefix', () => {
    const blocked = folderOptions(t, 'skill', folders, 4, ['a']).filter((o) => o.disabled).map((o) => o.value);
    expect(blocked).toEqual(['a', 'a/b']);
  });

  it('blocks only the root when the root is disabled', () => {
    const blocked = folderOptions(t, 'skill', folders, 4, ['']).filter((o) => o.disabled).map((o) => o.value);
    expect(blocked).toEqual(['']);
  });
});

describe('canMove', () => {
  it('takes a plain skill', () => {
    expect(canMove(skill('a/demo'))).toBe(true);
  });

  it.each([
    ['an agent', skill('a/x.md', { kind: 'agent' })],
    ['a skill in a tracked repo', skill('_team/demo', { isInRepo: true, repoPath: '_team' })],
    ['a skill below a followed link', skill('mine/demo', { linkName: 'mine', linkTarget: '/elsewhere' })],
  ])('refuses %s', (_, s) => expect(canMove(s)).toBe(false));
});

describe('canMoveFolder', () => {
  it('takes a plain folder', () => {
    expect(canMoveFolder('frontend/svelte')).toBe(true);
  });

  it.each([
    ['the root', '', {}],
    ['a tracked repo root', 'org/_team', { repo: true }],
    ['a followed link', 'mine', { link: {} }],
    ['a folder inside a tracked repo', '_team/plugins', {}],
  ])('refuses %s', (_, path, node) => expect(canMoveFolder(path, node)).toBe(false));

  it('refuses a folder below a followed link', () => {
    expect(canMoveFolder('mine/sub', {}, ['mine'])).toBe(false);
    expect(canMoveFolder('mine2/sub', {}, ['mine'])).toBe(true);
  });
});

describe('planFolder', () => {
  const resources = [
    skill('frontend/pdf'),
    skill('frontend/docx'),
    skill('frontend/svelte/one'),
    skill('frontend/svelte/two'),
    skill('frontend/_acme/skills/x', { isInRepo: true, repoPath: 'frontend/_acme' }),
    skill('other/elsewhere'),
  ];

  it('lists direct skills, first-level subfolders and the total', () => {
    const plan = planFolder(resources, 'frontend');
    expect(plan.direct.map((s) => s.relPath)).toEqual(['frontend/pdf', 'frontend/docx']);
    expect(plan.subfolders).toEqual([{ name: 'svelte', count: 2 }]);
    expect(plan.count).toBe(4);
  });

  it('names a tracked repo inside the folder as blocked, relative to the folder', () => {
    expect(planFolder(resources, 'frontend').blocked).toEqual([{ path: '_acme', why: 'repo' }]);
  });

  it('counts a skill at the folder root with what is below it', () => {
    const plan = planFolder([skill('suite'), skill('suite/inner')], 'suite');
    expect(plan.count).toBe(2);
    expect(plan.direct.map((s) => s.relPath)).toEqual(['suite', 'suite/inner']);
  });

  it('has no blockers for a plain folder', () => {
    expect(planFolder(resources, 'other').blocked).toEqual([]);
  });
});
