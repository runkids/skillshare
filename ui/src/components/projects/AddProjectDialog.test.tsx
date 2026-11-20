import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { mcpApi } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import AddProjectDialog from './AddProjectDialog';

vi.mock('../../api/client', async (load) => ({
  ...(await load<typeof import('../../api/client')>()),
  api: { saveProject: vi.fn(), scanProjects: vi.fn() },
}));

vi.mock('../../api/mcp', async (load) => ({
  ...(await load<typeof import('../../api/mcp')>()),
  mcpApi: { save: vi.fn() },
}));

vi.mock('./ProjectTools', () => ({
  default: ({ onChange }: { onChange: (next: string[]) => void }) => (
    <button type="button" onClick={() => onChange(['claude'])}>
      pick claude
    </button>
  ),
}));

const tools = [{ name: 'claude', skillsPath: '.claude/skills', agentsPath: '' }];

const storagePaths = {
  shared: '/home/me/.config/skillshare/config.yaml',
  local: '/home/me/.local/state/skillshare/projects/a.yaml',
};

const onScanned = vi.fn();

const open = (defaultStorage?: 'shared' | 'local') =>
  render(
    <I18nProvider>
      <AddProjectDialog
        tools={tools}
        common={['claude']}
        defaultStorage={defaultStorage}
        storagePaths={storagePaths}
        existing={[]}
        onClose={() => {}}
        onAdded={() => {}}
        onScanned={onScanned}
      />
    </I18nProvider>,
  );

const submit = async () => {
  await userEvent.type(screen.getByLabelText('Project folder'), '~/work/docs');
  await userEvent.click(screen.getByRole('button', { name: 'pick claude' }));
  await userEvent.click(screen.getByRole('button', { name: 'Add project' }));
};

beforeEach(() => {
  Element.prototype.scrollIntoView = vi.fn();
  vi.clearAllMocks();
  vi.mocked(api.saveProject).mockResolvedValue({
    success: true,
    root: '~/work/docs',
    origin: 'shared',
    originPath: '',
  });
});

describe('Add project dialog', () => {
  it('starts on this machine when the server defaults to local', async () => {
    open('local');
    await submit();
    await waitFor(() =>
      expect(api.saveProject).toHaveBeenCalledWith(
        expect.objectContaining({ create: true, storage: 'local' }),
      ),
    );
  });

  it('saves in config.yaml by default', async () => {
    open();
    await submit();
    await waitFor(() =>
      expect(api.saveProject).toHaveBeenCalledWith(expect.objectContaining({ storage: 'shared' })),
    );
  });

  it('saves on this machine only after picking it in the Saved to menu', async () => {
    open();
    await userEvent.click(screen.getByRole('combobox', { name: 'Saved to' }));
    await userEvent.click(screen.getByRole('option', { name: /This machine only/ }));
    await submit();
    await waitFor(() =>
      expect(api.saveProject).toHaveBeenCalledWith(expect.objectContaining({ storage: 'local' })),
    );
  });

  it('writes checkouts only after their review is confirmed', async () => {
    const row = (path: string) => ({
      root: `~/work/docs/${path}`,
      path,
      name: path,
      result: 'added' as const,
    });
    vi.mocked(api.scanProjects)
      .mockResolvedValueOnce({ applied: false, rows: [row('.'), row('app-a')], warnings: [] })
      .mockResolvedValueOnce({ applied: true, rows: [row('.'), row('app-a')], warnings: [] });
    open();
    await userEvent.click(screen.getByRole('checkbox', { name: /Also add each checkout/ }));
    await userEvent.type(screen.getByLabelText('Look in'), '.worktrees{Enter}');
    await userEvent.type(screen.getByLabelText('Project folder'), '~/work/docs');
    await userEvent.click(screen.getByRole('button', { name: 'pick claude' }));
    await userEvent.click(screen.getByRole('button', { name: 'Review' }));
    await screen.findByRole('button', { name: 'Add 2 projects' });
    expect(api.scanProjects).toHaveBeenCalledTimes(1);
    expect(api.scanProjects).toHaveBeenLastCalledWith(
      expect.objectContaining({ root: '~/work/docs', scan: ['.', '.worktrees'], dryRun: true }),
    );

    await userEvent.click(screen.getByRole('button', { name: 'Add 2 projects' }));
    await waitFor(() => expect(onScanned).toHaveBeenCalledWith(2));
    expect(api.scanProjects).toHaveBeenLastCalledWith(
      expect.objectContaining({ scan: ['.', '.worktrees'], dryRun: false }),
    );
    // The default storage is left to the server, as the CLI does without a flag.
    expect(vi.mocked(api.scanProjects).mock.lastCall?.[0].storage).toBeUndefined();
    expect(api.saveProject).not.toHaveBeenCalled();
  });

  it('stays open when the write finds nothing left to add', async () => {
    const row = (path: string, result: 'added' | 'kept') => ({
      root: `~/work/docs/${path}`,
      path,
      name: path,
      result,
    });
    vi.mocked(api.scanProjects)
      .mockResolvedValueOnce({ applied: false, rows: [row('.', 'added')], warnings: [] })
      .mockResolvedValueOnce({ applied: false, rows: [row('.', 'kept')], warnings: [] });
    open();
    await userEvent.click(screen.getByRole('checkbox', { name: /Also add each checkout/ }));
    await userEvent.type(screen.getByLabelText('Project folder'), '~/work/docs');
    await userEvent.click(screen.getByRole('button', { name: 'pick claude' }));
    await userEvent.click(screen.getByRole('button', { name: 'Review' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Add 1 project' }));
    await screen.findByText('Nothing new to add.');
    expect(onScanned).not.toHaveBeenCalled();
  });

  it('returns to the form with the error when MCP fails after the write', async () => {
    const row = { root: '~/work/docs', path: '.', name: 'docs', result: 'added' as const };
    vi.mocked(api.scanProjects).mockResolvedValue({ applied: true, rows: [row], warnings: [] });
    vi.mocked(mcpApi.save).mockRejectedValue(new Error('mcp save failed'));
    open();
    await userEvent.click(screen.getByRole('checkbox', { name: /^MCP/ }));
    await userEvent.click(screen.getByRole('checkbox', { name: /Also add each checkout/ }));
    await userEvent.type(screen.getByLabelText('Project folder'), '~/work/docs');
    await userEvent.click(screen.getByRole('button', { name: 'pick claude' }));
    await userEvent.click(screen.getByRole('button', { name: 'Review' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Add 1 project' }));
    await screen.findByText('mcp save failed');
    expect(screen.getByRole('button', { name: 'Review' })).toBeInTheDocument();
    expect(onScanned).not.toHaveBeenCalled();
  });
});
