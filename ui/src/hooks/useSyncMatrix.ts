import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import type { SyncMatrixEntry } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';

const EMPTY: SyncMatrixEntry[] = [];

function summarize(entries: SyncMatrixEntry[]): { synced: number; total: number } {
  const applicable = entries.filter((e) => e.status !== 'na');
  return {
    synced: applicable.filter((e) => e.status === 'synced').length,
    total: applicable.length,
  };
}

export function useSyncMatrix() {
  const { data, isLoading, error } = useQuery({
    queryKey: queryKeys.syncMatrix(),
    queryFn: () => api.getSyncMatrix(),
    staleTime: staleTimes.syncMatrix,
  });

  const matrix = data?.entries ?? EMPTY;

  const helpers = useMemo(() => {
    const bySkill = new Map<string, SyncMatrixEntry[]>();
    const byTarget = new Map<string, SyncMatrixEntry[]>();

    for (const entry of matrix) {
      const skillEntries = bySkill.get(entry.skill) ?? [];
      skillEntries.push(entry);
      bySkill.set(entry.skill, skillEntries);

      const targetEntries = byTarget.get(entry.target) ?? [];
      targetEntries.push(entry);
      byTarget.set(entry.target, targetEntries);
    }

    return {
      getSkillTargets(flatName: string): SyncMatrixEntry[] {
        return bySkill.get(flatName) ?? [];
      },
      getTargetSkills(targetName: string): SyncMatrixEntry[] {
        return byTarget.get(targetName) ?? [];
      },
      getSkillSummary(flatName: string) {
        return summarize(bySkill.get(flatName) ?? []);
      },
      getTargetSummary(targetName: string) {
        return summarize(byTarget.get(targetName) ?? []);
      },
    };
  }, [matrix]);

  return {
    matrix,
    isLoading,
    error,
    ...helpers,
  };
}
