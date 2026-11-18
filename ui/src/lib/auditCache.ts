import type { QueryClient } from '@tanstack/react-query';
import type { AuditAllResponse } from '../api/client';
import { queryKeys, staleTimes } from './queryKeys';

type AuditCacheState =
  | {
      dataUpdatedAt: number;
      isInvalidated?: boolean;
    }
  | undefined;

export function isAuditCacheFresh(state: AuditCacheState): boolean {
  if (!state || state.dataUpdatedAt === 0) return false;
  if (state.isInvalidated) return false;
  return Date.now() - state.dataUpdatedAt <= staleTimes.audit;
}

export function getCachedAuditResult(
  queryClient: QueryClient,
  kind: 'skills' | 'agents',
  installedCount?: number,
): AuditAllResponse | null {
  const state = queryClient.getQueryState(queryKeys.audit.all(kind));
  if (!isAuditCacheFresh(state)) return null;

  const data = queryClient.getQueryData<AuditAllResponse>(queryKeys.audit.all(kind)) ?? null;
  if (!data) return null;
  if (installedCount != null && data.summary.total !== installedCount) return null;

  return data;
}

export function clearAuditCache(queryClient: QueryClient) {
  queryClient.removeQueries({ queryKey: queryKeys.audit.all('skills'), exact: true });
  queryClient.removeQueries({ queryKey: queryKeys.audit.all('agents'), exact: true });
  queryClient.removeQueries({ queryKey: queryKeys.audit.skills });
}
