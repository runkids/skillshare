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
