import { apiFetch } from './http';

export interface MemoryNote {
  path: string;
  title: string;
  version: string;
  content?: string;
  invalid?: string;
}

export interface MemoryIndex {
  version: string;
  unindexed: string[];
  broken_links: string[];
  error?: string;
}

/** passive agents update notes only when asked; active ones also save lasting facts. */
export type MemoryUpdateMode = 'passive' | 'active';

export type MemoryInstructions = Record<MemoryUpdateMode, string>;

export interface MemoryGuidanceTarget {
  name: string;
  state: 'unconfigured' | 'configured' | 'outdated' | 'broken';
  file?: string;
  detail?: string;
  mode?: MemoryUpdateMode;
}

export interface MemoryGuidance {
  scope: 'global' | 'project';
  instructions: MemoryInstructions;
  targets: MemoryGuidanceTarget[];
}

export interface MemoryGuidancePlan {
  token: string;
  changes: { path: string; before: string; after: string; targets: string[]; created: boolean }[];
  skipped: { target: string; reason: string }[];
  warnings: { code: string; path: string; targets?: string[]; target?: string; limit?: number; chars?: number }[];
}

export interface MemoryNotesResponse {
  root: string;
  initialized: boolean;
  notes: MemoryNote[];
  instructions: MemoryInstructions;
  index?: MemoryIndex;
}

export const memoryApi = {
  listMemoryNotes: (search = '') => apiFetch<MemoryNotesResponse>(`/extras/memory/notes?search=${encodeURIComponent(search)}`),
  initMemory: () => apiFetch<{ success: boolean; root: string }>('/extras/memory/init', { method: 'POST' }),
  getMemoryGuidance: () => apiFetch<MemoryGuidance>('/extras/memory/guidance'),
  planMemoryGuidance: (targets: string[], modes: Record<string, MemoryUpdateMode>) => apiFetch<MemoryGuidancePlan>('/extras/memory/guidance/plan', {
    method: 'POST', body: JSON.stringify({ targets, modes }),
  }),
  applyMemoryGuidance: (targets: string[], modes: Record<string, MemoryUpdateMode>, token: string) => apiFetch<{ success: boolean; applied: string[]; errors: { path: string; error: string }[]; targets: MemoryGuidanceTarget[] }>('/extras/memory/guidance/apply', {
    method: 'POST', body: JSON.stringify({ targets, modes, token }),
  }),
  linkMemoryIndex: (path: string, version: string) => apiFetch<MemoryNote>('/extras/memory/index', {
    method: 'PUT', body: JSON.stringify({ path, version }),
  }),
  readMemoryNote: (path: string) => apiFetch<MemoryNote>(`/extras/memory/notes/content?path=${encodeURIComponent(path)}`),
  writeMemoryNote: (path: string, content: string, version: string) => apiFetch<MemoryNote>('/extras/memory/notes/content', {
    method: 'PUT', body: JSON.stringify({ path, content, version }),
  }),
  moveMemoryNote: (path: string, newPath: string, version: string) => apiFetch<MemoryNote>('/extras/memory/move', {
    method: 'POST', body: JSON.stringify({ path, new_path: newPath, version }),
  }),
  deleteMemoryNote: (path: string, version: string) => apiFetch<{ success: boolean; path: string }>('/extras/memory/notes/content', {
    method: 'DELETE', body: JSON.stringify({ path, version }),
  }),
};
