import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type DiffTarget, type Target } from '../../api/client';
import { hooksApi, type HookPlan } from '../../api/hooks';
import { mcpApi, type MCPPlan } from '../../api/mcp';
import { changeSets, countChanges, countEdited, discardable, extraGroups, failureExplanation, groupByFolder, groupInSync, HOOKS_CHANGED, hooksGroups, MCP_CHANGED, mcpGroups, otherWarnings, pendingCount, resourceGroups, runSync, type SyncFailure } from './syncView';

vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { sync: vi.fn(), syncExtras: vi.fn() } }));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { preview: vi.fn(), configure: vi.fn() } }));
vi.mock('../../api/hooks', async (load) => ({ ...await load<typeof import('../../api/hooks')>(), hooksApi: { preview: vi.fn(), configure: vi.fn(), syncProject: vi.fn() } }));

const target = (name: string, mode = 'merge') => ({ name, mode, path: `/home/${name}/skills` }) as Target;
const diff: DiffTarget[] = [
  { target: 'claude', items: [
    { skill: 'pdf', action: 'link', reason: 'source only' },
    { skill: 'notes', action: 'skip', reason: 'local copy (sync --force to replace)' },
    { skill: 'scratch', action: 'local', reason: 'local only' },
    { skill: 'reviewer.md', action: 'link', reason: 'source only', kind: 'agent' },
  ] },
  { target: 'codex', items: [{ skill: 'scratch', action: 'local', reason: 'local only' }] },
];

describe('resourceGroups', () => {
  it('counts kept local copies only when force is on', () => {
    const targets = [target('claude'), target('codex')];
    const all = new Set(['skill', 'agent'] as const);
    expect([countChanges(resourceGroups(diff, targets, all, false).groups), countChanges(resourceGroups(diff, targets, all, true).groups)]).toEqual([2, 3]);
  });

  it('shows a kept legacy entry without counting it, even with force', () => {
    const kept: DiffTarget[] = [{ target: 'claude', items: [{ skill: 'bmad-ux', action: 'kept', reason: 'local folder; the skill stays at _bmad__ux' }] }];
    const groups = resourceGroups(kept, [target('claude', 'copy')], new Set(['skill'] as const), true).groups;
    expect([countChanges(groups), groups[0].rows[0].detail]).toEqual([0, 'local folder; the skill stays at _bmad__ux']);
  });

  it('keeps targets apart when their kept entries sit at different old names', () => {
    const kept: DiffTarget[] = [
      { target: 'claude', items: [{ skill: 'bmad-ux', action: 'kept', reason: 'local folder; the skill stays at _bmad__ux' }] },
      { target: 'cursor', items: [{ skill: 'bmad-ux', action: 'kept', reason: 'local folder; the skill stays at ux' }] },
    ];
    const groups = resourceGroups(kept, [target('claude', 'copy'), target('cursor', 'copy')], new Set(['skill'] as const), false).groups;
    expect(changeSets(groups)).toHaveLength(2);
  });

  it('says a copy-mode copy in a merge target is replaced by a link', () => {
    const copied: DiffTarget[] = [{ target: 'claude', items: [{ skill: 'pdf', action: 'update', reason: 'copy mode copy (sync replaces with link)' }] }];
    expect(resourceGroups(copied, [target('claude')], new Set(['skill']), false).groups[0].rows[0].text).toBe('sync.row.copyToLink');
  });

  it('treats a target with only local items as in sync', () => {
    expect(resourceGroups(diff, [target('claude'), target('codex')], new Set(['skill', 'agent']), false).inSync).toEqual(['codex']);
  });

  it('says an ignored skill is pruned because it is ignored', () => {
    const pruned: DiffTarget[] = [{ target: 'claude', items: [{ skill: 'team__drafts', action: 'prune', reason: 'orphan symlink' }] }];
    expect(resourceGroups(pruned, [target('claude')], new Set(['skill']), false, { skill: ['team/drafts'] }).groups[0].rows[0].text).toBe('sync.row.pruneIgnored');
  });

  it('leaves out agents when only skills are included', () => {
    const { groups } = resourceGroups(diff, [target('claude')], new Set(['skill']), false);
    expect(groups[0].rows.map((r) => r.name)).toEqual(['pdf', 'notes']);
  });
});

describe('changeSets', () => {
  it('puts targets with the same changes in one set', () => {
    const same: DiffTarget[] = ['claude', 'codex', 'cursor'].map((name) => ({ target: name, items: [{ skill: name === 'cursor' ? 'other' : 'pdf', action: 'link', reason: 'new' }] }));
    const { groups } = resourceGroups(same, ['claude', 'codex', 'cursor'].map((n) => target(n)), new Set(['skill']), false);
    expect(changeSets(groups).map((set) => set.targets.map((g) => g.name))).toEqual([['claude', 'codex'], ['cursor']]);
  });
});

