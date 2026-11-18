import { http, BASE, createSSEStream, kindQuery } from './http';
import type {
  AuditAllResponse,
  AuditPolicy,
  AuditRulesResponse,
  AuditSkillResponse,
  CompiledRulesResponse,
} from './types/audit';

export const auditApi = {
  auditAll: (kind?: 'skills' | 'agents') => http.get<AuditAllResponse>(`/audit${kindQuery(kind)}`),
  auditSkill: (name: string, kind?: 'skill' | 'agent') =>
    http.get<AuditSkillResponse>(
      `/audit/${encodeURIComponent(name)}${kindQuery(kind === 'agent' ? kind : undefined)}`,
    ),
  auditAllStream: (
    onStart: (total: number) => void,
    onProgress: (scanned: number) => void,
    onDone: (data: AuditAllResponse) => void,
    onError: (err: Error) => void,
    kind?: 'skills' | 'agents',
  ): EventSource =>
    createSSEStream(
      BASE + `/audit/stream${kindQuery(kind)}`,
      {
        start: (d) => onStart(d.total),
        progress: (d) => onProgress(d.scanned),
        done: onDone,
      },
      onError,
      'Audit stream failed',
    ),
  getAuditRules: () => http.get<AuditRulesResponse>('/audit/rules'),
  putAuditRules: (raw: string) => http.put<{ success: boolean }>('/audit/rules', { raw }),
  initAuditRules: () => http.post<{ success: boolean; path: string }>('/audit/rules'),
  getCompiledRules: () => http.get<CompiledRulesResponse>('/audit/rules/compiled'),
  /** Severity at which install and sync refuse a resource. */
  setAuditThreshold: (blockThreshold: string) =>
    http.patch<AuditPolicy>('/audit/policy', { blockThreshold }),
  /** Preset for how strict a scan is: default, strict, or permissive. */
  setAuditProfile: (profile: string) => http.patch<AuditPolicy>('/audit/policy', { profile }),
  toggleRule: (req: { id?: string; pattern?: string; enabled: boolean; severity?: string }) =>
    http.post<{ success: boolean }>('/audit/rules/toggle', req),
  resetRules: () => http.post<{ success: boolean }>('/audit/rules/reset'),
};
