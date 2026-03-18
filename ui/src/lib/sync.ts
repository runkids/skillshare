import type { QueryClient } from '@tanstack/react-query';

import type { SyncResult } from '../api/client';

import { queryKeys } from './queryKeys';

/** Summarize sync results into totals for toast messages. */
export function summarizeSyncResults(results: SyncResult[]) {
  const totalLinked = results.reduce((sum, r) => sum + (r.linked?.length ?? 0), 0);
  const totalUpdated = results.reduce((sum, r) => sum + (r.updated?.length ?? 0), 0);
  return { totalLinked, totalUpdated, targets: results.length };
}

/** Build a human-readable sync success toast message. */
export function formatSyncToast(results: SyncResult[]): string {
  const { totalLinked, totalUpdated, targets } = summarizeSyncResults(results);
  return `Sync complete! ${totalLinked} linked, ${totalUpdated} updated across ${targets} target(s).`;
}

/** Invalidate queries that depend on sync state. */
export function invalidateAfterSync(queryClient: QueryClient) {
  queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
  queryClient.invalidateQueries({ queryKey: queryKeys.overview });
}
