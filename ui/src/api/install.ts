import { http, BASE, createSSEStream } from './http';
import type {
  BatchInstallResult,
  CheckResult,
  ConfigEntry,
  DiscoverResult,
  DiscoveredSkill,
  HubConfigResponse,
  HubIndex,
  HubSavedEntry,
  InstallFromConfigResult,
  InstallResult,
  SearchResult,
  SkillPreview,
} from './types/install';

export const installApi = {
  hubIndex: () => http.get<HubIndex>('/hub/index'),
  getHubConfig: () => http.get<HubConfigResponse>('/hub/saved'),
  putHubConfig: (data: { hubs: HubSavedEntry[]; default: string }) =>
    http.put<{ success: boolean }>('/hub/saved', data),
  addHub: (hub: { label: string; url: string }) =>
    http.post<{ success: boolean }>('/hub/saved', hub),
  removeHub: (label: string) =>
    http.delete<{ success: boolean }>(`/hub/saved/${encodeURIComponent(label)}`),
  search: (q: string, limit = 20) =>
    http.get<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),
  searchHub: (q: string, hubURL: string) =>
    http.get<{ results: SearchResult[] }>(
      `/search?q=${encodeURIComponent(q)}&hub=${encodeURIComponent(hubURL)}`,
    ),
  preview: (source: string) =>
    http.get<SkillPreview>(`/preview?source=${encodeURIComponent(source)}`),
  check: () => http.get<CheckResult>('/check'),
  checkStream: (
    onDiscovering: () => void,
    onStart: (total: number) => void,
    onProgress: (checked: number) => void,
    onDone: (data: CheckResult) => void,
    onError: (err: Error) => void,
    /** Tracked repos and skills to check; every one of them when omitted. */
    names?: string[],
  ): EventSource =>
    createSSEStream(
      BASE +
        '/check/stream' +
        (names?.length ? `?${new URLSearchParams({ names: names.join(',') })}` : ''),
      {
        discovering: () => onDiscovering(),
        start: (d) => onStart(d.total),
        progress: (d) => onProgress(d.checked),
        done: onDone,
      },
      onError,
      'Check stream failed',
    ),
  discover: (source: string, branch?: string) =>
    http.post<DiscoverResult>('/discover', { source, branch }),
  install: (opts: {
    source: string;
    name?: string;
    force?: boolean;
    skipAudit?: boolean;
    track?: boolean;
    into?: string;
    branch?: string;
    kind?: 'skill' | 'agent';
  }) => http.post<InstallResult>('/install', opts),
  // Entries recorded in config but not on disk (bare `skillshare install`)
  missingConfigEntries: () =>
    http.get<{ entries: ConfigEntry[]; file: string }>('/install/missing'),
  installFromConfig: () => http.post<InstallFromConfigResult>('/install/from-config'),
  installBatch: (opts: {
    source: string;
    skills: DiscoveredSkill[];
    force?: boolean;
    skipAudit?: boolean;
    into?: string;
    name?: string;
    branch?: string;
    kind?: 'skill' | 'agent';
  }) => http.post<BatchInstallResult>('/install/batch', opts),
};
