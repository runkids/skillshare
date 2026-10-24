import type { Skill, SourceLink, SyncMatrixEntry } from '../api/client';
import type { useT } from '../i18n';
import { resolveSource, type SourceType } from '../components/SourceBadge';
import { parseRemoteURL } from './parseRemoteURL';
import { folderOf, formatTrackedRepoName } from './resourceNames';

export type SourceFilter = 'all' | SourceType;
export type SortType = 'name-asc' | 'name-desc' | 'newest' | 'oldest';

export const SOURCE_ORDER: SourceType[] = ['tracked', 'github', 'remote', 'local'];
// Source names stay in English, like the CLI.
export const SOURCE_LABEL: Record<SourceFilter, string> = { all: 'All', tracked: 'Tracked', github: 'GitHub', remote: 'Remote', local: 'Local' };

/** "1 skill" / "3 agents". These keys are named by kind, not `.one`/`.other`, so `plural` does not fit them. */
export function countLabel(t: ReturnType<typeof useT>, kind: Skill['kind'], count: number): string {
  return t(`resources.count.${kind}${count === 1 ? '' : 's'}`, { count });
}

/**
 * Which of `items` each target actually receives, keyed by target name.
 * Reads the sync matrix — the same source the Targets column renders — so a
 * filter result always matches the icons the rows show.
 */
export function syncedByTarget(items: Skill[], matrix: SyncMatrixEntry[]): Map<string, Set<string>> {
  const names = new Set(items.map((s) => s.flatName));
  const byTarget = new Map<string, Set<string>>();
  for (const e of matrix) {
    if (e.status !== 'synced' || !names.has(e.skill)) continue;
    let set = byTarget.get(e.target);
    if (!set) byTarget.set(e.target, (set = new Set()));
    set.add(e.skill);
  }
  return byTarget;
}

/** The project a target name belongs to, or '' for a global target. */
export const projectOf = (name: string) => (name.includes('@') ? name.slice(0, name.lastIndexOf('@')) : '');

/**
 * Folds each project's targets into one filter entry keyed `<project>@`, holding what any of its
 * tools receives; global targets keep their own entries.
 */
export function byTargetOrProject(byTarget: Map<string, Set<string>>): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>();
  for (const [name, names] of byTarget) {
    const project = projectOf(name);
    const key = project ? `${project}@` : name;
    out.set(key, new Set([...(out.get(key) ?? []), ...names]));
  }
  return out;
}

// Group key for sorting: tracked repo name or first dir segment.
function sortGroup(s: Skill): string {
  const slash = s.relPath.indexOf('/');
  return slash > 0 ? s.relPath.slice(0, slash) : '';
}

export function sortSkills(skills: Skill[], sortType: SortType): Skill[] {
  const byName = (a: Skill, b: Skill) => sortGroup(a).localeCompare(sortGroup(b)) || a.name.localeCompare(b.name);
  const byDate = (dir: 1 | -1) => (a: Skill, b: Skill) => {
    if (!a.installedAt && !b.installedAt) return byName(a, b);
    if (!a.installedAt) return 1;
    if (!b.installedAt) return -1;
    return dir * (new Date(a.installedAt).getTime() - new Date(b.installedAt).getTime());
  };
  const sorted = [...skills];
  switch (sortType) {
    case 'name-asc': return sorted.sort(byName);
    case 'name-desc': return sorted.sort((a, b) => sortGroup(a).localeCompare(sortGroup(b)) || b.name.localeCompare(a.name));
    case 'newest': return sorted.sort(byDate(-1));
    case 'oldest': return sorted.sort(byDate(1));
  }
}

export function repoOf(s: Skill): string | undefined {
  return s.isInRepo ? folderOf(s) : undefined;
}

export function sourceLinkOf(s: Skill): SourceLink | undefined {
  return s.kind === 'skill' && s.linkName && s.linkTarget ? { name: s.linkName, target: s.linkTarget } : undefined;
}

/** Parent folder shown under the name. Inside a repo group the repo prefix is already in the header. */
export function parentPath(s: Skill, inGroup = false): string {
  const i = s.relPath.lastIndexOf('/');
  if (i <= 0) return '';
  const dir = s.relPath.slice(0, i);
  const repo = inGroup ? sourceLinkOf(s)?.name ?? repoOf(s) : undefined;
  if (repo) return dir === repo ? '' : dir.slice(repo.length + 1);
  return formatTrackedRepoName(dir);
}

export function sourceName(s: Skill): string {
  const link = sourceLinkOf(s);
  if (link) return formatTrackedRepoName(link.name);
  const repo = repoOf(s);
  if (repo) return formatTrackedRepoName(repo);
  if (s.source) return parseRemoteURL(s.source)?.ownerRepo ?? s.source;
  return SOURCE_LABEL.local;
}

/* -- Source groups -------------------------------- */

export interface Group { key: string; source: SourceType; repo?: string; link?: SourceLink; items: Skill[] }

