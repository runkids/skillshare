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
