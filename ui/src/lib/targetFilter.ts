import type { SyncMatrixEntry, Target } from '../api/client';

export type FilterPatch = { include?: string[]; exclude?: string[]; agent_include?: string[]; agent_exclude?: string[] };

/** filepath.Match on a trimmed pattern. Flat names have no slashes, so `*` never needs to stop at one. */
function matches(pattern: string, name: string): boolean {
  const p = pattern.trim();
  if (!/[*?[]/.test(p)) return p === name;
  const source = p.replace(/[.+^${}()|\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.').replace(/\[!/g, '[^');
  try {
    return new RegExp(`^${source}$`).test(name);
  } catch {
    return false;
  }
}

/**
 * The target filter change that turns one resource on or off for a target.
 * Null when only editing something the user wrote can change it: a glob pattern that also
 * matches other resources, the resource's own targets field, or a symlink-mode target.
 */
export function targetFilterPatch(entry: SyncMatrixEntry, target: Target, kind: 'skill' | 'agent', flatName: string): FilterPatch | null {
  const agent = kind === 'agent';
  // Sync matches agent patterns without the .md extension.
  const name = agent ? flatName.replace(/\.md$/, '') : flatName;
  const include = (agent ? target.agentInclude : target.include) ?? [];
  const exclude = (agent ? target.agentExclude : target.exclude) ?? [];
  const patch = (inc: string[] | undefined, exc: string[] | undefined): FilterPatch =>
    agent ? { agent_include: inc, agent_exclude: exc } : { include: inc, exclude: exc };

  if (entry.status === 'synced') return patch(undefined, [...exclude, name]);
  if (entry.status !== 'excluded' && entry.status !== 'not_included') return null;

  const excluding = exclude.filter((p) => matches(p, name));
  if (excluding.some((p) => p.trim() !== name)) return null;
  const nextExclude = excluding.length > 0 ? exclude.filter((p) => p.trim() !== name) : undefined;
  return patch(entry.status === 'not_included' ? [...include, name] : undefined, nextExclude);
}
