import type { SyncMatrixEntry, Target } from '../../api/client';

export type TargetState = 'synced' | 'pending' | 'missing' | 'migrate' | 'problem' | 'unknown';

/** Where a target stands, and how many skills and agents the next sync would add. */
export function targetHealth(target: Target): { state: TargetState; pending: number } {
  const agentPending = Math.max(0, (target.agentExpectedCount ?? 0) - (target.agentLinkedCount ?? 0));
  // With skills off the skills folder is not skillshare's, so its status says nothing about this target.
  if (target.skillsEnabled === false) return { state: agentPending > 0 ? 'pending' : 'synced', pending: agentPending };
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

/** Filter patterns match agent names without the .md extension. */
export const patternName = (entry: SyncMatrixEntry) => (entry.kind === 'agent' ? entry.skill.replace(/\.md$/, '') : entry.skill);

/**
 * The filter edit a click on a preview row makes, or null when naming the row can't change its result
 * (a wildcard exclude, or a resource that declares its own targets).
 * Exclude wins over include, so a synced row is excluded by name unless dropping its own include keeps other includes.
 */
export function togglePatterns(entry: SyncMatrixEntry, include: string[], exclude: string[]): { include: string[]; exclude: string[] } | null {
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
      return exclude.includes(name) ? { include, exclude: exclude.filter((p) => p !== name) } : null;
    default:
      return null;
  }
}

/** "a, b and c" in the reader's language. */
export function joinList(items: string[], locale: string) {
  const parts = new Intl.ListFormat(locale, { type: 'conjunction' }).formatToParts(items);
  if (!locale.startsWith('zh')) return parts.map((p) => p.value).join('');
  // Chinese joins with a bare 和, which runs into Latin names such as MCP; Taiwan UI copy says 與.
  const latin = /[\x21-\x7e]/;
  return parts.map((p, i) => {
    if (p.type !== 'literal' || p.value !== '和') return p.value;
    const before = latin.test(parts[i - 1]?.value.slice(-1) ?? '') ? ' ' : '';
    const after = latin.test(parts[i + 1]?.value[0] ?? '') ? ' ' : '';
    return before + (locale === 'zh-TW' ? '與' : '和') + after;
  }).join('');
}

/** Keep source paths while translating skillshare-owned diagnostic prose. */
export function targetWarning(warning: string): { key: string; path: string } | null {
  const selection = warning.match(/^Target ".*": skipped ambiguous .* variant (.+); use include\/exclude/);
  if (selection) return { key: 'sync.skipped.ambiguousVariant', path: selection[1] };
  const naming = warning.match(/^Target '.*': skipped (.+) because /);
  return naming ? { key: 'sync.skipped.hintNaming', path: naming[1] } : null;
}

/** Independent remedies for skills omitted from this target. */
export function targetSkipHints(target: Target) {
  const keys = [];
  if (target.namingSkippedSkillCount) keys.push(target.collisionCount ? 'sync.skipped.hintMixed' as const : 'sync.skipped.hintNaming' as const);
  else if (target.collisionCount) keys.push('sync.skipped.hint' as const);
  if (target.selectionSkippedSkillCount) keys.push('sync.skipped.hintSelection' as const);
  return keys;
}
