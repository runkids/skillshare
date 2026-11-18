import { http, BASE, createSSEStream } from './http';
import type {
  DiffTarget,
  FolderConflict,
  IgnoreSources,
  SyncMatrixEntry,
  SyncResponse,
} from './types/sync';

export const syncApi = {
  getSyncMatrix: (target?: string) =>
    http.get<{ entries: SyncMatrixEntry[] }>(
      `/sync-matrix${target ? '?target=' + encodeURIComponent(target) : ''}`,
    ),
  previewSyncMatrix: (
    target: string,
    include: string[],
    exclude: string[],
    agentInclude?: string[],
    agentExclude?: string[],
  ) =>
    http.post<{ entries: SyncMatrixEntry[] }>('/sync-matrix/preview', {
      target,
      include,
      exclude,
      ...(agentInclude && { agent_include: agentInclude }),
      ...(agentExclude && { agent_exclude: agentExclude }),
    }),
  /** `project` (a root under projects) limits the sync to that project's targets. */
  sync: (opts: { dryRun?: boolean; force?: boolean; kind?: 'skill' | 'agent'; project?: string }) =>
    http.post<SyncResponse>('/sync', opts),
  diff: (target?: string) =>
    http.get<{ diffs: DiffTarget[]; folder_conflicts?: FolderConflict[] } & IgnoreSources>(
      `/diff${target ? '?target=' + encodeURIComponent(target) : ''}`,
    ),
  diffStream: (
    onDiscovering: () => void,
    onStart: (total: number) => void,
    onResult: (diff: DiffTarget, checked: number) => void,
    onDone: (data: { diffs: DiffTarget[] } & IgnoreSources) => void,
    onError: (err: Error) => void,
  ): EventSource =>
    createSSEStream(
      BASE + '/diff/stream',
      {
        discovering: () => onDiscovering(),
        start: (d) => onStart(d.total),
        result: (d) => onResult(d.diff, d.checked),
        done: onDone,
      },
      onError,
      'Diff stream failed',
    ),
};
