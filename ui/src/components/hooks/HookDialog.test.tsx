import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { hooksApi } from '../../api/hooks';
import type { HookCandidate, HookInventory } from '../../api/hooks';
import HookDialog from './HookDialog';
import HooksImportDialog from './HooksImportDialog';
import HooksPreview from './HooksPreview';
import HooksRemoveDialog from './HooksRemoveDialog';

vi.mock('../CodeEditor', () => ({
  default: ({ value, onChange, ariaLabel }: { value: string; onChange: (v: string) => void; ariaLabel: string }) => <textarea aria-label={ariaLabel} value={value} onChange={(e) => onChange(e.target.value)} />,
}));
vi.mock('../../api/hooks', async (load) => ({
  ...await load<typeof import('../../api/hooks')>(),
  hooksApi: { catalog: vi.fn(), preview: vi.fn(), save: vi.fn(), configure: vi.fn(), import: vi.fn() },
}));

const catalog = { claude: { timeoutUnit: 'seconds' as const, events: [
  { name: 'PostToolUse', description: 'After a tool succeeds', matcher: true },
  { name: 'Stop', description: 'When the main agent finishes', matcher: false },
] } };

const wrap = (ui: React.ReactNode) => render(<MemoryRouter><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><I18nProvider>{ui}</I18nProvider></QueryClientProvider></MemoryRouter>);

const openClaude = async (user: ReturnType<typeof userEvent.setup>) => {
  wrap(<HookDialog existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} />);
  await user.type(screen.getByLabelText('Name'), 'lint');
  await user.click(screen.getByRole('checkbox', { name: /Claude/ }));
};

describe('hook dialog', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
    HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it("offers the target's documented events with what each does, and a tool filter only where the event takes one", async () => {
    const user = userEvent.setup();
    await openClaude(user);
    await user.click(screen.getByRole('combobox', { name: 'Event 1' }));
    const post = await screen.findByRole('option', { name: /^PostToolUse/ });
    expect(post).toHaveTextContent('Filters tools');
    expect(screen.getByRole('option', { name: /^Stop/ })).not.toHaveTextContent('Filters tools');
    await user.click(screen.getByRole('option', { name: /^Stop/ }));
    expect(screen.queryByLabelText('Tool filter 1')).not.toBeInTheDocument();
  });

  it('lints the native JSON by line and names an unknown event with the closest one', async () => {
    const user = userEvent.setup();
    await openClaude(user);
    await user.click(screen.getByRole('tab', { name: 'Native JSON' }));
    const editor = screen.getByLabelText('Claude Native JSON');
    await user.clear(editor);
    await user.click(editor);
    await user.paste('{\n  "Stopp": []\n}');
    expect(screen.getByText(/Line 2: "Stopp" is not an event this target documents\. Did you mean Stop\?/)).toBeInTheDocument();
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
    expect(screen.getByRole('tab', { name: /Native JSON/ })).toContainElement(screen.getByLabelText('1 errors'));
    await user.click(screen.getByRole('tab', { name: 'Fields' }));
    expect(screen.getByRole('alert')).toHaveTextContent('Fix the JSON before switching to fields. Line 4:');
    expect(screen.getByLabelText('Claude Native JSON')).toHaveValue(broken);
  });
});

describe('expanded editor', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.catalog).mockResolvedValue(catalog);
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
  });

  const expand = async (user: ReturnType<typeof userEvent.setup>) => {
    await openClaude(user);
    await user.click(screen.getByRole('tab', { name: 'Native JSON' }));
    const editor = screen.getByLabelText('Claude Native JSON');
    await user.clear(editor);
    await user.click(editor);
    await user.paste('{ "Stop": [] }');
    await user.click(screen.getByRole('button', { name: 'Expand' }));
    return screen.getByRole('group', { name: 'Claude Native JSON' });
  };

  it('opens the same editor over the dialog, and Esc brings it back with the text kept', async () => {
    const user = userEvent.setup();
    const expanded = await expand(user);
    const editor = within(expanded).getByLabelText('Claude Native JSON');
    await user.type(editor, ' ');
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('group', { name: 'Claude Native JSON' })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Claude Native JSON')).toHaveValue('{ "Stop": [] } ');
    expect(screen.getByLabelText('Name')).toBeInTheDocument();
  });

  it('leaves an Esc the editor already used (its completion list) to the editor', async () => {
    const user = userEvent.setup();
    const expanded = await expand(user);
    const esc = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
    esc.preventDefault();
    within(expanded).getByLabelText('Claude Native JSON').dispatchEvent(esc);
    expect(screen.getByRole('group', { name: 'Claude Native JSON' })).toBeInTheDocument();
  });

  it('saves with Cmd+S while expanded', async () => {
    const user = userEvent.setup();
    await expand(user);
    await user.keyboard('{Meta>}s{/Meta}');
    await waitFor(() => expect(hooksApi.save).toHaveBeenCalledTimes(1));
  });
});

describe('copying to a target without close events', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(hooksApi.catalog).mockResolvedValue({
      claude: { timeoutUnit: 'seconds', events: [{ name: 'Notification', description: 'When a notification is sent', matcher: true }] },
      cursor: { timeoutUnit: 'seconds', events: [{ name: 'stop', description: 'When the agent stops', matcher: false }] },
    });
    HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it('says the events must be picked by hand instead of promising the closest ones', async () => {
    const user = userEvent.setup();
    await openClaude(user);
    await user.click(screen.getByRole('combobox', { name: 'Event 1' }));
    await user.click(await screen.findByRole('option', { name: /^Notification/ }));
    await user.type(screen.getByLabelText('Command 1'), './ping.sh');
    await user.click(screen.getByRole('checkbox', { name: /Cursor/ }));
    expect(await screen.findByText('Claude already runs on Notification. Cursor has no close match for them; after copying, pick each event yourself.')).toBeInTheDocument();
    expect(screen.queryByText(/copying picks the closest/)).not.toBeInTheDocument();
  });
});

