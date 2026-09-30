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
    const before = '{\n  "hooks": {\n    "PreToolUse": [\n      { "hooks": [] }\n    ]\n  }\n}';
    const after = '{\n  "hooks": {\n    "PreToolUse": [\n      { "hooks": [] }\n    ],\n    "Stop": [\n      { "hooks": [] }\n    ]\n  }\n}';
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
    vi.mocked(hooksApi.preview).mockResolvedValue({ revision: 'r', fingerprint: 'fp', sourcePath: '/s.yaml', blocked: false, changes: [{ target: 'claude', path, name: 'guard', action: 'remove' }] });
    vi.mocked(hooksApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
    const onSaved = vi.fn();
    wrap(<HooksRemoveDialog name="guard" onClose={vi.fn()} onSaved={onSaved} />);
    expect(await screen.findByText('Leaves the files for now, but the next sync also deletes it from the Claude settings files.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Stop managing' }));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(hooksApi.configure).toHaveBeenCalledWith({ name: 'guard', remove: true, unmanage: true }, 'r', false);
  });
});
