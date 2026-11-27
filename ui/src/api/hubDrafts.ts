import { http } from './http';

export interface HubEntry {
  id: string;
  data: {
    name?: string;
    description?: string;
    source?: string;
    skill?: string;
    tags?: string[];
    [key: string]: unknown;
  };
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
export interface HubProblem {
  entryId: string;
  code: string;
}
export interface DraftResponse {
  draft: HubDraft;
  problems: HubProblem[];
  /** Branch or tag each entry's source names, keyed by entry id; entries on the default branch are absent. */
  refs?: Record<string, string>;
}
/** Versions a source can be pinned to. `source` is set only when a `ref` was asked for. */
export interface HubRefs {
  pinnable: boolean;
  current: string;
  defaultBranch: string;
  branches: string[];
  tags: string[];
  source?: string;
}
const root = '/hub/drafts';

export const hubDrafts = {
  list: () => http.get<HubDraft[]>(root),
  get: (id: string) => http.get<DraftResponse>(`${root}/${encodeURIComponent(id)}`),
  candidates: () => http.get<HubEntry[]>(`${root}/candidates`),
  create: (draft: Partial<HubDraft>) => http.post<DraftResponse>(root, draft),
  save: (draft: HubDraft) =>
    http.put<DraftResponse>(`${root}/${encodeURIComponent(draft.id)}`, draft),
  remove: (draft: HubDraft) =>
    http.delete(
      `${root}/${encodeURIComponent(draft.id)}?revision=${encodeURIComponent(draft.revision)}`,
    ),
  import: (raw: string) => http.postRaw<DraftResponse>(`${root}/import`, raw),
  export: (draft: HubDraft) =>
    http.post<Record<string, unknown>>(`${root}/${encodeURIComponent(draft.id)}/export`, {
      revision: draft.revision,
    }),
};

/** Lists a source's branches and tags; with `ref` ("" = default branch), also returns the source rewritten to it. */
export const hubRefs = (source: string, ref?: string) =>
  http.post<HubRefs>('/hub/refs', ref === undefined ? { source } : { source, ref });
/** Lists a git source's branches and tags, also when its URL cannot pin a ref (SSH, other hosts). */
export const gitRefs = (source: string) =>
  http.post<HubRefs>('/hub/refs', { source, anyGit: true });

export function hubAddCommand(location: string, label: string): string {
  const quote = (s: string) => `'${s.replace(/'/g, `'"'"'`)}'`;
  return `skillshare hub add ${quote(location.trim())}${label.trim() ? ` --label ${quote(label.trim())}` : ''}`;
}
