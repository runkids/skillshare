import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type DiffTarget, type Target } from '../../api/client';
import { mcpApi, type MCPPlan } from '../../api/mcp';
import { countChanges, countEdited, extraGroups, MCP_CHANGED, pendingCount, resourceGroups, runSync } from './syncView';

vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { sync: vi.fn(), syncExtras: vi.fn() } }));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { preview: vi.fn(), configure: vi.fn() } }));

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

describe('extraGroups', () => {
  it('keeps files edited in the target unless force is on', () => {
    const extras = [{ name: 'rules', target: '/home/rules', mode: 'merge', items: [
      { action: 'create', file: 'a.md', reason: 'missing in target' },
      { action: 'update', file: 'b.md', reason: 'not a symlink' },
    ] }] as Parameters<typeof extraGroups>[0];
    expect([countChanges(extraGroups(extras, false)), countChanges(extraGroups(extras, true)), countEdited(extraGroups(extras, false))]).toEqual([1, 2, 1]);
  });
});

describe('pendingCount', () => {
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

  it('stops before MCP when its changes differ after syncing resources', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValueOnce(plan).mockResolvedValueOnce({ ...plan, revision: 'after', changes: [{ ...change, action: 'remove' }] });
    await expect(runSync(run)).rejects.toThrow(MCP_CHANGED);
    expect(mcpApi.configure).not.toHaveBeenCalled();
  });
});
