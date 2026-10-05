import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../../api/client';
import { ompExtensionsApi, type OmpExtensionRow, type OmpExtensionsPlan, type OmpExtensionsView } from '../../api/ompExtensions';
import { I18nProvider } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import TargetOmpExtensions from './TargetOmpExtensions';

vi.mock('../../api/ompExtensions', async (load) => ({
  ...await load<typeof import('../../api/ompExtensions')>(),
  ompExtensionsApi: { get: vi.fn(), preview: vi.fn(), apply: vi.fn() },
}));

const dir = '/home/me/.omp/agent/extensions/';
const row = (name: string, over: Partial<OmpExtensionRow> = {}): OmpExtensionRow => ({
  key: `ext:${name}`, path: `${dir}${name}.ts`, name, derivedId: `ext:${name}`, source: 'native', scope: 'global', selection: 'selected', enabled: true, selectable: true, readOnlyReason: '', owner: 'native', notes: [], ...over,
});
const view = (over: Partial<OmpExtensionsView> = {}): OmpExtensionsView => ({
  target: 'omp', scope: 'global', root: '/home/me/.omp/agent', settingsPath: '/home/me/.omp/agent/config.yml', revision: 'rev1', readOnly: false, reasons: [], warnings: [],
  rows: [
    row('notes'),
    row('lint', { selection: 'disabled', enabled: false }),
    row('skillshare-guard', { source: 'hook', owner: 'hooks', hooksTarget: 'omp', selectable: false, readOnlyReason: 'Managed by the Hooks tab.' }),
    row('bundled', { source: 'plugin', owner: 'plugin', selection: 'shadowed', selectable: false, readOnlyReason: 'Installed plugin: toolkit. Enable or disable the plugin instead.', notes: ['Installed plugin: toolkit'] }),
    row('weird', { selection: 'unknown', enabled: null, selectable: false, readOnlyReason: 'config.yml names this file by an explicit path, which bypasses disabledExtensions.' }),
  ],
  ...over,
});
const plan: OmpExtensionsPlan = { revision: 'plan1', settingsPath: '/home/me/.omp/agent/config.yml', rows: [{ key: 'ext:notes', derivedId: 'ext:notes', before: true, after: false }], warnings: ['A running Oh My Pi session keeps notes loaded until it restarts.'] };

const show = (data: OmpExtensionsView) => {
  vi.mocked(ompExtensionsApi.get).mockResolvedValue(data);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  render(<MemoryRouter><QueryClientProvider client={client}><I18nProvider><TargetOmpExtensions name={data.target} /></I18nProvider></QueryClientProvider></MemoryRouter>);
  return invalidate;
};
const notesSwitch = () => screen.findByRole('switch', { name: `Select ${dir}notes.ts in omp` });

