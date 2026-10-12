import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import UpdatePage, { countUpdates, isForceRetryable, updateUnits } from './UpdatePage';
import { api } from '../api/client';
import { queryKeys } from '../lib/queryKeys';

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listSkills: vi.fn(),
      checkStream: vi.fn(),
      updateAllStream: vi.fn(),
      missingConfigEntries: vi.fn(),
      installFromConfig: vi.fn(),
      batchUninstall: vi.fn(),
      sync: vi.fn().mockResolvedValue({ results: [] }),
      diff: vi.fn(),
      listTargets: vi.fn(),
    },
  };
});

function renderUpdatePage(kind: 'skill' | 'agent' = 'skill', queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})) {
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <UpdatePage kind={kind} />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

async function findRow(name: string) {
  const row = (await screen.findByText(name, { selector: '.nm' })).closest('.ss-r');
  expect(row).not.toBeNull();
  return within(row as HTMLElement);
}

function cacheStatus(name: string, status: string) {
  localStorage.setItem(
    'skillshare.updateCheckCache.global',
    JSON.stringify({ version: 2, items: { [name]: { status, checkedAt: new Date(Date.now() - 60_000).toISOString() } } }),
  );
}

const noResults = { results: [], summary: { updated: 0, upToDate: 0, blocked: 0, errors: 0, skipped: 0 } };
const emptyDiff = { diffs: [], ignored_count: 0, ignored_skills: [], ignore_root: '', ignore_repos: [] };

describe('UpdatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.mocked(api.missingConfigEntries).mockResolvedValue({ entries: [], file: '.metadata.json' });
    vi.mocked(api.diff).mockResolvedValue(emptyDiff);
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [{ name: 'claude', path: '/targets/claude', mode: 'merge', targetNaming: 'flat', status: 'merged', linkedCount: 1, localCount: 0, include: [], exclude: [], expectedSkillCount: 1, skillsEnabled: true }], sourceSkillCount: 1 });
  });

  const nestedSkill = {
    name: 'agent-browser',
    kind: 'skill' as const,
    flatName: 'tools__agent-browser',
    relPath: 'tools/agent-browser',
    sourcePath: '/skills/tools/agent-browser',
    isInRepo: false,
    source: 'https://github.com/vercel-labs/agent-browser/skills/agent-browser',
    type: 'github-subdir',
  };

  it('sends relative paths when updating nested GitHub-installed skills', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [nestedSkill],
    });
    vi.mocked(api.updateAllStream).mockImplementation((_onStart, _onResult, onDone) => {
      queueMicrotask(() => onDone(noResults));
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();

    await user.click(await screen.findByRole('checkbox', { name: 'agent-browser' }));
    await user.click(screen.getByRole('button', { name: /update 1 selected/i }));

    expect(api.updateAllStream).toHaveBeenCalledWith(
      expect.any(Function),
      expect.any(Function),
      expect.any(Function),
      expect.any(Function),
      { names: ['tools/agent-browser'], force: false },
    );
  });

  it('updates a tracked repo once for all of its skills', async () => {
    const inRepo = (name: string) => ({
      ...nestedSkill,
      name,
      flatName: `_team__${name}`,
      relPath: `_team/${name}`,
      isInRepo: true,
      source: 'https://github.com/example/team',
    });
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [inRepo('lint'), inRepo('review')] });
    vi.mocked(api.updateAllStream).mockImplementation((_onStart, _onResult, onDone) => {
      queueMicrotask(() => onDone(noResults));
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();

    await user.click(await screen.findByRole('button', { name: /update all/i }));

    expect(vi.mocked(api.updateAllStream).mock.calls[0][4]).toEqual({ names: ['_team'], force: false });
  });

  it.each(['skill', 'agent'] as const)('does not offer a %s sync write when the fresh diff only changes the other kind', async (kind) => {
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [{ ...nestedSkill, kind }] });
    const result = { name: nestedSkill.relPath, action: 'updated' as const, isRepo: false };
    vi.mocked(api.updateAllStream).mockImplementation((_onStart, onResult, onDone) => {
      queueMicrotask(() => {
        onResult(result);
        onDone({ results: [result], summary: { ...noResults.summary, updated: 1 } });
      });
      return { close: vi.fn() } as unknown as EventSource;
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(queryKeys.diff(), { ...emptyDiff, diffs: [{ target: 'claude', items: [{ skill: 'old', kind, action: 'link', reason: 'source only' }] }] });
    vi.mocked(api.diff).mockResolvedValue({ ...emptyDiff, diffs: [{ target: 'claude', items: [{ skill: 'other', kind: kind === 'skill' ? 'agent' : 'skill', action: 'link', reason: 'source only' }] }] });

    const user = userEvent.setup();
    renderUpdatePage(kind, queryClient);
    await user.click(await screen.findByRole('button', { name: /update all/i }));

    await waitFor(() => expect(api.diff).toHaveBeenCalled());
    await screen.findByText('Everything is up to date. No sync needed.');
    expect(screen.queryByRole('button', { name: 'Sync Now' })).not.toBeInTheDocument();
    expect(api.sync).not.toHaveBeenCalled();
  });

  it('previews pending changes before writing only the updated kind', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [nestedSkill] });
    vi.mocked(api.diff).mockResolvedValue({ ...emptyDiff, diffs: [{ target: 'claude', items: [{ skill: 'tools__agent-browser', kind: 'skill', action: 'link', reason: 'source only' }] }] });
    const result = { name: nestedSkill.relPath, action: 'updated' as const, isRepo: false };
    vi.mocked(api.updateAllStream).mockImplementation((_onStart, onResult, onDone) => {
      queueMicrotask(() => {
        onResult(result);
        onDone({ results: [result], summary: { ...noResults.summary, updated: 1 } });
      });
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();
    await user.click(await screen.findByRole('button', { name: /update all/i }));
    await user.click(await screen.findByRole('button', { name: 'Sync Now' }));
    const dialog = await screen.findByRole('dialog', { name: 'Sync skills' });
    expect(api.sync).not.toHaveBeenCalled();
    vi.mocked(api.diff).mockResolvedValue(emptyDiff);
    await user.click(await within(dialog).findByRole('button', { name: 'Sync Now' }));
    await waitFor(() => expect(api.sync).toHaveBeenCalledWith({ dryRun: false, kind: 'skill' }));
    await user.click(await within(dialog).findByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Sync Now' })).not.toBeInTheDocument());
  });

  it('installs entries missing on disk from config (issue #212)', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [nestedSkill] });
    vi.mocked(api.missingConfigEntries).mockResolvedValue({
      entries: [{ name: '_team-skills', source: 'https://github.com/example/team-skills', tracked: true, branch: 'main' }],
      file: '.metadata.json',
    });
    vi.mocked(api.installFromConfig).mockResolvedValue({ installed: 1, installedRepos: 1, installedRepoSkills: 3, skipped: 0, failed: [] });

    const user = userEvent.setup();
    renderUpdatePage();

    expect(await screen.findByText('_team-skills')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /install missing/i }));

    expect(await screen.findByText(/^Installed 1 tracked repo/)).toBeInTheDocument();
  });

  it('lists skills deleted upstream and prunes them', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [nestedSkill] });
    vi.mocked(api.batchUninstall).mockResolvedValue({ results: [] } as never);
    vi.mocked(api.checkStream).mockImplementation((_a, _b, _c, onDone) => {
      queueMicrotask(() => onDone({ tracked_repos: [], skills: [{ name: 'tools/agent-browser', source: nestedSkill.source, version: '1', status: 'stale' }] }));
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();
    await user.click(await screen.findByRole('button', { name: /check for updates/i }));
    const row = await findRow('agent-browser');
    await user.click(await row.findByRole('button', { name: 'Prune' }));

    await waitFor(() => expect(api.batchUninstall).toHaveBeenCalledWith({ names: ['tools/agent-browser'], force: true }));
  });

  it('does not offer prune from a stale status saved by an earlier session', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [nestedSkill] });
    cacheStatus('tools/agent-browser', 'stale');
    renderUpdatePage();

    await findRow('agent-browser');
    expect(screen.queryByRole('button', { name: /^Prune/ })).not.toBeInTheDocument();
  });

  it('prunes only the skill deleted upstream when another folder has one of the same name', async () => {
    const foo = (dir: string) => ({ ...nestedSkill, name: 'foo', flatName: `${dir}__foo`, relPath: `${dir}/foo`, sourcePath: `/skills/${dir}/foo`, source: `https://github.com/o/r/${dir}/foo` });
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [foo('a'), foo('b')] });
    vi.mocked(api.batchUninstall).mockResolvedValue({ results: [] } as never);
    vi.mocked(api.checkStream).mockImplementation((_a, _b, _c, onDone) => {
      queueMicrotask(() => onDone({ tracked_repos: [], skills: [
        { name: 'a/foo', source: 'https://github.com/o/r/a/foo', version: '1', status: 'up_to_date' },
        { name: 'b/foo', source: 'https://github.com/o/r/b/foo', version: '1', status: 'stale' },
      ] }));
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();
    await user.click(await screen.findByRole('button', { name: /check for updates/i }));
    await user.click(await screen.findByRole('button', { name: 'Prune all' }));

    await waitFor(() => expect(api.batchUninstall).toHaveBeenCalledWith({ names: ['b/foo'], force: true }));
  });
});

