import { apiFetch } from './http';
import type { TargetInstructionsSetup } from './types/instructions';
import type { ProjectInput, ProjectList, SkillsDetach, SkillsOffPreview, Target } from './types/targets';

export const targetsApi = {
  /** Own targets by default. 'projects' lists the targets of projects alone, 'all' what a sync writes. */
  listTargets: (scope?: 'projects' | 'all') => apiFetch<{ targets: Target[]; sourceSkillCount: number }>(`/targets${scope ? `?scope=${scope}` : ''}`),
  listProjects: () => apiFetch<ProjectList>('/projects'),
  saveProject: (project: ProjectInput) => apiFetch<{ success: boolean; root: string }>('/projects', { method: 'PUT', body: JSON.stringify(project) }),
  removeProject: (root: string) => apiFetch<{ success: boolean }>(`/projects?root=${encodeURIComponent(root)}`, { method: 'DELETE' }),
  convertProject: (root: string) => apiFetch<{ success: boolean; root: string }>('/projects/convert', { method: 'POST', body: JSON.stringify({ root }) }),
  /** skillsEnabled false adds a target that skillshare writes no skills to, and creates no skills folder. */
  addTarget: (name: string, path: string, agentPath?: string, instructions?: TargetInstructionsSetup, skillsEnabled = true) =>
    apiFetch<{ success: boolean }>('/targets', {
      method: 'POST',
      body: JSON.stringify({ name, path, ...(agentPath && { agentPath }), ...(instructions && { instructions }), ...(!skillsEnabled && { skills_enabled: false }) }),
    }),
  /** Adds another config folder of a built-in Agent, such as a second account. cli runs its plugin commands instead of the Agent's executable. */
  addAgentConfigDir: (name: string, agent: string, configDir: string, cli?: string) =>
    apiFetch<{ success: boolean }>('/targets', { method: 'POST', body: JSON.stringify({ name, agent, configDir, ...(cli && { cli }) }) }),
  removeTarget: (name: string) =>
    apiFetch<{ success: boolean; warnings?: string[] }>(`/targets/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  /** skills_enabled false also removes the links skillshare made in the skills folder; detach says what went and what stayed. */
  updateTarget: (name: string, opts: { include?: string[]; exclude?: string[]; mode?: string; target_naming?: string; agent_mode?: string; agent_include?: string[]; agent_exclude?: string[]; agent_extension?: string; skills_enabled?: boolean }) =>
    apiFetch<{ success: boolean; detach?: SkillsDetach }>(`/targets/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify(opts),
    }),
  /** What turning skills off would remove and keep, without writing anything. */
  skillsOffPreview: (name: string) =>
    apiFetch<SkillsOffPreview>(`/targets/${encodeURIComponent(name)}/skills-off-preview`),
};
