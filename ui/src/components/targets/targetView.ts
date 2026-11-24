import type { SyncMatrixEntry, Target } from '../../api/client';
import type { useT } from '../../i18n';

type T = ReturnType<typeof useT>;

export type TargetState = 'synced' | 'pending' | 'missing' | 'migrate' | 'problem' | 'unknown';

/** Where a target stands, and how many skills and agents the next sync would add. */
export function targetHealth(target: Target): { state: TargetState; pending: number } {
  const agentPending = Math.max(
    0,
    (target.agentExpectedCount ?? 0) - (target.agentLinkedCount ?? 0),
  );
  // With skills off the skills folder is not skillshare's, so its status says nothing about this target.
  if (target.skillsEnabled === false)
    return { state: agentPending > 0 ? 'pending' : 'synced', pending: agentPending };
  const pending = Math.max(0, target.expectedSkillCount - target.linkedCount) + agentPending;
  switch (target.status) {
    case 'merged':
    case 'copied':
    case 'linked':
      return { state: pending > 0 ? 'pending' : 'synced', pending };
    case 'not exist':
      return { state: 'missing', pending };
    case 'has files':
      // A real folder sits where the symlink goes, so sync moves its files into the source first.
      if (target.mode === 'symlink') return { state: 'migrate', pending };
      // Merge and copy keep local files: here it only says nothing from the source is in the folder yet.
      return { state: pending > 0 ? 'pending' : 'synced', pending };
    case 'conflict':
    case 'broken':
      return { state: 'problem', pending };
    default:
      return { state: 'unknown', pending };
  }
}

/** Why a preview row syncs or not. `filtered`: the filters list an include. */
export function previewReason(t: T, entry: SyncMatrixEntry, filtered: boolean) {
  switch (entry.status) {
    case 'skipped':
      return t('sync.skipped.ambiguousVariant');
    case 'synced':
      return t(filtered ? 'targetDetail.reason.included' : 'targetDetail.reason.noFilter');
    case 'excluded':
      return t('targetDetail.reason.excluded', { pattern: entry.reason });
    case 'not_included':
      return t('targetDetail.reason.notIncluded');
    case 'skill_target_mismatch':
      return t('targetDetail.reason.declared', { targets: entry.reason });
    default:
      return entry.reason;
  }
}

/** Filter patterns match agent names without the .md extension. */
export const patternName = (entry: SyncMatrixEntry) =>
  entry.kind === 'agent' ? entry.skill.replace(/\.md$/, '') : entry.skill;

/** Preview rows whose pattern name contains the search text, ignoring case. */
export function searchEntries(entries: SyncMatrixEntry[], query: string) {
  const q = query.trim().toLowerCase();
  return q ? entries.filter((e) => patternName(e).toLowerCase().includes(q)) : entries;
}

/**
 * The filter edit a click on a preview row makes, or null when naming the row can't change its result
 * (a wildcard exclude, or a resource that declares its own targets).
 * Exclude wins over include, so a synced row is excluded by name unless dropping its own include keeps other includes.
 */
export function togglePatterns(
  entry: SyncMatrixEntry,
  include: string[],
  exclude: string[],
): { include: string[]; exclude: string[] } | null {
  const name = patternName(entry);
  switch (entry.status) {
    case 'synced':
      return include.includes(name) && include.length > 1
        ? { include: include.filter((p) => p !== name), exclude }
        : { include, exclude: [...exclude, name] };
    case 'skipped':
      return { include, exclude: [...new Set([...exclude, name])] };
    case 'not_included':
      return { include: [...include, name], exclude };
    case 'excluded':
      return exclude.includes(name)
        ? { include, exclude: exclude.filter((p) => p !== name) }
        : null;
    default:
      return null;
  }
}

/**
 * Whether a filter pattern matches a name the way the server's filepath.Match does:
 * `*` and `?` never cross `/`, `[...]` is a class (`[^...]` negated), `\` escapes. A malformed pattern matches nothing.
 */
