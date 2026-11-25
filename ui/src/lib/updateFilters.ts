import type { Skill } from '../api/client';
import { resolveSource } from '../components/SourceBadge';
import { globToRegex } from './glob';
import { folderOf } from './resourceNames';

/** What the Updates list filters: a tracked repo, or one installed item. */
export interface FilterUnit {
  name: string;
  label: string;
  isRepo: boolean;
  items: Skill[];
  source?: string;
}

/** Local items have nothing to update from, so they never reach the Updates list. */
export type UpdateSource = 'tracked' | 'github' | 'remote';
export const UPDATE_SOURCES: UpdateSource[] = ['tracked', 'github', 'remote'];

/** Check results grouped the way the Status filter offers them. */
export type UpdateStatus = 'updates' | 'modified' | 'failed' | 'current';
export const UPDATE_STATUSES: UpdateStatus[] = ['updates', 'modified', 'failed', 'current'];

export interface UpdateFilters {
  query: string;
  source: UpdateSource | 'all';
  status: UpdateStatus | 'all';
  /** A folder key ('' is the source root), or null for every folder. */
  folder: string | null;
}

export const NO_FILTERS: UpdateFilters = { query: '', source: 'all', status: 'all', folder: null };

export function unitSource(unit: FilterUnit): UpdateSource {
  if (unit.isRepo) return 'tracked';
  const source = resolveSource(unit.items[0]?.type, false);
  return source === 'github' ? 'github' : 'remote';
}

/** A tracked repo is its own folder; an item lives in the folder its path names. */
export function unitFolder(unit: FilterUnit): string {
  return unit.isRepo ? unit.name : unit.items[0] ? folderOf(unit.items[0]) : '';
}

/** The Status filter group of a check status; null for one that is not known yet. */
export function statusGroup(status: string): UpdateStatus | null {
  switch (status) {
    case 'behind':
    case 'update-available':
      return 'updates';
    case 'dirty':
      return 'modified';
    case 'error':
      return 'failed';
    case 'up-to-date':
      return 'current';
    default:
      return null;
  }
}

/**
 * Whether a unit passes the filters. `skip` leaves one filter out, so each chip can count
 * what its options would show given the search and the other chips.
 */
export function matchesFilters(
  unit: FilterUnit,
  filters: UpdateFilters,
  status: string,
  skip?: 'source' | 'status' | 'folder',
): boolean {
  const query = filters.query.trim();
  if (query) {
    const re = globToRegex(query);
    const glob = /[*?]/.test(query);
    const names = [unit.label, unit.name, ...unit.items.map((s) => s.relPath)];
    if (!names.some((n) => re.test(n)) && (glob || !re.test(unit.source ?? ''))) return false;
  }
  if (skip !== 'source' && filters.source !== 'all' && unitSource(unit) !== filters.source)
    return false;
  if (skip !== 'status' && filters.status !== 'all' && statusGroup(status) !== filters.status)
    return false;
  if (skip !== 'folder' && filters.folder !== null && unitFolder(unit) !== filters.folder)
    return false;
  return true;
}

/**
 * Whether an up-to-date row folds away: only while nothing is searched for or filtered by status,
 * so what needs attention leads, and never while an update run reports on it.
 */
export function foldsAway(filters: UpdateFilters, status: string, inRun: boolean): boolean {
  return (
    status === 'up-to-date' && !inRun && filters.status === 'all' && filters.query.trim() === ''
  );
}

/**
 * Whether the upstream preview can show this unit's changes: it diffs git commits, so a skill
 * installed from a local path has nothing to compare even when its source changed.
 */
export function hasUpstreamPreview(unit: FilterUnit): boolean {
  return unit.isRepo || !!unit.items[0]?.repoUrl;
}

/** Labels shared by more than one unit: those rows need their folder to tell them apart. */
export function repeatedLabels(units: FilterUnit[]): Set<string> {
  const seen = new Set<string>();
  const repeated = new Set<string>();
  for (const u of units) {
    if (seen.has(u.label)) repeated.add(u.label);
    seen.add(u.label);
  }
  return repeated;
}

