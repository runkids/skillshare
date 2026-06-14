import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import UpdatePage from './UpdatePage';
import { api } from '../api/client';

vi.mock('react-virtuoso', () => ({
  Virtuoso: ({ totalCount, itemContent }: { totalCount: number; itemContent: (index: number) => React.ReactNode }) => (
    <div>{Array.from({ length: totalCount }, (_, index) => <div key={index}>{itemContent(index)}</div>)}</div>
  ),
}));

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
      <I18nProvider>
        <ToastProvider>
          <UpdatePage />
        </ToastProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
}

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

    await user.click(await screen.findByRole('button', { name: /check all/i }));

    const row = await screen.findByText('agent-browser').then((el) => el.closest('button'));
    expect(row).not.toBeNull();
    await waitFor(() => {
      expect(within(row as HTMLElement).getByText('Update available')).toBeInTheDocument();
    });
    expect(within(row as HTMLElement).queryByText('Checking')).not.toBeInTheDocument();
  });

  it('restores cached check status and last check time on entry', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [nestedSkill],
    });
    localStorage.setItem(
      'skillshare.updateCheckCache.global',
      JSON.stringify({
        version: 1,
        items: {
          'agent-browser': {
            status: 'update-available',
            checkedAt: new Date(Date.now() - 60_000).toISOString(),
          },
        },
      }),
    );

    renderUpdatePage();

    const row = await screen.findByText('agent-browser').then((el) => el.closest('button'));
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByText('Update available')).toBeInTheDocument();
    expect(within(row as HTMLElement).getByText(/checked 1m ago/i)).toBeInTheDocument();
    expect(api.checkStream).not.toHaveBeenCalled();
  });

  it('sends relative paths when updating nested GitHub-installed skills', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [nestedSkill],
    });
    vi.mocked(api.updateAllStream).mockImplementation((_onStart, _onResult, onDone) => {
      queueMicrotask(() => onDone({ results: [], summary: { updated: 0, upToDate: 0, blocked: 0, errors: 0, skipped: 0 } }));
      return { close: vi.fn() } as unknown as EventSource;
    });

    const user = userEvent.setup();
    renderUpdatePage();

    await user.click(await screen.findByText('agent-browser'));
    await user.click(screen.getByRole('button', { name: /update selected \(1\)/i }));

    expect(api.updateAllStream).toHaveBeenCalledWith(
      expect.any(Function),
      expect.any(Function),
      expect.any(Function),
      expect.any(Function),
      { names: ['tools/agent-browser'], force: false },
    );
  });

  it('keeps updated items checked as up to date when returning to the list', async () => {
    const updatedResult = {
      name: 'tools/agent-browser',
      action: 'updated',
      message: 'reinstalled from source',
      isRepo: false,
    };
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [nestedSkill],
    });
    localStorage.setItem(
      'skillshare.updateCheckCache.global',
      JSON.stringify({
        version: 1,
        items: {
          'agent-browser': {
            status: 'update-available',
            checkedAt: new Date(Date.now() - 60_000).toISOString(),
          },
        },
      }),
    );
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

    await user.click(await screen.findByText('agent-browser'));
    await user.click(screen.getByRole('button', { name: /update selected \(1\)/i }));
    await user.click(await screen.findByRole('button', { name: /back to list/i }));

    const row = await screen.findByText('agent-browser').then((el) => el.closest('button'));
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByText('Up to date')).toBeInTheDocument();
    expect(within(row as HTMLElement).queryByText('Unchecked')).not.toBeInTheDocument();
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

    const rehydrateBtn = screen.getByRole('button', { name: /rehydrate/i });
    await user.click(rehydrateBtn);

    await waitFor(() => expect(api.rehydrateTrackedRepos).toHaveBeenCalled());
  });
});
