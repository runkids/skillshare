import { describe, expect, it } from 'vitest';
import type { Skill } from '../api/client';
import { folderOptions } from './folderOptions';
import { existingFolders, isValidFolderName, isValidIntoPath, linkPrefix, newFolderPath, skillNameFromSource } from './moveFolders';

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
