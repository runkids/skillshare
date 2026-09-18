import { describe, expect, it } from 'vitest';
import type { Skill, SyncMatrixEntry } from '../api/client';
import { syncedByTarget } from './ResourcesPage';

const skill = (flatName: string): Skill => ({
  name: flatName, kind: 'skill', flatName, relPath: flatName, sourcePath: '', isInRepo: false,
});

const entry = (s: string, target: string, status: SyncMatrixEntry['status']): SyncMatrixEntry =>
  ({ skill: s, target, status, reason: '' });

describe('syncedByTarget', () => {
  it('indexes only entries that are synced and belong to the given items', () => {
    const index = syncedByTarget([skill('a'), skill('b')], [
      entry('a', 'claude', 'synced'),
      entry('b', 'claude', 'synced'),
      entry('a', 'cursor', 'not_included'),
      entry('gone', 'codex', 'synced'),
    ]);

    expect([...index.keys()]).toEqual(['claude']);
    expect([...index.get('claude')!]).toEqual(['a', 'b']);
  });
});
