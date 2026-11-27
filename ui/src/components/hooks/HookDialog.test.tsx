import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { hooksApi } from '../../api/hooks';
import { ApiError } from '../../api/http';
import type { HookCandidate, HookInventory } from '../../api/hooks';
import HookDialog from './HookDialog';
import HooksImportDialog from './HooksImportDialog';
import HooksRemoveDialog from './HooksRemoveDialog';

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
    catalog: vi.fn(),
    preview: vi.fn(),
    save: vi.fn(),
    configure: vi.fn(),
    import: vi.fn(),
    previewNative: vi.fn(),
    applyNative: vi.fn(),
  },
}));

const catalog = {
  claude: {
    timeoutUnit: 'seconds' as const,
    events: [
      { name: 'PostToolUse', description: 'After a tool succeeds', matcher: true },
      { name: 'Stop', description: 'When the main agent finishes', matcher: false },
    ],
  },
};

const wrap = (ui: React.ReactNode) =>
  render(
    <MemoryRouter>
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <I18nProvider>{ui}</I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );

const openClaude = async (user: ReturnType<typeof userEvent.setup>) => {
  wrap(<HookDialog existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} />);
  await user.type(screen.getByLabelText('Name'), 'lint');
  await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
};

describe('Git raw YAML dialog', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
  });
  it('edits and saves commands and files together without Agent event fields', async () => {
    const user = userEvent.setup();
    const binding = {
      commands: {
        'tool.check': { events: ['pre-commit'], command: '{files}/check.sh', parallel: false },
      },
      files: { 'check.sh': '#!/bin/sh\nexit 0\n' },
    };
    wrap(
      <HookDialog
        initial={{ name: 'guard', entry: { bindings: { git: binding } } }}
        existingNames={['guard']}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );
    const editor = screen.getByLabelText('Git Binding YAML');
    expect((editor as HTMLTextAreaElement).value).toContain('commands:');
    expect(screen.queryByLabelText('Event 1')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Fields' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith({
        name: 'guard',
        entry: { bindings: { git: binding } },
      }),
    );
  });
});

describe('hook dialog', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
    HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it('keeps invalid JSON and says why when switching back to fields', async () => {
    const user = userEvent.setup();
    await openClaude(user);
    await user.click(screen.getByRole('tab', { name: 'Native JSON' }));
    const editor = screen.getByLabelText('Claude Native JSON');
    const broken = '{\n  "Stop": [\n    { "hooks": [] }\n    { "hooks": [] }\n  ]\n}';
    await user.clear(editor);
    await user.click(editor);
    await user.paste(broken);
    expect(screen.getByRole('tab', { name: /Native JSON/ })).toContainElement(
      screen.getByLabelText('1 errors'),
    );
    await user.click(screen.getByRole('tab', { name: 'Fields' }));
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Fix the JSON before switching to fields. Line 4:',
    );
    expect(screen.getByLabelText('Claude Native JSON')).toHaveValue(broken);
  });
});

