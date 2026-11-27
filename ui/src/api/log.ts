import { http } from './http';
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
    return http.get<LogListResponse>(`/log?${params.toString()}`);
  },
  clearLog: (type?: string) => http.delete<{ success: boolean }>(`/log?type=${type ?? 'ops'}`),
  getLogStats: (type?: string, filters?: { cmd?: string; status?: string; since?: string }) => {
    const params = new URLSearchParams();
    params.set('type', type ?? 'ops');
    if (filters?.cmd) params.set('cmd', filters.cmd);
    if (filters?.status) params.set('status', filters.status);
    if (filters?.since) params.set('since', filters.since);
    return http.get<LogStatsResponse>(`/log/stats?${params.toString()}`);
  },
};
