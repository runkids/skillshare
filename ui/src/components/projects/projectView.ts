import type { Project, ProjectList, Target } from '../../api/client';
import type { mcpApi } from '../../api/mcp';
import type { HookInventory } from '../../api/hooks';
import { scopeChanges, writes as hookWrites } from '../hooks/hooksView';
import { projectOf, writes } from '../mcp/mcpView';
import { targetHealth } from '../targets/targetView';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;

/** A project as the pages show it. `declared` is false for a folder that only mcp.projects names. */
export type ProjectRow = Project & { declared: boolean };

export const projectUrl = (path: string, tab?: 'agents' | 'mcp' | 'hooks') => `/projects/${encodeURIComponent(path)}${tab ? `?tab=${tab}` : ''}`;

export const baseName = (path: string) => path.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || path;

/** Projects from the projects section, then the folders only mcp.projects names, in name order. */
export function projectRows(list: ProjectList | undefined, mcp: MCPList | undefined, hooks?: HookInventory): ProjectRow[] {
  const rows: ProjectRow[] = (list?.projects ?? []).map((p) => ({ ...p, declared: true }));
  const named = [...Object.keys(mcp?.source.projects ?? {}), ...Object.keys(hooks?.source.projects ?? {})];
  const seen = new Set(rows.map((p) => p.path));
  const ownConfig = new Set([...(mcp?.projectConfigs ?? []), ...(hooks?.projectConfigs ?? [])]);
  for (const root of named) {
    if (seen.has(root)) continue;
    seen.add(root);
    const hasOwnConfig = ownConfig.has(root);
    rows.push({ root, path: root, name: baseName(root), targets: [], skills: null, agents: null, groups: [], missing: false, hasOwnConfig, declared: false });
  }
  return rows.sort((a, b) => a.name.localeCompare(b.name));
}

/** Tools grouped by the skills folder they write, as the server groups a saved project. */
export function toolGroups(tools: ProjectList['tools'], selected: string[]) {
  const groups: { tools: string[]; skillsPath: string }[] = [];
  for (const name of selected) {
    const tool = tools.find((x) => x.name === name);
    if (!tool) continue;
    const group = groups.find((g) => g.skillsPath === tool.skillsPath);
    if (group) group.tools.push(name);
    else groups.push({ tools: [name], skillsPath: tool.skillsPath });
  }
  return groups;
}

export type ProjectState = 'missing' | 'conflict' | 'pending' | 'synced' | 'idle';

/** Where a project stands across its targets, its MCP files and its hooks. */
export function projectHealth(project: ProjectRow, targets: Target[], mcp: MCPList | undefined, hooks?: HookInventory): { state: ProjectState; count: number } {
  if (project.missing) return { state: 'missing', count: 0 };
  const mine = targets.filter((tg) => tg.project === project.path).map(targetHealth);
  const roots = Object.keys(mcp?.source.projects ?? {});
  const changes = (mcp?.plan?.changes ?? []).filter((c) => projectOf(roots, c) === project.path);
  const hookChanges = hooks ? scopeChanges(hooks, project.path) : [];
  const conflicts = changes.filter((c) => c.action === 'conflict').length + hookChanges.filter((c) => c.action === 'conflict').length + mine.filter((h) => h.state === 'problem').length;
  if (conflicts > 0) return { state: 'conflict', count: conflicts };
  const pending = mine.reduce((n, h) => n + h.pending, 0) + changes.filter(writes).length + hookChanges.filter(hookWrites).length;
  if (pending > 0) return { state: 'pending', count: pending };
  return { state: mine.length > 0 || mcp?.source.projects?.[project.path] || hooks?.source.projects?.[project.path] ? 'synced' : 'idle', count: 0 };
}
