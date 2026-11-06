import { api } from '../../api/client';
import { hooksApi } from '../../api/hooks';
import { mcpApi } from '../../api/mcp';
import type { ProjectRow } from './projectView';

/** A local copy of a project that config.yaml also declares. Removing it leaves the shared one in place. */
export const hasSharedCopy = (p: Pick<ProjectRow, 'origin' | 'sharedCopy'>) => p.origin === 'local' && Boolean(p.sharedCopy);

/**
 * Shared-only rows use explicit shared removal; registrations leave the layer that owns them.
 * `alsoShared` also removes config.yaml's copy of a project this machine overrides.
 * Returns true when config.yaml's version still applies afterwards.
 */
export async function removeProject(project: Pick<ProjectRow, 'declared' | 'root' | 'path'>, shared: { mcp: boolean; hooks: boolean }, alsoShared = false) {
  if (project.declared) {
    const res = await api.removeProject(project.root, alsoShared);
    return Boolean(res.remaining);
  }
  if (shared.mcp) await mcpApi.save({ project: project.path, remove: true });
  if (shared.hooks) await hooksApi.save({ project: project.path, remove: true });
  return false;
}
