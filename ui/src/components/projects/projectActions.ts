import { api } from '../../api/client';
import { hooksApi } from '../../api/hooks';
import { mcpApi } from '../../api/mcp';
import type { ProjectRow } from './projectView';

/** Shared-only rows use explicit shared removal; registrations are local. */
export async function removeProject(project: Pick<ProjectRow, 'declared' | 'root' | 'path'>, shared: { mcp: boolean; hooks: boolean }) {
  if (project.declared) {
    await api.removeProject(project.root);
    return;
  }
  if (shared.mcp) await mcpApi.save({ project: project.path, remove: true });
  if (shared.hooks) await hooksApi.save({ project: project.path, remove: true });
}
