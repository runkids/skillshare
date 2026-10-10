import { describe, expect, it } from 'vitest';
import type { Skill } from '../api/client';
import { existingFolders, isValidIntoPath } from './moveFolders';

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
    expect(existingFolders([skill('a/b/c/demo'), skill('solo')])).toEqual(['a', 'a/b', 'a/b/c']);
  });

  it('skips folders with a _-prefixed segment', () => {
    expect(existingFolders([skill('org/_scratch/demo'), skill('ok/demo')])).toEqual(['ok', 'org']);
  });

  it('skips folders the server would reject as --into', () => {
    expect(existingFolders([skill('my folder/demo'), skill('v1.2/demo'), skill('ok/demo')])).toEqual(['ok']);
  });

  it('skips a tracked repo and everything under it', () => {
    const inRepo = skill('org/_team/skills/demo', { isInRepo: true, repoPath: 'org/_team' });
    expect(existingFolders([inRepo, skill('org/own')])).toEqual(['org']);
  });

  it('skips a followed source link and everything under it', () => {
    const linked = skill('mine/sub/demo', { linkName: 'mine', linkTarget: '/elsewhere' });
    expect(existingFolders([linked, skill('plain/demo')])).toEqual(['plain']);
  });

  it('skips a folder that is itself a skill', () => {
    expect(existingFolders([skill('suite'), skill('suite/inner'), skill('grp/demo')])).toEqual(['grp']);
  });

  it('takes agent folders from agent file paths', () => {
    expect(existingFolders([skill('team/reviewer.md', { kind: 'agent' })])).toEqual(['team']);
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