describe('discardable', () => {
  const skill = (flatName: string) => ({ name: flatName, kind: 'skill', flatName, relPath: flatName, sourcePath: '', isInRepo: false }) as const;
  const newRows: DiffTarget[] = [{ target: 'claude', items: [
    { skill: 'fresh', action: 'link', reason: 'new' },
    { skill: 'old', action: 'link', reason: 'new' },
    { skill: 'handmade', action: 'link', reason: 'new' },
    { skill: 'gone', action: 'link', reason: 'missing in target' },
  ] }];

  it('keeps skills already synced to another target and ones to copy again', () => {
    const { groups } = resourceGroups(newRows, [target('claude')], new Set(['skill']), false);
    const skills = ['fresh', 'old', 'handmade', 'gone'].map((n) => skill(n));
    const expected = (name: string) => (name === 'old' ? ['claude', 'codex'] : ['claude']);
    expect(discardable(groups, skills, expected).map((s) => s.flatName)).toEqual(['fresh', 'handmade']);
  });

  it('leaves a tracked repo alone when only some of its skills are new', () => {
    // Uninstalling a tracked repo takes all of it, so one new skill must not trash synced siblings.
    const rows: DiffTarget[] = [{ target: 'claude', items: [{ skill: '_team__new', action: 'link', reason: 'new' }, { skill: '_solo__a', action: 'link', reason: 'new' }, { skill: '_solo__b', action: 'link', reason: 'new' }] }];
    const { groups } = resourceGroups(rows, [target('claude')], new Set(['skill']), false);
    const inRepo = (flatName: string, repoPath: string) => ({ ...skill(flatName), isInRepo: true, repoPath });
    const skills = [inRepo('_team__new', '_team'), inRepo('_team__old', '_team'), inRepo('_solo__a', '_solo'), inRepo('_solo__b', '_solo')];
    expect(discardable(groups, skills, () => ['claude']).map((s) => s.flatName)).toEqual(['_solo__a', '_solo__b']);
  });
});

describe('extraGroups', () => {
  it('keeps files edited in the target unless force is on', () => {
    const extras = [{ name: 'rules', target: '/home/rules', mode: 'merge', items: [
      { action: 'create', file: 'a.md', reason: 'missing in target' },
      { action: 'update', file: 'b.md', reason: 'not a symlink' },
    ] }] as Parameters<typeof extraGroups>[0];
    expect([countChanges(extraGroups(extras, false)), countChanges(extraGroups(extras, true)), countEdited(extraGroups(extras, false))]).toEqual([1, 2, 1]);
  });
});

describe('hooksGroups', () => {
  it('makes one group per native file and names the project a file belongs to', () => {
    const plan: HookPlan = { revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [
      { target: 'codex', path: '/home/me/.codex/hooks.json', name: 'lint', action: 'add' },
      { target: 'codex', path: '/work/app/.codex/hooks.json', name: 'fmt', root: '/work/app', action: 'conflict', message: 'not owned' },
      { target: 'codex', path: '/home/me/.codex/hooks.json', name: 'same', action: 'unchanged' },
    ] };
    const groups = hooksGroups(plan);
    expect(groups.map((g) => [g.path, g.project, g.rows.map((r) => r.name)])).toEqual([
      ['/home/me/.codex/hooks.json', undefined, ['lint']],
      ['/work/app/.codex/hooks.json', '/work/app', ['fmt']],
    ]);
    expect(countChanges(groups)).toBe(1);
  });
});

describe('pendingCount', () => {
  it('counts hook changes unless the hooks plan is blocked', () => {
    const changes = [{ target: 'codex', path: '/h.json', name: 'lint', action: 'add' }];
    expect(pendingCount(diff, [target('claude'), target('codex')], [], null, { revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes })).toBe(3);
    expect(pendingCount(diff, [target('claude'), target('codex')], [], null, { revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: true, changes })).toBe(2);
  });

  it('leaves out kept copies and a blocked MCP plan', () => {
    const plan = { blocked: true, changes: [{ target: 'claude', path: '/claude.json', name: 'docs', action: 'add' }] } as unknown as MCPPlan;
    expect(pendingCount(diff, [target('claude'), target('codex')], [], plan)).toBe(2);
  });
});