describe('Oh My Pi target Extensions tab', () => {
  beforeEach(() => vi.clearAllMocks());

  it('lists a read-only target with its reasons and offers no mutation control', async () => {
    show(view({ readOnly: true, reasons: ['Oh My Pi 18.6.1 is not installed here, so its settings format is not verified.'], rows: view().rows.map((r) => ({ ...r, selectable: false, readOnlyReason: r.readOnlyReason || 'The target is read-only.' })) }));
    const list = await screen.findByLabelText('Extensions configured for omp');
    expect(within(list).getByText('notes.ts')).toBeInTheDocument();
    expect(within(list).getByText('Managed by the Hooks tab.')).toBeInTheDocument();
    expect(within(list).getByText('Disabled')).toBeInTheDocument();
    expect(within(list).getByText('Shadowed')).toBeInTheDocument();
    expect(within(list).getByText("Can't tell")).toBeInTheDocument();
    expect(screen.getByText('Oh My Pi 18.6.1 is not installed here, so its settings format is not verified.')).toBeInTheDocument();
    expect(screen.getByText(/Read-only: Skillshare lists what omp's settings/)).toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Review changes' })).not.toBeInTheDocument();
    expect(ompExtensionsApi.get).toHaveBeenCalledWith('omp');
  });

  it('keeps hook, plugin and explicit-path rows without a switch and says why, linking the hook back to Hooks', async () => {
    show(view());
    const list = await screen.findByLabelText('Extensions configured for omp');
    expect(within(list).getAllByRole('switch')).toHaveLength(2);
    expect(within(list).getByText('Managed by the Hooks tab.')).toBeInTheDocument();
    expect(within(list).getByText('Installed plugin: toolkit. Enable or disable the plugin instead.')).toBeInTheDocument();
    expect(within(list).getByText('config.yml names this file by an explicit path, which bypasses disabledExtensions.')).toBeInTheDocument();
    expect(within(list).getAllByText('Read-only')).toHaveLength(3);
    expect(screen.getByRole('link', { name: 'Managed in Hooks' })).toHaveAttribute('href', '/targets/omp?tab=hooks');
    expect(screen.getByRole('link', { name: 'Open Plugins' })).toHaveAttribute('href', '/plugins');
    expect(screen.queryByText(/Read-only: Skillshare lists/)).not.toBeInTheDocument();
    expect(screen.getByText(/Selection means what the settings configure, not what is loaded or running/)).toBeInTheDocument();
  });

  it('groups files by source, scope and folder, showing their shared path once and notes only in Details', async () => {
    const user = userEvent.setup();
    show(view({ rows: [
      row('notes', { notes: ['Native file inspected without execution.'] }),
      row('lint', { selection: 'disabled', enabled: false }),
      row('project', { scope: 'project' }),
      row('nested', { path: `${dir}nested/index.ts` }),
      row('configured', { source: 'configured' }),
      row('hook', { source: 'hook', selectable: false, owner: 'hooks', hooksTarget: 'omp', readOnlyReason: 'Managed by Hooks.', notes: ['Managed by Hooks; review changes in the Hooks page.'] }),
    ] }));
    const list = await screen.findByLabelText('Extensions configured for omp');
    const groups = within(list).getAllByRole('region', { name: 'Extensions folder' });
    expect(groups).toHaveLength(3);
    expect(within(groups[0]).getByText('1 / 2 on')).toBeInTheDocument();
    expect(within(groups[0]).getAllByText('~/.omp/agent/extensions/')).toHaveLength(1);
    expect(within(groups[0]).getByText('notes.ts')).toHaveAttribute('title', `${dir}notes.ts`);
    expect(screen.queryByText('Native file inspected without execution.')).not.toBeInTheDocument();
    expect(screen.queryByText('Managed by Hooks; review changes in the Hooks page.')).not.toBeInTheDocument();
    expect(screen.getByText('Managed by Hooks.')).toBeInTheDocument();
    expect(within(list).getByRole('region', { name: 'Configured extensions' })).toBeInTheDocument();
    expect(within(list).getByRole('region', { name: 'Ambient hooks' })).toBeInTheDocument();
    await user.click(within(groups[0]).getByRole('button', { name: 'Details' }));
    expect(within(groups[0]).getByRole('region', { name: 'Details of Extensions folder' })).toHaveTextContent('Native file inspected without execution.');
    expect(ompExtensionsApi.preview).not.toHaveBeenCalled();
  });

  it('labels a plugin folder by its location, not by the first module name', async () => {
    show(view({ rows: [
      row('lint', { path: '/plugins/tools/lint.ts', source: 'plugin', owner: 'plugin', selectable: false }),
      row('format', { path: '/plugins/tools/format.ts', source: 'plugin', owner: 'plugin', selectable: false }),
    ] }));
    const group = await screen.findByRole('region', { name: 'Plugins · tools' });
    expect(within(group).queryByText('plugin')).not.toBeInTheDocument();
    expect(within(group).queryByText('Plugins')).not.toBeInTheDocument();
    expect(within(group).getByText('global')).toBeInTheDocument();
    expect(within(group).getByRole('link', { name: 'Open Plugins' })).toHaveAttribute('href', '/plugins');
    expect(within(group).getByText('/plugins/tools/')).toBeInTheDocument();
    expect(within(group).getByText('lint.ts')).toBeInTheDocument();
    expect(within(group).getByText('format.ts')).toBeInTheDocument();
    expect(within(group).queryByRole('switch')).not.toBeInTheDocument();
  });

  it('uses the native plugin identity and version across its extension folders, with Pi-style Details', async () => {
    const user = userEvent.setup();
    show(view({ rows: [
      row('index', { path: '/plugins/@scope/ponytail/pi-extension/index.js', source: 'plugin', owner: 'plugin', selectable: false, pluginName: '@scope/ponytail', pluginRoot: '/plugins/@scope/ponytail', pluginVersion: '4.12.0', notes: ['Installed plugin: @scope/ponytail'] }),
      row('extra', { path: '/plugins/@scope/ponytail/extensions/extra.ts', source: 'plugin', owner: 'plugin', selectable: false, pluginName: '@scope/ponytail', pluginRoot: '/plugins/@scope/ponytail', pluginVersion: '4.12.0', notes: ['Installed plugin: @scope/ponytail'] }),
    ] }));
    const group = await screen.findByRole('region', { name: 'Plugins · ponytail' });
    expect(screen.getAllByRole('region', { name: 'Plugins · ponytail' })).toHaveLength(1);
    expect(within(group).getByText('4.12.0')).toBeInTheDocument();
    expect(within(group).getByText('Plugins · ponytail')).toHaveAttribute('title', '@scope/ponytail');
    expect(within(group).getByText('pi-extension/index.js')).toBeInTheDocument();
    expect(within(group).getByText('extensions/extra.ts')).toBeInTheDocument();
    expect(within(group).queryByRole('switch')).not.toBeInTheDocument();
    await user.click(within(group).getByRole('button', { name: 'Details' }));
    const details = within(group).getByRole('region', { name: 'Details of Plugins · ponytail' });
    expect(details).toHaveClass('grid', 'grid-cols-[max-content_1fr]');
    expect(within(details).getByText('Source')).toBeInTheDocument();
    expect(within(details).getByText('/plugins/@scope/ponytail')).toBeInTheDocument();
    expect(within(details).getByText('@scope/ponytail')).toBeInTheDocument();
    expect(within(details).getByText('Selection identifiers')).toBeInTheDocument();
    expect(within(details).queryByText('Installed plugin: @scope/ponytail')).not.toBeInTheDocument();
    expect(within(group).queryByRole('button', { name: /^(Collapse|Expand) / })).not.toBeInTheDocument();
  });

  it('shows a plugin-relative shared folder like Pi, keeping its full source in Details', async () => {
    const user = userEvent.setup();
    show(view({ rows: [row('index', { path: '/plugins/superpowers/.pi/extensions/superpowers.ts', source: 'plugin', owner: 'plugin', selectable: false, pluginName: 'superpowers', pluginRoot: '/plugins/superpowers', pluginVersion: '6.4.2', readOnlyReason: 'Managed by its plugin; change it on the Plugins page.' })] }));
    const group = await screen.findByRole('region', { name: 'Plugins · superpowers' });
    expect(within(group).getByText('.pi/extensions/')).toBeInTheDocument();
    expect(within(group).queryByText('/plugins/superpowers/.pi/extensions/')).not.toBeInTheDocument();
    expect(within(group).getByText('superpowers.ts')).toHaveAttribute('title', '/plugins/superpowers/.pi/extensions/superpowers.ts');
    expect(within(group).getByText('Managed by its plugin; change it on the Plugins page.')).toBeInTheDocument();
    await user.click(within(group).getByRole('button', { name: 'Details' }));
    expect(within(group).getByRole('region', { name: 'Details of Plugins · superpowers' })).toHaveTextContent('/plugins/superpowers');
  });

  it('keeps Windows filenames relative to the shared directory', async () => {
    show(view({ rows: [
      row('notes', { path: 'C:\\Users\\me\\.omp\\agent\\extensions\\notes.ts' }),
      row('lint', { path: 'C:\\Users\\me\\.omp\\agent\\extensions\\lint.ts' }),
    ] }));
    const group = await screen.findByRole('region', { name: 'Extensions folder' });
    expect(within(group).getAllByText('~\\.omp\\agent\\extensions\\')).toHaveLength(1);
    expect(within(group).getByText('notes.ts')).toHaveAttribute('title', 'C:\\Users\\me\\.omp\\agent\\extensions\\notes.ts');
    expect(within(group).getByText('lint.ts')).toBeInTheDocument();
  });

  it('keeps rows, drafts and their selected count visible when Details is toggled', async () => {
    const user = userEvent.setup();
    show(view({ rows: [row('notes'), row('lint', { selection: 'disabled', enabled: false })] }));
    await user.click(await notesSwitch());
    const group = screen.getByRole('region', { name: 'Extensions folder' });
    expect(within(group).getByText('0 / 2 on')).toBeInTheDocument();
    expect(within(group).queryByRole('button', { name: /^(Collapse|Expand) / })).not.toBeInTheDocument();
    await user.click(within(group).getByRole('button', { name: 'Details' }));
    expect(within(group).getByRole('region', { name: 'Details of Extensions folder' })).toBeInTheDocument();
    expect(within(group).getAllByRole('switch')).toHaveLength(2);
    expect(screen.getByRole('toolbar', { name: '1 extension change' })).toBeInTheDocument();
    await user.click(within(group).getByRole('button', { name: 'Details' }));
    expect(within(group).queryByRole('region', { name: 'Details of Extensions folder' })).not.toBeInTheDocument();
    expect(await notesSwitch()).toHaveAttribute('aria-checked', 'false');
    expect(ompExtensionsApi.apply).not.toHaveBeenCalled();
  });

  it('previews a switched file against the view revision, applies the plan revision and refreshes the tab and Plugins', async () => {
    const user = userEvent.setup();
    vi.mocked(ompExtensionsApi.preview).mockResolvedValue(plan);
    vi.mocked(ompExtensionsApi.apply).mockResolvedValue({ ...plan, backupId: 'b1' });
    const invalidate = show(view());
    const sw = await notesSwitch();
    expect(sw).toHaveAttribute('aria-checked', 'true');
    await user.click(sw);
    expect(sw).toHaveAttribute('aria-checked', 'false');
    expect(screen.getByText('Selected → Disabled')).toBeInTheDocument();
    expect(screen.getByRole('toolbar', { name: '1 extension change' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog', { name: 'Review changes — omp' });
    await waitFor(() => expect(ompExtensionsApi.preview).toHaveBeenCalledWith('omp', [{ key: 'ext:notes', enabled: false }], 'rev1'));
    expect(await within(dialog).findByText('notes')).toBeInTheDocument();
    expect(within(dialog).getByText(`${dir}notes.ts`)).toBeInTheDocument();
    expect(within(dialog).getByRole('alert')).toHaveTextContent('A running Oh My Pi session keeps notes loaded until it restarts.');
    expect(within(dialog).getByText(/Previewed revision plan1/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Apply changes' }));
    await waitFor(() => expect(ompExtensionsApi.apply).toHaveBeenCalledWith('omp', [{ key: 'ext:notes', enabled: false }], 'plan1'));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.ompExtensions('omp') });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.plugins });
    expect(await screen.findByRole('status')).toHaveTextContent('Saved to ~/.omp/agent/config.yml');
    expect(screen.queryByRole('toolbar')).not.toBeInTheDocument();
  });

  it('refuses a stale preview, keeps the draft and offers to review again', async () => {
    const user = userEvent.setup();
    vi.mocked(ompExtensionsApi.preview).mockRejectedValue(new ApiError(409, 'changed', { code: 'omp_extensions_stale' }));
    const invalidate = show(view());
    await user.click(await notesSwitch());
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog', { name: 'Review changes — omp' });
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('The settings file changed after this view was read. Nothing was written.');
    expect(within(dialog).queryByRole('button', { name: 'Apply changes' })).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Review again' }));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.ompExtensions('omp') });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(await notesSwitch()).toHaveAttribute('aria-checked', 'false');
    expect(screen.getByRole('toolbar', { name: '1 extension change' })).toBeInTheDocument();
  });

  it('shows busy and read-only refusals as the server states them', async () => {
    const user = userEvent.setup();
    vi.mocked(ompExtensionsApi.preview).mockResolvedValue(plan);
    vi.mocked(ompExtensionsApi.apply).mockRejectedValueOnce(new ApiError(409, 'busy', { code: 'omp_extensions_busy' })).mockRejectedValueOnce(new ApiError(409, 'config.yml is managed by another tool', { code: 'omp_extensions_read_only' }));
    show(view());
    await user.click(await notesSwitch());
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await user.click(await screen.findByRole('button', { name: 'Apply changes' }));
    // The plan's own warning stays above the refusal.
    await waitFor(() => expect(screen.getAllByRole('alert').at(-1)).toHaveTextContent('Nothing was written; try again in a moment.'));
    await user.click(screen.getByRole('button', { name: 'Apply changes' }));
    await waitFor(() => expect(screen.getAllByRole('alert').at(-1)).toHaveTextContent('config.yml is managed by another tool'));
  });

  it('drops a draft the refreshed view no longer offers a switch for', async () => {
    const user = userEvent.setup();
    vi.mocked(ompExtensionsApi.preview).mockRejectedValue(new ApiError(409, 'changed', { code: 'omp_extensions_stale' }));
    show(view());
    await user.click(await notesSwitch());
    // The file changed under the draft: the row is now an explicit path, so the draft goes with it.
    vi.mocked(ompExtensionsApi.get).mockResolvedValue(view({ revision: 'rev2', rows: view().rows.map((r) => (r.name === 'notes' ? { ...r, selectable: false, readOnlyReason: 'Now an explicit path.' } : r)) }));
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await user.click(await screen.findByRole('button', { name: 'Review again' }));
    await waitFor(() => expect(screen.queryByRole('toolbar')).not.toBeInTheDocument());
    expect(screen.getByText('Now an explicit path.')).toBeInTheDocument();
    expect(screen.queryByRole('switch', { name: `Select ${dir}notes.ts in omp` })).not.toBeInTheDocument();
  });

  it('shows the server’s warnings as sent and the empty state without an add button', async () => {
    show(view({ rows: [], warnings: ['settings.json lists extensions/gone.ts, which does not exist'] }));
    expect(await screen.findByText('No extensions configured')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('settings.json lists extensions/gone.ts, which does not exist');
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });
});
