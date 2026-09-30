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
import HooksUnmanagedNote from './HooksUnmanagedNote';

vi.mock('../../api/hooks', async (load) => ({
  ...await load<typeof import('../../api/hooks')>(),
  hooksApi: { list: vi.fn(), preview: vi.fn(), save: vi.fn(), configure: vi.fn(), syncProject: vi.fn(), render: vi.fn() },
}));

const APP = '/work/app';
const change = (over: Partial<HookChange>): HookChange => ({ target: 'claude', path: '/home/u/.claude/settings.json', name: 'g', action: 'unchanged', ...over });
// Global work is pending and another project conflicts; only the selected root's own changes matter to it.
const plan = (mine: HookChange[]): HookPlan => ({
  revision: 'rev-all', fingerprint: 'fp', sourcePath: '/s.yaml', blocked: true,
  changes: [change({ name: 'g', action: 'add' }), change({ name: 'o', root: '/work/other', path: '/work/other/.claude/settings.json', action: 'conflict' }), ...mine],
});
const mine = (action: string) => change({ name: 'lint', root: APP, path: `${APP}/.claude/settings.json`, action });

const wrap = (ui: React.ReactNode) => render(<MemoryRouter><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><I18nProvider><ToastProvider>{ui}</ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);

describe('project scope in hooks dialogs', () => {
  beforeEach(() => vi.resetAllMocks());
  afterEach(() => vi.restoreAllMocks());

  it('saves and syncs a project hook with one configure call carrying the whole plan revision', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('add')]));
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    const onSaved = vi.fn();
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={onSaved} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await user.type(screen.getByLabelText('Event 1'), 'PostToolUse');
    await user.type(screen.getByLabelText('Command 1'), './lint.sh');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    // Neither the global pending change nor the other project's conflict is shown or blocks this root.
    const save = await screen.findByRole('button', { name: 'Save and sync' });
    expect(screen.queryByText('o')).not.toBeInTheDocument();
    expect(save).toBeEnabled();
    await user.click(save);
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(hooksApi.configure).toHaveBeenCalledTimes(1);
    expect(hooksApi.configure).toHaveBeenCalledWith(expect.objectContaining({ project: APP, name: 'lint' }), 'rev-all', true);
    expect(hooksApi.syncProject).not.toHaveBeenCalled();
  });

  it('keeps a matcher-only extra row, marks what it is missing and blocks saving', async () => {
    const user = userEvent.setup();
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={vi.fn()} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await user.type(screen.getByLabelText('Event 1'), 'Stop');
    await user.type(screen.getByLabelText('Command 1'), 'true');
    await user.click(screen.getByRole('button', { name: 'Add command' }));
    await user.type(screen.getByLabelText('Matcher 2'), 'Bash');
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Preview' })).toBeDisabled();
    expect(screen.getByLabelText('Event 2').parentElement).toHaveClass('err');
    expect(screen.getByLabelText('Command 2').parentElement).toHaveClass('err');
    expect(screen.getByLabelText('Matcher 2')).toHaveValue('Bash');
  });

  it('keeps the preview current while its Save and sync is in flight', async () => {
    const user = userEvent.setup();
    let finish = () => {};
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('add')]));
    vi.mocked(hooksApi.configure).mockReturnValue(new Promise((resolve) => { finish = () => resolve({ applied: [], backupIds: [] }); }));
    const onSaved = vi.fn();
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={onSaved} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await user.type(screen.getByLabelText('Event 1'), 'Stop');
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
    vi.mocked(hooksApi.preview).mockResolvedValueOnce({ ...plan([mine('add')]), revision: 'rev-1' }).mockResolvedValueOnce({ ...plan([mine('add')]), revision: 'rev-2' });
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={vi.fn()} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await user.type(screen.getByLabelText('Event 1'), 'Stop');
    await user.type(screen.getByLabelText('Command 1'), 'true');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    await user.click(await screen.findByRole('button', { name: 'Back' }));
    await user.type(screen.getByLabelText('Command 1'), ' --x');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    await user.click(await screen.findByRole('button', { name: 'Save and sync' }));
    await waitFor(() => expect(hooksApi.configure).toHaveBeenCalledWith(expect.objectContaining({ name: 'lint' }), 'rev-2', true));
    expect(JSON.stringify(vi.mocked(hooksApi.configure).mock.calls[0][0])).toContain('true --x');
  });

  it('blocks a project only for a conflict in its own root', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('conflict')]));
    wrap(<HookDialog existingNames={[]} project={APP} onClose={vi.fn()} onSaved={vi.fn()} />);
    await user.type(screen.getByLabelText('Name'), 'lint');
    await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
    await user.type(screen.getByLabelText('Event 1'), 'PostToolUse');
    await user.type(screen.getByLabelText('Command 1'), './lint.sh');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    expect(await screen.findByRole('button', { name: 'Save and sync' })).toBeDisabled();
    expect(screen.getByRole('checkbox', { name: 'Take over the conflicting native hooks' })).toBeInTheDocument();
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
    wrap(<HooksSyncDialog project={APP} takeover={{ name: 'lint', entry: { bindings: {} } }} onClose={vi.fn()} />);
    await user.click(await screen.findByRole('button', { name: 'Sync Now' }));
    await waitFor(() => expect(hooksApi.configure).toHaveBeenCalledWith({ project: APP, name: 'lint', entry: { bindings: {} }, replace: true }, 'rev-all', true));
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
    expect(hooksApi.configure).toHaveBeenCalledWith({ project: APP, name: 'lint', remove: true }, 'rev-all', true);
    expect(hooksApi.syncProject).not.toHaveBeenCalled();
  });

  it("previews a project removal with its root's whole sync, not another root's conflict", async () => {
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([mine('remove'), { ...mine('add'), name: 'fmt' }]));
    wrap(<HooksRemoveDialog name="lint" project={APP} onClose={vi.fn()} onSaved={vi.fn()} />);
    const button = await screen.findByRole('button', { name: 'Remove and sync' });
    await waitFor(() => expect(button).toBeEnabled());
    // The sync writes this root's other pending hook too, so the preview shows it; other scopes stay out.
    expect(screen.getByText('fmt')).toBeInTheDocument();
    expect(screen.queryByText('o')).not.toBeInTheDocument();
    expect(screen.queryByText('g')).not.toBeInTheDocument();
    expect(screen.queryByText(/Nothing is written until/)).not.toBeInTheDocument();
  });

  it('previews a global removal with the whole plan its sync applies', async () => {
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([]));
    wrap(<HooksRemoveDialog name="g" onClose={vi.fn()} onSaved={vi.fn()} />);
    expect(await screen.findByText('o')).toBeInTheDocument();
    expect(screen.getByText('g')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove and sync' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Remove from source only' })).toBeEnabled();
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
    await user.type(screen.getByLabelText('Event 1'), 'PostToolUse');
    await user.type(screen.getByLabelText('Command 1'), './lint.sh');
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    await user.click(await screen.findByRole('button', { name: 'Save and sync' }));
    expect(await screen.findByText('preview is stale')).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByLabelText('Command 1')).toHaveValue('./lint.sh');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(hooksApi.save).toHaveBeenCalledWith(expect.objectContaining({ project: APP, name: 'lint' })));
  });
});

