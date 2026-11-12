import { apiFetch, BASE, createSSEStream } from './http';
import type { AnalyzeResponse, DoctorCheck, DoctorResponse, VersionCheck } from './types/diagnostics';

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
  /** Runs doctor, passing each check to onCheck as it is found; resolves with the full result. */
  doctorStream: (onCheck: (check: DoctorCheck) => void, signal?: AbortSignal) =>
    new Promise<DoctorResponse>((resolve, reject) => {
      const es = createSSEStream(BASE + '/doctor/stream', { check: onCheck, done: resolve }, reject, 'Health check failed');
      signal?.addEventListener('abort', () => es.close());
    }),
  analyze: () => apiFetch<AnalyzeResponse>('/analyze'),
};