function compilePattern(pattern: string): RegExp | null {
  const lit = (c: string) => c.replace(/[\\^$.*+?()[\]{}|/-]/g, '\\$&');
  let re = '';
  for (let i = 0; i < pattern.length; i++) {
    const c = pattern[i];
    if (c === '*') re += '[^/]*';
    else if (c === '?') re += '[^/]';
    else if (c === '\\') {
      if (++i === pattern.length) return null;
      re += lit(pattern[i]);
    } else if (c === '[') {
      let cls = '';
      let j = i + 1;
      if (pattern[j] === '^') {
        cls = '^';
        j++;
      }
      const first = j;
      for (; j < pattern.length && (pattern[j] !== ']' || j === first); j++) {
        if (pattern[j] === '\\') j++;
        if (j === pattern.length) return null;
        cls += pattern[j] === '-' && j > first && pattern[j + 1] !== ']' ? '-' : lit(pattern[j]);
      }
      if (j === pattern.length || !cls.replace('^', '')) return null;
      re += `[${cls}]`;
      i = j;
    } else re += lit(c);
  }
  return new RegExp(`^${re}$`);
}

// Hovering re-renders every row and rule count, so each pattern compiles once.
const compiled = new Map<string, RegExp | null>();

export function patternMatches(pattern: string, name: string): boolean {
  if (!compiled.has(pattern)) compiled.set(pattern, compilePattern(pattern));
  return compiled.get(pattern)?.test(name) ?? false;
}

export type RowAction = 'exclude' | 'include' | 'unexclude';

/** The action a preview row offers and the filters after it, or null where naming the row can't change its result. */
export function rowAction(
  entry: SyncMatrixEntry,
  include: string[],
  exclude: string[],
): { action: RowAction; next: { include: string[]; exclude: string[] } } | null {
  const next = togglePatterns(entry, include, exclude);
  if (!next) return null;
  const action: RowAction =
    next.include.length > include.length
      ? 'include'
      : next.exclude.length < exclude.length
        ? 'unexclude'
        : 'exclude';
  return { action, next };
}

/** The rows whose result flipped between two previews, and how many of them now skip or sync. */
export function previewChange(before: SyncMatrixEntry[], after: SyncMatrixEntry[]) {
  const was = new Map(before.map((e) => [e.skill, e.status === 'synced']));
  const changed = after.filter(
    (e) => was.has(e.skill) && was.get(e.skill) !== (e.status === 'synced'),
  );
  const synced = changed.filter((e) => e.status === 'synced').length;
  return { changed: changed.map((e) => e.skill), synced, skipped: changed.length - synced };
}

/** "a, b and c" in the reader's language. */
export function joinList(items: string[], locale: string) {
  const parts = new Intl.ListFormat(locale, { type: 'conjunction' }).formatToParts(items);
  if (!locale.startsWith('zh')) return parts.map((p) => p.value).join('');
  // Chinese joins with a bare 和, which runs into Latin names such as MCP; Taiwan UI copy says 與.
  const latin = /[\x21-\x7e]/;
  return parts
    .map((p, i) => {
      if (p.type !== 'literal' || p.value !== '和') return p.value;
      const before = latin.test(parts[i - 1]?.value.slice(-1) ?? '') ? ' ' : '';
      const after = latin.test(parts[i + 1]?.value[0] ?? '') ? ' ' : '';
      return before + (locale === 'zh-TW' ? '與' : '和') + after;
    })
    .join('');
}

/** Keep source paths while translating skillshare-owned diagnostic prose. */
export function targetWarning(warning: string): { key: string; path: string } | null {
  const selection = warning.match(
    /^Target ".*": skipped ambiguous .* variant (.+); use include\/exclude/,
  );
  if (selection) return { key: 'sync.skipped.ambiguousVariant', path: selection[1] };
  const naming = warning.match(/^Target '.*': skipped (.+) because /);
  return naming ? { key: 'sync.skipped.hintNaming', path: naming[1] } : null;
}

/** Independent remedies for skills omitted from this target. */
export function targetSkipHints(target: Target) {
  const keys = [];
  if (target.namingSkippedSkillCount)
    keys.push(
      target.collisionCount
        ? ('sync.skipped.hintMixed' as const)
        : ('sync.skipped.hintNaming' as const),
    );
  else if (target.collisionCount) keys.push('sync.skipped.hint' as const);
  if (target.selectionSkippedSkillCount) keys.push('sync.skipped.hintSelection' as const);
  return keys;
}
