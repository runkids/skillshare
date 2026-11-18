import { apiFetch } from './http';
import type { LogListResponse, LogStatsResponse } from './types/log';

export const logApi = {
  listLog: (
    type?: string,
    limit?: number,
    filters?: { cmd?: string; status?: string; since?: string },
  ) => {
    const params = new URLSearchParams();
    params.set('type', type ?? 'ops');
    params.set('limit', String(limit ?? 100));
    if (filters?.cmd) params.set('cmd', filters.cmd);
    if (filters?.status) params.set('status', filters.status);
    if (filters?.since) params.set('since', filters.since);
    return apiFetch<LogListResponse>(`/log?${params.toString()}`);
  },
  clearLog: (type?: string) =>
    apiFetch<{ success: boolean }>(`/log?type=${type ?? 'ops'}`, { method: 'DELETE' }),
  getLogStats: (type?: string, filters?: { cmd?: string; status?: string; since?: string }) => {
    const params = new URLSearchParams();
    params.set('type', type ?? 'ops');
    if (filters?.cmd) params.set('cmd', filters.cmd);
    if (filters?.status) params.set('status', filters.status);
    if (filters?.since) params.set('since', filters.since);
    return apiFetch<LogStatsResponse>(`/log/stats?${params.toString()}`);
  },
};
