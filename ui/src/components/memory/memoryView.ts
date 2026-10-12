import type { MemoryNote } from '../../api/memory';

export function updatedNotes(notes: MemoryNote[]) {
  return notes.filter((note) => !note.invalid && !note.starter).sort((a, b) =>
    (Date.parse(b.modified_at ?? '') || 0) - (Date.parse(a.modified_at ?? '') || 0) || a.path.localeCompare(b.path));
}
