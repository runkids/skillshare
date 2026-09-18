import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PluginsPage from './PluginsPage';
import { pluginsApi } from '../api/plugins';

vi.mock('../api/plugins', async (importOriginal) => ({ ...await importOriginal<typeof import('../api/plugins')>(), pluginsApi: { list: vi.fn(), files: vi.fn(), file: vi.fn(), discover: vi.fn(), preview: vi.fn(), apply: vi.fn() } }));
vi.mock('../i18n', () => ({ useT: () => (key: string) => key }));
vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../components/plugins/PluginAddDialog', () => ({ default: () => null }));

function mount() { return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><PluginsPage /></QueryClientProvider>); }

describe('PluginsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{target:'codex',label:'Codex',project:false,operations:['add','sync','import']}], packages: { demo: { bindings: { codex: { id: 'demo@market' } } } }, hosts: [{ target: 'codex', version: '0.154', status: 'ready', installed: [{ id: 'demo@market', enabled: false }] }] });
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'reviewed', blocked: false, changes: [{ name: 'demo', target: 'codex', id: 'demo@market', action: 'selection' }] });
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [] }, failure: '' });
  });
  it('draws the plugin list before the Agents have answered', async () => {
    vi.mocked(pluginsApi.list).mockImplementation((hosts = true) => (hosts ? new Promise(() => {}) : Promise.resolve({ targetDefinitions: [{ target: 'codex', label: 'Codex', project: false, operations: ['add'] }], packages: { demo: { bindings: { codex: { id: 'demo@market' } } } }, hosts: [] })));
    mount();
    expect(await screen.findByText('demo')).toBeInTheDocument();
    expect(screen.getByText('plugins.checking')).toBeInTheDocument();
    expect(screen.getByText('plugins.hostsAsking')).toBeInTheDocument();
  });
  it('lists what Sync would do when a tick differs from what the Agent has', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{ target: 'codex', label: 'Codex', project: false, operations: ['add', 'sync'] }], packages: { demo: { bindings: { codex: { id: 'demo@market', sync: false } } } }, hosts: [{ target: 'codex', version: '0.154', status: 'ready', installed: [{ id: 'demo@market', enabled: true }] }] });
    mount();
    expect(await screen.findByText('plugins.pendingCount.one')).toBeInTheDocument();
    expect(screen.getByText('uninstall')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'plugins.sync' })).toBeEnabled();
  });
  it('saves sync selection independently of native enabled state', async () => {
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.chooseAgents' }));
    const checkbox = screen.getByRole('checkbox', { name: 'Codex' });
    expect(checkbox).toBeChecked();
    expect(screen.getByText('plugins.nativeDisabled')).toBeInTheDocument();
    fireEvent.click(checkbox);
    await waitFor(() => expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'disable', name: 'demo', targets: ['codex'] }, 'reviewed'));
  });
  it('requires a preview before sync and preserves partial failures', async () => {
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'Native authentication required' }] }, failure: 'One target failed' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.syncAgain' }));
    await screen.findByRole('dialog');
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.apply' }));
    expect(await screen.findByText('Native authentication required')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('One target failed');
  });
  it('lists the Agents the source can also go to as unticked, and a tick there previews an install', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: ['codex', 'claude', 'cursor'].map((target) => ({ target, label: target, project: false, operations: ['add', 'sync'] })), packages: { demo: { bindings: { codex: { id: 'demo@market', source: 'owner/demo', plugin: 'demo' } } } }, hosts: [] });
    vi.mocked(pluginsApi.discover).mockResolvedValue({ source: 'https://github.com/owner/demo', digest: 'd', candidates: [{ name: 'demo', description: '', version: '1', targets: ['codex', 'claude'], components: [] }] });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.chooseAgents' }));
    const claude = await screen.findByRole('checkbox', { name: 'claude' });
    expect(claude).not.toBeChecked();
    // Cursor has no manifest in this source: counted, not offered.
    expect(screen.queryByRole('checkbox', { name: 'cursor' })).toBeNull();
    expect(screen.getByRole('button', { name: 'plugins.moreBlocked' })).toBeEnabled();
    fireEvent.click(claude);
    await waitFor(() => expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'add', source: 'https://github.com/owner/demo', sourceRef: undefined, entry: undefined, plugin: 'demo', name: 'demo', targets: ['claude'] }));
  });
  it('opens the files of a plugin that has a local copy, on its README', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{ target: 'codex', label: 'Codex', project: false, operations: ['add', 'sync'] }], packages: { demo: { bindings: { codex: { id: 'demo@market', source: 'owner/demo' } } } }, hosts: [] });
    vi.mocked(pluginsApi.files).mockResolvedValue({ files: ['README.md', 'skills/hi/SKILL.md'] });
    vi.mocked(pluginsApi.file).mockResolvedValue({ content: 'const greeting = 1;' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.moreActions' }));
    fireEvent.mouseDown(screen.getByRole('menuitem', { name: 'plugins.viewFiles' }));
    expect(await screen.findByRole('button', { name: 'SKILL.md' })).toBeInTheDocument();
    await waitFor(() => expect(pluginsApi.file).toHaveBeenCalledWith('demo', 'README.md'));
  });
  it('updates only the Agents that can be updated from here', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{ target: 'codex', label: 'Codex', project: false, operations: ['add', 'sync'] }, { target: 'claude', label: 'Claude', project: true, operations: ['add', 'sync', 'update'] }], packages: { demo: { bindings: { codex: { id: 'demo@market' }, claude: { id: 'demo@market' } } } }, hosts: [] });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.moreActions' }));
    fireEvent.mouseDown(screen.getByRole('menuitem', { name: 'plugins.updateLatest' }));
    await waitFor(() => expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'update', name: 'demo', targets: ['claude'] }));
  });
  it('separates unsupported integrations from errors and links official docs', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      packages: {},
      targetDefinitions: [
        { target: 'kimi', label: 'Kimi Code', project: false, operations: [] },
        { target: 'codex', label: 'Codex', project: false, operations: ['add'] },
      ],
      hosts: [
        { target: 'kimi', version: '', status: 'blocked', installed: [], error: 'not automated', errorKey: 'plugins.problem.kimi' },
        { target: 'codex', version: '', status: 'blocked', installed: [], error: 'check failed', errorKey: 'plugins.error.commandFailed' },
      ],
    });
    mount();
    expect(await screen.findByText('plugins.hostManual')).toBeInTheDocument();
    expect(screen.getByText('plugins.hostManualHelp')).toBeInTheDocument();
    const manual = screen.getByText('plugins.hostManual').parentElement!.parentElement!;
    expect(manual).toHaveTextContent('Kimi Code');
    expect(manual).not.toHaveTextContent('Codex');
    // A row is a name until it is asked about; its reason and docs are one click in.
    for (const name of ['Kimi Code', 'Codex']) fireEvent.click(screen.getByRole('button', { name: new RegExp(name) }));
    expect(screen.getByRole('link', { name: 'Kimi Code · plugins.officialDocs' })).toHaveAttribute('href', 'https://www.kimi.com/code/docs/en/kimi-code-cli/customization/plugins');
    expect(screen.getByRole('link', { name: 'Codex · plugins.officialDocs' })).toHaveAttribute('rel', 'noopener noreferrer');
  });

});