describe('update failure message helpers', () => {
  const auditBlocked =
    'security audit failed — findings at/above CRITICAL detected:\n' +
    '  CRITICAL: Prompt injection attempt detected (SKILL.md:28)\n\n' +
    'Use --force to override or --skip-audit to bypass scanning: blocked by security audit';

  it('excludes followed checkouts from the update badge while keeping managed repos', () => {
    const resources = ['_dev', '_managed'].map((repo) => ({
      name: repo, kind: 'skill' as const, flatName: `${repo}__child`, relPath: `${repo}/child`, sourcePath: '', isInRepo: true,
    }));
    const units = updateUnits(resources, 'skill', [{ name: '_dev', target: '/code/dev' }]);
    expect(units.map((unit) => unit.name)).toEqual(['_managed']);
    expect(countUpdates(new Map(resources.map((item) => [item.relPath, { status: 'behind' as const }])), units)).toBe(1);
  });

  it('offers force retry for failures force can actually resolve', () => {
    expect(isForceRetryable(auditBlocked)).toBe(true);
    expect(isForceRetryable('non-fast-forward pull rejected (try force update)')).toBe(true);
  });

  it('hides force retry where retrying with force fails identically', () => {
    expect(
      isForceRetryable('failed to remove existing skill: unlinkat /x/trash.md: permission denied'),
    ).toBe(false);
    // scan failures stay fail-closed, so force cannot get past them
    expect(
      isForceRetryable('post-update audit failed: scanner crashed — rolled back (use --skip-audit to bypass): blocked by security audit'),
    ).toBe(false);
    expect(isForceRetryable(undefined)).toBe(false);
  });

});
