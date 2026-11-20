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
  ...(await load<typeof import('../api/hooks')>()),
  hooksApi: {
    list: vi.fn(),
    catalog: vi.fn(),
    preview: vi.fn(),
    render: vi.fn(),
    save: vi.fn(),
    configure: vi.fn(),
    import: vi.fn(),
    previewRestore: vi.fn(),
    restore: vi.fn(),
    previewCodexTrust: vi.fn(),
    trustCodex: vi.fn(),
  },
}));

const plan = (changes: HookPlan['changes']) => ({
  revision: 'rev-1',
  fingerprint: 'fp',
  sourcePath: '/s.yaml',
  blocked: false,
  changes,
});
const inventory = (over: Partial<HookInventory> = {}): HookInventory => ({
  source: {
    path: '/home/u/.config/skillshare/config.yaml',
    configPath: '',
    entries: {
      guard: {
        description: 'Block force pushes',
        bindings: {
          claude: {
            events: {
              PreToolUse: [
                {
                  matcher: 'Bash',
                  hooks: [{ type: 'command', command: './guard.sh', timeout: 10 }],
                },
              ],
            },
          },
          opencode: { code: 'export const Plugin = async () => ({})' },
        },
      },
    },
  },
  targets: [
    { name: 'claude', kind: 'command', note: '' },
    { name: 'opencode', kind: 'code', note: '' },
  ],
  paths: {
    claude: '/home/u/.claude/settings.json',
    opencode: '/home/u/.config/opencode/plugins/skillshare-guard.ts',
  },
  plan: plan([
    { target: 'claude', path: '/home/u/.claude/settings.json', name: 'guard', action: 'unchanged' },
    {
      target: 'opencode',
      path: '/home/u/.config/opencode/plugins/skillshare-guard.ts',
      name: 'guard',
      action: 'add',
    },
  ]),
  previewError: '',
  backups: [],
  unmanaged: [],
  ...over,
});

const catalog = {
  claude: {
    timeoutUnit: 'seconds' as const,
    events: [
      { name: 'PreToolUse', description: 'Before a tool runs', matcher: true },
      { name: 'PostToolUse', description: 'After a tool succeeds', matcher: true },
      { name: 'Stop', description: 'When the main agent finishes', matcher: false },
    ],
  },
};

const renderPage = () =>
  render(
    <MemoryRouter>
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <I18nProvider>
          <ToastProvider>
            <HooksPage />
          </ToastProvider>
        </I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );

