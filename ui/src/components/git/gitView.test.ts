import { describe, expect, it } from 'vitest';
import { parseStatusLine } from './gitView';

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