describe('hooks import', () => {
  beforeEach(() => vi.resetAllMocks());

  const candidate = (name: string, command: string): HookCandidate => ({
    name,
    problems: [],
    warnings: [],
    entry: {
      bindings: { claude: { events: { Stop: [{ hooks: [{ type: 'command', command }] }] } } },
    },
  });
  const data = {
    source: { path: '/s.yaml', configPath: '/s.yaml', entries: {} },
    targets: [],
    backups: [],
    plan: null,
    previewError: '',
    paths: { claude: '/home/u/.claude/settings.json' },
    unmanaged: [
      {
        target: 'claude',
        path: '/home/u/.claude/settings.json',
        names: ['notify', 'format'],
        importable: true,
      },
    ],
  } as unknown as HookInventory;

  it('saves only the checked candidates, one hook each, taking over their registrations', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.import).mockResolvedValue([
      candidate('notify', './notify.sh'),
      candidate('format', './format.sh'),
    ]);
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    const onImported = vi.fn();
    wrap(
      <HooksImportDialog
        data={data}
        onClose={vi.fn()}
        onChanged={vi.fn()}
        onImported={onImported}
      />,
    );
    const group = await screen.findByRole('region', { name: 'Claude' });
    expect(within(group).getByTitle('/home/u/.claude/settings.json')).toBeInTheDocument();
    // Nothing is imported unless it was picked.
    expect(await within(group).findByRole('checkbox', { name: 'Import format' })).not.toBeChecked();
    expect(screen.getByRole('button', { name: 'Import 0' })).toBeDisabled();
    await user.click(within(group).getByRole('checkbox', { name: 'Import notify' }));
    expect(screen.getByText('1 selected, from 1 target')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Import 1' }));
    await waitFor(() => expect(onImported).toHaveBeenCalledWith(1, 0));
    expect(hooksApi.import).toHaveBeenCalledWith({ from: 'claude' });
    expect(hooksApi.save).toHaveBeenCalledTimes(1);
    expect(hooksApi.save).toHaveBeenCalledWith(
      expect.objectContaining({ entry: candidate('notify', './notify.sh').entry, adopt: true }),
    );
  });

  it("keeps an Antigravity block's own name, since the block is the hook", async () => {
    const user = userEvent.setup();
    const block: HookCandidate = {
      name: 'lint',
      problems: [],
      warnings: [],
      entry: { bindings: { antigravity: { events: { Stop: [{ command: './run-lint.sh' }] } } } },
    };
    vi.mocked(hooksApi.import).mockResolvedValue([block]);
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    const agy = {
      ...data,
      paths: { antigravity: '/home/u/.gemini/config/hooks.json' },
      unmanaged: [
        {
          target: 'antigravity',
          path: '/home/u/.gemini/config/hooks.json',
          names: ['lint'],
          importable: true,
        },
      ],
    } as unknown as HookInventory;
    wrap(
      <HooksImportDialog data={agy} onClose={vi.fn()} onChanged={vi.fn()} onImported={vi.fn()} />,
    );
    await user.click(await screen.findByRole('button', { expanded: false }));
    const name = screen.getByRole('textbox', { name: 'Name for lint' });
    expect(name).toHaveValue('lint');
    expect(name).toBeDisabled();
    await user.click(screen.getByRole('checkbox', { name: 'Import lint' }));
    await user.click(screen.getByRole('button', { name: 'Import 1' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'lint', adopt: true }),
      ),
    );
  });

  it('keeps <target>-<event> names when multiple candidates share a dispatcher binary', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.import).mockResolvedValue([
      candidate('claude-pretooluse', '~/.local/bin/hyp hook claude pre-tool-use'),
      candidate('claude-stop', '~/.local/bin/hyp hook claude stop'),
    ]);
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    wrap(
      <HooksImportDialog
        data={{
          ...data,
          unmanaged: [
            {
              target: 'claude',
              path: '/home/u/.claude/settings.json',
              names: ['claude-pretooluse', 'claude-stop'],
              importable: true,
            },
          ],
        }}
        onClose={vi.fn()}
        onChanged={vi.fn()}
        onImported={vi.fn()}
      />,
    );
    const group = await screen.findByRole('region', { name: 'Claude' });
    await user.click(
      await within(group).findByRole('checkbox', { name: 'Import claude-pretooluse' }),
    );
    await user.click(within(group).getByRole('checkbox', { name: 'Import claude-stop' }));
    await user.click(screen.getByRole('button', { name: 'Import 2' }));
    await waitFor(() => expect(hooksApi.save).toHaveBeenCalledTimes(2));
    expect(hooksApi.save).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ name: 'claude-pretooluse', adopt: true }),
    );
    expect(hooksApi.save).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ name: 'claude-stop', adopt: true }),
    );
  });

  const editable = (name: string, command: string): HookCandidate => ({
    ...candidate(name, command),
    editable: true,
  });
  const nativePlan = {
    revision: 'r1',
    fingerprint: 'r1',
    sourcePath: '/s.yaml',
    blocked: false,
    changes: [],
    files: [],
  };

  it('removes only the file it was asked to, when two Copilot files read as the same name', async () => {
    const user = userEvent.setup();
    const file = (source: string, command: string): HookCandidate => ({
      name: 'lint',
      source,
      editable: true,
      problems: [],
      warnings: [],
      entry: {
        bindings: { copilot: { events: { sessionStart: [{ type: 'command', bash: command }] } } },
      },
    });
    vi.mocked(hooksApi.import).mockResolvedValue([
      file('lint.json', './a.sh'),
      file('skillshare-lint.json', './b.sh'),
    ]);
    vi.mocked(hooksApi.previewNative).mockResolvedValue(nativePlan);
    const copilot = {
      ...data,
      paths: { copilot: '/home/u/.copilot/hooks' },
      unmanaged: [
        {
          target: 'copilot',
          path: '/home/u/.copilot/hooks',
          names: ['lint.json', 'skillshare-lint.json'],
          importable: true,
        },
      ],
    } as unknown as HookInventory;
    wrap(
      <HooksImportDialog
        data={copilot}
        onClose={vi.fn()}
        onChanged={vi.fn()}
        onImported={vi.fn()}
      />,
    );
    await user.click(
      (await screen.findAllByRole('button', { name: 'Remove lint from the target' }))[0],
    );
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await waitFor(() =>
      expect(hooksApi.previewNative).toHaveBeenCalledWith([
        {
          target: 'copilot',
          source: 'lint.json',
          before: file('lint.json', './a.sh').entry.bindings.copilot.events,
        },
      ]),
    );
  });

  it('writes a removal only after the review, and before importing the rest', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.import).mockResolvedValue([
      editable('notify', './notify.sh'),
      editable('format', './format.sh'),
    ]);
    vi.mocked(hooksApi.previewNative).mockResolvedValue(nativePlan);
    vi.mocked(hooksApi.applyNative).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    const onImported = vi.fn();
    wrap(
      <HooksImportDialog
        data={data}
        onClose={vi.fn()}
        onChanged={vi.fn()}
        onImported={onImported}
      />,
    );
    await user.click(await screen.findByRole('checkbox', { name: 'Import format' }));
    await user.click(screen.getByRole('button', { name: 'Remove notify from the target' }));
    expect(screen.getByRole('checkbox', { name: 'Import notify' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: /^Import \d/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const edits = [
      { target: 'claude', before: editable('notify', './notify.sh').entry.bindings.claude.events },
    ];
    await waitFor(() => expect(hooksApi.previewNative).toHaveBeenCalledWith(edits));
    expect(hooksApi.applyNative).not.toHaveBeenCalled();
    expect(hooksApi.save).not.toHaveBeenCalled();
    await user.click(await screen.findByRole('button', { name: 'Apply changes' }));
    await waitFor(() => expect(onImported).toHaveBeenCalledWith(1, 1));
    expect(hooksApi.applyNative).toHaveBeenCalledWith(edits, 'r1');
    expect(hooksApi.save).toHaveBeenCalledTimes(1);
    expect(hooksApi.save).toHaveBeenCalledWith(
      expect.objectContaining({ entry: candidate('format', './format.sh').entry, adopt: true }),
    );
    expect(vi.mocked(hooksApi.applyNative).mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(hooksApi.save).mock.invocationCallOrder[0],
    );
  });

  it('sends an edited hook as it was read and as edited, and leaves an untouched one alone', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    vi.mocked(hooksApi.import).mockResolvedValue([editable('notify', './notify.sh')]);
    vi.mocked(hooksApi.previewNative).mockResolvedValue(nativePlan);
    wrap(
      <HooksImportDialog data={data} onClose={vi.fn()} onChanged={vi.fn()} onImported={vi.fn()} />,
    );
    await user.click(await screen.findByRole('checkbox', { name: 'Import notify' }));
    await user.click(screen.getByRole('button', { name: 'Edit notify' }));
    await user.click(screen.getByRole('button', { name: 'Done' }));
    expect(screen.getByRole('button', { name: 'Import 1' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Edit notify' }));
    await user.type(screen.getByLabelText('Command 1'), ' --fast');
    await user.click(screen.getByRole('button', { name: 'Done' }));
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await waitFor(() =>
      expect(hooksApi.previewNative).toHaveBeenCalledWith([
        {
          target: 'claude',
          before: { Stop: [{ hooks: [{ type: 'command', command: './notify.sh' }] }] },
          after: { Stop: [{ hooks: [{ type: 'command', command: './notify.sh --fast' }] }] },
        },
      ]),
    );
  });

  it('after a write went through and an import failed, sends neither the change nor that hook again', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    vi.mocked(hooksApi.import)
      .mockResolvedValueOnce([editable('notify', './notify.sh'), editable('format', './format.sh')])
      // Read again, the edit may sit under other rows than the one it was made in: an edit can move a
      // command to another event, and candidates are listed per event.
      .mockResolvedValue([
        editable('notify', './notify.sh'),
        editable('moved', './notify.sh --fast'),
        editable('format', './format.sh'),
      ]);
    vi.mocked(hooksApi.previewNative).mockResolvedValue(nativePlan);
    vi.mocked(hooksApi.applyNative).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(hooksApi.save)
      .mockRejectedValueOnce(new Error('name taken'))
      .mockResolvedValue({ applied: [], backupIds: [] });
    const onChanged = vi.fn();
    const onImported = vi.fn();
    wrap(
      <HooksImportDialog
        data={data}
        onClose={vi.fn()}
        onChanged={onChanged}
        onImported={onImported}
      />,
    );
    await user.click(await screen.findByRole('checkbox', { name: 'Import format' }));
    await user.click(screen.getByRole('button', { name: 'Edit notify' }));
    await user.type(screen.getByLabelText('Command 1'), ' --fast');
    await user.click(screen.getByRole('button', { name: 'Done' }));
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await user.click(await screen.findByRole('button', { name: 'Apply changes' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('name taken');
    expect(onChanged).toHaveBeenCalled();
    // The file now holds the edit, so nothing is pending, and neither the edited hook nor a row the edit
    // produced is picked up as an import.
    await screen.findByRole('checkbox', { name: 'Import moved' });
    expect(screen.queryByRole('button', { name: 'Review changes' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Import 1' }));
    await waitFor(() => expect(onImported).toHaveBeenCalledWith(1, 0));
    expect(hooksApi.applyNative).toHaveBeenCalledTimes(1);
    expect(hooksApi.save).toHaveBeenLastCalledWith(
      expect.objectContaining({ entry: candidate('format', './format.sh').entry }),
    );
  });

  it('reads no Agent whose unmanaged hooks can only be listed', async () => {
    vi.mocked(hooksApi.import).mockResolvedValue([]);
    const inline = { target: 'codex', path: '/home/u/.codex/config.toml', names: ['Stop'] };
    const listed = {
      ...data,
      unmanaged: [...data.unmanaged, { ...inline, importable: false }],
    } as unknown as HookInventory;
    wrap(
      <HooksImportDialog
        data={listed}
        onClose={vi.fn()}
        onChanged={vi.fn()}
        onImported={vi.fn()}
      />,
    );
    await waitFor(() => expect(hooksApi.import).toHaveBeenCalledWith({ from: 'claude' }));
    expect(hooksApi.import).toHaveBeenCalledTimes(1);
  });

  it('keeps the changes of a file a failed write did not reach', async () => {
    const user = userEvent.setup();
    const geminiPath = '/home/u/.gemini/settings.json';
    const both = {
      ...data,
      paths: { ...data.paths, gemini: geminiPath },
      unmanaged: [
        ...data.unmanaged,
        { target: 'gemini', path: geminiPath, names: ['cleanup'], importable: true },
      ],
    } as unknown as HookInventory;
    const cleanup: HookCandidate = {
      name: 'cleanup',
      problems: [],
      warnings: [],
      editable: true,
      entry: {
        bindings: {
          gemini: {
            events: { SessionEnd: [{ hooks: [{ type: 'command', command: './cleanup.sh' }] }] },
          },
        },
      },
    };
    let written = false;
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    vi.mocked(hooksApi.import).mockImplementation(async ({ from }) =>
      from === 'gemini' ? [cleanup] : written ? [] : [editable('notify', './notify.sh')],
    );
    vi.mocked(hooksApi.previewNative).mockResolvedValue(nativePlan);
    vi.mocked(hooksApi.applyNative).mockImplementation(async () => {
      written = true;
      throw new ApiError(
        409,
        'permission denied; already applied: /home/u/.claude/settings.json; backup IDs: b1',
        { params: { applied: ['/home/u/.claude/settings.json'] } },
      );
    });
    wrap(
      <HooksImportDialog data={both} onClose={vi.fn()} onChanged={vi.fn()} onImported={vi.fn()} />,
    );
    await user.click(await screen.findByRole('button', { name: 'Remove notify from the target' }));
    await user.click(screen.getByRole('button', { name: /Gemini/ }));
    await user.click(await screen.findByRole('button', { name: 'Remove cleanup from the target' }));
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await user.click(await screen.findByRole('button', { name: 'Apply changes' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('permission denied');
    await user.click(await screen.findByRole('button', { name: 'Review changes' }));
    await waitFor(() =>
      expect(hooksApi.previewNative).toHaveBeenLastCalledWith([
        { target: 'gemini', before: cleanup.entry.bindings.gemini.events },
      ]),
    );
  });
});

describe('hooks remove', () => {
  beforeEach(() => vi.resetAllMocks());

  it('stops managing without syncing, and says the source-only choice still deletes on the next sync', async () => {
    const user = userEvent.setup();
    const path = '/home/u/.claude/settings.json';
    vi.mocked(hooksApi.preview).mockImplementation(async (m) => ({
      revision: m?.unmanage ? 'kept' : 'r',
      fingerprint: 'fp',
      sourcePath: '/s.yaml',
      blocked: false,
      changes: m?.unmanage ? [] : [{ target: 'claude', path, name: 'guard', action: 'remove' }],
    }));
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    const onSaved = vi.fn();
    wrap(<HooksRemoveDialog name="guard" onClose={vi.fn()} onSaved={onSaved} />);
    const sourceOnly = screen.getByRole('button', { name: 'Remove from source only' });
    await waitFor(() =>
      expect(sourceOnly).toHaveAccessibleDescription(
        'Leaves the files for now, but the next sync also deletes it from the Claude settings files.',
      ),
    );
    await user.hover(sourceOnly);
    expect(await screen.findByRole('tooltip')).toHaveTextContent(
      'Leaves the files for now, but the next sync also deletes it from the Claude settings files.',
    );
    await user.click(screen.getByRole('button', { name: 'Stop managing' }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(true));
    expect(hooksApi.configure).toHaveBeenCalledWith(
      { name: 'guard', remove: true, unmanage: true },
      'kept',
      false,
    );
  });
});

describe('account hook bindings', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.catalog).mockResolvedValue({
      codex: {
        timeoutUnit: 'seconds',
        events: [{ name: 'SessionStart', description: 'When a session starts', matcher: false }],
      },
    });
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it('offers the account with its Agent catalog and saves under the account key', async () => {
    const user = userEvent.setup();
    wrap(
      <HookDialog
        accounts={{ 'codex-2': 'codex' }}
        existingNames={[]}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );
    await user.type(screen.getByLabelText('Name'), 'account');
    await user.click(screen.getByRole('checkbox', { name: 'codex-2 (Codex)' }));
    await user.click(screen.getByRole('combobox', { name: 'Event 1' }));
    await user.click(await screen.findByRole('option', { name: /^SessionStart/ }));
    await user.type(screen.getByLabelText('Command 1'), 'echo account');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith({
        name: 'account',
        entry: {
          bindings: {
            'codex-2': {
              events: { SessionStart: [{ hooks: [{ type: 'command', command: 'echo account' }] }] },
            },
          },
        },
      }),
    );
  });
});
