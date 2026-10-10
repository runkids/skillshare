import { apiFetch, kindQuery } from './http';
import type { BatchToggleResult, MoveRequest, MoveResult, BatchUninstallRequest, BatchUninstallResult, CollectResult, CollectScanResult, CreateSkillRequest, CreateSkillResponse, Overview, LinkedRepo, Skill, SkillFileContent, SourceLink, SourceLinkRequest, SourceLinkResult, TemplatesResponse } from './types/resources';

export const resourcesApi = {
  createSourceLink: (body: SourceLinkRequest) =>
    apiFetch<SourceLinkResult>('/source-links', { method: 'POST', body: JSON.stringify(body) }),
  removeSourceLink: (name: string) =>
    apiFetch<{ success: boolean; name: string }>(`/source-links/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  getOverview: () => apiFetch<Overview>('/overview'),
  listSkills: (kind?: 'skill' | 'agent') =>
    apiFetch<{ resources: Skill[]; sourceLinks?: SourceLink[]; sourceLinkWarnings?: string[]; linked_repos?: LinkedRepo[] }>(`/resources${kindQuery(kind)}`),
  getResource: (name: string, kind?: 'skill' | 'agent') =>
    apiFetch<{ resource: Skill; skillMdContent: string; files: string[] }>(
      `/resources/${encodeURIComponent(name)}${kindQuery(kind)}`
    ),
  deleteResource: (name: string, kind?: 'skill' | 'agent') =>
    apiFetch<{ success: boolean }>(
      `/resources/${encodeURIComponent(name)}${kindQuery(kind)}`,
      { method: 'DELETE' }
    ),
  disableResource: (name: string, kind?: 'skill' | 'agent') =>
    apiFetch<{ success: boolean; name: string; disabled: boolean }>(
      `/resources/${encodeURIComponent(name)}/disable${kindQuery(kind)}`,
      { method: 'POST' }
    ),
  enableResource: (name: string, kind?: 'skill' | 'agent') =>
    apiFetch<{ success: boolean; name: string; disabled: boolean }>(
      `/resources/${encodeURIComponent(name)}/enable${kindQuery(kind)}`,
      { method: 'POST' }
    ),
  batchUninstall: (opts: BatchUninstallRequest) =>
    apiFetch<BatchUninstallResult>('/uninstall/batch', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  moveResources: (opts: MoveRequest) =>
    apiFetch<MoveResult>('/resources/batch/move', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  getTemplates: async () => {
    const res = await apiFetch<TemplatesResponse>('/resources/templates');
    // Normalize: Go omits nil slices, so scaffoldDirs may be undefined
    for (const p of res.patterns) {
      if (!p.scaffoldDirs) p.scaffoldDirs = [];
    }
    return res;
  },
  previewSkill: (data: Omit<CreateSkillRequest, 'scaffoldDirs'>) =>
    apiFetch<{ content: string; path: string }>(
      `/resources/templates/preview?${new URLSearchParams(Object.entries(data).filter(([, v]) => v) as [string, string][])}`,
    ),
  createSkill: (data: CreateSkillRequest) =>
    apiFetch<CreateSkillResponse>('/resources', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  batchSetTargets: (folder: string, target: string | null) =>
    apiFetch<{ updated: number; skipped: number; errors: string[] }>('/resources/batch/targets', {
      method: 'POST',
      body: JSON.stringify({ folder, target: target ?? '' }),
    }),
  batchToggleResources: (names: string[], enable: boolean, kind?: 'skill' | 'agent') =>
    apiFetch<BatchToggleResult>('/resources/batch/toggle', {
      method: 'POST',
      body: JSON.stringify({ names, enable, kind }),
    }),
  setSkillTargets: (name: string, target: string | null) =>
    apiFetch<{ success: boolean }>(`/resources/${encodeURIComponent(name)}/targets`, {
      method: 'PATCH',
      body: JSON.stringify({ target: target ?? '' }),
    }),
  // Repo uninstall
  deleteRepo: (name: string, force = false) =>
    apiFetch<{ success: boolean; name: string }>(`/repos/${encodeURIComponent(name)}${force ? '?force=true' : ''}`, { method: 'DELETE' }),
  // Skill file content
  getSkillFile: (skillName: string, filepath: string) =>
    apiFetch<SkillFileContent>(`/resources/${encodeURIComponent(skillName)}/files/${filepath}`),
  // Save SKILL.md / agent markdown.
  saveSkillContent: (name: string, content: string, kind?: 'skill' | 'agent') =>
    apiFetch<{
      bytesWritten: number;
      path: string;
      contentType: string;
      savedAt: string;
    }>(`/resources/${encodeURIComponent(name)}/content${kindQuery(kind)}`, {
      method: 'PUT',
      body: JSON.stringify({ content }),
    }),
  // Update source URL for a tracked skill or agent.
  updateSkillSource: (name: string, source: string, kind?: 'skill' | 'agent') =>
    apiFetch<{ success: boolean; source: string; repoUrl: string }>(
      `/resources/${encodeURIComponent(name)}/source${kindQuery(kind)}`,
      {
        method: 'PATCH',
        body: JSON.stringify({ source }),
      },
    ),
  // Launch an external editor (VS Code / Cursor / $EDITOR) against the skill file.
  openSkillInEditor: (
    name: string,
    opts?: { editor?: string; kind?: 'skill' | 'agent' }
  ) =>
    apiFetch<{ editor: string; path: string; pid: number }>(
      `/resources/${encodeURIComponent(name)}/open-in-editor${kindQuery(opts?.kind)}`,
      {
        method: 'POST',
        body: JSON.stringify({ editor: opts?.editor ?? 'auto' }),
      }
    ),
  collectScan: (target?: string, kind?: 'skill' | 'agent') => {
    const params = new URLSearchParams();
    if (target) params.set('target', target);
    if (kind) params.set('kind', kind);
    const qs = params.toString();
    return apiFetch<CollectScanResult>(`/collect/scan${qs ? '?' + qs : ''}`);
  },
  collect: (opts: { skills: { name: string; targetName: string; kind?: string }[]; force?: boolean }) =>
    apiFetch<CollectResult>('/collect', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
};
