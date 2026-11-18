import { http } from './http';
import type { TargetInstructionsSetup } from './types/instructions';
import type {
  ProjectInput,
  ProjectList,
  ProjectMutation,
  ProjectScan,
  ProjectScanInput,
  SkillsDetach,
  SkillsOffPreview,
  Target,
} from './types/targets';

export const targetsApi = {
  /** Own targets by default. 'projects' lists the targets of projects alone, 'all' what a sync writes. */
  listTargets: (scope?: 'projects' | 'all') =>
    http.get<{ targets: Target[]; sourceSkillCount: number }>(
      `/targets${scope ? `?scope=${scope}` : ''}`,
    ),
  listProjects: () => http.get<ProjectList>('/projects'),
  saveProject: (project: ProjectInput) => http.put<ProjectMutation>('/projects', project),
  removeProject: (root: string, shared = false) =>
    http.delete<ProjectMutation>(
      `/projects?root=${encodeURIComponent(root)}${shared ? '&shared=true' : ''}`,
    ),
  /** Adds root and each Git checkout below the scan folders; a dry run writes nothing. */
  scanProjects: (input: ProjectScanInput) => http.post<ProjectScan>('/projects/scan', input),
  convertProject: (root: string) => http.post<ProjectMutation>('/projects/convert', { root }),
  /** skillsEnabled false adds a target that skillshare writes no skills to, and creates no skills folder. */
  addTarget: (
    name: string,
    path: string,
    agentPath?: string,
    instructions?: TargetInstructionsSetup,
    skillsEnabled = true,
  ) =>
    http.post<{ success: boolean }>('/targets', {
      name,
      path,
      ...(agentPath && { agentPath }),
      ...(instructions && { instructions }),
      ...(!skillsEnabled && { skills_enabled: false }),
    }),
  /** Adds another config folder of a built-in Agent, such as a second account. cli runs its plugin commands instead of the Agent's executable. */
  addAgentConfigDir: (name: string, agent: string, configDir: string, cli?: string) =>
    http.post<{ success: boolean }>('/targets', { name, agent, configDir, ...(cli && { cli }) }),
  removeTarget: (name: string) =>
    http.delete<{ success: boolean; warnings?: string[] }>(`/targets/${encodeURIComponent(name)}`),
  /** skills_enabled false also removes the links skillshare made in the skills folder; detach says what went and what stayed. */
  updateTarget: (
    name: string,
    opts: {
      include?: string[];
      exclude?: string[];
      mode?: string;
      target_naming?: string;
      agent_mode?: string;
      agent_include?: string[];
      agent_exclude?: string[];
      agent_extension?: string;
      skills_enabled?: boolean;
    },
  ) =>
    http.patch<{ success: boolean; detach?: SkillsDetach }>(
      `/targets/${encodeURIComponent(name)}`,
      opts,
    ),
  /** What turning skills off would remove and keep, without writing anything. */
  skillsOffPreview: (name: string) =>
    http.get<SkillsOffPreview>(`/targets/${encodeURIComponent(name)}/skills-off-preview`),
};
