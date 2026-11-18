import { apiFetch, BASE, createSSEStream, kindQuery } from './http';
import type {
  AuditAllResponse,
  AuditPolicy,
  AuditRulesResponse,
  AuditSkillResponse,
  CompiledRulesResponse,
} from './types/audit';

export const auditApi = {
  auditAll: (kind?: 'skills' | 'agents') => apiFetch<AuditAllResponse>(`/audit${kindQuery(kind)}`),
  auditSkill: (name: string, kind?: 'skill' | 'agent') =>
    apiFetch<AuditSkillResponse>(
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
  getAuditRules: () => apiFetch<AuditRulesResponse>('/audit/rules'),
  putAuditRules: (raw: string) =>
    apiFetch<{ success: boolean }>('/audit/rules', {
      method: 'PUT',
      body: JSON.stringify({ raw }),
    }),
  initAuditRules: () =>
    apiFetch<{ success: boolean; path: string }>('/audit/rules', {
      method: 'POST',
    }),
  getCompiledRules: () => apiFetch<CompiledRulesResponse>('/audit/rules/compiled'),
  /** Severity at which install and sync refuse a resource. */
  setAuditThreshold: (blockThreshold: string) =>
    apiFetch<AuditPolicy>('/audit/policy', {
      method: 'PATCH',
      body: JSON.stringify({ blockThreshold }),
    }),
  /** Preset for how strict a scan is: default, strict, or permissive. */
  setAuditProfile: (profile: string) =>
    apiFetch<AuditPolicy>('/audit/policy', {
      method: 'PATCH',
      body: JSON.stringify({ profile }),
    }),
  toggleRule: (req: { id?: string; pattern?: string; enabled: boolean; severity?: string }) =>
    apiFetch<{ success: boolean }>('/audit/rules/toggle', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  resetRules: () =>
    apiFetch<{ success: boolean }>('/audit/rules/reset', {
      method: 'POST',
    }),
};
