import { apiFetch, BASE, createSSEStream } from './http';
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
  hubIndex: () => apiFetch<HubIndex>('/hub/index'),
  getHubConfig: () => apiFetch<HubConfigResponse>('/hub/saved'),
  putHubConfig: (data: { hubs: HubSavedEntry[]; default: string }) =>
    apiFetch<{ success: boolean }>('/hub/saved', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  addHub: (hub: { label: string; url: string }) =>
    apiFetch<{ success: boolean }>('/hub/saved', {
      method: 'POST',
      body: JSON.stringify(hub),
    }),
  removeHub: (label: string) =>
    apiFetch<{ success: boolean }>(`/hub/saved/${encodeURIComponent(label)}`, {
      method: 'DELETE',
    }),
  search: (q: string, limit = 20) =>
    apiFetch<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),
  searchHub: (q: string, hubURL: string) =>
    apiFetch<{ results: SearchResult[] }>(
      `/search?q=${encodeURIComponent(q)}&hub=${encodeURIComponent(hubURL)}`,
    ),
  preview: (source: string) =>
    apiFetch<SkillPreview>(`/preview?source=${encodeURIComponent(source)}`),
  check: () => apiFetch<CheckResult>('/check'),
  checkStream: (
    onDiscovering: () => void,
    onStart: (total: number) => void,
    onProgress: (checked: number) => void,
    onDone: (data: CheckResult) => void,
    onError: (err: Error) => void,
  ): EventSource =>
    createSSEStream(
      BASE + '/check/stream',
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
    apiFetch<DiscoverResult>('/discover', {
      method: 'POST',
      body: JSON.stringify({ source, branch }),
    }),
  install: (opts: {
    source: string;
    name?: string;
    force?: boolean;
    skipAudit?: boolean;
    track?: boolean;
    into?: string;
    branch?: string;
    kind?: 'skill' | 'agent';
  }) =>
    apiFetch<InstallResult>('/install', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  // Entries recorded in config but not on disk (bare `skillshare install`)
  missingConfigEntries: () =>
    apiFetch<{ entries: ConfigEntry[]; file: string }>('/install/missing'),
  installFromConfig: () =>
    apiFetch<InstallFromConfigResult>('/install/from-config', { method: 'POST' }),
  installBatch: (opts: {
    source: string;
    skills: DiscoveredSkill[];
    force?: boolean;
    skipAudit?: boolean;
    into?: string;
    name?: string;
    branch?: string;
    kind?: 'skill' | 'agent';
  }) =>
    apiFetch<BatchInstallResult>('/install/batch', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
};
