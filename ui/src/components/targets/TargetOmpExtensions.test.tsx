import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../../api/client';
import {
  ompExtensionsApi,
  type OmpExtensionRow,
  type OmpExtensionsPlan,
  type OmpExtensionsView,
} from '../../api/ompExtensions';
import { I18nProvider } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import TargetOmpExtensions from './TargetOmpExtensions';

vi.mock('../../api/ompExtensions', async (load) => ({
  ...(await load<typeof import('../../api/ompExtensions')>()),
  ompExtensionsApi: { get: vi.fn(), preview: vi.fn(), apply: vi.fn() },
}));

const dir = '/home/me/.omp/agent/extensions/';
const row = (name: string, over: Partial<OmpExtensionRow> = {}): OmpExtensionRow => ({
  key: `ext:${name}`,
  path: `${dir}${name}.ts`,
  name,
  derivedId: `ext:${name}`,
  source: 'native',
  scope: 'global',
  selection: 'selected',
  enabled: true,
  selectable: true,
  readOnlyReason: '',
  owner: 'native',
  notes: [],
  ...over,
});
const view = (over: Partial<OmpExtensionsView> = {}): OmpExtensionsView => ({
  target: 'omp',
  scope: 'global',
  root: '/home/me/.omp/agent',
  settingsPath: '/home/me/.omp/agent/config.yml',
  revision: 'rev1',
  readOnly: false,
  reasons: [],
  warnings: [],
  rows: [
    row('notes'),
    row('lint', { selection: 'disabled', enabled: false }),
    row('skillshare-guard', {
      source: 'hook',
      owner: 'hooks',
      hooksTarget: 'omp',
      selectable: false,
      readOnlyReason: 'Managed by the Hooks tab.',
    }),
    row('bundled', {
      source: 'plugin',
      owner: 'plugin',
      selection: 'shadowed',
      selectable: false,
      readOnlyReason: 'Installed plugin: toolkit. Enable or disable the plugin instead.',
      notes: ['Installed plugin: toolkit'],
    }),
    row('weird', {
      selection: 'unknown',
      enabled: null,
      selectable: false,
      readOnlyReason:
        'config.yml names this file by an explicit path, which bypasses disabledExtensions.',
    }),
  ],
  ...over,
});
const plan: OmpExtensionsPlan = {
  revision: 'plan1',
  settingsPath: '/home/me/.omp/agent/config.yml',
  rows: [{ key: 'ext:notes', derivedId: 'ext:notes', before: true, after: false }],
  warnings: ['A running Oh My Pi session keeps notes loaded until it restarts.'],
};

const show = (data: OmpExtensionsView) => {
  vi.mocked(ompExtensionsApi.get).mockResolvedValue(data);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  render(
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <I18nProvider>
          <TargetOmpExtensions name={data.target} />
        </I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
  return invalidate;
};
const notesSwitch = () => screen.findByRole('switch', { name: `Select ${dir}notes.ts in omp` });

describe('Oh My Pi target Extensions tab', () => {
  beforeEach(() => vi.clearAllMocks());

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
    await waitFor(() =>
      expect(ompExtensionsApi.preview).toHaveBeenCalledWith(
        'omp',
        [{ key: 'ext:notes', enabled: false }],
        'rev1',
      ),
    );
    expect(await within(dialog).findByText('notes')).toBeInTheDocument();
    expect(within(dialog).getByText(`${dir}notes.ts`)).toBeInTheDocument();
    expect(within(dialog).getByRole('alert')).toHaveTextContent(
      'A running Oh My Pi session keeps notes loaded until it restarts.',
    );
    expect(within(dialog).getByText(/Previewed revision plan1/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Apply changes' }));
    await waitFor(() =>
      expect(ompExtensionsApi.apply).toHaveBeenCalledWith(
        'omp',
        [{ key: 'ext:notes', enabled: false }],
        'plan1',
      ),
    );
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.ompExtensions('omp') });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.plugins });
    expect(await screen.findByRole('status')).toHaveTextContent('Saved to ~/.omp/agent/config.yml');
    expect(screen.queryByRole('toolbar')).not.toBeInTheDocument();
  });

  it('refuses a stale preview, keeps the draft and offers to review again', async () => {
    const user = userEvent.setup();
    vi.mocked(ompExtensionsApi.preview).mockRejectedValue(
      new ApiError(409, 'changed', { code: 'omp_extensions_stale' }),
    );
    const invalidate = show(view());
    await user.click(await notesSwitch());
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    const dialog = await screen.findByRole('dialog', { name: 'Review changes — omp' });
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'The settings file changed after this view was read. Nothing was written.',
    );
    expect(within(dialog).queryByRole('button', { name: 'Apply changes' })).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Review again' }));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.ompExtensions('omp') });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(await notesSwitch()).toHaveAttribute('aria-checked', 'false');
    expect(screen.getByRole('toolbar', { name: '1 extension change' })).toBeInTheDocument();
  });

  it('drops a draft the refreshed view no longer offers a switch for', async () => {
    const user = userEvent.setup();
    vi.mocked(ompExtensionsApi.preview).mockRejectedValue(
      new ApiError(409, 'changed', { code: 'omp_extensions_stale' }),
    );
    show(view());
    await user.click(await notesSwitch());
    // The file changed under the draft: the row is now an explicit path, so the draft goes with it.
    vi.mocked(ompExtensionsApi.get).mockResolvedValue(
      view({
        revision: 'rev2',
        rows: view().rows.map((r) =>
          r.name === 'notes'
            ? { ...r, selectable: false, readOnlyReason: 'Now an explicit path.' }
            : r,
        ),
      }),
    );
    await user.click(screen.getByRole('button', { name: 'Review changes' }));
    await user.click(await screen.findByRole('button', { name: 'Review again' }));
    await waitFor(() => expect(screen.queryByRole('toolbar')).not.toBeInTheDocument());
    expect(screen.getByText('Now an explicit path.')).toBeInTheDocument();
    expect(
      screen.queryByRole('switch', { name: `Select ${dir}notes.ts in omp` }),
    ).not.toBeInTheDocument();
  });
});
