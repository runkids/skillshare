import { diffLines } from '../skill-editor/DiffView';

export type FileChange = 'New' | 'Changed' | 'Deleted' | 'Renamed';

/** Reads one `git status --short` line. The server may trim the leading space of an unstaged change. */
export function parseStatusLine(line: string): { path: string; change: FileChange } {
  const m = line.match(/^\s*(\S{1,2})\s+(.*)$/);
  const code = m?.[1] ?? '';
  const path = m?.[2] ?? line.trim();
  if (code.includes('D')) return { path, change: 'Deleted' };
  if (code === '??' || code.includes('A')) return { path, change: 'New' };
  if (code.includes('R')) return { path: path.split(' -> ').pop() ?? path, change: 'Renamed' };
  return { path, change: 'Changed' };
}

const fileLines = (content: string) => content.replace(/\n$/, '').split('\n');

// diffLines keeps an n×m table, so two large previews that differ throughout would
// freeze the dialog; past this many cells the versions are shown without marks.
const MAX_DIFF_CELLS = 1_000_000;

/** 1-based lines that only one version of a conflicting file has, as CodeView numbers them. */
export function conflictMarks(local: string, remote: string) {
  const a = fileLines(local);
  const b = fileLines(remote);
  // Conflicts usually touch a few lines, so only the differing middle needs the table.
  let head = 0;
  while (head < a.length && head < b.length && a[head] === b[head]) head++;
  let tail = 0;
  while (tail < a.length - head && tail < b.length - head && a[a.length - 1 - tail] === b[b.length - 1 - tail]) tail++;
  const midA = a.slice(head, a.length - tail);
  const midB = b.slice(head, b.length - tail);
  if (midA.length * midB.length > MAX_DIFF_CELLS) return undefined;
  const marks = { local: [] as number[], remote: [] as number[] };
  let l = head;
  let r = head;
  for (const op of diffLines(midA, midB)) {
    if (op.t === 'eq') {
      l++;
      r++;
    } else if (op.t === 'del') marks.local.push(++l);
    else marks.remote.push(++r);
  }
  return marks;
}
