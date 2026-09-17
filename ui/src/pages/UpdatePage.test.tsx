import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import UpdatePage, { isForceRetryable, stripCliHint } from './UpdatePage';
import { api } from '../api/client';

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listSkills: vi.fn(),
      checkStream: vi.fn(),
      updateAllStream: vi.fn(),
      missingTrackedRepos: vi.fn(),
      rehydrateTrackedRepos: vi.fn(),
    },
  };
});

function renderUpdatePage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <UpdatePage kind="skill" />
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
    JSON.stringify({ version: 1, items: { [name]: { status, checkedAt: new Date(Date.now() - 60_000).toISOString() } } }),
  );
}

const noResults = { results: [], summary: { updated: 0, upToDate: 0, blocked: 0, errors: 0, skipped: 0 } };

describe('UpdatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.mocked(api.missingTrackedRepos).mockResolvedValue({ repos: [] });
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

  it('matches check results returned by relative path so nested skills do not stay checking', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [nestedSkill],
    });
    vi.mocked(api.checkStream).mockImplementation((_onDiscovering, _onStart, _onProgress, onDone) => {
      queueMicrotask(() => {
        onDone({
          tracked_repos: [],
          skills: [
            {
              name: 'tools/agent-browser',
              source: 'https://github.com/vercel-labs/agent-browser/skills/agent-browser',
              version: 'abc1234',
              status: 'update_available',
            },
          ],
        });
      });
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();

    await user.click(await screen.findByRole('button', { name: /check for updates/i }));

    const row = await findRow('agent-browser');
    await waitFor(() => {
      expect(row.getByText('Update available')).toBeInTheDocument();
    });
    expect(row.queryByText('Checking')).not.toBeInTheDocument();
  });

  it('restores cached check status and last check time on entry', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [nestedSkill],
    });
    cacheStatus('agent-browser', 'update-available');

    renderUpdatePage();

    const row = await findRow('agent-browser');
    expect(row.getByText('Update available')).toBeInTheDocument();
    expect(screen.getByText(/checked 1 minute ago/i)).toBeInTheDocument();
    expect(api.checkStream).not.toHaveBeenCalled();
  });

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

  it('marks updated items as up to date', async () => {
    const updatedResult = {
      name: 'tools/agent-browser',
      action: 'updated',
      message: 'reinstalled from source',
      isRepo: false,
    };
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [nestedSkill],
    });
    cacheStatus('agent-browser', 'update-available');
    vi.mocked(api.updateAllStream).mockImplementation((onStart, onResult, onDone) => {
      queueMicrotask(() => {
        onStart(1);
        onResult(updatedResult);
        onDone({
          results: [updatedResult],
          summary: { updated: 1, upToDate: 0, blocked: 0, errors: 0, skipped: 0 },
        });
      });
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();

    const row = await findRow('agent-browser');
    await user.click(row.getByRole('button', { name: /^update$/i }));

    await waitFor(() => expect(row.getByText('Updated')).toBeInTheDocument());
    expect(row.getByText('Up to date')).toBeInTheDocument();
  });

  it('warns about missing tracked repos and rehydrates on click (issue #212)', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [nestedSkill] });
    vi.mocked(api.missingTrackedRepos).mockResolvedValue({
      repos: [{ name: '_team-skills', source: 'https://github.com/example/team-skills', branch: 'main' }],
    });
    vi.mocked(api.rehydrateTrackedRepos).mockResolvedValue({
      results: [{ name: '_team-skills', action: 'rehydrated' }],
    });

    const user = userEvent.setup();
    renderUpdatePage();

    // Banner lists the missing repo.
    expect(await screen.findByText('_team-skills')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /rehydrate/i }));

    await waitFor(() => expect(api.rehydrateTrackedRepos).toHaveBeenCalled());
  });
});

describe('update failure message helpers', () => {
  const auditBlocked =
    'security audit failed — findings at/above CRITICAL detected:\n' +
    '  CRITICAL: Prompt injection attempt detected (SKILL.md:28)\n\n' +
    'Use --force to override or --skip-audit to bypass scanning: blocked by security audit';

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

  it('drops CLI-only hints from messages shown in the web UI', () => {
    const shown = stripCliHint(auditBlocked);
    expect(shown).not.toContain('--force');
    expect(shown).not.toContain('--skip-audit');
    expect(shown).toContain('CRITICAL: Prompt injection attempt detected (SKILL.md:28)');
    expect(shown).toContain('blocked by security audit');

    expect(stripCliHint('rolled back (use --skip-audit to bypass)')).toBe('rolled back');
    expect(stripCliHint('pull rejected (try force update)')).toBe('pull rejected');
  });
});
