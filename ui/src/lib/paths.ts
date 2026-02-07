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
