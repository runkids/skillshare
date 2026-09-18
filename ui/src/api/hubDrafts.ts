import { apiFetch } from './client';

export interface HubEntry {
  id: string;
  data: { name?: string; description?: string; source?: string; skill?: string; tags?: string[]; [key: string]: unknown };
}
export interface HubDraft {
  id: string;
  revision: string;
  name: string;
  description: string;
  updatedAt: string;
  entries: HubEntry[];
  fields: Record<string, unknown>;
}
export interface HubProblem { entryId: string; code: string }
export interface DraftResponse { draft: HubDraft; problems: HubProblem[] }
const root = '/hub/drafts';
const json = (method: string, data: unknown) => ({ method, body: JSON.stringify(data) });
export const hubDrafts = {
  list: () => apiFetch<HubDraft[]>(root),
  get: (id: string) => apiFetch<DraftResponse>(`${root}/${encodeURIComponent(id)}`),
  candidates: () => apiFetch<HubEntry[]>(`${root}/candidates`),
  create: (draft: Partial<HubDraft>) => apiFetch<DraftResponse>(root, json('POST', draft)),
  save: (draft: HubDraft) => apiFetch<DraftResponse>(`${root}/${encodeURIComponent(draft.id)}`, json('PUT', draft)),
  remove: (draft: HubDraft) => apiFetch(`${root}/${encodeURIComponent(draft.id)}?revision=${encodeURIComponent(draft.revision)}`, { method: 'DELETE' }),
  import: (raw: string) => apiFetch<DraftResponse>(`${root}/import`, { method: 'POST', body: raw }),
  export: (draft: HubDraft) => apiFetch<Record<string, unknown>>(`${root}/${encodeURIComponent(draft.id)}/export`, json('POST', { revision: draft.revision })),
};

export function hubAddCommand(location: string, label: string): string {
  const quote = (s: string) => `'${s.replace(/'/g, `'"'"'`)}'`;
  return `skillshare hub add ${quote(location.trim())}${label.trim() ? ` --label ${quote(label.trim())}` : ''}`;
}
