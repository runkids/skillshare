import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import type { Target } from '../api/client';
import { mcpApi } from '../api/mcp';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import SyncPage from './SyncPage';

vi.mock('../api/client', async (load) => ({
  ...await load<typeof import('../api/client')>(),
  api: { listTargets: vi.fn(), listSkills: vi.fn(() => Promise.resolve({ resources: [] })), getSyncMatrix: vi.fn(() => Promise.resolve({ entries: [] })), diff: vi.fn(), diffExtras: vi.fn(), listLog: vi.fn(), skillsOffPreview: vi.fn(), updateTarget: vi.fn(), sync: vi.fn(), syncExtras: vi.fn() },
}));
vi.mock('../api/mcp', async (load) => ({ ...await load<typeof import('../api/mcp')>(), mcpApi: { list: vi.fn() } }));
vi.mock('../api/hooks', async (load) => ({ ...await load<typeof import('../api/hooks')>(), hooksApi: { list: vi.fn(() => Promise.reject(new Error('offline'))) } }));

const target = (name: string, over: Partial<Target> = {}) => ({
  name, path: '/home/me/.agents/skills', mode: 'merge', targetNaming: 'flat', status: 'merged', linkedCount: 3, localCount: 0,
  include: [], exclude: [], expectedSkillCount: 3, skillsEnabled: true, ...over,
}) as Target;

describe('Sync page folder conflicts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target('codex'), target('universal')], sourceSkillCount: 3 });
    vi.mocked(api.diff).mockResolvedValue({
      diffs: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [],
      folder_conflicts: [{ path: '/home/me/.agents/skills', targets: ['codex', 'universal'], keep: 'universal', stop: ['codex'] }],
    });
    vi.mocked(api.diffExtras).mockResolvedValue({ extras: [] });
    vi.mocked(api.listLog).mockResolvedValue({ entries: [] } as never);
    vi.mocked(mcpApi.list).mockResolvedValue({ paths: {}, source: { targets: [], servers: {} } } as never);
    vi.mocked(api.skillsOffPreview).mockResolvedValue({ remove: [], keep: [], sharedWith: 'universal' });
    vi.mocked(api.updateTarget).mockResolvedValue({ success: true });
  });

  it('names the targets that undo each other and stops skills for the one to drop', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><SyncPage /></ToastProvider></I18nProvider></QueryClientProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByText(/codex and universal sync skills to the same folder .*\.agents\/skills with different settings, so each sync undoes the other\./)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Stop syncing skills for codex' }));
    await user.click(await screen.findByRole('button', { name: 'Stop syncing' }));
    await waitFor(() => expect(api.updateTarget).toHaveBeenCalledWith('codex', { skills_enabled: false }));
  });

  it('lists only the settings that differ, keep target first, and links the one to change', async () => {
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target('codex', { mode: 'copy', exclude: ['feature-radar*'] }), target('universal')], sourceSkillCount: 3 });
    render(
      <MemoryRouter>
        <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><SyncPage /></ToastProvider></I18nProvider></QueryClientProvider>
      </MemoryRouter>,
    );
    const columns = await screen.findAllByRole('columnheader');
    expect(columns.map((c) => c.textContent)).toEqual(['universal · keeps writing', 'codex']);
    expect(screen.getAllByRole('rowheader').map((r) => r.textContent)).toEqual(['Sync mode', 'Filters']);
    const cells = screen.getAllByRole('cell').map((c) => c.textContent);
    expect(cells).toEqual(['merge', 'copy', 'all skills', 'excludes feature-radar*']);
    expect(screen.getByRole('link', { name: 'codex' })).toHaveAttribute('href', '/targets/codex');
  });
});