describe('sync box and unmanaged note', () => {
  beforeEach(() => vi.resetAllMocks());

  it('offers sync for a pending removal even when the scope has no entries left', () => {
    wrap(<HooksSyncBox plan={{ revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [mine('remove')] }} project={APP} />);
    expect(screen.getByRole('button', { name: 'Sync hooks' })).toBeEnabled();
  });

  it('lets a blocked card open its preview while the final sync stays disabled', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(plan([]));
    wrap(<HooksSyncBox plan={plan([])} />);
    expect(screen.queryByRole('button', { name: 'Sync hooks' })).not.toBeInTheDocument();
    // The conflict is named with its Agent, its shortened path and a link to its project's Hooks tab.
    expect(screen.getByText('o')).toBeInTheDocument();
    expect(screen.getByTitle('/work/other/.claude/settings.json')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '/work/other' })).toHaveAttribute('href', '/projects/%2Fwork%2Fother?tab=hooks');
    await user.click(screen.getByRole('button', { name: 'View conflicts' }));
    const dialog = await screen.findByRole('dialog');
    expect(await within(dialog).findByRole('button', { name: 'Sync Now' })).toBeDisabled();
    expect(hooksApi.configure).not.toHaveBeenCalled();
    expect(hooksApi.syncProject).not.toHaveBeenCalled();
  });

  it('keeps a conflict-only card reviewable', () => {
    wrap(<HooksSyncBox plan={{ revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: true, changes: [mine('conflict')] }} project={APP} />);
    expect(screen.getByRole('button', { name: 'View conflicts' })).toBeEnabled();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('keeps the sync label for an ordinary pending plan', () => {
    wrap(<HooksSyncBox plan={{ revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [mine('add')] }} project={APP} />);
    expect(screen.getByRole('button', { name: 'Sync hooks' })).toBeEnabled();
  });

  it('lists each Agent once, counts every hook and titles the full native path', () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    wrap(<HooksUnmanagedNote onImport={vi.fn()} entries={[
      { target: 'claude', path: '/home/u/.claude/settings.json', names: ['a', 'b'] },
      { target: 'claude', path: `${APP}/.claude/settings.json`, names: ['c'] },
    ]} />);
    expect(screen.getByText(/3/)).toBeInTheDocument();
    expect(screen.getByTitle(`${APP}/.claude/settings.json`)).toBeInTheDocument();
    expect(within(document.body).getAllByTitle(/settings\.json$/)).toHaveLength(2);
    expect(error).not.toHaveBeenCalled();
  });
});

