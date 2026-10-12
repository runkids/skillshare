import { expect, it } from 'vitest';
import type { MemoryNote } from '../../api/memory';
import { updatedNotes } from './memoryView';

it('sorts real notes by time, omits untouched starters and unsupported files, and preserves the query cache', () => {
  const notes: MemoryNote[] = [
    { path: 'older.md', title: 'Older', version: 'a', modified_at: '2026-10-10T00:00:00Z' },
    { path: 'INDEX.md', title: 'Shared memory', version: 'b', starter: true, modified_at: '2026-10-12T00:00:00Z' },
    { path: 'wiki/new note.md', title: 'Newer', version: 'c', modified_at: '2026-10-11T00:00:00Z' },
    { path: 'invalid.md', title: 'Invalid', version: '', invalid: 'Unsupported' },
    { path: 'edited-index.md', title: 'Edited', version: 'd', modified_at: '2026-10-11T00:00:00.123Z' },
  ];
  const snapshot = [...notes];
  expect(updatedNotes(notes).map((note) => note.path)).toEqual(['edited-index.md', 'wiki/new note.md', 'older.md']);
  expect(notes).toEqual(snapshot);
});
