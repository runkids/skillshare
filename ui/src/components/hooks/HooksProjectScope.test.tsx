import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { hooksApi, type HookChange, type HookPlan } from '../../api/hooks';
import { ToastProvider } from '../Toast';
import HooksScope from './HooksScope';
import HookDialog from './HookDialog';
import HooksRemoveDialog from './HooksRemoveDialog';
import HooksSyncBox, { HooksSyncDialog } from './HooksSyncBox';

vi.mock('../CodeEditor', () => ({
  default: ({
    value,
    onChange,
    ariaLabel,
  }: {
    value: string;
    onChange: (v: string) => void;
    ariaLabel: string;
  }) => (
    <textarea aria-label={ariaLabel} value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
vi.mock('../../api/hooks', async (load) => ({
  ...(await load<typeof import('../../api/hooks')>()),
  hooksApi: {
    list: vi.fn(),
    catalog: vi.fn(),
    preview: vi.fn(),
    save: vi.fn(),
    configure: vi.fn(),
    syncProject: vi.fn(),
    render: vi.fn(),
  },
}));

const APP = '/work/app';
const change = (over: Partial<HookChange>): HookChange => ({
  target: 'claude',
  path: '/home/u/.claude/settings.json',
  name: 'g',
  action: 'unchanged',
  ...over,
});
// Global work is pending and another project conflicts; only the selected root's own changes matter to it.
const plan = (mine: HookChange[]): HookPlan => ({
  revision: 'rev-all',
  fingerprint: 'fp',
  sourcePath: '/s.yaml',
  blocked: true,
  changes: [
    change({ name: 'g', action: 'add' }),
    change({
      name: 'o',
      root: '/work/other',
      path: '/work/other/.claude/settings.json',
      action: 'conflict',
    }),
    ...mine,
  ],
});
const mine = (action: string) =>
  change({ name: 'lint', root: APP, path: `${APP}/.claude/settings.json`, action });

const catalog = {
  claude: {
    timeoutUnit: 'seconds' as const,
    events: [
      { name: 'PostToolUse', description: 'After a tool succeeds', matcher: true },
      { name: 'Stop', description: 'When the main agent finishes', matcher: false },
    ],
  },
};
const pickEvent = async (user: ReturnType<typeof userEvent.setup>, row: number, event: string) => {
  await user.click(screen.getByRole('combobox', { name: `Event ${row}` }));
  await user.click(await screen.findByRole('option', { name: new RegExp(`^${event}`) }));
};

const wrap = (ui: React.ReactNode) =>
  render(
    <MemoryRouter>
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <I18nProvider>
          <ToastProvider>{ui}</ToastProvider>
        </I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );

describe('project scope in hooks dialogs', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
    HTMLElement.prototype.scrollIntoView = vi.fn();
  });
  afterEach(() => vi.restoreAllMocks());

  it('saves and syncs a project hook with one configure call carrying the whole plan revision', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('add')]));
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    const onSaved = vi.fn();
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={onSaved} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await pickEvent(user, 1, 'PostToolUse');
    await user.type(screen.getByLabelText('Command 1'), './lint.sh');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    // Neither the global pending change nor the other project's conflict is shown or blocks this root.
    const save = await screen.findByRole('button', { name: 'Save and sync' });
    expect(screen.queryByText('o')).not.toBeInTheDocument();
    expect(save).toBeEnabled();
    await user.click(save);
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(true));
    expect(hooksApi.configure).toHaveBeenCalledTimes(1);
    expect(hooksApi.configure).toHaveBeenCalledWith(
      expect.objectContaining({ project: APP, name: 'lint' }),
      'rev-all',
      true,
    );
    expect(hooksApi.syncProject).not.toHaveBeenCalled();
  });

  it('keeps an extra command that has no command text and blocks saving', async () => {
    const user = userEvent.setup();
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={vi.fn()} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await pickEvent(user, 1, 'Stop');
    await user.type(screen.getByLabelText('Command 1'), 'true');
    await user.click(screen.getByRole('button', { name: 'Add command' }));
    expect(screen.getByLabelText('Command 2')).toHaveValue('');
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Preview' })).toBeDisabled();
  });

  it('keeps the preview current while its Save and sync is in flight', async () => {
    const user = userEvent.setup();
    let finish = () => {};
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('add')]));
    vi.mocked(hooksApi.configure).mockReturnValue(
      new Promise((resolve) => {
        finish = () => resolve({ applied: [], backupIds: [] });
      }),
    );
    const onSaved = vi.fn();
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={onSaved} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await pickEvent(user, 1, 'Stop');
    await user.type(screen.getByLabelText('Command 1'), 'true');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    await user.click(await screen.findByRole('button', { name: 'Save and sync' }));
    await waitFor(() => expect(hooksApi.configure).toHaveBeenCalled());
    expect(screen.queryByText(/changed since this preview/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save and sync' })).toBeDisabled();
    finish();
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  it('previews again after an edit and syncs the edited hook with the new revision', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview)
      .mockResolvedValueOnce({ ...plan([mine('add')]), revision: 'rev-1' })
      .mockResolvedValueOnce({ ...plan([mine('add')]), revision: 'rev-2' });
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={vi.fn()} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await pickEvent(user, 1, 'Stop');
    await user.type(screen.getByLabelText('Command 1'), 'true');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    await user.click(await screen.findByRole('button', { name: 'Back' }));
    await user.type(screen.getByLabelText('Command 1'), ' --x');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    await user.click(await screen.findByRole('button', { name: 'Save and sync' }));
    await waitFor(() =>
      expect(hooksApi.configure).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'lint' }),
        'rev-2',
        true,
      ),
    );
    expect(JSON.stringify(vi.mocked(hooksApi.configure).mock.calls[0][0])).toContain('true --x');
  });

  it('previews a project sync with only its own root and applies it as that root', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('add')]));
    vi.mocked(hooksApi.syncProject).mockResolvedValue({ applied: [], backupIds: [] });
    wrap(<HooksSyncDialog project={APP} onClose={vi.fn()} />);
    const sync = await screen.findByRole('button', { name: 'Sync Now' });
    await waitFor(() => expect(sync).toBeEnabled());
    expect(screen.queryByText('o')).not.toBeInTheDocument();
    await user.click(sync);
    await waitFor(() => expect(hooksApi.syncProject).toHaveBeenCalledWith(APP, 'rev-all'));
  });

  it('applies a project takeover through one scoped mutation, not the whole plan', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('add')]));
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    wrap(
      <HooksSyncDialog
        project={APP}
        takeover={{ name: 'lint', entry: { bindings: {} } }}
        onClose={vi.fn()}
      />,
    );
    await user.click(await screen.findByRole('button', { name: 'Sync Now' }));
    await waitFor(() =>
      expect(hooksApi.configure).toHaveBeenCalledWith(
        { project: APP, name: 'lint', entry: { bindings: {} }, replace: true },
        'rev-all',
        true,
      ),
    );
    expect(screen.queryByText('o')).not.toBeInTheDocument();
  });

  it('removes a project hook and syncs it in one call', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('remove')]));
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    const onSaved = vi.fn();
    wrap(<HooksRemoveDialog name="lint" project={APP} onClose={vi.fn()} onSaved={onSaved} />);
    const button = await screen.findByRole('button', { name: 'Remove and sync' });
    await waitFor(() => expect(button).toBeEnabled());
    await user.click(button);
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(hooksApi.configure).toHaveBeenCalledTimes(1);
    expect(hooksApi.configure).toHaveBeenCalledWith(
      { project: APP, name: 'lint', remove: true },
      'rev-all',
      true,
    );
    expect(hooksApi.syncProject).not.toHaveBeenCalled();
  });

  it('keeps the draft and shows the error when a sync is refused, and still allows a source-only save', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('add')]));
    vi.mocked(hooksApi.configure).mockRejectedValueOnce(new Error('preview is stale'));
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    const onSaved = vi.fn();
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={onSaved} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await pickEvent(user, 1, 'PostToolUse');
    await user.type(screen.getByLabelText('Command 1'), './lint.sh');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    await user.click(await screen.findByRole('button', { name: 'Save and sync' }));
    expect(await screen.findByText('preview is stale')).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByLabelText('Command 1')).toHaveValue('./lint.sh');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith(
        expect.objectContaining({ project: APP, name: 'lint' }),
      ),
    );
  });
});

