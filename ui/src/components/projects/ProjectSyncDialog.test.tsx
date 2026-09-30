import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import type { Target } from '../../api/client';
import { hooksApi } from '../../api/hooks';
import type { HookPlan } from '../../api/hooks';
import { mcpApi } from '../../api/mcp';
import type { MCPPlan } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import ProjectSyncDialog from './ProjectSyncDialog';
import type { ProjectRow } from './projectView';

vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { diff: vi.fn(), sync: vi.fn() } }));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { list: vi.fn(), preview: vi.fn(), syncProject: vi.fn(), configure: vi.fn() } }));
vi.mock('../../api/hooks', async (load) => ({ ...await load<typeof import('../../api/hooks')>(), hooksApi: { list: vi.fn(), preview: vi.fn(), syncProject: vi.fn(), configure: vi.fn() } }));

const project = { root: '~/work/app', path: '/work/app', name: 'app', declared: true, targets: ['claude'], skills: null, agents: null, groups: [], missing: false, hasOwnConfig: false } as ProjectRow;
const targets = [
  { name: 'app@claude', project: '/work/app', mode: 'merge', path: '/work/app/.claude/skills' },
  { name: 'claude', mode: 'merge', path: '/home/me/.claude/skills' },
] as Target[];
const plan: MCPPlan = {
  revision: 'r1', sourcePath: '', blocked: false,
  changes: [
    { target: 'cursor', path: '/work/app/.cursor/mcp.json', name: 'docs', root: '/work/app', action: 'add' },
    { target: 'cursor', path: '/home/me/.cursor/mcp.json', name: 'shared', action: 'add' },
  ],
};

const hookPlan: HookPlan = {
  revision: 'h1', fingerprint: 'fp', sourcePath: '', blocked: false,
  changes: [
    { target: 'codex', path: '/work/app/.codex/hooks.json', name: 'lint', root: '/work/app', action: 'add' },
    { target: 'codex', path: '/home/me/.codex/hooks.json', name: 'global-hook', action: 'add' },
    { target: 'claude', path: '/work/other/.claude/settings.json', name: 'other-hook', root: '/work/other', action: 'add' },
  ],
};
const inventory = (plan: HookPlan | null) => ({ source: { path: '', configPath: '', entries: {}, projects: { '/work/app': {} } }, targets: [], paths: {}, plan, previewError: '', backups: [], unmanaged: [] });