describe('hooks import', () => {
  beforeEach(() => vi.resetAllMocks());

  const candidate = (name: string, command: string): HookCandidate => ({
    name, problems: [], warnings: [],
    entry: { bindings: { claude: { events: { Stop: [{ hooks: [{ type: 'command', command }] }] } } } },
  });
  const data = {
    source: { path: '/s.yaml', configPath: '/s.yaml', entries: {} }, targets: [], backups: [], plan: null, previewError: '',
    paths: { claude: '/home/u/.claude/settings.json' },
    unmanaged: [{ target: 'claude', path: '/home/u/.claude/settings.json', names: ['notify', 'format'] }],
  } as unknown as HookInventory;

  it('saves only the checked candidates, one hook each, taking over their registrations', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.import).mockResolvedValue([candidate('notify', './notify.sh'), candidate('format', './format.sh')]);
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    const onImported = vi.fn();
    wrap(<HooksImportDialog data={data} onClose={vi.fn()} onImported={onImported} />);
    const group = await screen.findByRole('region', { name: 'Claude' });
    expect(within(group).getByTitle('/home/u/.claude/settings.json')).toBeInTheDocument();
    await user.click(within(group).getByRole('checkbox', { name: 'format' }));
    expect(screen.getByText('1 selected')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Import 1' }));
    await waitFor(() => expect(onImported).toHaveBeenCalledWith(1));
    expect(hooksApi.import).toHaveBeenCalledWith({ from: 'claude' });
    expect(hooksApi.save).toHaveBeenCalledTimes(1);
    expect(hooksApi.save).toHaveBeenCalledWith(expect.objectContaining({ entry: candidate('notify', './notify.sh').entry, adopt: true }));
  });
});

describe('hooks preview', () => {
  it("shows each file's event changes and diff, marking the user's own hooks as untouched", () => {
    const path = '/home/u/.claude/settings.json';
    const before = '{\n  "hooks": {\n    "PreToolUse": [{ "hooks": [] }]\n  }\n}';
    const after = '{\n  "hooks": {\n    "PreToolUse": [{ "hooks": [] }],\n    "Stop": [\n      { "hooks": [] }\n    ]\n  }\n}';
    wrap(<HooksPreview
      plan={{ revision: 'r', fingerprint: 'fp', sourcePath: '/s.yaml', blocked: false,
        changes: [{ target: 'claude', path, name: 'lint', action: 'update', events: { added: ['Stop'] } }],
        files: [{ target: 'claude', path, before, after }] }}
      unmanaged={[{ target: 'claude', path, names: ['PreToolUse'] }]}
    />);
    const card = screen.getByRole('region', { name: path });
    expect(within(card).getByText('+ Stop')).toBeInTheDocument();
    const diff = within(card).getByLabelText(`Changes to ${path}`);
    expect(within(diff).getByText('"Stop": [')).toBeInTheDocument();
    expect(within(diff).getByText("your own hook, left as is")).toBeInTheDocument();
  });
});

describe('hooks remove', () => {
  beforeEach(() => vi.resetAllMocks());

  it('stops managing without syncing, and says the source-only choice still deletes on the next sync', async () => {
    const user = userEvent.setup();
    const path = '/home/u/.claude/settings.json';
    vi.mocked(hooksApi.preview).mockImplementation(async (m) => ({ revision: m?.unmanage ? 'kept' : 'r', fingerprint: 'fp', sourcePath: '/s.yaml', blocked: false, changes: m?.unmanage ? [] : [{ target: 'claude', path, name: 'guard', action: 'remove' }] }));
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    const onSaved = vi.fn();
    wrap(<HooksRemoveDialog name="guard" onClose={vi.fn()} onSaved={onSaved} />);
    expect(await screen.findByText('Leaves the files for now, but the next sync also deletes it from the Claude settings files.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Stop managing' }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(true));
    expect(hooksApi.configure).toHaveBeenCalledWith({ name: 'guard', remove: true, unmanage: true }, 'kept', false);
  });

  it('counts many targets instead of listing them, and names the other hooks a sync also writes', async () => {
    const agents = ['claude', 'codex', 'gemini', 'cursor'];
    vi.mocked(hooksApi.preview).mockResolvedValue({ revision: 'r', fingerprint: 'fp', sourcePath: '/s.yaml', blocked: false, changes: [
      ...agents.map((target) => ({ target, path: `/home/u/${target}.json`, name: 'guard', action: 'remove' })),
      { target: 'claude', path: '/home/u/claude.json', name: 'lint', action: 'add' },
      { target: 'claude', path: '/home/u/claude.json', name: 'fmt', action: 'unchanged' },
    ] });
    wrap(<HooksRemoveDialog name="guard" onClose={vi.fn()} onSaved={vi.fn()} />);
    expect(await screen.findByText('Deletes what it wrote from the 4 target settings files now.')).toBeInTheDocument();
    expect(screen.getByText('Syncing also writes 1 other pending hook: lint.')).toBeInTheDocument();
  });
});
