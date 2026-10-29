import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api } from '../api/client';
import type { Overview, Target } from '../api/client';
import { hooksApi } from '../api/hooks';
import type { HookInventory } from '../api/hooks';
import { mcpApi } from '../api/mcp';
import { pluginsApi } from '../api/plugins';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import DashboardPage from './DashboardPage';

const appContext = vi.hoisted(() => ({ isProjectMode: false }));
vi.mock('../context/AppContext', () => ({ useAppContext: () => appContext }));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      getOverview: vi.fn(), listTargets: vi.fn(), listExtras: vi.fn(), listLog: vi.fn(),
      auditAll: vi.fn(), check: vi.fn(), getVersionCheck: vi.fn(), deleteRepo: vi.fn(), update: vi.fn(),
    },
  };
});
vi.mock('../api/mcp', async (load) => {
  const actual = await load<typeof import('../api/mcp')>();
  return { ...actual, mcpApi: { ...actual.mcpApi, list: vi.fn() } };
});
vi.mock('../api/hooks', async (load) => {
  const actual = await load<typeof import('../api/hooks')>();
  return { ...actual, hooksApi: { ...actual.hooksApi, list: vi.fn() } };
});
vi.mock('../api/plugins', async (load) => {
  const actual = await load<typeof import('../api/plugins')>();
  return { ...actual, pluginsApi: { ...actual.pluginsApi, list: vi.fn() } };
});

function target(name: string, status: string): Target {
  return { name, path: `/home/dev/.${name}/skills`, mode: 'merge', targetNaming: 'flat', status, skillsEnabled: true, linkedCount: 3, localCount: 0, include: [], exclude: [], expectedSkillCount: 3 } as Target;
}

function renderPage(client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <QueryClientProvider client={client}><I18nProvider><ToastProvider><DashboardPage /></ToastProvider></I18nProvider></QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    appContext.isProjectMode = false;
    vi.mocked(api.getOverview).mockResolvedValue({ skillCount: 3, agentCount: 0, source: '/home/dev/skills', trackedRepos: [] } as unknown as Overview);
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target('codex', 'merged'), target('cursor', 'not exist')], sourceSkillCount: 3 } as Awaited<ReturnType<typeof api.listTargets>>);
    vi.mocked(api.listExtras).mockResolvedValue({ extras: [] } as unknown as Awaited<ReturnType<typeof api.listExtras>>);
    vi.mocked(api.listLog).mockResolvedValue({ entries: [] } as unknown as Awaited<ReturnType<typeof api.listLog>>);
    vi.mocked(api.auditAll).mockReturnValue(new Promise(() => {}));
    vi.mocked(api.check).mockReturnValue(new Promise(() => {}));
    vi.mocked(api.getVersionCheck).mockReturnValue(new Promise(() => {}));
    vi.mocked(mcpApi.list).mockReturnValue(new Promise(() => {}));
    vi.mocked(pluginsApi.list).mockReturnValue(new Promise(() => {}));
    vi.mocked(hooksApi.list).mockResolvedValue({ source: { entries: { lint: { bindings: {} }, fmt: { bindings: {} } } } } as unknown as HookInventory);
  });

  describe('updating a tracked repo', () => {
    beforeEach(() => {
      vi.mocked(api.getOverview).mockResolvedValue({ skillCount: 3, agentCount: 0, source: '/home/dev/skills', trackedRepos: [{ name: '_team', skillCount: 1, dirty: false }] } as unknown as Overview);
      vi.mocked(api.check).mockResolvedValue({ tracked_repos: [{ name: '_team', status: 'behind', behind: 2 }], linked_repos: [], skills: [] } as unknown as Awaited<ReturnType<typeof api.check>>);
    });

    it('offers Update only after a check finds the repo behind', async () => {
      renderPage();
      await screen.findByRole('button', { name: 'Repo actions' });
      expect(screen.queryByRole('button', { name: 'Update' })).not.toBeInTheDocument();
      await userEvent.click(screen.getByRole('button', { name: 'Check for updates' }));
      expect(await screen.findByRole('button', { name: 'Update' })).toBeInTheDocument();
    });

    it('drops Update once the repo is pulled', async () => {
      vi.mocked(api.update).mockResolvedValue({ results: [{ name: '_team', action: 'updated', message: 'done' }] } as unknown as Awaited<ReturnType<typeof api.update>>);
      renderPage();
      await userEvent.click(await screen.findByRole('button', { name: 'Check for updates' }));
      await userEvent.click(await screen.findByRole('button', { name: 'Update' }));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Update' })).not.toBeInTheDocument());
    });

    it('forgets the check after leaving the Dashboard, since updates elsewhere do not touch it', async () => {
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      const { unmount } = renderPage(client);
      await userEvent.click(await screen.findByRole('button', { name: 'Check for updates' }));
      await screen.findByRole('button', { name: 'Update' });
      unmount();
      renderPage(client);
      await screen.findByRole('button', { name: 'Repo actions' });
      expect(screen.queryByRole('button', { name: 'Update' })).not.toBeInTheDocument();
    });
  });

  describe('uninstalling a tracked repo', () => {
    const dirty = new ApiError(409, 'uncommitted changes (use force to override)', { code: 'repo_dirty' });

    async function confirmUninstall() {
      vi.mocked(api.getOverview).mockResolvedValue({ skillCount: 3, agentCount: 0, source: '/home/dev/skills', trackedRepos: [{ name: '_team', skillCount: 1, dirty: false }] } as unknown as Overview);
      renderPage();
      await userEvent.click(await screen.findByRole('button', { name: 'Repo actions' }));
      await userEvent.click(screen.getByRole('menuitem', { name: 'Uninstall' }));
      await userEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    }

    it('removes a clean repo in one request without force', async () => {
      vi.mocked(api.deleteRepo).mockResolvedValue({ success: true, name: '_team' });
      await confirmUninstall();
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(vi.mocked(api.deleteRepo).mock.calls).toEqual([['_team', false]]);
    });

    it('retries a dirty repo with force once the user confirms', async () => {
      vi.mocked(api.deleteRepo).mockRejectedValueOnce(dirty).mockResolvedValue({ success: true, name: '_team' });
      await confirmUninstall();
      expect(await screen.findByText(/"team" has uncommitted changes/)).toBeInTheDocument();
      await userEvent.click(screen.getByRole('button', { name: 'Retry with Force' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(vi.mocked(api.deleteRepo).mock.calls).toEqual([['_team', false], ['_team', true]]);
    });

    it('leaves a dirty repo alone when the user cancels', async () => {
      vi.mocked(api.deleteRepo).mockRejectedValue(dirty);
      await confirmUninstall();
      await screen.findByRole('button', { name: 'Retry with Force' });
      await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(vi.mocked(api.deleteRepo).mock.calls).toEqual([['_team', false]]);
    });

  });
});
