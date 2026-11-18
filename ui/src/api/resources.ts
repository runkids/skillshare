import { http, kindQuery } from './http';
import type {
  BatchToggleResult,
  MoveRequest,
  MoveResult,
  BatchUninstallRequest,
  BatchUninstallResult,
  CollectResult,
  CollectScanResult,
  CreateSkillRequest,
  CreateSkillResponse,
  Overview,
  LinkedRepo,
  LocalFileChange,
  Skill,
  SkillFileContent,
  SourceLink,
  SourceLinkRequest,
  SourceLinkResult,
  TemplatesResponse,
} from './types/resources';

export const resourcesApi = {
  createSourceLink: (body: SourceLinkRequest) => http.post<SourceLinkResult>('/source-links', body),
  removeSourceLink: (name: string) =>
    http.delete<{ success: boolean; name: string; warning?: string }>(
      `/source-links/${encodeURIComponent(name)}`,
    ),
  getOverview: () => http.get<Overview>('/overview'),
  listSkills: (kind?: 'skill' | 'agent', signal?: AbortSignal) =>
    http.get<{
      resources: Skill[];
      sourceLinks?: SourceLink[];
      sourceLinkWarnings?: string[];
      linked_repos?: LinkedRepo[];
    }>(`/resources${kindQuery(kind)}`, signal),
  getResource: (name: string, kind?: 'skill' | 'agent') =>
    http.get<{ resource: Skill; skillMdContent: string; files: string[] }>(
      `/resources/${encodeURIComponent(name)}${kindQuery(kind)}`,
    ),
  deleteResource: (name: string, kind?: 'skill' | 'agent') =>
    http.delete<{ success: boolean }>(`/resources/${encodeURIComponent(name)}${kindQuery(kind)}`),
  disableResource: (name: string, kind?: 'skill' | 'agent') =>
    http.post<{ success: boolean; name: string; disabled: boolean }>(
      `/resources/${encodeURIComponent(name)}/disable${kindQuery(kind)}`,
    ),
  enableResource: (name: string, kind?: 'skill' | 'agent') =>
    http.post<{ success: boolean; name: string; disabled: boolean }>(
      `/resources/${encodeURIComponent(name)}/enable${kindQuery(kind)}`,
    ),
  batchUninstall: (opts: BatchUninstallRequest) =>
    http.post<BatchUninstallResult>('/uninstall/batch', opts),
  moveResources: (opts: MoveRequest) => http.post<MoveResult>('/resources/batch/move', opts),
  getTemplates: async () => {
    const res = await http.get<TemplatesResponse>('/resources/templates');
    // Normalize: Go omits nil slices, so scaffoldDirs may be undefined
    for (const p of res.patterns) {
      if (!p.scaffoldDirs) p.scaffoldDirs = [];
    }
    return res;
  },
  previewSkill: (data: Omit<CreateSkillRequest, 'scaffoldDirs'>) =>
    http.get<{ content: string; path: string }>(
      `/resources/templates/preview?${new URLSearchParams(Object.entries(data).filter(([, v]) => v) as [string, string][])}`,
    ),
  createSkill: (data: CreateSkillRequest) => http.post<CreateSkillResponse>('/resources', data),
  batchSetTargets: (folder: string, target: string | null) =>
    http.post<{ updated: number; skipped: number; errors: string[] }>('/resources/batch/targets', {
      folder,
      target: target ?? '',
    }),
  batchToggleResources: (names: string[], enable: boolean, kind?: 'skill' | 'agent') =>
    http.post<BatchToggleResult>('/resources/batch/toggle', { names, enable, kind }),
  setSkillTargets: (name: string, target: string | null) =>
    http.patch<{ success: boolean }>(`/resources/${encodeURIComponent(name)}/targets`, {
      target: target ?? '',
    }),
  // Repo uninstall
  deleteRepo: (name: string, force = false, recordOnly = false) =>
    http.delete<{ success: boolean; name: string; movedToTrash: boolean }>(
      `/repos/${encodeURIComponent(name)}${force ? '?force=true' : ''}${recordOnly ? `${force ? '&' : '?'}recordOnly=true` : ''}`,
    ),
  // Skill file content
  getSkillFile: (skillName: string, filepath: string) =>
    http.get<SkillFileContent>(`/resources/${encodeURIComponent(skillName)}/files/${filepath}`),
  // Files changed since install, diffed against the installed version; name is the skill's relPath.
  getLocalChanges: (name: string) =>
    http.get<{ files: LocalFileChange[] }>(`/resources/${encodeURIComponent(name)}/local-changes`),
  // Save SKILL.md / agent markdown.
  saveSkillContent: (name: string, content: string, kind?: 'skill' | 'agent') =>
    http.put<{
      bytesWritten: number;
      path: string;
      contentType: string;
      savedAt: string;
    }>(`/resources/${encodeURIComponent(name)}/content${kindQuery(kind)}`, { content }),
  // Update source URL for a tracked skill or agent.
  updateSkillSource: (name: string, source: string, kind?: 'skill' | 'agent') =>
    http.patch<{ success: boolean; source: string; repoUrl: string }>(
      `/resources/${encodeURIComponent(name)}/source${kindQuery(kind)}`,
      { source },
    ),
  // Launch an external editor (VS Code / Cursor / $EDITOR) against the skill file.
  openSkillInEditor: (name: string, opts?: { editor?: string; kind?: 'skill' | 'agent' }) =>
    http.post<{ editor: string; path: string; pid: number }>(
      `/resources/${encodeURIComponent(name)}/open-in-editor${kindQuery(opts?.kind)}`,
      { editor: opts?.editor ?? 'auto' },
    ),
  collectScan: (target?: string, kind?: 'skill' | 'agent') => {
    const params = new URLSearchParams();
    if (target) params.set('target', target);
    if (kind) params.set('kind', kind);
    const qs = params.toString();
    return http.get<CollectScanResult>(`/collect/scan${qs ? '?' + qs : ''}`);
  },
  collect: (opts: {
    skills: { name: string; targetName: string; kind?: string }[];
    force?: boolean;
  }) => http.post<CollectResult>('/collect', opts),
};
