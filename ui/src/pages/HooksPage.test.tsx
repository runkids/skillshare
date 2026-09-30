import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { hooksApi, type HookInventory, type HookPlan } from '../api/hooks';
import { ToastProvider } from '../components/Toast';
import HooksPage from './HooksPage';

vi.mock('../api/hooks', async (load) => ({
  ...await load<typeof import('../api/hooks')>(),
  hooksApi: { list: vi.fn(), preview: vi.fn(), render: vi.fn(), save: vi.fn(), configure: vi.fn(), import: vi.fn(), previewRestore: vi.fn(), restore: vi.fn() },
}));

const plan = (changes: HookPlan['changes']) => ({ revision: 'rev-1', fingerprint: 'fp', sourcePath: '/s.yaml', blocked: false, changes });
const inventory = (over: Partial<HookInventory> = {}): HookInventory => ({
  source: { path: '/home/u/.config/skillshare/config.yaml', configPath: '', entries: {
    guard: { description: 'Block force pushes', bindings: {
      claude: { events: { PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: './guard.sh', timeout: 10 }] }] } },
      opencode: { code: 'export const Plugin = async () => ({})' },
    } },
  } },
  targets: [{ name: 'claude', kind: 'command', note: '' }, { name: 'opencode', kind: 'code', note: '' }],
  paths: { claude: '/home/u/.claude/settings.json', opencode: '/home/u/.config/opencode/plugins/skillshare-guard.ts' },
  plan: plan([
    { target: 'claude', path: '/home/u/.claude/settings.json', name: 'guard', action: 'unchanged' },
    { target: 'opencode', path: '/home/u/.config/opencode/plugins/skillshare-guard.ts', name: 'guard', action: 'add' },
  ]),
  previewError: '', backups: [], unmanaged: [],
  ...over,
});

const renderPage = () => render(<MemoryRouter><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><I18nProvider><ToastProvider><HooksPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);

