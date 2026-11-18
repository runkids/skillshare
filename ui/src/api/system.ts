import { http, BASE, createSSEStream } from './http';
import type {
  AnalyzeResponse,
  DoctorCheck,
  DoctorResponse,
  VersionCheck,
} from './types/diagnostics';

export const systemApi = {
  getUpdatePolicy: () =>
    http.get<{ enabled: boolean; configured: boolean; envOverride: boolean }>('/update-settings'),
  getVersionCheck: (manual = false) =>
    http.get<VersionCheck>(manual ? '/version?manual=true' : '/version'),
  upgradeApp: () =>
    http.post<{
      ok: boolean;
      updated: boolean;
      devMode?: boolean;
      latestVersion?: string;
      output?: string;
    }>('/upgrade'),
  restartApp: (opts?: { clearCache?: boolean }) =>
    http.post<{ ok: boolean; restarting: boolean }>('/restart', {
      clearCache: opts?.clearCache ?? true,
    }),
  health: () => http.get<{ status: string; version: string; uptime_seconds: number }>('/health'),
  /** Runs doctor, passing each check to onCheck as it is found; resolves with the full result. */
  doctorStream: (onCheck: (check: DoctorCheck) => void, signal?: AbortSignal) =>
    new Promise<DoctorResponse>((resolve, reject) => {
      const es = createSSEStream(
        BASE + '/doctor/stream',
        { check: onCheck, done: resolve },
        reject,
        'Health check failed',
      );
      signal?.addEventListener('abort', () => es.close());
    }),
  analyze: () => http.get<AnalyzeResponse>('/analyze'),
};
