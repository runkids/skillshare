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

/**
 * The GitHub page of what a unit installs from: a tracked repo's home, or the folder of a
 * skill installed from a subdirectory, as its file tree or its commit history. null for anything
 * not hosted on github.com.
 */
export function githubPage(unit: FilterUnit, view: 'tree' | 'commits' = 'tree'): string | null {
  const raw = unit.isRepo ? (unit.items[0]?.repoUrl ?? unit.source) : unit.source;
  if (!raw) return null;
  const path = raw
    .trim()
    .replace(/^git@github\.com:/, 'github.com/')
    .replace(/^[a-z+]+:\/\/(?:[^@/]+@)?/i, '')
    .replace(/\.git$/, '');
  const [host, owner, repo, ...rest] = path.split('/').filter(Boolean);
  if (host !== 'github.com' || !owner || !repo) return null;
  const home = `https://github.com/${owner}/${repo.replace(/\.git$/, '')}`;
  if (unit.isRepo || rest.length === 0) return home;
  // A source written as a browser URL already names its ref: github.com/o/r/tree/<ref>/<dir>.
  if (rest[0] === 'tree' && rest.length > 1) return `${home}/${view}/${rest.slice(1).join('/')}`;
  return `${home}/${view}/${unit.items[0]?.branch || 'HEAD'}/${rest.join('/')}`;
}