describe('project row preview', () => {
  beforeEach(() => vi.resetAllMocks());

  const IDENTICAL =
    'an identical hook exists that Skillshare does not manage; import it or explicitly replace it';
  const lintEntry = {
    bindings: { claude: { events: { Stop: [{ hooks: [{ type: 'command', command: 'true' }] }] } } },
  };
  const conflicted = () => plan([{ ...mine('conflict'), message: IDENTICAL }]);
  const conflictData = () =>
    ({
      source: {
        path: '/s.yaml',
        configPath: '/s.yaml',
        entries: { g: { bindings: {} } },
        projects: { [APP]: { entries: { lint: lintEntry } } },
      },
      targets: [],
      paths: {},
      backups: [],
      unmanaged: [],
      projectConfigs: [],
      projectPaths: {},
      plan: conflicted(),
    }) as never;
  const takeover = { project: APP, name: 'lint', entry: lintEntry, replace: true };

  it('opens the take-over preview for its own conflict from the rail and writes only after Sync Now', async () => {
    const user = userEvent.setup();
    // With replace the preview resolves the conflict into an ordinary write.
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('update')]));
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    wrap(<HooksScope data={conflictData()} project={APP} header={() => null} />);
    await user.click(screen.getByRole('button', { name: 'Take over native hooks · lint' }));
    const dialog = await screen.findByRole('dialog', { name: 'Take over lint?' });
    expect(hooksApi.preview).toHaveBeenCalledWith(takeover);
    expect(hooksApi.configure).not.toHaveBeenCalled();
    const sync = await within(dialog).findByRole('button', { name: 'Sync Now' });
    await waitFor(() => expect(sync).toBeEnabled());
    await user.click(sync);
    await waitFor(() => expect(hooksApi.configure).toHaveBeenCalledWith(takeover, 'rev-all', true));
  });

  it('stops managing the project only after the confirmation, without syncing', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.save).mockResolvedValue(undefined as never);
    const onRemoved = vi.fn();
    wrap(<HooksScope data={conflictData()} project={APP} onRemoved={onRemoved} />);
    await user.click(screen.getByRole('button', { name: 'Stop managing hooks here' }));
    expect(hooksApi.save).not.toHaveBeenCalled();
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Stop managing hooks here' }));
    await waitFor(() => expect(onRemoved).toHaveBeenCalled());
    expect(hooksApi.save).toHaveBeenCalledWith({ project: APP, remove: true });
    expect(hooksApi.configure).not.toHaveBeenCalled();
  });

  it('keeps the page when stopping fails', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.save).mockRejectedValue(new Error('refused'));
    const onRemoved = vi.fn();
    wrap(<HooksScope data={conflictData()} project={APP} onRemoved={onRemoved} />);
    await user.click(screen.getByRole('button', { name: 'Stop managing hooks here' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Stop managing hooks here' }));
    expect(await screen.findByText('refused')).toBeInTheDocument();
    expect(onRemoved).not.toHaveBeenCalled();
  });

  it('offers no Codex trust in a project, even with a Codex hook', () => {
    wrap(
      <HooksSyncBox
        project={APP}
        plan={{
          ...plan([]),
          blocked: false,
          changes: [change({ target: 'codex', name: 'lint', root: APP, action: 'add' })],
        }}
      />,
    );
    expect(screen.getByRole('button', { name: 'Sync hooks' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Trust Codex hooks' })).not.toBeInTheDocument();
  });

  it('allows taking over all conflicting native hooks when multiple hooks conflict', async () => {
    const user = userEvent.setup();
    const conflictMsg =
      'an identical hook exists that Skillshare does not manage; import it or explicitly replace it';
    vi.mocked(hooksApi.preview).mockImplementation(async (m) => {
      const taken = new Set([
        ...(m?.replaceNames ?? []),
        ...(m?.replace && m.name ? [m.name] : []),
      ]);
      const changes = ['claude-stop', 'claude-pretooluse'].map((name) =>
        change({
          target: 'claude',
          name,
          action: taken.has(name) ? 'adopt' : 'conflict',
          message: taken.has(name) ? undefined : conflictMsg,
        }),
      );
      return {
        ...plan(changes),
        blocked: changes.some((c) => c.action === 'conflict'),
      };
    });
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });

    wrap(
      <HooksSyncDialog
        takeover={{ name: 'claude-stop', entry: { bindings: { claude: { events: {} } } } }}
        canTakeOver={(c) => c.action === 'conflict' && !c.root}
        onClose={vi.fn()}
      />,
    );

    const takeoverAllBtn = await screen.findByRole('button', {
      name: 'Take over the conflicting native hooks',
    });
    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeDisabled();

    await user.click(takeoverAllBtn);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Sync Now' })).toBeEnabled());
    expect(
      screen.queryByRole('button', { name: 'Take over the conflicting native hooks' }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Sync Now' }));
    await waitFor(() =>
      expect(hooksApi.configure).toHaveBeenCalledWith(
        { replaceNames: ['claude-stop', 'claude-pretooluse'] },
        expect.any(String),
        true,
      ),
    );
  });

  it('hides take-over controls once staged even when a conflict cannot be replaced', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue({
      ...plan([
        change({
          target: 'claude',
          name: 'claude-stop',
          action: 'conflict',
          message: 'managed by another Skillshare config',
        }),
      ]),
      blocked: true,
    });

    wrap(
      <HooksSyncDialog canTakeOver={(c) => c.action === 'conflict' && !c.root} onClose={vi.fn()} />,
    );

    const takeoverAllBtn = await screen.findByRole('button', {
      name: 'Take over the conflicting native hooks',
    });
    await user.click(takeoverAllBtn);

    await waitFor(() =>
      expect(hooksApi.preview).toHaveBeenCalledWith({ replaceNames: ['claude-stop'] }),
    );
    expect(
      screen.queryByRole('button', { name: 'Take over the conflicting native hooks' }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /Take over native hooks/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeDisabled();
  });
});
