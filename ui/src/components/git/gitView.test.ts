import { describe, expect, it } from 'vitest';
import { conflictMarks, parseStatusLine } from './gitView';

describe('parseStatusLine', () => {
  it('reads untracked, modified, deleted and renamed lines', () => {
    expect([' M pdf/SKILL.md', 'M pdf/SKILL.md', '?? notes/SKILL.md', ' D old/SKILL.md', 'R  a.md -> b.md'].map(parseStatusLine)).toEqual([
      { path: 'pdf/SKILL.md', change: 'Changed' },
      { path: 'pdf/SKILL.md', change: 'Changed' },
      { path: 'notes/SKILL.md', change: 'New' },
      { path: 'old/SKILL.md', change: 'Deleted' },
      { path: 'b.md', change: 'Renamed' },
    ]);
  });
});

describe('conflictMarks', () => {
  it('marks the lines only one version has', () => {
    expect(conflictMarks('a\nversion: 1\nb\nc\n', 'a\nversion: 2\nb\nhooks\nc\n')).toEqual({ local: [2], remote: [2, 4] });
  });

  it('skips marking when the differing lines are too many to compare', () => {
    expect(conflictMarks('a\n'.repeat(1001), 'b\n'.repeat(1001))).toBeUndefined();
  });
});