describe('Project sync dialog', () => {
  it('previews and syncs only this project, leaving global changes out', async () => {
    vi.mocked(api.diff).mockResolvedValue({ diffs: [
      { target: 'app@claude', items: [{ skill: 'team-a', action: 'link', reason: 'new' }] },
      { target: 'claude', items: [{ skill: 'global-only', action: 'link', reason: 'new' }] },
    ] } as Awaited<ReturnType<typeof api.diff>>);
    vi.mocked(mcpApi.list).mockResolvedValue({ source: { path: '', configPath: '', targets: null, servers: {}, projects: { '/work/app': {} } }, projectConfigs: [], paths: {}, detected: [], plan, previewError: '', backups: [], unmanaged: [] });
    vi.mocked(mcpApi.preview).mockResolvedValue({ ...plan, revision: 'r2' });
    vi.mocked(api.sync).mockResolvedValue({ results: [], warnings: [] } as unknown as Awaited<ReturnType<typeof api.sync>>);
    vi.mocked(mcpApi.syncProject).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(hooksApi.list).mockResolvedValue(inventory(hookPlan));
    vi.mocked(hooksApi.preview).mockResolvedValue({ ...hookPlan, revision: 'h2' });
    vi.mocked(hooksApi.syncProject).mockResolvedValue({ applied: [], backupIds: [] });
    const user = userEvent.setup();
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ProjectSyncDialog open onClose={vi.fn()} project={project} targets={targets} /></I18nProvider></QueryClientProvider></MemoryRouter>);

    // A row per target: the project's own, never the global one; MCP rows name their servers.
    expect(await screen.findByText('app@claude')).toBeInTheDocument();
    expect(screen.getByText('docs')).toBeInTheDocument();
    expect(screen.queryByText('claude')).not.toBeInTheDocument();
    expect(screen.queryByText('shared')).not.toBeInTheDocument();
    expect(await screen.findByText(/lint/)).toBeInTheDocument();
    expect(screen.queryByText('global-hook')).not.toBeInTheDocument();
    expect(screen.queryByText('other-hook')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Sync Now' }));
    expect(await screen.findByText('app is synced.')).toBeInTheDocument();
    expect(api.sync).toHaveBeenCalledWith({ force: false, project: '~/work/app' });
    await waitFor(() => expect(mcpApi.syncProject).toHaveBeenCalledWith('/work/app', 'r2'));
    expect(mcpApi.configure).not.toHaveBeenCalled();
    await waitFor(() => expect(hooksApi.syncProject).toHaveBeenCalledWith('/work/app', 'h2'));
    expect(hooksApi.configure).not.toHaveBeenCalled();
  });

  it('syncs MCP when only the config needs saving without settings 0.23.0 retired', async () => {
    vi.mocked(api.diff).mockResolvedValue({ diffs: [{ target: 'app@claude', items: [{ skill: 'team-a', action: 'link', reason: 'new' }] }] } as Awaited<ReturnType<typeof api.diff>>);
    const migrates = { ...plan, changes: [], migrates: true };
    vi.mocked(mcpApi.list).mockResolvedValue({ source: { path: '', configPath: '', targets: null, servers: {}, projects: { '/work/app': {} } }, projectConfigs: [], paths: {}, detected: [], plan: migrates, previewError: '', backups: [], unmanaged: [] });
    vi.mocked(mcpApi.preview).mockResolvedValue({ ...migrates, revision: 'r2' });
    vi.mocked(api.sync).mockResolvedValue({ results: [], warnings: [] } as unknown as Awaited<ReturnType<typeof api.sync>>);
    vi.mocked(mcpApi.syncProject).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(hooksApi.list).mockResolvedValue(inventory(null));
    const user = userEvent.setup();
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ProjectSyncDialog open onClose={vi.fn()} project={project} targets={targets} /></I18nProvider></QueryClientProvider></MemoryRouter>);

    await user.click(await screen.findByRole('button', { name: 'Sync Now' }));
    await waitFor(() => expect(mcpApi.syncProject).toHaveBeenCalledWith('/work/app', 'r2'));
  });

  it('lists a failed target like the Sync page, without Turn on Force', async () => {
    vi.mocked(api.diff).mockResolvedValue({ diffs: [{ target: 'app@claude', items: [{ skill: 'team-a', action: 'link', reason: 'new' }] }] } as Awaited<ReturnType<typeof api.diff>>);
    vi.mocked(mcpApi.list).mockResolvedValue({ source: { path: '', configPath: '', targets: null, servers: {}, projects: { '/work/app': {} } }, projectConfigs: [], paths: {}, detected: [], plan: { ...plan, changes: [] }, previewError: '', backups: [], unmanaged: [] });
    vi.mocked(hooksApi.list).mockResolvedValue(inventory(null));
    const message = 'app@claude: sync failed: conflict - symlink points to /x';
    vi.mocked(api.sync).mockResolvedValue({ results: [{ target: 'app@claude', linked: [], updated: [], skipped: [], pruned: [] }], warnings: [message, 'backup skipped'],
      failed: [{ target: 'app@claude', part: 'skill', error: 'conflict - symlink points to /x', message, conflict: true }] } as unknown as Awaited<ReturnType<typeof api.sync>>);
    const user = userEvent.setup();
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ProjectSyncDialog open onClose={vi.fn()} project={project} targets={targets} /></I18nProvider></QueryClientProvider></MemoryRouter>);

    await user.click(await screen.findByRole('button', { name: 'Sync Now' }));
    expect(await screen.findByRole('heading', { name: '1 target failed' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open target' })).toHaveAttribute('href', '/targets/app%40claude');
    expect(screen.queryByRole('button', { name: 'Turn on Force' })).not.toBeInTheDocument();
    expect(screen.getByText('backup skipped')).toBeInTheDocument();
    expect(screen.queryByText(message)).not.toBeInTheDocument();
    expect(screen.queryByText('app is synced.')).not.toBeInTheDocument();
  });

  // A row per file counts the conflicts; the note says which entry and why.
  it('names each MCP conflict with its reason', async () => {
    const conflict = { target: 'cursor', path: '/work/app/.cursor/mcp.json', name: 'shared', root: '/work/app', action: 'conflict', message: 'existing entry is not managed; import it to explicitly adopt it' };
    vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as unknown as Awaited<ReturnType<typeof api.diff>>);
    vi.mocked(mcpApi.list).mockResolvedValue({ source: { path: '', configPath: '', targets: null, servers: {}, projects: { '/work/app': {} } }, projectConfigs: [], paths: {}, detected: [], plan: { ...plan, blocked: true, changes: [conflict] }, previewError: '', backups: [], unmanaged: [] });
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ProjectSyncDialog open onClose={vi.fn()} project={project} targets={targets} /></I18nProvider></QueryClientProvider></MemoryRouter>);

    expect((await screen.findByText('Cursor · shared')).parentElement).toHaveTextContent('The Agent already has this entry, not yet managed by skillshare');
  });
});