describe('runSync', () => {
  const change = { target: 'claude', path: '/claude.json', name: 'docs', action: 'add' };
  const plan: MCPPlan = { revision: 'before', sourcePath: '/config.yaml', blocked: false, changes: [change] };
  const run = { resources: 'both' as const, extras: true, mcp: plan, force: false };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(mcpApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(api.sync).mockResolvedValue({ results: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] });
    vi.mocked(api.syncExtras).mockResolvedValue({ extras: [] });
  });

  it('writes nothing when the MCP plan changed since it was reviewed', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValueOnce({ ...plan, changes: [{ ...change, action: 'update' }] });
    await expect(runSync(run)).rejects.toThrow(MCP_CHANGED);
    expect(api.sync).not.toHaveBeenCalled();
  });

  it('applies MCP with the fresh revision when only the revision moved', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValueOnce({ ...plan, revision: 'mid', changes: [change, { ...change, name: 'other', action: 'unchanged' }] }).mockResolvedValueOnce({ ...plan, revision: 'after' });
    await runSync(run);
    expect(mcpApi.configure).toHaveBeenCalledWith({}, 'after', true);
  });

  it('reports a failed extras target and still applies MCP', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValue(plan);
    vi.mocked(api.syncExtras).mockResolvedValueOnce({ extras: [{ name: 'rules', targets: [
      { target: 'claude', mode: 'merge', synced: 0, skipped: 0, pruned: 0, error: 'permission denied' },
      { target: 'codex', mode: 'merge', synced: 1, skipped: 0, pruned: 0 },
    ] }] });
    const { failures } = await runSync(run);
    expect(failures).toEqual([{ target: 'claude', part: 'extra', extra: 'rules', error: 'permission denied' }]);
    expect(mcpApi.configure).toHaveBeenCalled();
  });

  it('reports the targets the server failed to sync', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValue(plan);
    vi.mocked(api.sync).mockResolvedValueOnce({ results: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [],
      failed: [{ target: 'codex', part: 'skill', error: 'conflict - symlink points to /x', message: 'codex: sync failed: conflict - symlink points to /x', conflict: true }] });
    const { failures } = await runSync(run);
    expect(failures).toEqual([{ target: 'codex', part: 'skill', error: 'conflict - symlink points to /x', conflict: true }]);
  });

  it('stops before MCP when its changes differ after syncing resources', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValueOnce(plan).mockResolvedValueOnce({ ...plan, revision: 'after', changes: [{ ...change, action: 'remove' }] });
    await expect(runSync(run)).rejects.toThrow(MCP_CHANGED);
    expect(mcpApi.configure).not.toHaveBeenCalled();
  });
});

describe('mcpGroups', () => {
  const plan = { revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [
    { target: 'claude', path: '/home/u/.claude.json', name: 'context7', root: '/work/app', switch: true, action: 'remove' },
    { target: 'opencode', path: '/work/app/opencode.json', name: 'context7', root: '/work/app', switch: true, action: 'add' },
    { target: 'opencode', path: '/work/app/opencode.json', name: 'own', root: '/work/app', action: 'add' },
  ] };

  it('says a switch-only entry turns the server off or back on in the project', () => {
    expect(mcpGroups(plan).flatMap((g) => g.rows.map((r) => r.text))).toEqual(['sync.row.mcp.switch.remove', 'sync.row.mcp.switch.add', 'sync.row.mcp.add']);
  });

  it("names the project on Claude's off list, which sits in the global file", () => {
    expect(mcpGroups(plan).map((g) => g.project)).toEqual(['/work/app', undefined]);
  });

  // Issue #303: taking over an entry the Agent already has is a change Sync applies.
  it('counts taking over an existing entry as a change', () => {
    const adopt = { ...plan, changes: [{ target: 'claude', path: '/home/u/.claude.json', name: 'mcp-test', action: 'adopt' }] };
    expect(countChanges(mcpGroups(adopt))).toBe(1);
  });
});

describe('groupInSync', () => {
  it('keeps global targets apart and lists each project with its tools', () => {
    expect(groupInSync(['api-server@claude', 'api-server@opencode', 'claude', 'shop-web@cursor', 'universal'])).toEqual({
      global: ['claude', 'universal'],
      projects: [{ project: 'api-server', tools: ['claude', 'opencode'] }, { project: 'shop-web', tools: ['cursor'] }],
    });
  });
});

describe('groupByFolder', () => {
  it('writes a shared folder once', () => {
    expect(groupByFolder(['security/a', 'security/b', 'top'])).toEqual([{ folder: 'security/', items: ['a', 'b'] }, { folder: '', items: ['top'] }]);
  });
});

describe('otherWarnings', () => {
  it('leaves out warnings that a failed target already reports', () => {
    const res = { results: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [],
      warnings: ['codex: sync failed: denied', 'backup skipped'],
      failed: [{ target: 'codex', part: 'skill' as const, error: 'denied', message: 'codex: sync failed: denied' }] };
    expect(otherWarnings(res)).toEqual(['backup skipped']);
  });
});

