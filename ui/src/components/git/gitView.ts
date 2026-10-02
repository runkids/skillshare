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

/** 1-based lines that only one version of a conflicting file has, as CodeView numbers them. */
export function conflictMarks(local: string, remote: string) {
  const marks = { local: [] as number[], remote: [] as number[] };
  let l = 0;
  let r = 0;
  for (const op of diffLines(fileLines(local), fileLines(remote))) {
    if (op.t === 'eq') {
      l++;
      r++;
    } else if (op.t === 'del') marks.local.push(++l);
    else marks.remote.push(++r);
  }
  return marks;
}
