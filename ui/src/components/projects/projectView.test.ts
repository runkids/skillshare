import { describe, expect, it } from 'vitest';
import type { ProjectList } from '../../api/client';
import type { HookChange, HookInventory } from '../../api/hooks';
import { projectHealth, projectRows, toolGroups } from './projectView';

const tools: ProjectList['tools'] = [
  { name: 'claude', skillsPath: '.claude/skills', agentsPath: '.claude/agents' },
  { name: 'codex', skillsPath: '.agents/skills', agentsPath: '' },
  { name: 'cursor', skillsPath: '.agents/skills', agentsPath: '.cursor/agents' },
];

describe('toolGroups', () => {
  it('puts tools that share a skills folder in one group', () => {
    expect(toolGroups(tools, ['claude', 'codex', 'cursor'])).toEqual([
      { tools: ['claude'], skillsPath: '.claude/skills' },
      { tools: ['codex', 'cursor'], skillsPath: '.agents/skills' },
    ]);
  });
});

describe('projectRows', () => {
  it('adds the folders only mcp.projects names, undeclared', () => {
    const list = { projects: [{ root: '~/a', path: '/home/u/a', name: 'a', targets: ['claude'], skills: null, agents: null, groups: [], missing: false, hasOwnConfig: false }], convertible: [], tools };
    const mcp = { source: { projects: { '/home/u/a': {}, '/home/u/work/b': {} } }, projectConfigs: ['/home/u/work/b'] } as unknown as Parameters<typeof projectRows>[1];
    const rows = projectRows(list, mcp);
    expect(rows.map((p) => [p.name, p.declared, p.hasOwnConfig])).toEqual([['a', true, false], ['b', false, true]]);
  });
});

describe('projects and hooks', () => {
  const inventory = (projects: HookInventory['source']['projects'], changes: HookChange[] = []) =>
    ({ source: { path: '', configPath: '', entries: {}, projects }, projectConfigs: ['/home/u/work/b'], targets: [], paths: {}, plan: { revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes }, previewError: '', backups: [], unmanaged: [] }) as HookInventory;
  const list = { projects: [{ root: '~/a', path: '/home/u/a', name: 'a', targets: ['claude'], skills: null, agents: null, groups: [], missing: false, hasOwnConfig: false }], convertible: [], tools };

  it('adds the folders only hooks.projects names, undeclared, with their own-config flag', () => {
    const rows = projectRows(list, undefined, inventory({ '/home/u/a': {}, '/home/u/work/b': {} }));
    expect(rows.map((p) => [p.name, p.declared, p.hasOwnConfig])).toEqual([['a', true, false], ['b', false, true]]);
  });

  it("counts a project's own hook changes and never another root's", () => {
    const hooks = inventory({ '/home/u/a': {}, '/home/u/z': {} }, [
      { target: 'codex', path: '/home/u/a/.codex/hooks.json', name: 'lint', root: '/home/u/a', action: 'add' },
      { target: 'codex', path: '/home/u/z/.codex/hooks.json', name: 'x', root: '/home/u/z', action: 'conflict' },
      { target: 'codex', path: '/home/.codex/hooks.json', name: 'g', action: 'add' },
    ]);
    const [a] = projectRows(list, undefined, hooks);
    expect(projectHealth(a, [], undefined, hooks)).toEqual({ state: 'pending', count: 1 });
  });

  it('reports a conflict in the projects hook files', () => {
    const hooks = inventory({ '/home/u/a': {} }, [{ target: 'codex', path: '/p', name: 'lint', root: '/home/u/a', action: 'conflict' }]);
    const [a] = projectRows(list, undefined, hooks);
    expect(projectHealth(a, [], undefined, hooks)).toEqual({ state: 'conflict', count: 1 });
  });
});
