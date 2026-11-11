import { apiFetch } from './http';
import type { AnalyzeResponse, DoctorResponse, VersionCheck } from './types/diagnostics';

export const systemApi = {
  getUpdatePolicy: () => apiFetch<{ enabled: boolean; configured: boolean; envOverride: boolean }>('/update-settings'),
  getVersionCheck: (manual = false) => apiFetch<VersionCheck>(manual ? '/version?manual=true' : '/version'),
  upgradeApp: () => apiFetch<{ ok: boolean; updated: boolean; devMode?: boolean; latestVersion?: string; output?: string }>('/upgrade', { method: 'POST' }),
  restartApp: (opts?: { clearCache?: boolean }) =>
    apiFetch<{ ok: boolean; restarting: boolean }>('/restart', {
      method: 'POST',
      body: JSON.stringify({ clearCache: opts?.clearCache ?? true }),
    }),
  health: () => apiFetch<{ status: string; version: string; uptime_seconds: number }>('/health'),
  doctor: () => apiFetch<DoctorResponse>('/doctor'),
  analyze: () => apiFetch<AnalyzeResponse>('/analyze'),
};