describe('project row preview', () => {
  beforeEach(() => vi.resetAllMocks());

  it("previews a project hook under its own root with that hook's entry", async () => {
    const user = userEvent.setup();
    const entry = { bindings: { claude: { events: { Stop: [{ hooks: [{ type: 'command', command: 'true' }] }] } } } };
    vi.mocked(hooksApi.render).mockResolvedValue({ rendered: [{ target: 'claude', path: `${APP}/.claude/settings.json`, content: 'projectClaudeFile' }] });
    const data = { source: { path: '/s.yaml', configPath: '/s.yaml', entries: { g: { bindings: {} } }, projects: { [APP]: { entries: { lint: entry } } } }, targets: [], paths: {}, backups: [], unmanaged: [], projectConfigs: [], projectPaths: {} } as never;
    wrap(<HooksScope data={data} project={APP} header={() => null} />);
    await user.click(await screen.findByRole('button', { name: 'More actions for lint' }));
    await user.click(screen.getByRole('menuitem', { name: 'View what each Agent gets' }));
    const dialog = await screen.findByRole('dialog');
    expect(hooksApi.render).toHaveBeenCalledWith({ project: APP, name: 'lint', entry });
    expect(await within(dialog).findByText('projectClaudeFile')).toBeInTheDocument();
    expect(within(dialog).getByTitle(APP)).toBeInTheDocument();
  });

  const IDENTICAL = 'an identical hook exists that Skillshare does not manage; import it or explicitly replace it';
  const lintEntry = { bindings: { claude: { events: { Stop: [{ hooks: [{ type: 'command', command: 'true' }] }] } } } };
  const conflicted = () => plan([{ ...mine('conflict'), message: IDENTICAL }]);
  const conflictData = () => ({ source: { path: '/s.yaml', configPath: '/s.yaml', entries: { g: { bindings: {} } }, projects: { [APP]: { entries: { lint: lintEntry } } } }, targets: [], paths: {}, backups: [], unmanaged: [], projectConfigs: [], projectPaths: {}, plan: conflicted() }) as never;
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

  it('offers the same take-over from the conflict review, with the conflict worded for this UI', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue(conflicted());
    wrap(<HooksScope data={conflictData()} project={APP} header={() => null} />);
    await user.click(screen.getByRole('button', { name: 'View conflicts' }));
    const review = await screen.findByRole('dialog');
    expect(await within(review).findByText(/is not managed by Skillshare/)).toBeInTheDocument();
    expect(within(review).queryByText(IDENTICAL)).not.toBeInTheDocument();
    await user.hover(within(review).getByText(/is not managed by Skillshare/));
    expect(await screen.findByRole('tooltip')).toHaveTextContent(/Take over native hooks/);
    await user.click(within(review).getByRole('button', { name: 'Take over native hooks · lint' }));
    expect(await screen.findByRole('dialog', { name: 'Take over lint?' })).toBeInTheDocument();
    expect(hooksApi.preview).toHaveBeenLastCalledWith(takeover);
  });

  it('shows the whole plan global sync writes in the global rail, with project links and no foreign take-over', () => {
    const data = { source: { path: '/s.yaml', configPath: '/s.yaml', entries: {}, projects: { [APP]: { entries: { lint: lintEntry, fmt: lintEntry } } } }, targets: [], paths: {}, backups: [], unmanaged: [], projectConfigs: [], projectPaths: {},
      plan: { revision: 'r', fingerprint: 'fp', sourcePath: '/s.yaml', blocked: true, changes: [{ ...mine('add'), name: 'fmt' }, { ...mine('conflict'), message: IDENTICAL }] } } as never;
    wrap(<HooksScope data={data} header={() => null} />);
    expect(screen.getByText('fmt')).toBeInTheDocument();
    expect(screen.getAllByRole('link', { name: APP }).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'View conflicts' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Take over native hooks/ })).not.toBeInTheDocument();
  });

  it("tells a hook's settings write from its script write in the pending list", () => {
    wrap(<HooksSyncBox project={APP} plan={{ revision: 'r', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [
      mine('add'), { ...mine('add'), path: `${APP}/.claude/hooks/skillshare/lint/check.sh` },
    ] }} />);
    expect(screen.getByText('→ Claude · settings.json')).toBeInTheDocument();
    expect(screen.getByText('→ Claude · check.sh')).toBeInTheDocument();
  });
});
