/**
 * Replace home directory prefix with ~/
 * Handles macOS (/Users/x), Linux (/home/x), and Windows (C:\Users\x)
 */
export function shortenHome(path: string): string {
  return path
    .replace(/^\/Users\/[^/]+/, '~')
    .replace(/^\/home\/[^/]+/, '~')
    .replace(/^[A-Z]:\\Users\\[^\\]+/i, '~');
}

/** Keep both ends of a long local path: where it lives and what it is called are what identify it. */
export function shortenPath(path: string): string {
  const short = shortenHome(path);
  const parts = short.split('/');
  // Only a local path: `owner/repo` and URLs are already as short as they get.
  if (!/^[~/]/.test(short) || parts.length <= 5) return short;
  return [...parts.slice(0, 2), '…', ...parts.slice(-2)].join('/');
}

/** A path inside `root` relative to it; any other path with home shortened. */
export function relativeTo(root: string, path = ''): string {
  return path.startsWith(`${root}/`) ? path.slice(root.length + 1) : shortenHome(path);
}

/** Last segment of a local path, splitting on / and on Windows \ alike. */
export function fileName(path: string): string {
  return path.split(/[\\/]/).pop() || path;
}
