import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../../api/client';
import { piExtensionsApi, type PiExtensionsPlan, type PiExtensionsView } from '../../api/piExtensions';
import { I18nProvider } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import TargetPiExtensions from './TargetPiExtensions';

vi.mock('../../api/piExtensions', async (load) => ({
  ...await load<typeof import('../../api/piExtensions')>(),
  piExtensionsApi: { get: vi.fn(), preview: vi.fn(), apply: vi.fn() },
}));

const pkg = '/home/me/pkgs/tools';
const global = (over: Partial<PiExtensionsView> = {}): PiExtensionsView => ({
  target: 'pi', scope: 'global', settingsPath: '/home/me/.pi/agent/settings.json', version: '0.99.2', minVersion: '0.99.2',
  editable: true, revision: 'rev1', folders: [],
  packages: [{
    index: 0, source: pkg, identity: pkg, kind: 'local', form: 'object', scope: 'global', install: 'present', rules: ['-extensions/b.ts', '!extensions/slow-*.ts'], otherKeys: ['autoUpdate'],
    rows: [
      { path: 'extensions/a.ts', file: 'present', selection: 'loads', origin: 'default', editable: true },
      { path: 'extensions/b.ts', file: 'present', selection: 'skipped', origin: 'rule', rule: '-extensions/b.ts', editable: true },
      { path: 'extensions/slow-x.ts', file: 'present', selection: 'unknown', origin: 'glob', globs: ['!extensions/slow-*.ts'], editable: false },
    ],
  }],
  ...over,
});
const plan: PiExtensionsPlan = {
  revision: 'rev1', settingsPath: '/home/me/.pi/agent/settings.json',
  entries: [{ scope: 'global', index: 0, source: pkg, identity: pkg, before: ['-extensions/b.ts', '!extensions/slow-*.ts'], after: ['!extensions/slow-*.ts', '-extensions/a.ts', '-extensions/b.ts'], converted: false, keptKeys: ['autoUpdate'] }],
  rows: [{ scope: 'global', index: 0, path: 'extensions/a.ts', before: 'loads', after: 'skipped', removed: [], added: ['-extensions/a.ts'] }],
};

const project = (over: Partial<PiExtensionsView> = {}): PiExtensionsView => global({
  target: 'acme@pi', scope: 'project', settingsPath: '/code/acme/.pi/settings.json', trust: { saved: 'untrusted', default: 'ask' },
  packages: [
    { ...global().packages[0], scope: 'project', shape: 'delta', rules: ['-extensions/a.ts'], otherKeys: ['autoload'], rows: [
      { path: 'extensions/a.ts', file: 'present', selection: 'skipped', origin: 'project', rule: '-extensions/a.ts', editable: true },
      { path: 'extensions/b.ts', file: 'present', selection: 'loads', origin: 'inherited', editable: true },
    ] },
    { ...global().packages[0], index: 0, source: 'npm:@acme/lint@1.2.0', identity: 'npm:@acme/lint', kind: 'npm', scope: 'global', shape: 'global', rules: null, otherKeys: [], rows: [
      { path: 'extensions/lint.ts', file: 'present', selection: 'loads', origin: 'default', editable: true },
    ] },
  ],
  ...over,
});

const show = (data: PiExtensionsView) => {
  vi.mocked(piExtensionsApi.get).mockResolvedValue(data);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  render(<MemoryRouter><QueryClientProvider client={client}><I18nProvider><TargetPiExtensions name={data.target} /></I18nProvider></QueryClientProvider></MemoryRouter>);
  return invalidate;
};

