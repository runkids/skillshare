import type { QueryClient } from '@tanstack/react-query';
import type { SyncMatrixEntry, Target } from '../../api/client';
import { queryKeys } from '../../lib/queryKeys';

export type TargetState = 'synced' | 'pending' | 'missing' | 'problem' | 'unknown';

/** Where a target stands, and how many skills and agents the next sync would add. */
export function targetHealth(target: Target): { state: TargetState; pending: number } {
  const pending = Math.max(0, target.expectedSkillCount - target.linkedCount)
    + Math.max(0, (target.agentExpectedCount ?? 0) - (target.agentLinkedCount ?? 0));
  switch (target.status) {
    case 'merged':
    case 'copied':
    case 'linked':
      return { state: pending > 0 ? 'pending' : 'synced', pending };
    case 'not exist':
      return { state: 'missing', pending };
    case 'conflict':
    case 'broken':
    case 'has files':
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
    case 'not_included':
      return { include: [...include, name], exclude };
    case 'excluded':
      return exclude.includes(name) ? { include, exclude: exclude.filter((p) => p !== name) } : null;
    default:
      return null;
  }
}

/** Everything that shows a target's config or its effect. */
export function refreshTargets(queryClient: QueryClient) {
  for (const key of [queryKeys.targets.all, queryKeys.targets.available, queryKeys.config, queryKeys.overview, queryKeys.diff(), queryKeys.syncMatrix()]) {
    void queryClient.invalidateQueries({ queryKey: key });
  }
}
