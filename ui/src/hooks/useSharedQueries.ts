import { useQuery, type UseQueryOptions } from '@tanstack/react-query';
import { api } from '../api/client';
import { hooksApi } from '../api/hooks';
import { mcpApi } from '../api/mcp';
import { queryKeys, staleTimes } from '../lib/queryKeys';

// Queries that several pages share. Each key and its fetcher live here once;
// callers pass only what differs per page, such as enabled or polling.
type Options<T> = Omit<UseQueryOptions<T>, 'queryKey' | 'queryFn'>;
type Data<F extends (...args: never[]) => Promise<unknown>> = Awaited<ReturnType<F>>;

export const useOverviewQuery = (options?: Options<Data<typeof api.getOverview>>) =>
  useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview, ...options });

// No default staleTime: pages disagree (the sidebar, Sync, and Add target use
// staleTimes.extras; the rest refetch on mount), so each passes its own.
export const useMcpQuery = (options?: Options<Data<typeof mcpApi.list>>) =>
  useQuery({ queryKey: queryKeys.mcp, queryFn: () => mcpApi.list(), ...options });

export const useHooksQuery = (options?: Options<Data<typeof hooksApi.list>>) =>
  useQuery({ queryKey: queryKeys.hooks, queryFn: () => hooksApi.list(), ...options });

export const useSkillsQuery = (options?: Options<Data<typeof api.listSkills>>) =>
  useQuery({ queryKey: queryKeys.skills.all, queryFn: () => api.listSkills(), staleTime: staleTimes.skills, ...options });

export const useAvailableTargetsQuery = (options?: Options<Data<typeof api.availableTargets>>) =>
  useQuery({ queryKey: queryKeys.targets.available, queryFn: () => api.availableTargets(), staleTime: staleTimes.targets, ...options });

/** Own and project targets together. */
export const useSyncedTargetsQuery = (options?: Options<Data<typeof api.listTargets>>) =>
  useQuery({ queryKey: queryKeys.targets.synced, queryFn: () => api.listTargets('all'), staleTime: staleTimes.targets, ...options });

/** Pending sync changes across all targets. */
export const useDiffQuery = (options?: Options<Data<typeof api.diff>>) =>
  useQuery({ queryKey: queryKeys.diff(), queryFn: () => api.diff(), staleTime: staleTimes.diff, ...options });