describe('Pi target Extensions tab', () => {
  beforeEach(() => vi.clearAllMocks());

  it('previews the switched extension, applies that revision and refreshes the tab and Plugins', async () => {
    const user = userEvent.setup();
    vi.mocked(piExtensionsApi.preview).mockResolvedValue(plan);
    vi.mocked(piExtensionsApi.apply).mockResolvedValue(plan);
    const invalidate = show(global());
    await user.click(await screen.findByRole('switch', { name: `Load extensions/a.ts from ${pkg} in pi` }));
    expect(screen.getByText('1 extension change')).toBeInTheDocument();
    expect(screen.getByText('On → Off')).toBeInTheDocument();
    expect(piExtensionsApi.preview).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog');
    const change = [{ scope: 'global', index: 0, source: pkg, path: 'extensions/a.ts', action: 'exclude' }];
    expect(piExtensionsApi.preview).toHaveBeenCalledWith('pi', change);
    expect(await within(dialog).findByText(/"!extensions\/slow-\*\.ts","-extensions\/a\.ts"/)).toBeInTheDocument();
    expect(within(dialog).getByText('Kept as is in this entry: autoUpdate')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Apply changes' }));
    await waitFor(() => expect(piExtensionsApi.apply).toHaveBeenCalledWith('pi', change, 'rev1'));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.piExtensions('pi') });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.plugins });
    expect(await screen.findByRole('status')).toHaveTextContent('keep the old set until Pi reloads');
  });

  it('shows a switchable extension\'s selection on its switch, with no column of selections', async () => {
    show(global());
    expect(await screen.findByRole('switch', { name: `Load extensions/a.ts from ${pkg} in pi` })).toBeChecked();
    expect(screen.getByRole('switch', { name: `Load extensions/b.ts from ${pkg} in pi` })).not.toBeChecked();
    for (const gone of ['Configured', 'On', 'Off']) {
      expect(screen.queryByText(gone)).not.toBeInTheDocument();
    }
  });

  it('counts the extensions a package loads, pending switches included', async () => {
    const user = userEvent.setup();
    show(global());
    expect(await screen.findByText('1 / 3 on')).toBeInTheDocument();
    await user.click(screen.getByRole('switch', { name: `Load extensions/b.ts from ${pkg} in pi` }));
    expect(screen.getByText('2 / 3 on')).toBeInTheDocument();
  });

  it('names the shared folder once and leaves the package default unsaid', async () => {
    show(global());
    expect(await screen.findByText('extensions/')).toBeInTheDocument();
    expect(screen.getByTitle('extensions/a.ts')).toHaveTextContent(/^a\.ts$/);
    expect(screen.queryByText('Package default')).not.toBeInTheDocument();
  });

  it('collapses a package to its header', async () => {
    const user = userEvent.setup();
    show(global());
    await user.click(await screen.findByRole('button', { name: `Collapse ${pkg}` }));
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.getByText('1 / 3 on')).toBeInTheDocument();
  });

  it('explains what the page does in one note shown on hover and on focus', async () => {
    const user = userEvent.setup();
    show(global());
    const info = await screen.findByRole('button', { name: 'About this page' });
    expect(screen.getAllByRole('button', { name: /^About/ })).toHaveLength(1);
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    await user.hover(info);
    expect(await screen.findByRole('tooltip')).toHaveTextContent(/changes settings only.*Pi decides.*reload/);
    await user.unhover(info);
    await waitFor(() => expect(screen.queryByRole('tooltip')).not.toBeInTheDocument());
    await user.tab();
    expect(info).toHaveFocus();
    expect(await screen.findByRole('tooltip')).toHaveTextContent(/Pi decides/);
  });

  it('says why a pattern-decided extension has no switch and what to do', async () => {
    show(global());
    await screen.findByTitle('extensions/slow-x.ts');
    expect(screen.getAllByRole('switch')).toHaveLength(2);
    expect(screen.queryByRole('switch', { name: /slow-x/ })).not.toBeInTheDocument();
    expect(screen.getByText('Can\'t tell')).toBeInTheDocument();
    expect(screen.getByText(/pattern Skillshare can't evaluate: !extensions\/slow-\*\.ts\. Change it with pi config\./)).toBeInTheDocument();
  });

  it('marks a missing file in its row and keeps the source and rules in details', async () => {
    const user = userEvent.setup();
    const base = global().packages[0];
    show(global({ packages: [{ ...base, rows: [...base.rows, { path: 'extensions/gone.ts', file: 'missing', selection: 'none', origin: 'rule', rule: '-extensions/gone.ts', editable: true }] }] }));
    expect(await screen.findByText('File missing')).toBeInTheDocument();
    expect(screen.queryByText(/Other keys kept/)).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Details' }));
    const details = screen.getByRole('region', { name: `Details of ${pkg}` });
    expect(details).toHaveTextContent('-extensions/b.ts, !extensions/slow-*.ts');
    expect(details).toHaveTextContent('autoUpdate');
  });

  it('refuses a stale apply and offers to review again', async () => {
    const user = userEvent.setup();
    vi.mocked(piExtensionsApi.preview).mockResolvedValue(plan);
    vi.mocked(piExtensionsApi.apply).mockRejectedValue(new ApiError(409, 'changed', { code: 'pi_extensions_stale' }));
    const invalidate = show(global());
    await user.click(await screen.findByRole('switch', { name: `Load extensions/a.ts from ${pkg} in pi` }));
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await user.click(await screen.findByRole('button', { name: 'Apply changes' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('The settings file changed after this preview. Nothing was written.');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Review again' }));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.piExtensions('pi') });
  });

  it('says so when Pi holds the settings lock', async () => {
    const user = userEvent.setup();
    vi.mocked(piExtensionsApi.preview).mockResolvedValue(plan);
    vi.mocked(piExtensionsApi.apply).mockRejectedValue(new ApiError(409, 'busy', { code: 'pi_extensions_busy' }));
    show(global());
    await user.click(await screen.findByRole('switch', { name: `Load extensions/a.ts from ${pkg} in pi` }));
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await user.click(await screen.findByRole('button', { name: 'Apply changes' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Nothing was written; try again in a moment.');
  });

  it('removes an exact rule and leaves the result to the remaining rules', async () => {
    const user = userEvent.setup();
    vi.mocked(piExtensionsApi.preview).mockResolvedValue(plan);
    show(global());
    await user.click(await screen.findByRole('button', { name: 'Remove rule' }));
    // Not computed here: no switch claims the result until the preview does.
    expect(screen.queryByRole('switch', { name: `Load extensions/b.ts from ${pkg} in pi` })).not.toBeInTheDocument();
    expect(screen.getByText('Off → Shown in review')).toBeInTheDocument();
    expect(screen.getByText('Rule removed · the remaining rules decide')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await waitFor(() => expect(piExtensionsApi.preview).toHaveBeenCalledWith('pi', [{ scope: 'global', index: 0, source: pkg, path: 'extensions/b.ts', action: 'default' }]));
  });

  it('switches a rule away instead of writing its opposite when that gives the other state', async () => {
    const user = userEvent.setup();
    vi.mocked(piExtensionsApi.preview).mockResolvedValue(plan);
    const view = global();
    view.packages[0].rows[1] = { ...view.packages[0].rows[1], unruled: 'loads' };
    show(view);
    const sw = await screen.findByRole('switch', { name: `Load extensions/b.ts from ${pkg} in pi` });
    // The switch already does what Remove rule would.
    expect(screen.queryByRole('button', { name: 'Remove rule' })).not.toBeInTheDocument();
    await user.click(sw);
    expect(sw).toBeChecked();
    expect(screen.getByText('Off → On')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await waitFor(() => expect(piExtensionsApi.preview).toHaveBeenCalledWith('pi', [{ scope: 'global', index: 0, source: pkg, path: 'extensions/b.ts', action: 'default' }]));
  });

  it.each([
    { shape: 'delta' as const, rule: 'extensions/*.ts', selection: 'loads' as const },
    { shape: 'delta' as const, rule: '!extensions/*.ts', selection: 'skipped' as const },
    { shape: 'deltaOnly' as const, rule: 'extensions/*.ts', selection: 'loads' as const },
    { shape: 'deltaOnly' as const, rule: '!extensions/*.ts', selection: 'skipped' as const },
  ])('keeps $shape glob $rule switchable without offering Remove rule', async ({ shape, rule, selection }) => {
    const user = userEvent.setup();
    const data = project();
    data.packages = [{ ...data.packages[0], shape, rules: [rule], rows: [
      { path: 'extensions/a.ts', file: 'present', selection, origin: 'project', rule, editable: true },
    ] }];
    vi.mocked(piExtensionsApi.preview).mockResolvedValue({ revision: 'glob-preview', settingsPath: data.settingsPath, entries: [], rows: [] });
    show(data);
    const toggle = await screen.findByRole('switch', { name: `Load extensions/a.ts from ${pkg} in acme@pi` });
    expect(toggle).toHaveAttribute('aria-checked', String(selection === 'loads'));
    expect(screen.queryByRole('button', { name: 'Remove rule' })).not.toBeInTheDocument();
    expect(piExtensionsApi.preview).not.toHaveBeenCalled();
    await user.click(toggle);
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await waitFor(() => expect(piExtensionsApi.preview).toHaveBeenCalledWith('acme@pi', [{ scope: 'project', index: 0, source: pkg, path: 'extensions/a.ts', action: selection === 'loads' ? 'exclude' : 'select' }]));
    expect(piExtensionsApi.apply).not.toHaveBeenCalled();
  });

  it('says why a package has no switches and what to do instead', async () => {
    const row = { path: 'extensions/a.ts', file: 'present' as const, selection: 'loads' as const, origin: 'default', editable: false };
    const base = global().packages[0];
    show(global({ packages: [
      { ...base, form: 'string', rules: null, otherKeys: [], readOnly: 'otherResources', rows: [row] },
      { ...base, index: 1, source: '/home/me/one.ts', identity: '/home/me/one.ts', form: 'string', rules: null, otherKeys: [], readOnly: 'singleFile', rows: [{ ...row, path: 'one.ts', origin: 'file' }] },
    ] }));
    expect(await screen.findByText(/could change which of those Pi loads, so it is view only\. Change it with pi config\./)).toBeInTheDocument();
    expect(screen.getByText(/Pi loads this file as it is and ignores filters/)).toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });

  it('is read-only on a Pi version Skillshare has not verified and says what to do', async () => {
    show(global({ editable: false, readOnly: 'unsupportedVersion', version: '0.99.1', target: 'pi-work', scope: 'account', packages: global().packages.map((p) => ({ ...p, rows: p.rows.map((r) => ({ ...r, editable: false })) })) }));
    expect(await screen.findByText('Read-only: pi-work runs Pi 0.99.1, and Skillshare needs Pi 0.99.2 or later. Use pi config, or update Pi.')).toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remove rule' })).not.toBeInTheDocument();
  });

  it('saves a project override of a global package and of the project entry, whatever Pi trusts', async () => {
    const user = userEvent.setup();
    const projectPlan: PiExtensionsPlan = {
      revision: 'rev2', settingsPath: '/code/acme/.pi/settings.json',
      entries: [
        { scope: 'project', index: 0, source: pkg, identity: pkg, before: ['-extensions/a.ts'], after: null, converted: false, keptKeys: ['autoload'], removed: true },
        { scope: 'global', index: 0, source: 'npm:@acme/lint@1.2.0', identity: 'npm:@acme/lint', before: null, after: ['-extensions/lint.ts'], converted: false, keptKeys: ['autoload'], created: true, reference: 'npm:@acme/lint@1.2.0' },
      ],
      rows: [
        { scope: 'project', index: 0, path: 'extensions/a.ts', before: 'skipped', after: 'loads', removed: ['-extensions/a.ts'], added: [] },
        { scope: 'global', index: 0, path: 'extensions/lint.ts', before: 'loads', after: 'skipped', removed: [], added: ['-extensions/lint.ts'] },
      ],
    };
    vi.mocked(piExtensionsApi.preview).mockResolvedValue(projectPlan);
    vi.mocked(piExtensionsApi.apply).mockResolvedValue(projectPlan);
    show(project());
    expect(await screen.findByText('Changes pi (global)')).toBeInTheDocument();
    expect(screen.getByText('From pi (global)', { selector: '.ss-tag' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Remove rule' }));
    await user.click(screen.getByRole('switch', { name: 'Load extensions/lint.ts from npm:@acme/lint in acme@pi' }));
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog', { name: 'Save project settings — acme@pi' });
    const changes = [
      { scope: 'project', index: 0, source: pkg, path: 'extensions/a.ts', action: 'default' },
      { scope: 'global', index: 0, source: 'npm:@acme/lint@1.2.0', path: 'extensions/lint.ts', action: 'exclude' },
    ];
    expect(piExtensionsApi.preview).toHaveBeenCalledWith('acme@pi', changes);
    expect(await within(dialog).findByText(/New project entry for npm:@acme\/lint with source npm:@acme\/lint@1\.2\.0/)).toBeInTheDocument();
    expect(within(dialog).getByText(/No project rule is left, so this project entry is removed/)).toBeInTheDocument();
    expect(within(dialog).getByText(/only when it trusts this project/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Save project settings' }));
    await waitFor(() => expect(piExtensionsApi.apply).toHaveBeenCalledWith('acme@pi', changes, 'rev2'));
    expect(await screen.findByRole('status')).toHaveTextContent('Pi uses it only if it trusts this project');
  });

  it('keeps the trust hints of a project in the page note', async () => {
    const user = userEvent.setup();
    show(project());
    await user.hover(await screen.findByRole('button', { name: 'About this page' }));
    const tip = await screen.findByRole('tooltip');
    expect(tip).toHaveTextContent('Saved choice in Pi: untrusted · Pi default for new folders: ask · hints, not Pi\'s decision');
    expect(tip).toHaveTextContent(/never trusts it for you/);
  });

  it('says why a global source can\'t get a project override', async () => {
    const base = project().packages[1];
    show(project({ packages: [{ ...base, source: 'git:https://***@example.com/acme/lint', readOnly: 'credentials', rows: base.rows.map((r) => ({ ...r, editable: false })) }] }));
    expect(await screen.findByText(/never copies into a project/)).toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });

  it('lists an unnamed path of a project-only override as not loaded and selectable', async () => {
    const base = project().packages[0];
    show(project({ packages: [{ ...base, shape: 'deltaOnly', rows: [{ path: 'extensions/c.ts', file: 'present', selection: 'skipped', origin: 'unnamed', editable: true }] }] }));
    expect(await screen.findByText('Not named by this project entry, so Pi doesn\'t load it')).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: `Load extensions/c.ts from ${pkg} in acme@pi` })).not.toBeChecked();
  });

  it('shows extensions an extra links into Pi’s folder as read-only, with where to change them', async () => {
    show(global({ packages: [], folders: [{ path: '/home/me/.pi/agent/extensions', kind: 'folder', rows: [{ path: 'extensions/guard.ts', file: 'present', selection: 'loads', provenance: 'extras', extra: 'pi-ext' }] }] }));
    expect(await screen.findByText('Extras · pi-ext — change it in Extras')).toBeInTheDocument();
    expect(screen.getByText(/Pi loads the files in this folder unless its settings turn them off/)).toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });

  it('labels whose settings decide each folder in a project', async () => {
    const row = { file: 'present', selection: 'loads', provenance: 'native' } as const;
    show(project({ packages: [], folders: [
      { path: '/code/acme/.pi/extensions', kind: 'folder', scope: 'project', rows: [{ ...row, path: 'extensions/local.ts' }] },
      { path: '/home/me/.pi/agent/extensions', kind: 'folder', scope: 'global', rows: [{ ...row, path: 'extensions/scratch.ts' }] },
    ] }));
    const header = (path: string) => screen.getByTitle(path).parentElement!;
    expect(await screen.findByTitle('extensions/local.ts')).toBeInTheDocument();
    expect(header('/code/acme/.pi/extensions')).toHaveTextContent('Project only');
    expect(header('/home/me/.pi/agent/extensions')).toHaveTextContent('From pi (global)');
  });

  it('opens Plugins to add a package with this target ticked', async () => {
    show(global({ packages: [] }));
    expect(await screen.findByRole('link', { name: /Add a package/ })).toHaveAttribute('href', '/plugins?add=pi');
  });

  it('opens Plugins to add a package with nothing ticked from a project', async () => {
    show(project({ packages: [] }));
    expect(await screen.findByRole('link', { name: /Add a package/ })).toHaveAttribute('href', '/plugins?add=');
  });

  it('shows the error when the tab cannot load', async () => {
    vi.mocked(piExtensionsApi.get).mockRejectedValue(new Error('not a Pi target: claude'));
    render(<MemoryRouter><QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><I18nProvider><TargetPiExtensions name="claude" /></I18nProvider></QueryClientProvider></MemoryRouter>);
    expect(await screen.findByText('not a Pi target: claude')).toBeInTheDocument();
  });
});
