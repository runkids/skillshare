import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import type { Overview } from '../api/client';
import { hooksApi } from '../api/hooks';
import type { HookInventory, HookPlan } from '../api/hooks';
import { mcpApi } from '../api/mcp';
import type { MCPPlan } from '../api/mcp';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import BackupPage from './BackupPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      getOverview: vi.fn(),
      listBackups: vi.fn(),
      createBackup: vi.fn(),
      deleteBackup: vi.fn(),
      deleteAllBackups: vi.fn(),
      patchConfig: vi.fn(),
      validateRestore: vi.fn(),
      restore: vi.fn(),
      listFileBackups: vi.fn(),
      getFileBackupVersions: vi.fn(),
      getFileBackupVersion: vi.fn(),
      restoreFileBackup: vi.fn(),
    },
  };
});
vi.mock('../api/mcp', async (load) => ({ ...await load<typeof import('../api/mcp')>(), mcpApi: { list: vi.fn(), previewRestore: vi.fn(), restore: vi.fn() } }));
vi.mock('../api/hooks', async (load) => ({ ...await load<typeof import('../api/hooks')>(), hooksApi: { list: vi.fn(), previewRestore: vi.fn(), restore: vi.fn() } }));

const TS = '2026-09-28_10-52-00';

function renderPage(tab = '') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={[`/backup${tab ? `?tab=${tab}` : ''}`]}>
      <QueryClientProvider client={client}><I18nProvider><ToastProvider><BackupPage /></ToastProvider></I18nProvider></QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('BackupPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getOverview).mockResolvedValue({} as Overview);
    vi.mocked(api.listBackups).mockResolvedValue({
      totalSizeBytes: 10,
      retention: { maxAgeDays: 30, maxCount: 10, maxSizeMB: 500 },
      backups: [{ timestamp: TS, path: `/home/me/.local/share/skillshare/backups/${TS}`, targets: ['claude-agents'], entries: [{ name: 'claude-agents', sizeBytes: 10, files: 2 }], date: new Date().toISOString(), sizeBytes: 10 }],
    });
    vi.mocked(api.listFileBackups).mockResolvedValue({ files: [{ path: '/home/me/.claude/CLAUDE.md', versions: 1, latest: new Date().toISOString(), target: 'claude' }] });
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: [], servers: {} }, projectConfigs: [], paths: {}, detected: [], plan: null, previewError: '', unmanaged: [],
      backups: [
        { id: '1790000000000000000-a', target: 'claude', path: '/home/me/.claude.json', servers: [{ name: 'github', change: 'added' }] },
        { id: '1780000000000000000-b', target: 'claude', path: '/home/me/.claude.json', servers: [{ name: 'linear', change: 'removed' }] },
      ],
    } as Awaited<ReturnType<typeof mcpApi.list>>);
  });

  it('restores an agents snapshot under its snapshot name', async () => {
    vi.mocked(api.validateRestore).mockResolvedValue({ valid: true, error: '', conflicts: [], backupSizeBytes: 10, currentIsSymlink: false });
    vi.mocked(api.restore).mockResolvedValue({ success: true, target: 'claude-agents', timestamp: TS });
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /claude agents/, expanded: false }));
    await user.click(screen.getByRole('button', { name: 'Restore' }));
    const dialog = await screen.findByRole('dialog');
    await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Restore' })).toBeEnabled());
    await user.click(within(dialog).getByRole('button', { name: 'Restore' }));

    await waitFor(() => expect(api.restore).toHaveBeenCalledWith({ timestamp: TS, target: 'claude-agents', force: false }));
  });

  it('says the list is loading while backups are read', async () => {
    vi.mocked(api.listBackups).mockReturnValue(new Promise(() => {}));
    renderPage();

    expect(await screen.findByText('Loading backups…')).toBeInTheDocument();
  });

  it('says a backup is running until it finishes', async () => {
    vi.mocked(api.createBackup).mockReturnValue(new Promise(() => {}));
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /Back up now/ }));

    expect(await screen.findByText(/Backing up…/)).toBeInTheDocument();
  });

  it('groups backups under the day they were taken', async () => {
    renderPage();

    expect(await screen.findByText('Today')).toBeInTheDocument();
  });

  it('shows how many files each snapshot folder holds once a backup is opened', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /claude agents/, expanded: false }));

    expect(screen.getByText('2 files · 10 B')).toBeInTheDocument();
  });

  it('counts a single snapshot in the singular', async () => {
    renderPage();

    expect(await screen.findByText(/^1 backup ·/)).toBeInTheDocument();
  });

  it('deletes a backup only after confirmation', async () => {
    vi.mocked(api.deleteBackup).mockResolvedValue({ success: true });
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /claude agents/, expanded: false }));
    await user.click(screen.getByRole('button', { name: 'Delete this backup' }));
    expect(api.deleteBackup).not.toHaveBeenCalled();
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(api.deleteBackup).toHaveBeenCalledWith(TS));
  });

  it('deletes every target folder backup only after confirmation', async () => {
    vi.mocked(api.deleteAllBackups).mockResolvedValue({ success: true, removed: 1 });
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: 'Delete all' }));
    expect(api.deleteAllBackups).not.toHaveBeenCalled();
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete all' }));

    await waitFor(() => expect(api.deleteAllBackups).toHaveBeenCalled());
  });

  it('saves the count and size limits from the retention summary', async () => {
    vi.mocked(api.patchConfig).mockResolvedValue({ mode: 'merge', logMaxEntries: null });
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: 'Kept: 30 days · 10 backups · 500 MB' }));
    const panel = screen.getByRole('dialog', { name: 'Kept automatically' });
    await user.click(within(within(panel).getByRole('radiogroup', { name: 'Most backups' })).getByRole('radio', { name: '20' }));
    await user.click(within(within(panel).getByRole('radiogroup', { name: 'Most total size' })).getByRole('radio', { name: 'No limit' }));
    await user.click(within(panel).getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(api.patchConfig).toHaveBeenCalledWith({ backupMaxCount: 20, backupMaxSizeMB: 0 }));
  });

  it('asks to cut the link when the file is now a link, and says so to the server', async () => {
    vi.mocked(api.getFileBackupVersions).mockResolvedValue({
      path: '/home/me/.claude/CLAUDE.md',
      current: { exists: true, link_to: '/home/me/.config/skillshare/extras/personal/AGENTS.md' },
      versions: [{ id: '1790000000000000000.convert', kind: 'history', reason: 'convert', time: new Date().toISOString(), size: 12, preview: '# Notes' }],
    });
    vi.mocked(api.getFileBackupVersion).mockResolvedValue({ content: '# Notes\n', current: '# Shared\n' });
    vi.mocked(api.restoreFileBackup).mockResolvedValue({ success: true });
    const user = userEvent.setup();
    renderPage('files');

    expect(await screen.findByText('Before converting to AGENTS.md.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Preview and restore' }));
    await user.click(await within(screen.getByRole('dialog')).findByRole('button', { name: 'Restore and cut the link' }));

    await waitFor(() => expect(api.restoreFileBackup).toHaveBeenCalledWith({ path: '/home/me/.claude/CLAUDE.md', id: '1790000000000000000.convert', unlink: true }));
  });

  it('opens the MCP restore dialog on the backup picked', async () => {
    vi.mocked(mcpApi.previewRestore).mockResolvedValue({ revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [] } as MCPPlan);
    const user = userEvent.setup();
    renderPage('mcp');

    expect(await screen.findByText('Removed linear')).toBeInTheDocument();
    await user.click(screen.getAllByRole('button', { name: 'Preview and restore' })[1]);

    await waitFor(() => expect(mcpApi.previewRestore).toHaveBeenCalledWith('1780000000000000000-b'));
  });

  it('groups hook backups by Agent file and previews the one picked', async () => {
    vi.mocked(hooksApi.list).mockResolvedValue({
      source: { path: '', configPath: '', entries: {} }, targets: [], paths: {}, plan: null, previewError: '', unmanaged: [],
      backups: [
        { id: '1780000000000000000-a', target: 'codex', path: '/home/me/.codex/hooks.json' },
        { id: '1790000000000000000-b', target: 'codex', path: '/home/me/.codex/hooks.json' },
        { id: '1785000000000000000-c', target: 'claude', path: '/home/me/.claude/settings.json' },
      ],
    } as HookInventory);
    vi.mocked(hooksApi.previewRestore).mockResolvedValue({ revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [] } as HookPlan);
    const user = userEvent.setup();
    renderPage('hooks');

    expect(await screen.findByText('2 backups')).toBeInTheDocument();
    expect(screen.getByText('1 backup')).toBeInTheDocument();
    await user.click(screen.getAllByRole('button', { name: 'Preview and restore' })[1]);

    await waitFor(() => expect(hooksApi.previewRestore).toHaveBeenCalledWith('1780000000000000000-a'));
    expect(hooksApi.restore).not.toHaveBeenCalled();
  });

  it('says so when there are no hook backups', async () => {
    vi.mocked(hooksApi.list).mockResolvedValue({ source: { path: '', configPath: '', entries: {} }, targets: [], paths: {}, plan: null, previewError: '', unmanaged: [], backups: [] } as HookInventory);
    renderPage('hooks');

    expect(await screen.findByText('No hook backups yet')).toBeInTheDocument();
  });
});
