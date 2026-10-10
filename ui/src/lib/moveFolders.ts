import type { Skill } from '../api/client';
import { buildTree, folderPaths } from '../components/resources/tree';

const under = (path: string, root: string) => path === root || path.startsWith(`${root}/`);

/**
 * Source folders a skill (or agent) can be placed in: every ancestor folder of the given
 * resources, minus tracked repos, followed source links and folders that are skills themselves.
 * Mirrors what the server accepts for `--into`; the server stays authoritative.
 */
export function existingFolders(resources: Skill[]): string[] {
  const roots = resources.flatMap((r) => [r.repoPath, r.linkName].filter((p): p is string => !!p));
  const skillDirs = new Set(resources.filter((r) => r.kind === 'skill').map((r) => r.relPath));
  return folderPaths(buildTree(resources)).filter(
    (p) => isValidIntoPath(p) && !roots.some((r) => under(p, r)) && !skillDirs.has(p),
  ).sort((a, b) => a.localeCompare(b));
}

/** Client-side mirror of `validate.IntoPath`: each segment is a valid skill name. */
export function isValidIntoPath(path: string): boolean {
  return path.length <= 256 && path.split('/').every((seg) => /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/.test(seg) && seg.toLowerCase() !== 'agents');
}
