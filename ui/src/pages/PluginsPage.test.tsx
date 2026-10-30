import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PluginsPage from './PluginsPage';
import { pluginsApi, type PluginInventory } from '../api/plugins';
import { ToastProvider } from '../components/Toast';
import { MemoryRouter } from 'react-router-dom';

vi.mock('../api/plugins', async (importOriginal) => ({ ...await importOriginal<typeof import('../api/plugins')>(), pluginsApi: { list: vi.fn(), files: vi.fn(), file: vi.fn(), discover: vi.fn(), preview: vi.fn(), apply: vi.fn(), update: vi.fn(), stopUpdate: vi.fn() } }));
vi.mock('../i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('../i18n')>(), useT: () => (key: string) => key }));
vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../components/plugins/PluginAddDialog', () => ({ default: ({ initialTargets }: { initialTargets?: string[] }) => <div role="dialog" aria-label="add">{initialTargets?.join(',')}</div> }));
const synced = vi.hoisted(() => ({ targets: [{ name: 'pi' }] as { name: string; agent?: string }[] }));
vi.mock('../hooks/useSharedQueries', () => ({ useSyncedTargetsQuery: () => ({ data: synced }) }));

function mount(path = '/plugins', client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) { return render(<MemoryRouter initialEntries={[path]}><QueryClientProvider client={client}><ToastProvider><PluginsPage /></ToastProvider></QueryClientProvider></MemoryRouter>); }

describe('PluginsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // jsdom has no scrolling API; Select scrolls its focused option in browsers.
    HTMLElement.prototype.scrollIntoView = vi.fn();
    synced.targets = [{ name: 'pi' }];
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{target:'codex',label:'Codex',project:false,operations:['add','sync','import']}], packages: { demo: { bindings: { codex: { id: 'demo@market' } } } }, hosts: [{ target: 'codex', version: '0.154', status: 'ready', installed: [{ id: 'demo@market', enabled: false }] }] });
    vi.mocked(pluginsApi.discover).mockResolvedValue({ source: 'owner/repo', digest: 'fixture', candidates: [] });
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'reviewed', blocked: false, changes: [{ name: 'demo', target: 'codex', id: 'demo@market', action: 'selection' }] });
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [] }, failure: '' });
    vi.mocked(pluginsApi.update).mockImplementation(async (_items, on) => { on({ event: 'done', failure: '' }); });
  });
  it('shows full-inventory ref work in the row and Sync rail, then refreshes after Sync', async () => {
    let resolveFull!: (value: PluginInventory) => void;
    let synced = false;
    const initial: PluginInventory = { packages: { demo: { bindings: { pi: { id: '/local/old', source: 'owner/repo' } } } }, hosts: [], targetDefinitions: [{ target: 'pi', label: 'Pi', project: true, operations: ['sync'] }] };
    const inventory = (): PluginInventory => ({ ...initial, packages: { demo: { bindings: { pi: { ...initial.packages.demo.bindings.pi!, refChanged: !synced } } } }, hosts: [{ target: 'pi', version: '1', status: 'ready', installed: [{ id: '/local/old', enabled: true }] }] });
    const full = new Promise<PluginInventory>((resolve) => { resolveFull = resolve; });
    vi.mocked(pluginsApi.list).mockImplementation((agents) => agents === false ? Promise.resolve(initial) : synced ? Promise.resolve(inventory()) : full);
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'ref-review', blocked: false, changes: [{ name: 'demo', target: 'pi', id: '/local/old', action: 'update' }] });
    vi.mocked(pluginsApi.apply).mockImplementation(async () => { synced = true; return { result: { results: [] }, failure: '' }; });
    mount();
    await screen.findByText('demo');
    expect(screen.queryByText('plugins.pending')).not.toBeInTheDocument();
    resolveFull(inventory());
    expect(await screen.findByText('plugins.pending')).toBeInTheDocument();
    expect(screen.getByText('update')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.sync' }));
    const dialog = await screen.findByRole('dialog', { name: 'plugins.preview' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'plugins.apply' }));
    await waitFor(() => expect(screen.queryByText('plugins.pending')).not.toBeInTheDocument());
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'sync' }, 'ref-review');
    expect(await screen.findByText('targets.state.synced')).toBeInTheDocument();
  });
  it('imports into the explicitly selected declaration that needs a local binding', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      packages: { bar: { bindings: { pi: { id: '', imported: true } } } },
      targetDefinitions: [{ target: 'pi', label: 'Pi', project: true, operations: ['import'] }],
      hosts: [{ target: 'pi', version: '1', status: 'ready', installed: [{ id: 'npm:foo', enabled: true }] }],
    });
    mount();
    fireEvent.click((await screen.findAllByRole('button', { name: 'plugins.import' }))[0]);
    const selection = await screen.findByRole('combobox', { name: 'resources.col.name' });
    expect(selection).toHaveTextContent('npm:foo');
    fireEvent.click(selection);
    fireEvent.mouseDown(await screen.findByRole('option', { name: 'bar' }));
    expect(selection).toHaveTextContent('bar');
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.importOne' }));
    await waitFor(() => expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'import', from: 'pi', plugin: 'npm:foo', name: 'bar' }));
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
  it('imports every ticked plugin, each from its own preview, and keeps the import dialog open', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ packages: {}, targetDefinitions: [{ target: 'pi', label: 'Pi', project: true, operations: ['import'] }], hosts: [{ target: 'pi', version: '1.0.0', status: 'ready', installed: [{ id: 'npm:a', enabled: true }, { id: 'npm:b', enabled: true }, { id: 'npm:c', enabled: true }, { id: 'npm:f', enabled: true, filtered: true, importable: true }] }] });
    vi.mocked(pluginsApi.preview).mockImplementation(async (r) => ({ revision: `rev-${r.plugin}`, blocked: false, changes: [] }));
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [] }, failure: '' });
    mount();
    fireEvent.click((await screen.findAllByRole('button', { name: 'plugins.import' }))[0]);
    fireEvent.click(await screen.findByRole('checkbox', { name: 'Pi' }));
    expect(screen.getByRole('checkbox', { name: 'npm:f (Pi)' })).toBeDisabled();
    fireEvent.click(screen.getByRole('checkbox', { name: 'npm:b (Pi)' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.importSelected' }));
    await waitFor(() => expect(pluginsApi.apply).toHaveBeenCalledTimes(2));
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'import', from: 'pi', plugin: 'npm:a' }, 'rev-npm:a');
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'import', from: 'pi', plugin: 'npm:c' }, 'rev-npm:c');
    expect(screen.getByRole('dialog', { name: 'plugins.import' })).toBeInTheDocument();
  });
  it('goes on past a blocked import and keeps it ticked to retry', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ packages: {}, targetDefinitions: [{ target: 'pi', label: 'Pi', project: true, operations: ['import'] }], hosts: [{ target: 'pi', version: '1.0.0', status: 'ready', installed: [{ id: 'npm:a', enabled: true }, { id: 'npm:b', enabled: true }] }] });
    vi.mocked(pluginsApi.preview).mockImplementation(async (r) => ({ revision: 'rev', blocked: r.plugin === 'npm:a', changes: [] }));
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [] }, failure: '' });
    mount();
    fireEvent.click((await screen.findAllByRole('button', { name: 'plugins.import' }))[0]);
    fireEvent.click(await screen.findByRole('checkbox', { name: 'Pi' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.importSelected' }));
    expect(await within(screen.getByRole('dialog', { name: 'plugins.import' })).findByRole('alert')).toHaveTextContent('npm:a: plugins.blocked');
    expect(pluginsApi.apply).toHaveBeenCalledExactlyOnceWith({ action: 'import', from: 'pi', plugin: 'npm:b' }, 'rev');
    expect(screen.getByRole('checkbox', { name: 'npm:a (Pi)' })).toBeChecked();
  });
  const twoBehind = () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      targetDefinitions: [{ target: 'pi', label: 'Pi', project: false, operations: ['add', 'check', 'update'], npm: true }, { target: 'codex', label: 'Codex', project: false, operations: ['add', 'check', 'update'] }],
      packages: { driver: { bindings: { pi: { id: 'npm:driver' } } }, tail: { bindings: { pi: { id: 'npm:tail' }, codex: { id: 'tail@market' } } } },
      hosts: [{ target: 'pi', version: '0.99.2', status: 'ready', installed: [{ id: 'npm:driver', version: '1.0.0', enabled: true }, { id: 'npm:tail', version: '2.0.0', enabled: true }] }],
    });
    // The check the page runs on its own.
    vi.mocked(pluginsApi.preview).mockResolvedValueOnce({ revision: 'r', blocked: false, changes: [
      { name: 'driver', target: 'pi', id: 'npm:driver', action: 'update-available', binding: { id: 'npm:driver', version: '1.1.0' } },
      { name: 'tail', target: 'pi', id: 'npm:tail', action: 'update-available', binding: { id: 'npm:tail', version: '2.1.0' } },
      { name: 'tail', target: 'codex', id: 'tail@market', action: 'update-available', binding: { id: 'tail@market', version: '2.1.0' } },
    ] });
  };
  it('checks for updates on its own and sends every plugin ticked in one batch', async () => {
    twoBehind();
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.updateCount.other' }));
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'plugins.updateDialog.updateSelected.other' }));
    await waitFor(() => expect(pluginsApi.update).toHaveBeenCalledTimes(1));
    expect(vi.mocked(pluginsApi.update).mock.calls[0][0]).toEqual([
      { name: 'driver', targets: ['pi'], seen: { pi: { id: 'npm:driver', version: '1.1.0' } } },
      { name: 'tail', targets: ['pi', 'codex'], seen: { pi: { id: 'npm:tail', version: '2.1.0' }, codex: { id: 'tail@market', version: '2.1.0' } } },
    ]);
  });
  it('leaves out a plugin unticked and an Agent unticked under its plugin', async () => {
    twoBehind();
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.updateCount.other' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('checkbox', { name: 'driver' }));
    fireEvent.click(within(dialog).getByRole('checkbox', { name: 'Codex' }));
    fireEvent.click(within(dialog).getByRole('button', { name: 'plugins.updateDialog.updateSelected.one' }));
    await waitFor(() => expect(pluginsApi.update).toHaveBeenCalledTimes(1));
    expect(vi.mocked(pluginsApi.update).mock.calls[0][0].map(({ name, targets }) => ({ name, targets }))).toEqual([{ name: 'tail', targets: ['pi'] }]);
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
    expect(await screen.findAllByText('Native authentication required')).not.toHaveLength(0);
    expect(screen.getByRole('alert')).toHaveTextContent('Native authentication required');
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
  it('updates only the Agents that can be updated from here', async () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{ target: 'codex', label: 'Codex', project: false, operations: ['add', 'sync'] }, { target: 'claude', label: 'Claude', project: true, operations: ['add', 'sync', 'update'] }], packages: { demo: { bindings: { codex: { id: 'demo@market' }, claude: { id: 'demo@market' } } } }, hosts: [] });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.moreActions' }));
    fireEvent.mouseDown(screen.getByRole('menuitem', { name: 'plugins.updateLatest' }));
    await waitFor(() => expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'update', name: 'demo', targets: ['claude'] }));
  });
  const omp = (bindings: PluginInventory['packages'][string]['bindings'], installed: { id: string; enabled: boolean }[] = []): PluginInventory => ({
    targetDefinitions: [{ target: 'omp', label: 'Oh My Pi', project: true, operations: ['add', 'import', 'sync', 'check', 'update', 'remove', 'enable', 'disable'] }],
    packages: { guard: { source: 'owner/guard', bindings } },
    hosts: [{ target: 'omp', version: '18.6.1', status: 'ready', installed, noteKey: 'plugins.note.omp' }],
  });
  it('still blocks an unsafe OMP plan but allows a reviewed fresh install without a special confirmation', async () => {
    synced.targets = [{ name: 'omp' }, { name: 'omp-work', agent: 'omp' }];
    vi.mocked(pluginsApi.list).mockResolvedValue(omp({ omp: { id: 'guard', source: 'owner/guard', sync: false } }, [{ id: 'guard', enabled: true }]));
    // The page checks for updates on its own first.
    vi.mocked(pluginsApi.preview).mockResolvedValueOnce({ revision: 'c', blocked: false, changes: [] }).mockResolvedValueOnce({ revision: 'r1', blocked: true, changes: [{ name: 'guard', target: 'omp', id: 'guard', action: 'blocked', message: 'OMP scoped removal cannot verify this installation or runtime ownership.', messageKey: 'plugins.error.ompRemovalOwnership' }] });
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.sync' }));
    const dialog = await screen.findByRole('dialog', { name: 'plugins.preview' });
    expect(within(dialog).queryByText('plugins.ompRisk.text')).not.toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'plugins.apply' })).toBeDisabled();
    fireEvent.click(within(dialog).getByRole('button', { name: 'common.cancel' }));
    vi.mocked(pluginsApi.preview).mockResolvedValueOnce({ revision: 'r2', blocked: false, changes: [{ name: 'guard', target: 'omp', id: 'guard', action: 'install' }] });
    fireEvent.click(screen.getByRole('button', { name: 'plugins.sync' }));
    const again = await screen.findByRole('dialog', { name: 'plugins.preview' });
    await waitFor(() => expect(within(again).getByRole('button', { name: 'plugins.apply' })).toBeEnabled());
    expect(within(again).queryByText('plugins.ompRisk.text')).not.toBeInTheDocument();
    expect(within(again).queryByRole('checkbox')).not.toBeInTheDocument();
  });
});