export function groupBySource(items: Skill[], links: SourceLink[] = []): Group[] {
  const groups = new Map<string, Group>();
  for (const s of items) {
    const source = resolveSource(s.type, s.isInRepo);
    const link = sourceLinkOf(s);
    const repo = repoOf(s);
    const key = link ? `link:${link.name}` : repo ?? source;
    if (!groups.has(key)) groups.set(key, { key, source, repo, link, items: [] });
    groups.get(key)!.items.push(s);
  }
  for (const link of links) {
    const key = `link:${link.name}`;
    const group = groups.get(key);
    if (group) group.link = link;
    else groups.set(key, { key, source: 'local', link, items: [] });
  }
  return [...groups.values()].sort((a, b) => SOURCE_ORDER.indexOf(a.source) - SOURCE_ORDER.indexOf(b.source) || a.key.localeCompare(b.key));
}

/* -- Folder groups -------------------------------- */

export interface FolderGroup { key: string; repo: boolean; link?: SourceLink; items: Skill[] }

/** Root first, then folder name A→Z; items keep the order they came in (the current sort). */
export function groupByFolder(items: Skill[], links: SourceLink[] = []): FolderGroup[] {
  const groups = new Map<string, FolderGroup>();
  for (const s of items) {
    const link = sourceLinkOf(s);
    const key = link?.name ?? folderOf(s);
    if (!groups.has(key)) groups.set(key, { key, repo: !!repoOf(s), link, items: [] });
    groups.get(key)!.items.push(s);
  }
  for (const link of links) {
    const group = groups.get(link.name);
    if (group) group.link = link;
    else groups.set(link.name, { key: link.name, repo: false, link, items: [] });
  }
  return [...groups.values()].sort((a, b) => formatTrackedRepoName(a.key).localeCompare(formatTrackedRepoName(b.key)));
}

/** Cut groups down to the first `limit` items, preserving standalone empty groups. */
export function limitGroups<G extends { items: Skill[] }>(groups: G[], limit: number, collapsed: (g: G) => boolean = () => false): G[] {
  const out: G[] = [];
  let left = limit;
  for (const g of groups) {
    // A collapsed group shows only its head, so its items cost nothing.
    if (collapsed(g)) { out.push(g); continue; }
    if (left <= 0 && g.items.length > 0) continue;
    out.push({ ...g, items: g.items.slice(0, left) });
    left -= g.items.length;
  }
  return out;
}

/* -- Nested skills -------------------------------- */

export interface Nest { roots: Skill[]; children: Map<string, Skill[]> }

/** Puts each skill under the nearest skill in `items` whose folder holds it; both lists keep the order of `items`. */
export function nestSkills(items: Skill[]): Nest {
  const byPath = new Map(items.map((s) => [s.relPath, s]));
  const roots: Skill[] = [];
  const children = new Map<string, Skill[]>();
  for (const s of items) {
    let parent: Skill | undefined;
    for (let i = s.relPath.lastIndexOf('/'); i > 0 && !parent; i = s.relPath.lastIndexOf('/', i - 1)) parent = byPath.get(s.relPath.slice(0, i));
    if (!parent) { roots.push(s); continue; }
    if (!children.has(parent.flatName)) children.set(parent.flatName, []);
    children.get(parent.flatName)!.push(s);
  }
  return { roots, children };
}

/** How many skills in `items` sit anywhere under `s`'s folder. */
export function descendantCount(s: Skill, items: Skill[]): number {
  const prefix = `${s.relPath}/`;
  return items.reduce((n, x) => n + (x.relPath.startsWith(prefix) ? 1 : 0), 0);
}

/** The skills in `list` that none of `holders` holds in its folder; removing those folders removes the rest. */
export function outermost(list: Skill[], holders: Skill[] = list): Skill[] {
  return list.filter((s) => !holders.some((p) => s.relPath.startsWith(`${p.relPath}/`)));
}

/** `matched` plus each skill in `all` whose folder holds one of them, in the order of `all`, so a match keeps its packages around it. */
export function withAncestors(matched: Skill[], all: Skill[]): Skill[] {
  const keep = new Set(matched.map((s) => s.relPath));
  for (const s of matched) {
    for (let i = s.relPath.lastIndexOf('/'); i > 0; i = s.relPath.lastIndexOf('/', i - 1)) keep.add(s.relPath.slice(0, i));
  }
  return all.filter((s) => keep.has(s.relPath));
}

/** Splits target names into global tools and projects; `myapp@claude` is claude in the myapp project. */
export function splitTargets(names: string[]): { global: string[]; projects: [string, string[]][] } {
  const global: string[] = [];
  const projects = new Map<string, string[]>();
  for (const n of names) {
    const i = n.lastIndexOf('@');
    if (i < 0) {
      global.push(n);
      continue;
    }
    const project = n.slice(0, i);
    projects.set(project, [...(projects.get(project) ?? []), n.slice(i + 1)]);
  }
  return { global, projects: [...projects] };
}
