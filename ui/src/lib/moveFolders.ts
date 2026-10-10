import type { Skill } from '../api/client';
import { buildTree, findFolder, folderPaths } from '../components/resources/tree';

export interface FolderOption {
  path: string;
  /** Skills in this folder and every folder below it. */
  count: number;
}

const under = (path: string, root: string) => path === root || path.startsWith(`${root}/`);

/**
 * Source folders a skill (or agent) can be placed in: every ancestor folder of the given
 * resources, minus tracked repos, followed source links and folders that are skills themselves.
 * Mirrors what the server accepts for `--into`; the server stays authoritative.
 */
export function existingFolders(resources: Skill[]): FolderOption[] {
  const roots = resources.flatMap((r) => [r.repoPath, r.linkName].filter((p): p is string => !!p));
  const skillDirs = new Set(resources.filter((r) => r.kind === 'skill').map((r) => r.relPath));
  const tree = buildTree(resources);
  return folderPaths(tree)
    .filter((p) => isValidIntoPath(p) && !roots.some((r) => under(p, r)) && !skillDirs.has(p))
    .sort((a, b) => a.localeCompare(b))
    .map((path) => ({ path, count: findFolder(tree, path)!.count }));
}

/** Client-side mirror of `validate.SkillName`, which `validate.IntoPath` applies to each segment. */
export const isValidFolderName = (name: string) => /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/.test(name) && name.toLowerCase() !== 'agents';

/** Client-side mirror of `validate.IntoPath`. */
export const isValidIntoPath = (path: string) => path.length <= 256 && path.split('/').every(isValidFolderName);

/** The folder `name` makes inside `parent` ('' is the source root), or null when it is not a legal path. */
export function newFolderPath(parent: string, name: string): string | null {
  const path = parent ? `${parent}/${name}` : name;
  return isValidFolderName(name) && isValidIntoPath(path) ? path : null;
}

/** What a skill's flat name starts with inside `folder`: `a/b` gives `a__b__`. */
export const linkPrefix = (folder: string) => (folder ? `${folder.replace(/\//g, '__')}__` : '');

/**
 * The skill an install source names, when the source itself says so: a local path, or a
 * `owner/repo/<path>` shorthand or URL with a path below the repo. `owner/repo` alone names
 * no skill. Only a best guess for a caption; the server decides the real name.
 */
export function skillNameFromSource(source: string): string | null {
  const value = source.trim().replace(/\/+$/, '').replace(/\.git$/, '');
  if (/^[/~.]/.test(value) || /^[a-zA-Z]:\\/.test(value)) return lastName(value.split(/[\\/]/));
  if (value.startsWith('git@')) return null;
  const parts = value.replace(/^[a-z+]+:\/\//i, '').split('/');
  if (/\./.test(parts[0])) parts.shift(); // host
  // owner/repo/<path>; a github.com/.../tree/<branch>/ URL keeps its branch in the middle.
  if (parts.length < 3 || ['tree', 'blob'].includes(parts[2])) return null;
  return lastName(parts);
}

function lastName(parts: string[]): string | null {
  const name = parts[parts.length - 1];
  return isValidFolderName(name) ? name : null;
}
