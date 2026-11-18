import { describe, expect, it } from 'vitest';
import type { ProjectScanRow } from '../../api/client';
import { scanCounts, scanNote } from './scanView';

const row = (path: string, name: string, result: ProjectScanRow['result'], skip?: string) => ({
  root: `/w/suite/${path}`,
  path,
  name,
  result,
  skip,
});

describe('scan review', () => {
  it('counts each result', () => {
    const rows = [row('.', 'suite', 'added'), row('a', 'a', 'kept'), row('b', 'b', 'skipped')];
    expect(scanCounts(rows)).toEqual({ added: 1, kept: 1, skipped: 1 });
  });

  it('explains a checkout named from its path', () => {
    expect(scanNote(row('.worktrees/app-a', 'worktrees-app-a', 'added'))).toEqual({
      key: 'projects.scan.renamed',
      params: { base: 'app-a' },
    });
  });

  it('says nothing about a checkout that keeps its folder name', () => {
    expect(scanNote(row('app-a', 'app-a', 'added'))).toBeNull();
  });

  it('words a known skip code', () => {
    expect(scanNote(row('app-c', 'app-c', 'skipped', 'different'))?.key).toBe(
      'projects.scan.skip.different',
    );
  });

  it('leaves an unknown skip code to the server reason', () => {
    expect(scanNote(row('app-c', 'app-c', 'skipped', 'future-code'))).toBeNull();
  });
});