/** Hosts whose web pages follow a known layout, with the name the link shows. */
// `refs` are the browser-URL segments that put a ref before a folder on that host, as install
// reads them (splitGitWebRef and the GitHub parser); anywhere else they are folder names.
const HOSTS: Record<
  string,
  { host: string; tree: string; commits: string; compare: string | null; refs: string[] }
> = {
  'github.com': {
    host: 'GitHub',
    tree: 'tree',
    commits: 'commits',
    compare: 'compare',
    refs: ['tree', 'blob'],
  },
  'gitlab.com': {
    host: 'GitLab',
    tree: '-/tree',
    commits: '-/commits',
    compare: '-/compare',
    refs: ['-/tree', '-/blob'],
  },
  // Bitbucket's history page takes no folder, so both views open the folder, and a range of
  // commits opens the repository.
  'bitbucket.org': { host: 'Bitbucket', tree: 'src', commits: 'src', compare: null, refs: ['src'] },
};

/** A git URL or source as path segments, host first: scheme, user and `git@host:` removed. */
const segments = (raw: string) =>
  raw
    .trim()
    .replace(/^[^@/:]+@([^:/]+):/, '$1/')
    .replace(/^[a-z+]+:\/\/(?:[^@/]+@)?/i, '')
    .split('/')
    .filter(Boolean);

const bare = (segment: string) => segment.replace(/\.git$/, '');

/** The unit's repository on a known host, and the folder and ref its source names, if any. */
function locate(unit: FilterUnit) {
  const item = unit.items[0];
  const parts = segments((unit.isRepo ? item?.repoUrl : undefined) ?? unit.source ?? '');
  const site = HOSTS[parts[0] ?? ''];
  if (!site || parts.length < 3) return null;
  // The clone URL says where the repository path ends, which matters for GitLab subgroups;
  // without it, a ".git" segment marks the end, and otherwise owner/repo is assumed.
  const known = item?.repoUrl ? segments(item.repoUrl).map(bare) : [];
  const fromUrl =
    known.length > 2 && known.every((p, i) => bare(parts[i] ?? '') === p) ? known.length : 0;
  const marker = parts.findIndex((p, i) => i > 1 && p.endsWith('.git')) + 1;
  const end = fromUrl || marker || 3;
  const home = `https://${parts[0]}/${parts.slice(1, end).map(bare).join('/')}`;
  let rest = parts.slice(end);
  const branch = item?.branch ?? '';
  let ref = branch || 'HEAD';
  // A source written as a browser URL names a ref before its folder: …/tree/<ref>/<dir>,
  // …/-/tree/…, …/src/…. Updates follow the recorded branch, so it wins over that ref; when the
  // two match, the branch also says how many segments a ref like feature/x spans.
  const web = site.refs.find((m) => rest.slice(0, m.split('/').length).join('/') === m);
  const skip = web ? web.split('/').length : 0;
  if (web && rest.length > skip) {
    const own = branch.split('/');
    const matches = branch !== '' && own.every((p, i) => rest[skip + i] === p);
    if (!branch) ref = rest[skip];
    rest = rest.slice(skip + (matches ? own.length : 1));
  }
  return { site, home, ref, path: unit.isRepo ? null : rest.join('/') };
}

/**
 * The web page of a unit's source on GitHub, GitLab or Bitbucket: its folder, or with view
 * 'commits' the history of that folder. Other hosts lay their pages out in ways this cannot
 * know, so they get no link.
 */
export function sourcePage(
  unit: FilterUnit,
  view: 'tree' | 'commits' = 'tree',
): { url: string; host: string } | null {
  const at = locate(unit);
  if (!at) return null;
  // A tracked repo, or a root skill on the default branch, opens the repository's home.
  const url =
    at.path === null || (at.path === '' && at.ref === 'HEAD')
      ? at.home
      : `${at.home}/${at.site[view]}/${at.ref}${at.path && `/${at.path}`}`;
  return { url, host: at.site.host };
}

/** The page comparing two commits of a tracked repo; Bitbucket's falls back to the repo. */
export function compareUrl(
  unit: FilterUnit,
  from: string,
  to: string,
): { url: string; host: string } | null {
  const at = locate(unit);
  if (!at) return null;
  const url = at.site.compare ? `${at.home}/${at.site.compare}/${from}...${to}` : at.home;
  return { url, host: at.site.host };
}