describe('failureExplanation', () => {
  const failure = (error: string, extra: Partial<SyncFailure> = {}): SyncFailure => ({ target: 'codex', part: 'skill', error, ...extra });

  it.each([
    ['symlink conflict', failure('conflict - symlink points to /x (use --force to override)', { conflict: true }), 'sync.result.why.conflict'],
    ['permission denied', failure('mkdir /home/me/.codex/skills: permission denied'), 'sync.result.why.permission'],
    ['read-only file system', failure('open /mnt/skills/a: read-only file system'), 'sync.result.why.readOnly'],
    ['not a directory', failure('mkdir /home/me/.codex/skills: not a directory'), 'sync.result.why.notDir'],
    ['no such file or directory', failure('lstat /home/me/.codex: no such file or directory'), 'sync.result.why.missing'],
    ['Windows access denial', failure('mkdir C:\\Users\\me\\.codex\\skills: Access is denied.'), 'sync.result.why.permission'],
    ['Windows missing path', failure('open C:\\x: The system cannot find the path specified.'), 'sync.result.why.missing'],
    ['invalid target settings', failure('unknown mode "mirror"', { part: 'config' }), 'sync.result.why.config'],
  ])('explains a %s', (_, f, key) => {
    expect(failureExplanation(f)).toBe(key);
  });

  it('leaves an unknown error unexplained', () => {
    expect(failureExplanation(failure('disk quota exceeded'))).toBeNull();
  });

  describe('hooks', () => {
    const hookChange = { target: 'codex', path: '/h.json', name: 'lint', action: 'add' };
    const hooks: HookPlan = { revision: 'h1', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [hookChange] };

    beforeEach(() => vi.clearAllMocks());

    it('applies the fresh revision of a reviewed hooks plan, after checking it is unchanged', async () => {
      vi.mocked(hooksApi.preview).mockResolvedValue({ ...hooks, revision: 'h2' });
      vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
      await runSync({ resources: null, extras: false, mcp: null, hooks, force: false });
      expect(hooksApi.configure).toHaveBeenCalledWith({}, 'h2', true);
    });

    it('applies nothing when the hooks plan moved since it was reviewed', async () => {
      vi.mocked(hooksApi.preview).mockResolvedValue({ ...hooks, changes: [{ ...hookChange, action: 'update' }] });
      await expect(runSync({ resources: 'both', extras: false, mcp: null, hooks, force: false })).rejects.toThrow(HOOKS_CHANGED);
      expect(api.sync).not.toHaveBeenCalled();
      expect(hooksApi.configure).not.toHaveBeenCalled();
    });

    it('refuses a same-shaped plan whose content fingerprint differs, before any resource is written', async () => {
      vi.mocked(hooksApi.preview).mockResolvedValue({ ...hooks, fingerprint: 'other-command' });
      await expect(runSync({ resources: 'both', extras: false, mcp: null, hooks, force: false })).rejects.toThrow(HOOKS_CHANGED);
      expect(api.sync).not.toHaveBeenCalled();
      expect(hooksApi.configure).not.toHaveBeenCalled();
    });

    it('refuses hooks when the fingerprint changes during the run, after the resources were synced', async () => {
      vi.mocked(api.sync).mockResolvedValue({ results: [], failed: [] } as never);
      vi.mocked(hooksApi.preview).mockResolvedValueOnce({ ...hooks, revision: 'h2' }).mockResolvedValueOnce({ ...hooks, revision: 'h3', fingerprint: 'edited-mid-run' });
      await expect(runSync({ resources: 'both', extras: false, mcp: null, hooks, force: false })).rejects.toThrow(HOOKS_CHANGED);
      expect(api.sync).toHaveBeenCalledTimes(1);
      expect(hooksApi.configure).not.toHaveBeenCalled();
    });

    it("syncs one project's root and ignores the other roots' changes", async () => {
      const reviewed: HookPlan = { ...hooks, changes: [{ ...hookChange, root: '/work/app' }] };
      vi.mocked(hooksApi.preview).mockResolvedValue({ ...reviewed, revision: 'h3', changes: [...reviewed.changes, { ...hookChange, name: 'other', root: '/work/other' }] });
      vi.mocked(hooksApi.syncProject).mockResolvedValue({ applied: [], backupIds: [] });
      await runSync({ resources: null, extras: false, mcp: null, hooks: reviewed, force: false, project: { root: '~/work/app', path: '/work/app' } });
      expect(hooksApi.syncProject).toHaveBeenCalledWith('/work/app', 'h3');
      expect(hooksApi.configure).not.toHaveBeenCalled();
    });

    it('refuses a project whose root now has a conflict', async () => {
      const reviewed: HookPlan = { ...hooks, changes: [{ ...hookChange, root: '/work/app' }] };
      vi.mocked(hooksApi.preview).mockResolvedValue({ ...reviewed, changes: [{ ...hookChange, root: '/work/app', action: 'conflict' }] });
      await expect(runSync({ resources: null, extras: false, mcp: null, hooks: reviewed, force: false, project: { root: '~/work/app', path: '/work/app' } })).rejects.toThrow(HOOKS_CHANGED);
    });
  });
});
