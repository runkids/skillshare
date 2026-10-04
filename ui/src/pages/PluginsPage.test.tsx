import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PluginsPage from './PluginsPage';
import { pluginsApi, type PluginInventory } from '../api/plugins';
import { ApiError } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { MemoryRouter } from 'react-router-dom';

vi.mock('../api/plugins', async (importOriginal) => ({ ...await importOriginal<typeof import('../api/plugins')>(), pluginsApi: { list: vi.fn(), files: vi.fn(), file: vi.fn(), discover: vi.fn(), preview: vi.fn(), apply: vi.fn() } }));
vi.mock('../i18n', () => ({ useT: () => (key: string) => key }));
vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../components/plugins/PluginAddDialog', () => ({ default: ({ initialTargets }: { initialTargets?: string[] }) => <div role="dialog" aria-label="add">{initialTargets?.join(',')}</div> }));
vi.mock('../hooks/useSharedQueries', () => ({ useSyncedTargetsQuery: () => ({ data: { targets: [{ name: 'pi' }] } }) }));

function mount(path = '/plugins', client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) { return render(<MemoryRouter initialEntries={[path]}><QueryClientProvider client={client}><ToastProvider><PluginsPage /></ToastProvider></QueryClientProvider></MemoryRouter>); }

describe('PluginsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{target:'codex',label:'Codex',project:false,operations:['add','sync','import']}], packages: { demo: { bindings: { codex: { id: 'demo@market' } } } }, hosts: [{ target: 'codex', version: '0.154', status: 'ready', installed: [{ id: 'demo@market', enabled: false }] }] });
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'reviewed', blocked: false, changes: [{ name: 'demo', target: 'codex', id: 'demo@market', action: 'selection' }] });
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [] }, failure: '' });
  });
  it('separates an empty managed list from native registrations and links Pi extensions', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      packages: {},
      targetDefinitions: [{ target: 'pi', label: 'Pi', project: true, operations: ['import'] }],
      hosts: [{ target: 'pi', version: '1.0.0', status: 'ready', installed: ['a', 'b', 'c'].map((id) => ({ id, enabled: true })) }],
    });
    mount();
    const managed = await screen.findByRole('heading', { name: 'plugins.managedTitle' });
    expect(managed.parentElement).toHaveTextContent('0');
    expect(screen.getByRole('heading', { name: 'plugins.hostsTitle' })).toBeInTheDocument();
    expect(screen.getByText('plugins.hostsHelp')).toBeInTheDocument();
    expect(screen.getByText('plugins.emptyHelp')).toBeInTheDocument();
    expect(screen.getByText('plugins.hostRegistered.other')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Pi' }));
    expect(screen.getByRole('link', { name: 'plugins.piExtensions' })).toHaveAttribute('href', '/targets/pi?tab=extensions');
    expect(pluginsApi.preview).not.toHaveBeenCalled();
    expect(pluginsApi.apply).not.toHaveBeenCalled();
  });
  it('opens the add dialog for the target a target page linked from', async () => {
    mount('/plugins?add=pi-work');
    expect(await screen.findByRole('dialog', { name: 'add' })).toHaveTextContent('pi-work');
  });
  it('adds an imported registration to the managed count only after the reviewed import', async () => {
    let imported = false;
    vi.mocked(pluginsApi.list).mockImplementation(async (): Promise<PluginInventory> => ({
      packages: imported ? { demo: { bindings: { pi: { id: 'npm:demo' } } } } : {},
      targetDefinitions: [{ target: 'pi', label: 'Pi', project: true, operations: ['import'] }],
      hosts: [{ target: 'pi', version: '1.0.0', status: 'ready', installed: [{ id: 'npm:demo', enabled: true }] }],
    }));
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'import-reviewed', blocked: false, changes: [{ name: 'demo', target: 'pi', id: 'npm:demo', action: 'import' }] });
    vi.mocked(pluginsApi.apply).mockImplementation(async () => {
      imported = true;
      return { result: { results: [] }, failure: '' };
    });
    mount();
    const managed = await screen.findByRole('heading', { name: 'plugins.managedTitle' });
    expect(managed.parentElement).toHaveTextContent('0');
    fireEvent.click(screen.getAllByRole('button', { name: 'plugins.import' })[0]);
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.importOne' }));
    await screen.findByRole('dialog', { name: 'plugins.preview' });
    expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'import', from: 'pi', plugin: 'npm:demo' });
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    expect(managed.parentElement).toHaveTextContent('0');
    fireEvent.click(screen.getByRole('button', { name: 'plugins.apply' }));
    await waitFor(() => expect(managed.parentElement).toHaveTextContent('1'));
    expect(screen.queryByText('plugins.empty')).not.toBeInTheDocument();
    expect(screen.getByText('plugins.hostRegistered.one')).toBeInTheDocument();
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'import', from: 'pi', plugin: 'npm:demo' }, 'import-reviewed');
  });
  it.each([true, false])('only permits reviewed filtered imports when native preservation is supported: %s', async (importable) => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ packages: {}, targetDefinitions: [{ target: 'pi', label: 'Pi', project: true, operations: ['import'] }], hosts: [{ target: 'pi', version: '1.0.0', status: 'ready', installed: [{ id: 'npm:demo', enabled: true, filtered: true, importable }] }] });
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'filters-reviewed', blocked: false, changes: [{ name: 'demo', target: 'pi', id: 'npm:demo', action: 'import', preservedKeys: ['extensions', 'opaque', 'skills', 'source'] }] });
    mount();
    fireEvent.click((await screen.findAllByRole('button', { name: 'plugins.import' }))[0]);
    const button = await screen.findByRole('button', { name: 'plugins.importOne' });
    if (!importable) { expect(button).toBeDisabled(); return; }
    expect(button).toBeEnabled();
    fireEvent.click(button);
    await screen.findByRole('dialog', { name: 'plugins.preview' });
    expect(screen.getByText('plugins.preservedKeys')).toBeInTheDocument();
    expect(screen.getByText('extensions · opaque · skills · source')).toBeInTheDocument();
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.apply' }));
    await waitFor(() => expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'import', from: 'pi', plugin: 'npm:demo' }, 'filters-reviewed'));
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
  it('shows the version recorded at add time while no Agent is bound', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [], packages: { demo: { source: 'https://github.com/owner/demo', version: '2.3.4', bindings: {} } }, hosts: [] });
    mount();
    expect(await screen.findByText('2.3.4')).toBeInTheDocument();
  });
  it('asks the source for the version of a plugin added before it was recorded', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [], packages: { demo: { source: 'https://github.com/owner/demo', plugin: 'demo', bindings: {} } }, hosts: [] });
    vi.mocked(pluginsApi.discover).mockResolvedValue({ source: 'https://github.com/owner/demo', digest: 'd', candidates: [{ name: 'demo', description: '', version: '5.6.7', targets: [], components: [] }] });
    mount();
    expect(await screen.findByText('5.6.7')).toBeInTheDocument();
  });
  it('draws the logo a Codex manifest names', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [], packages: { demo: { source: 'https://github.com/owner/demo', plugin: 'demo', bindings: {} } }, hosts: [] });
    vi.mocked(pluginsApi.discover).mockResolvedValue({ source: 'https://github.com/owner/demo', digest: 'd', candidates: [{ name: 'demo', description: '', version: '1', targets: ['codex'], components: [], targetInfo: { codex: { manifest: '.codex-plugin/plugin.json', logo: 'data:image/png;base64,AA==', components: [] } } }] });
    const { container } = mount();
    await screen.findByText('demo');
    await waitFor(() => expect(container.querySelector('img')?.getAttribute('src')).toBe('data:image/png;base64,AA=='));
  });
  it('shows the installed version, and old → new once a check finds another', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{ target: 'codex', label: 'Codex', project: false, operations: ['add', 'check'] }], packages: { demo: { bindings: { codex: { id: 'demo@market', version: '1.0.0' } } } }, hosts: [] });
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'r', blocked: false, changes: [{ name: 'demo', target: 'codex', id: 'demo@market', action: 'update-available', binding: { id: 'demo@market', version: '1.1.0' } }] });
    mount();
    expect(await screen.findByText('1.0.0')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.check' }));
    expect(await screen.findAllByText('1.0.0 → 1.1.0')).toHaveLength(2);
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
  it('localizes the last action status and installation instructions', async () => {
    const message = 'Native installation recorded. Reload the Agent and complete any required login or hook trust.';
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'demo', target: 'codex', status: 'installed', message }] }, failure: '' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.syncAgain' }));
    await screen.findByRole('dialog');
    fireEvent.click(screen.getByRole('button', { name: 'plugins.apply' }));
    expect(await screen.findByText('plugins.outcome.installed')).toBeInTheDocument();
    expect(screen.getByText('plugins.outcome.installHelp')).toBeInTheDocument();
    expect(screen.queryByText(message)).not.toBeInTheDocument();
  });
  it('refreshes the Pi Extensions tabs after a sync, since they list the packages it changes', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(client, 'invalidateQueries');
    mount('/plugins', client);
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.syncAgain' }));
    await screen.findByRole('dialog');
    fireEvent.click(screen.getByRole('button', { name: 'plugins.apply' }));
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ['pi-extensions'] }));
  });
  it('requires a preview before sync and preserves partial failures', async () => {
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'demo', target: 'codex', status: 'failed', message: 'Native authentication required' }] }, failure: 'One target failed' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.syncAgain' }));
    await screen.findByRole('dialog');
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.apply' }));
    expect(await screen.findAllByText('Native authentication required')).not.toHaveLength(0);
    expect(screen.getByRole('alert')).toHaveTextContent('Native authentication required');
  });
  it('translates a keyed sync failure once in the alert', async () => {
    const failed = { status: 'failed', message: 'claude command failed', messageKey: 'plugins.error.commandFailed' };
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'a', target: 'claude', ...failed }, { name: 'b', target: 'claude', ...failed }] }, failure: 'claude command failed\nclaude command failed' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.syncAgain' }));
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.apply' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/^plugins\.error\.commandFailed$/);
  });
  it('keeps every distinct failure in the alert, keyed or not', async () => {
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [{ name: 'a', target: 'claude', status: 'failed', message: 'claude command failed', messageKey: 'plugins.error.commandFailed' }, { name: 'b', target: 'codex', status: 'failed', message: 'Native authentication required' }] }, failure: 'joined' });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.syncAgain' }));
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.apply' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('plugins.error.commandFailed Native authentication required');
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
  it('lists packages bound only to Pi targets in their own section', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      targetDefinitions: [
        { target: 'omo', label: 'omo', project: false, operations: ['add', 'sync'], npm: true },
        { target: 'codex', label: 'Codex', project: false, operations: ['add', 'sync'] },
      ],
      packages: { demo: { bindings: { codex: { id: 'demo@market' } } }, driver: { bindings: { omo: { id: 'npm:@scope/driver' } } } },
      hosts: [],
    });
    mount();
    const pi = (await screen.findByRole('heading', { name: 'plugins.piTitle' })).closest('section')!;
    const managed = screen.getByRole('heading', { name: 'plugins.managedTitle' }).closest('section')!;
    expect(pi).toHaveTextContent('driver');
    expect(managed).not.toHaveTextContent('driver');
    expect(managed).toHaveTextContent('demo');
  });
  it('offers an imported npm package to the other Pi targets, installing it from its identifier', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      targetDefinitions: [
        { target: 'omo', label: 'omo', project: false, operations: ['add', 'sync'], npm: true },
        { target: 'pi', label: 'Pi', project: true, operations: ['add', 'sync'], npm: true },
        { target: 'codex', label: 'Codex', project: false, operations: ['add', 'sync'] },
      ],
      packages: { driver: { bindings: { omo: { id: 'npm:@scope/driver' } } } },
      hosts: [],
    });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.chooseAgents' }));
    const pi = await screen.findByRole('checkbox', { name: 'Pi' });
    expect(screen.queryByRole('checkbox', { name: 'Codex' })).toBeNull();
    expect(pluginsApi.discover).not.toHaveBeenCalled();
    fireEvent.click(pi);
    await waitFor(() => expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'add', source: 'npm:@scope/driver', name: 'driver', targets: ['pi'] }));
  });
  it('asks for a managed plugin by name, and says why its source could not be read', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{ target: 'codex', label: 'codex', project: false, operations: ['add', 'sync'] }], packages: { demo: { bindings: { codex: { id: 'demo@market', source: 'https://github.com/owner/demo.git', plugin: 'demo' } } } }, hosts: [] });
    vi.mocked(pluginsApi.discover).mockRejectedValue(new ApiError(400, 'download plugin source: could not reach the plugin source', { code: 'plugins.error.network' }));
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.chooseAgents' }));
    expect(await screen.findByText('plugins.error.network')).toBeInTheDocument();
    expect(pluginsApi.discover).toHaveBeenCalledWith('https://github.com/owner/demo.git', undefined, undefined, 'demo');
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
  it('shares several plugins as one command that adds them in order', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [], packages: {
      a: { source: 'https://github.com/owner/a.git', plugin: 'a', bindings: {} },
      b: { source: 'https://github.com/owner/b.git', plugin: 'b', bindings: {} },
      local: { source: '/home/me/local', bindings: {} },
    }, hosts: [] });
    vi.mocked(pluginsApi.discover).mockReturnValue(new Promise(() => {}));
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.share' }));
    // A local directory only exists here, so it is not offered.
    expect(screen.queryByRole('checkbox', { name: 'local' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.copyShare' }));
    expect(writeText).toHaveBeenLastCalledWith('skillshare plugin add https://github.com/owner/a.git --plugin a -g --no-tui && skillshare plugin add https://github.com/owner/b.git --plugin b -g --no-tui');
    // Asking drops --no-tui, so each add opens its Agent picker.
    fireEvent.click(screen.getByRole('checkbox', { name: 'plugins.shareAsk' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.copyShare' }));
    expect(writeText).toHaveBeenLastCalledWith('skillshare plugin add https://github.com/owner/a.git --plugin a -g && skillshare plugin add https://github.com/owner/b.git --plugin b -g');
  });
  it('opens sharing from a row with only that plugin ticked', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [], packages: {
      a: { source: 'https://github.com/owner/a.git', plugin: 'a', bindings: {} },
      b: { source: 'https://github.com/owner/b.git', plugin: 'b', bindings: {} },
    }, hosts: [] });
    vi.mocked(pluginsApi.discover).mockReturnValue(new Promise(() => {}));
    mount();
    fireEvent.click((await screen.findAllByRole('button', { name: 'mcp.moreActions' }))[1]);
    fireEvent.mouseDown(screen.getByRole('menuitem', { name: 'plugins.share' }));
    expect(screen.getByRole('checkbox', { name: 'a' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'b' })).toBeChecked();
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
  it('says where Skillshare looked for a missing Codex CLI', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      packages: {},
      targetDefinitions: [{ target: 'codex', label: 'Codex', project: false, operations: ['add'] }],
      hosts: [{ target: 'codex', version: '', status: 'missing', installed: [], error: 'Codex CLI not found', errorKey: 'plugins.error.codexMissing', errorArgs: { looked: 'PATH, /Applications/Codex.app/Contents/Resources/codex' } }],
    });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: /Codex/ }));
    expect(screen.getByText('plugins.hostMissing').parentElement!.parentElement!).toHaveTextContent('plugins.error.codexMissing');
  });

});
