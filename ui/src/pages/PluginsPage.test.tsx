import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PluginsPage from './PluginsPage';
import { pluginsApi, type PluginInventory } from '../api/plugins';
import { ToastProvider } from '../components/Toast';
import { MemoryRouter } from 'react-router-dom';

vi.mock('../api/plugins', async (importOriginal) => ({ ...await importOriginal<typeof import('../api/plugins')>(), pluginsApi: { list: vi.fn(), files: vi.fn(), file: vi.fn(), discover: vi.fn(), preview: vi.fn(), apply: vi.fn() } }));
vi.mock('../i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('../i18n')>(), useT: () => (key: string) => key }));
vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../components/plugins/PluginAddDialog', () => ({ default: ({ initialTargets }: { initialTargets?: string[] }) => <div role="dialog" aria-label="add">{initialTargets?.join(',')}</div> }));
const synced = vi.hoisted(() => ({ targets: [{ name: 'pi' }] as { name: string; agent?: string }[] }));
vi.mock('../hooks/useSharedQueries', () => ({ useSyncedTargetsQuery: () => ({ data: synced }) }));

function mount(path = '/plugins', client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) { return render(<MemoryRouter initialEntries={[path]}><QueryClientProvider client={client}><ToastProvider><PluginsPage /></ToastProvider></QueryClientProvider></MemoryRouter>); }

describe('PluginsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    synced.targets = [{ name: 'pi' }];
    vi.mocked(pluginsApi.list).mockResolvedValue({ targetDefinitions: [{target:'codex',label:'Codex',project:false,operations:['add','sync','import']}], packages: { demo: { bindings: { codex: { id: 'demo@market' } } } }, hosts: [{ target: 'codex', version: '0.154', status: 'ready', installed: [{ id: 'demo@market', enabled: false }] }] });
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'reviewed', blocked: false, changes: [{ name: 'demo', target: 'codex', id: 'demo@market', action: 'selection' }] });
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [] }, failure: '' });
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
  const twoBehind = () => {
    vi.mocked(pluginsApi.list).mockResolvedValue({
      targetDefinitions: [{ target: 'pi', label: 'Pi', project: false, operations: ['add', 'check', 'update'], npm: true }, { target: 'codex', label: 'Codex', project: false, operations: ['add', 'check', 'update'] }],
      packages: { driver: { bindings: { pi: { id: 'npm:driver' } } }, tail: { bindings: { pi: { id: 'npm:tail' }, codex: { id: 'tail@market' } } } },
      hosts: [{ target: 'pi', version: '0.99.2', status: 'ready', installed: [{ id: 'npm:driver', version: '1.0.0', enabled: true }, { id: 'npm:tail', version: '2.0.0', enabled: true }] }],
    });
    // The check the page runs on its own; every update after it is a plan with nothing to say.
    vi.mocked(pluginsApi.preview).mockResolvedValueOnce({ revision: 'r', blocked: false, changes: [
      { name: 'driver', target: 'pi', id: 'npm:driver', action: 'update-available', binding: { id: 'npm:driver', version: '1.1.0' } },
      { name: 'tail', target: 'pi', id: 'npm:tail', action: 'update-available', binding: { id: 'npm:tail', version: '2.1.0' } },
      { name: 'tail', target: 'codex', id: 'tail@market', action: 'update-available', binding: { id: 'tail@market', version: '2.1.0' } },
    ] }).mockResolvedValue({ revision: 'u', blocked: false, changes: [] });
  };
  it('checks for updates on its own and applies every plugin ticked, one after another', async () => {
    twoBehind();
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.updateCount.other' }));
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'plugins.updateDialog.updateSelected.other' }));
    await waitFor(() => expect(pluginsApi.apply).toHaveBeenCalledTimes(2));
    expect(vi.mocked(pluginsApi.preview).mock.calls.slice(1).map((c) => c[0])).toEqual([{ action: 'update', name: 'driver', targets: ['pi'] }, { action: 'update', name: 'tail', targets: ['pi', 'codex'] }]);
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'update', name: 'tail', targets: ['pi', 'codex'] }, 'u');
  });
  it('leaves out a plugin unticked and an Agent unticked under its plugin', async () => {
    twoBehind();
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.updateCount.other' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('checkbox', { name: 'driver' }));
    fireEvent.click(within(dialog).getByRole('checkbox', { name: 'Codex' }));
    fireEvent.click(within(dialog).getByRole('button', { name: 'plugins.updateDialog.updateSelected.one' }));
    await waitFor(() => expect(pluginsApi.apply).toHaveBeenCalledTimes(1));
    expect(pluginsApi.apply).toHaveBeenCalledWith({ action: 'update', name: 'tail', targets: ['pi'] }, 'u');
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