describe('Hooks page', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.list).mockResolvedValue(inventory());
  });

  it('shows what Skillshare synchronized apart from what the Agent itself trusts', async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Show Agents for guard' }));
    expect(screen.getByText('Synced')).toBeInTheDocument();
    expect(screen.getByText('Not synced yet')).toBeInTheDocument();
    expect(screen.getAllByText(/Each Agent decides|check its own hook list/).length).toBeGreaterThan(0);
  });

  it('disables a hook by saving it as disabled, without touching its bindings', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    renderPage();
    await user.click(await screen.findByRole('switch', { name: 'Enable guard' }));
    await waitFor(() => expect(hooksApi.save).toHaveBeenCalledWith({ name: 'guard', entry: { ...inventory().source.entries.guard, enabled: false } }));
  });

  it('adds a code-Agent hook with its own code and no event fields', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.list).mockResolvedValue(inventory({ source: { path: '/s', configPath: '', entries: {} }, plan: null }));
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    renderPage();
    await user.click((await screen.findAllByRole('button', { name: 'Add hook' }))[0]);
    const dialog = await screen.findByRole('dialog', { name: 'Add hook' });
    await user.type(within(dialog).getByLabelText('Name'), 'audit');
    await user.click(within(dialog).getByRole('checkbox', { name: /OpenCode/ }));
    expect(within(dialog).queryByLabelText('Event 1')).not.toBeInTheDocument();
    expect(within(dialog).getByText(/not converted or checked against your OpenCode version/)).toBeInTheDocument();
    // Without code the hook is incomplete and cannot be saved.
    expect(within(dialog).getByRole('button', { name: 'Save' })).toBeDisabled();
  });

  it('builds a command hook from plain fields and saves it in the Agent-native shape', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.list).mockResolvedValue(inventory({ source: { path: '/s', configPath: '', entries: {} }, plan: null }));
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    renderPage();
    await user.click((await screen.findAllByRole('button', { name: 'Add hook' }))[0]);
    const dialog = await screen.findByRole('dialog', { name: 'Add hook' });
    await user.type(within(dialog).getByLabelText('Name'), 'lint');
    await user.click(within(dialog).getByRole('checkbox', { name: /Claude/ }));
    await user.type(within(dialog).getByLabelText('Event 1'), 'PostToolUse');
    await user.type(within(dialog).getByLabelText('Matcher 1'), 'Edit');
    await user.type(within(dialog).getByLabelText('Command 1'), './lint.sh');
    await user.type(within(dialog).getByLabelText('Timeout 1'), '20');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(hooksApi.save).toHaveBeenCalledWith({
      name: 'lint',
      entry: { bindings: { claude: { events: { PostToolUse: [{ matcher: 'Edit', hooks: [{ type: 'command', command: './lint.sh', timeout: 20 }] }] } } } },
    }));
  });

  it('offers a takeover only when the plan conflicts, and sends replace only then', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.list).mockResolvedValue(inventory({ plan: { ...plan([{ target: 'claude', path: '/home/u/.claude/settings.json', name: 'guard', action: 'conflict', message: 'identical unmanaged hook' }]), blocked: true } }));
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(hooksApi.preview).mockResolvedValue({ ...plan([{ target: 'claude', path: '/home/u/.claude/settings.json', name: 'guard', action: 'conflict' }]), blocked: true });
    renderPage();
    // A blocked plan does not stop source-only saves, and the toggle never replaces.
    await user.click(await screen.findByRole('switch', { name: 'Enable guard' }));
    await waitFor(() => expect(hooksApi.save).toHaveBeenCalledWith(expect.not.objectContaining({ replace: true })));
  });

  it('has no permanent trust banner above the list, and opens the native preview from the row menu', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.render).mockResolvedValue({ rendered: [
      { target: 'claude', path: '/home/u/.claude/settings.json', content: 'claude native file' },
      { target: 'opencode', path: '/home/u/.config/opencode/plugins/skillshare-guard.ts', content: 'opencodePluginSource' },
    ] });
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'More actions for guard' }));
    expect(screen.queryByText(/Synced means Skillshare wrote the native file/)).not.toBeInTheDocument();
    await user.click(screen.getByRole('menuitem', { name: 'View what each Agent gets' }));
    const dialog = await screen.findByRole('dialog', { name: 'View what each Agent gets' });
    await waitFor(() => expect(hooksApi.render).toHaveBeenCalledWith({ name: 'guard', entry: inventory().source.entries.guard }));
    expect(await within(dialog).findByText('Sync writes 2 files')).toBeInTheDocument();
    expect(within(dialog).getByText('claude native file')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: /OpenCode/ }));
    expect(within(dialog).getByText('opencodePluginSource')).toBeInTheDocument();
    expect(hooksApi.save).not.toHaveBeenCalled();
    expect(hooksApi.configure).not.toHaveBeenCalled();
  });

  it('applies only the plan it previewed when syncing', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue({ ...plan([{ target: 'opencode', path: '/p.ts', name: 'guard', action: 'add' }]), revision: 'rev-fresh' });
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: ['/p.ts'], backupIds: [] });
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Sync hooks' }));
    const dialog = await screen.findByRole('dialog', { name: 'Sync hooks' });
    await user.click(await within(dialog).findByRole('button', { name: 'Sync Now' }));
    await waitFor(() => expect(hooksApi.configure).toHaveBeenCalledWith({}, 'rev-fresh', true));
  });

  it('shows the empty state with add and import', async () => {
    vi.mocked(hooksApi.list).mockResolvedValue(inventory({ source: { path: '/s', configPath: '', entries: {} }, plan: null }));
    renderPage();
    expect(await screen.findByText('No hooks yet')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /Import from a target/ }).length).toBeGreaterThan(0);
  });

  it('keeps the sync box when the last hook was removed from the source but its native output is still pending', async () => {
    vi.mocked(hooksApi.list).mockResolvedValue(inventory({ source: { path: '/s', configPath: '', entries: {} }, plan: plan([{ target: 'claude', path: '/home/u/.claude/settings.json', name: 'guard', action: 'remove' }]) }));
    renderPage();
    expect(await screen.findByText('No hooks yet')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sync hooks' })).toBeEnabled();
  });

  it('surfaces a list error instead of an empty page', async () => {
    vi.mocked(hooksApi.list).mockRejectedValue(new Error('boom'));
    renderPage();
    expect(await screen.findByText('boom')).toBeInTheDocument();
  });
});
