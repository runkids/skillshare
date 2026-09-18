export const CODE_EXT = /\.(ts|tsx|js|jsx|mjs|go|py|rs|rb|sh|bash|zsh|ps1|json|ya?ml|toml)$/i;

export interface FileRow { path: string; label: string; depth: number; folder: boolean }

/** Rows for a file list drawn as a tree. A folder row is emitted once, the first time a file inside it appears. */
export function fileTree(paths: string[]): FileRow[] {
  const tree: FileRow[] = [];
  const seen = new Set<string>();
  for (const path of paths) {
    const parts = path.split('/');
    for (let i = 0; i < parts.length - 1; i++) {
      const dir = parts.slice(0, i + 1).join('/');
      if (!seen.has(dir)) {
        seen.add(dir);
        tree.push({ path: dir, label: parts[i], depth: i, folder: true });
      }
    }
    tree.push({ path, label: parts[parts.length - 1], depth: parts.length - 1, folder: false });
  }
  return tree;
}
