import type { Skill } from '../api/client';
import { buildTree, findFolder, folderPaths } from '../components/resources/tree';
import { repoOf, sourceLinkOf } from './resourceGrouping';

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

/** `name` inside `parent` ('' is the source root). */
export const joinFolder = (parent: string, name: string) => (parent ? `${parent}/${name}` : name);

/** The folder `name` makes inside `parent`, or null when it is not a legal path. */
export function newFolderPath(parent: string, name: string): string | null {
  const path = joinFolder(parent, name);
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

/** A skill the move dialog can take: not an agent, not in a tracked repo, not below a followed link. */
export const canMove = (s: Skill) => s.kind === 'skill' && !s.isInRepo && !sourceLinkOf(s);

/** The last segment of a path. */
export const baseName = (path: string) => path.slice(path.lastIndexOf('/') + 1);

/**
 * A folder the move dialog can take whole: not the source root, not a tracked repo or followed link,
 * not inside a `_`-prefixed (tracked) folder and not below a followed link (`linkNames`). The server's dry run has the last word.
 */
export const canMoveFolder = (path: string, node: { repo?: unknown; link?: unknown } = {}, linkNames: string[] = []) =>
  path !== '' && !node.repo && !node.link && !path.split('/').some((seg) => seg.startsWith('_')) && !linkNames.some((n) => under(path, n));

export interface FolderPlan {
  /** Skills directly in the folder. */
  direct: Skill[];
  /** First-level subfolders that hold skills, with how many. */
  subfolders: { name: string; count: number }[];
  /** Tracked repos and followed links inside the folder: the server refuses the whole folder because of them. */
  blocked: { path: string; why: 'repo' | 'link' }[];
  /** Every skill that moves with the folder. */
  count: number;
}

/** What moving `folder` takes along, from the loaded resources. */
export function planFolder(resources: Skill[], folder: string): FolderPlan {
  // A folder can itself be a skill (SKILL.md at its root); it moves with what is below it.
  const inside = resources.filter((s) => s.kind === 'skill' && (s.relPath === folder || s.relPath.startsWith(`${folder}/`)));
  const moving = inside.filter(canMove);
  const counts = new Map<string, number>();
  const direct: Skill[] = [];
  for (const s of moving) {
    const rest = s.relPath.slice(folder.length + 1);
    const i = rest.indexOf('/');
    if (s.relPath === folder) { direct.push(s); continue; }
    if (i < 0) direct.push(s);
    else counts.set(rest.slice(0, i), (counts.get(rest.slice(0, i)) ?? 0) + 1);
  }
  const blocked = new Map<string, 'repo' | 'link'>();
  for (const s of inside) {
    const link = sourceLinkOf(s)?.name;
    const repo = repoOf(s);
    if (link) blocked.set(link, 'link');
    else if (repo) blocked.set(repo, 'repo');
  }
  return {
    direct,
    subfolders: [...counts].map(([name, count]) => ({ name, count })).sort((a, b) => a.name.localeCompare(b.name)),
    blocked: [...blocked].map(([path, why]) => ({ path: path.startsWith(`${folder}/`) ? path.slice(folder.length + 1) : path, why })),
    count: moving.length,
  };
}