describe('Hooks page', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.list).mockResolvedValue(inventory());
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
    HTMLElement.prototype.scrollIntoView = vi.fn();
    // Nor Range.getClientRects, which the code editor measures a filled template with.
    Range.prototype.getClientRects = () =>
      ({
        length: 0,
        item: () => null,
        [Symbol.iterator]: [][Symbol.iterator],
      }) as unknown as DOMRectList;
  });

  it('disables a hook by saving it as disabled, without touching its bindings', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    renderPage();
    await user.click(await screen.findByRole('switch', { name: 'Enable guard' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith({
        name: 'guard',
        entry: { ...inventory().source.entries.guard, enabled: false },
      }),
    );
  });

  it('adds a code-Agent hook with its own code and no event fields', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.list).mockResolvedValue(
      inventory({ source: { path: '/s', configPath: '', entries: {} }, plan: null }),
    );
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    renderPage();
    await user.click((await screen.findAllByRole('button', { name: 'Add hook' }))[0]);
    const dialog = await screen.findByRole('dialog', { name: 'Add hook' });
    await user.type(within(dialog).getByLabelText('Name'), 'audit');
    await user.click(within(dialog).getByRole('checkbox', { name: /OpenCode/ }));
    // An untouched target is not filled yet, so the hook cannot be saved.
    expect(within(dialog).getByRole('button', { name: 'Save' })).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Start from scratch' }));
    expect(within(dialog).queryByLabelText('Event 1')).not.toBeInTheDocument();
    expect(
      within(dialog).getByText(/not converted or checked against your OpenCode version/),
    ).toBeInTheDocument();
  });

  it('builds a command hook from plain fields and saves it in the Agent-native shape', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.list).mockResolvedValue(
      inventory({ source: { path: '/s', configPath: '', entries: {} }, plan: null }),
    );
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    renderPage();
    await user.click((await screen.findAllByRole('button', { name: 'Add hook' }))[0]);
    const dialog = await screen.findByRole('dialog', { name: 'Add hook' });
    await user.type(within(dialog).getByLabelText('Name'), 'lint');
    await user.click(within(dialog).getByRole('checkbox', { name: /Claude/ }));
    await user.click(within(dialog).getByRole('combobox', { name: 'Event 1' }));
    await user.click(await screen.findByRole('option', { name: /^PostToolUse/ }));
    await user.type(within(dialog).getByLabelText('Tool filter 1'), 'Edit');
    await user.type(within(dialog).getByLabelText('Command 1'), './lint.sh');
    await user.type(within(dialog).getByLabelText('Timeout 1'), '20');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith({
        name: 'lint',
        entry: {
          bindings: {
            claude: {
              events: {
                PostToolUse: [
                  {
                    matcher: 'Edit',
                    hooks: [{ type: 'command', command: './lint.sh', timeout: 20 }],
                  },
                ],
              },
            },
          },
        },
      }),
    );
  });

  it('offers a takeover only when the plan conflicts, and sends replace only then', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.list).mockResolvedValue(
      inventory({
        plan: {
          ...plan([
            {
              target: 'claude',
              path: '/home/u/.claude/settings.json',
              name: 'guard',
              action: 'conflict',
              message: 'identical unmanaged hook',
            },
          ]),
          blocked: true,
        },
      }),
    );
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(hooksApi.preview).mockResolvedValue({
      ...plan([
        {
          target: 'claude',
          path: '/home/u/.claude/settings.json',
          name: 'guard',
          action: 'conflict',
        },
      ]),
      blocked: true,
    });
    renderPage();
    // A blocked plan does not stop source-only saves, and the toggle never replaces.
    await user.click(await screen.findByRole('switch', { name: 'Enable guard' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith(expect.not.objectContaining({ replace: true })),
    );
  });

  it('applies only the plan it previewed when syncing', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.preview).mockResolvedValue({
      ...plan([{ target: 'opencode', path: '/p.ts', name: 'guard', action: 'add' }]),
      revision: 'rev-fresh',
    });
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: ['/p.ts'], backupIds: [] });
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Sync hooks' }));
    const dialog = await screen.findByRole('dialog', { name: 'Sync hooks' });
    await user.click(await within(dialog).findByRole('button', { name: 'Sync Now' }));
    await waitFor(() => expect(hooksApi.configure).toHaveBeenCalledWith({}, 'rev-fresh', true));
  });

  it('shows a Codex hook command before trusting and sends the preview revision', async () => {
    const user = userEvent.setup();
    const hook = {
      target: 'codex-2',
      entry: 'guard',
      key: 'k',
      command: 'echo "a long one-liner" && ./guard.sh',
      action: 'needed' as const,
    };
    vi.mocked(hooksApi.list).mockResolvedValue(
      inventory({
        targets: [
          { name: 'claude', kind: 'command', note: '' },
          { name: 'codex-2', agent: 'codex', kind: 'command', note: '' },
        ],
        plan: plan([
          {
            target: 'codex-2',
            path: '/home/u/.codex-2/hooks.json',
            name: 'guard',
            action: 'unchanged',
          },
        ]),
      }),
    );
    vi.mocked(hooksApi.previewCodexTrust).mockResolvedValue({
      hooks: [hook],
      revision: 'rev-trust',
    });
    vi.mocked(hooksApi.trustCodex).mockResolvedValue({
      hooks: [{ ...hook, action: 'trusted' }],
      revision: 'rev-trust',
    });
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Trust Codex hooks' }));
    const dialog = await screen.findByRole('dialog', { name: 'Trust Codex hooks' });
    expect(await within(dialog).findByText(hook.command)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Trust these hooks' }));
    await waitFor(() => expect(hooksApi.trustCodex).toHaveBeenCalledWith('rev-trust'));
  });

  const openCodexTrust = async (
    hooks: {
      target: string;
      entry: string;
      key: string;
      command: string;
      action: 'needed' | 'unchanged';
      disabled?: boolean;
    }[],
    failed?: { target: string; error: string }[],
  ) => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.list).mockResolvedValue(
      inventory({
        targets: [
          { name: 'claude', kind: 'command', note: '' },
          { name: 'codex', kind: 'command', note: '' },
        ],
        plan: plan([
          {
            target: 'codex',
            path: '/home/u/.codex/hooks.json',
            name: 'guard',
            action: 'unchanged',
          },
        ]),
      }),
    );
    vi.mocked(hooksApi.previewCodexTrust).mockResolvedValue({
      hooks,
      revision: 'rev-trust',
      failed,
    });
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Trust Codex hooks' }));
    return { user, dialog: await screen.findByRole('dialog', { name: 'Trust Codex hooks' }) };
  };
  const codexHook = {
    target: 'codex',
    entry: 'guard',
    key: 'k',
    command: './guard.sh',
    action: 'needed' as const,
  };

  it('offers no retry when the Codex hooks changed since they were listed', async () => {
    vi.mocked(hooksApi.trustCodex).mockRejectedValue(
      new Error('Codex hooks changed since they were listed'),
    );
    const { user, dialog } = await openCodexTrust([codexHook]);
    await user.click(await within(dialog).findByRole('button', { name: 'Trust these hooks' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'Codex hooks changed since they were listed',
    );
    expect(
      within(dialog).queryByRole('button', { name: 'Trust these hooks' }),
    ).not.toBeInTheDocument();
  });

  it('has nothing to confirm when every Codex hook is already trusted', async () => {
    const { dialog } = await openCodexTrust([{ ...codexHook, action: 'unchanged' }]);
    expect(await within(dialog).findByText('No Codex hooks need trust.')).toBeInTheDocument();
    expect(
      within(dialog).queryByRole('button', { name: 'Trust these hooks' }),
    ).not.toBeInTheDocument();
  });

  it('marks a hook Codex has disabled', async () => {
    const { dialog } = await openCodexTrust([{ ...codexHook, disabled: true }]);
    expect(await within(dialog).findByText('Disabled in Codex')).toBeInTheDocument();
  });

  it('names a home Codex could not list and still offers the others', async () => {
    const { dialog } = await openCodexTrust(
      [codexHook],
      [{ target: 'codex-2', error: 'codex exited' }],
    );
    expect(
      await within(dialog).findByText(
        'Codex could not list the hooks of codex-2, so they are left out: codex exited',
      ),
    ).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Trust these hooks' })).toBeEnabled();
  });

  it('holds Codex trust back until pending changes are synced', async () => {
    vi.mocked(hooksApi.list).mockResolvedValue(
      inventory({
        targets: [
          { name: 'claude', kind: 'command', note: '' },
          { name: 'codex', kind: 'command', note: '' },
        ],
        plan: plan([
          { target: 'codex', path: '/home/u/.codex/hooks.json', name: 'guard', action: 'update' },
        ]),
      }),
    );
    renderPage();
    expect(await screen.findByRole('button', { name: 'Sync hooks' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Trust Codex hooks' })).not.toBeInTheDocument();
  });
});
