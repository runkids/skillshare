import { http } from './http';

export interface MemoryNote {
  path: string;
  title: string;
  version: string;
  content?: string;
  invalid?: string;
  modified_at?: string;
  starter?: boolean;
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
  warnings: {
    code: string;
    path: string;
    targets?: string[];
    target?: string;
    limit?: number;
    chars?: number;
  }[];
}

export interface MemoryNotesResponse {
  root: string;
  initialized: boolean;
  notes: MemoryNote[];
  instructions: MemoryInstructions;
  index?: MemoryIndex;
}

export interface MemoryOverviewResponse {
  root: string;
  count: number;
  initialized: boolean;
  notes: MemoryNote[];
}

export const memoryApi = {
  getMemoryOverview: () => http.get<MemoryOverviewResponse>('/extras/memory/recent'),
  listMemoryNotes: (search = '') =>
    http.get<MemoryNotesResponse>(`/extras/memory/notes?search=${encodeURIComponent(search)}`),
  initMemory: () => http.post<{ success: boolean; root: string }>('/extras/memory/init'),
  getMemoryGuidance: () => http.get<MemoryGuidance>('/extras/memory/guidance'),
  planMemoryGuidance: (targets: string[], modes: Record<string, MemoryUpdateMode>) =>
    http.post<MemoryGuidancePlan>('/extras/memory/guidance/plan', { targets, modes }),
  applyMemoryGuidance: (
    targets: string[],
    modes: Record<string, MemoryUpdateMode>,
    token: string,
  ) =>
    http.post<{
      success: boolean;
      applied: string[];
      errors: { path: string; error: string }[];
      targets: MemoryGuidanceTarget[];
    }>('/extras/memory/guidance/apply', { targets, modes, token }),
  linkMemoryIndex: (path: string, version: string) =>
    http.put<MemoryNote>('/extras/memory/index', { path, version }),
  readMemoryNote: (path: string) =>
    http.get<MemoryNote>(`/extras/memory/notes/content?path=${encodeURIComponent(path)}`),
  writeMemoryNote: (path: string, content: string, version: string) =>
    http.put<MemoryNote>('/extras/memory/notes/content', { path, content, version }),
  moveMemoryNote: (path: string, newPath: string, version: string) =>
    http.post<MemoryNote>('/extras/memory/move', { path, new_path: newPath, version }),
  deleteMemoryNote: (path: string, version: string) =>
    http.delete<{ success: boolean; path: string }>('/extras/memory/notes/content', {
      path,
      version,
    }),
};