describe('Sync page last sync', () => {
  const renderPage = () => render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><SyncPage /></ToastProvider></I18nProvider></QueryClientProvider>
    </MemoryRouter>,
  );

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target('codex')], sourceSkillCount: 3 });
    vi.mocked(api.diff).mockResolvedValue({ diffs: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] });
    vi.mocked(api.diffExtras).mockResolvedValue({ extras: [] });
    vi.mocked(mcpApi.list).mockResolvedValue({ paths: {}, source: { targets: [], servers: {} } } as never);
  });

  it('shows targets with the same changes once and offers to discard only skills never synced anywhere', async () => {
    const user = userEvent.setup();
    const skill = (flatName: string) => ({ name: flatName, kind: 'skill', flatName, relPath: flatName, sourcePath: '', isInRepo: true });
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target('codex'), target('cursor')], sourceSkillCount: 2 });
    vi.mocked(api.diff).mockResolvedValue({
      diffs: ['codex', 'cursor'].map((name) => ({ target: name, items: [{ skill: 'fresh', action: 'link', reason: 'new' }, { skill: 'old', action: 'link', reason: 'new' }] })),
      ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [],
    } as never);
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [skill('fresh'), skill('old')] } as never);
    // old is already in claude, so only fresh has never been synced.
    const entry = (s: string, target: string) => ({ skill: s, target, status: 'synced', reason: '' });
    vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [entry('fresh', 'codex'), entry('fresh', 'cursor'), entry('old', 'codex'), entry('old', 'cursor'), entry('old', 'claude')] } as never);
    renderPage();

    expect(await screen.findByText('2 targets')).toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: 'Discard all' }));
    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent('fresh');
    expect(dialog).not.toHaveTextContent('old');
  });

  it('does not offer Discard all for a skill a symlink-mode target already exposes', async () => {
    const skill = (flatName: string) => ({ name: flatName, kind: 'skill', flatName, relPath: flatName, sourcePath: '', isInRepo: true });
    const warning = `Target 'codex': skipped broken because SKILL.md name "different" does not match directory name "broken"`;
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target('codex', { targetNaming: 'standard' })], sourceSkillCount: 2 });
    vi.mocked(api.diff).mockResolvedValue({
      diffs: [{ target: 'codex', skippedCount: 1, namingSkippedCount: 1, warnings: [warning], items: [{ skill: 'fresh', action: 'link', reason: 'new' }, { skill: '(skill selection)', action: 'skip', reason: warning }] }],
      ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [],
    } as never);
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [skill('fresh'), skill('broken')] } as never);
    // claude links the whole source folder, so fresh is already live there.
    vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [
      { skill: 'fresh', target: 'codex', status: 'synced', reason: '' },
      { skill: 'fresh', target: 'claude', status: 'na', reason: 'symlink mode — filters not applicable', reasonCode: 'sync_matrix.symlink_filters_not_applicable' },
      { skill: 'broken', target: 'codex', status: 'synced', reason: '' },
    ] } as never);
    const original = renderPage();

    expect(await screen.findByText(/Sync 1 change/)).toBeInTheDocument();
    await waitFor(() => expect(api.getSyncMatrix).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 50));
    expect(screen.queryByRole('button', { name: 'Discard all' })).toBeNull();
    original.unmount();

    // Exercise the actual /sync action, runSync and ToastProvider. An interrupted
    // configured target can yield a targetless sync response before target IO.
    for (const scope of ['global', 'project'] as const) {
      for (const kind of ['entry-only', 'mixed', 'mixed-errors', 'ordinary', 'extras-off'] as const) {
        vi.clearAllMocks();
        const mixed = kind === 'mixed' || kind === 'mixed-errors';
        const interrupted = kind === 'entry-only' || mixed;
        const resourceError = kind === 'mixed-errors';
        const remedy = `skillshare install example-org/hub/rules --kind extra --name rules ${scope === 'project' ? '-p' : '-g'}`;
        const entryWarning = `extra "rules" has an interrupted publication; run: ${remedy}`;
        const healthy = { name: 'notes', targets: [{ target: 'cursor', mode: 'copy', synced: 2, skipped: 1, pruned: 1 }] };
        const extras = interrupted ? [{ name: 'rules', targets: [], warnings: [entryWarning] }, ...(mixed ? [healthy] : [])] : [healthy];
        const resources = {
          results: [{ target: 'cursor', linked: ['guide'], updated: ['reviewer.md'], skipped: ['local'], pruned: ['old'] }],
          ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [],
          ...(resourceError ? {
            warnings: ['codex: sync failed: permission denied', 'resource backup skipped'],
            failed: [{ target: 'codex', part: 'skill' as const, error: 'permission denied', message: 'codex: sync failed: permission denied' }],
          } : {}),
        };
        vi.mocked(api.listTargets).mockResolvedValue({ targets: [target('cursor'), target('codex')], sourceSkillCount: 1 });
        vi.mocked(api.diff).mockResolvedValue({ diffs: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] });
        vi.mocked(api.diffExtras).mockResolvedValue({ extras: [
          ...(interrupted ? [{ name: 'rules', target: '/fixture/rules', mode: 'copy', synced: true, items: [] }] : []),
          ...(kind !== 'entry-only' ? [{ name: 'notes', target: 'cursor', mode: 'copy', synced: false, items: [
            { action: 'create', file: 'a.md', reason: 'new file' },
            { action: 'update', file: 'b.md', reason: 'source changed' },
            { action: 'update', file: 'local.md', reason: 'content differs' },
          ] }] : []),
        ] });
        vi.mocked(api.listLog).mockResolvedValue({ entries: [] } as never);
        vi.mocked(api.sync).mockResolvedValue(resources);
        vi.mocked(api.syncExtras).mockResolvedValue({ extras });
        const user = userEvent.setup();
        const view = renderPage();
        try {
          await waitFor(() => expect(api.diffExtras).toHaveBeenCalled());
          if (kind === 'extras-off') await user.click(screen.getByRole('checkbox', { name: 'Extras' }));
          const syncButton = await screen.findByRole('button', { name: /^Sync(?: \d+ changes?)?$/ });
          await waitFor(() => expect(syncButton).toBeEnabled());
          await user.click(syncButton);
          await waitFor(() => expect(document.querySelector('[data-toast-container] > .ss-toast')).not.toBeNull());
          const toast = document.querySelector('[data-toast-container] > .ss-toast')!;
          expect(api.sync).toHaveBeenCalledExactlyOnceWith({ force: false });
          if (kind === 'extras-off') expect(api.syncExtras).not.toHaveBeenCalled();
          else expect(api.syncExtras).toHaveBeenCalledExactlyOnceWith({ force: false });
          if (interrupted) {
            expect.soft(toast, `${scope}/${kind} warning and remedy`).toHaveTextContent(entryWarning);
            expect.soft(toast.querySelector('svg')?.getAttribute('class'), `${scope}/${kind} warning severity`).toContain('text-[#FFD37A]');
            expect.soft(toast.querySelector('svg')?.getAttribute('class'), `${scope}/${kind} no success severity`).not.toContain('text-[#8FD6A4]');
            expect.soft(toast).not.toHaveTextContent('no files in source');
          } else {
            expect(toast).toHaveTextContent('Sync complete');
            expect(toast.querySelector('svg')?.getAttribute('class')).toContain('text-[#8FD6A4]');
            expect(toast).not.toHaveTextContent(entryWarning);
          }
          if (mixed) {
            expect.soft(toast, `${scope}/${kind} healthy delivery counts`).toHaveTextContent('2 file(s) to 1 target(s)');
            expect.soft(toast, `${scope}/${kind} skipped count`).toHaveTextContent('1 skipped');
          }
          if (resourceError) {
            expect(toast).toHaveTextContent('Sync finished with 1 failure');
            expect(await screen.findByText('permission denied', { exact: true })).toBeInTheDocument();
            expect(screen.getByText('resource backup skipped', { exact: true })).toBeInTheDocument();
            expect(screen.getByText('1 other target synced', { exact: true })).toBeInTheDocument();
          }
        } finally {
          view.unmount();
        }
      }
    }
  });

});
