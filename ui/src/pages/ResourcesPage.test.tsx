import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import type { Skill, SyncMatrixEntry } from '../api/client';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import { byTargetOrProject, splitTargets, syncedByTarget } from '../lib/resourceGrouping';
import ResourcesPage from './ResourcesPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  // Anything the page asks for that a test does not set up answers with an empty object.
  const stubs: Record<string | symbol, unknown> = {};
  return { ...actual, api: new Proxy(stubs, { get: (t, k) => (t[k] ??= vi.fn().mockResolvedValue({})) }) };
});

const skill = (flatName: string): Skill => ({
  name: flatName, kind: 'skill', flatName, relPath: flatName, sourcePath: '', isInRepo: false,
});

const entry = (s: string, target: string, status: SyncMatrixEntry['status']): SyncMatrixEntry =>
  ({ skill: s, target, status, reason: '' });

describe('splitTargets', () => {
  it('groups project targets by project and keeps global tools apart', () => {
    expect(splitTargets(['blog@claude', 'claude', 'codex', 'myapp@claude', 'myapp@codex'])).toEqual({
      global: ['claude', 'codex'],
      projects: [['blog', ['claude']], ['myapp', ['claude', 'codex']]],
    });
  });
});

describe('byTargetOrProject', () => {
  it('merges a project\'s tools into one entry and keeps global targets apart', () => {
    const index = byTargetOrProject(new Map([
      ['claude', new Set(['a'])],
      ['blog@claude', new Set(['a'])],
      ['blog@codex', new Set(['b'])],
    ]));

    expect([...index].map(([k, v]) => [k, [...v]])).toEqual([['claude', ['a']], ['blog@', ['a', 'b']]]);
  });
});

describe('syncedByTarget', () => {
  it('indexes only entries that are synced and belong to the given items', () => {
    const index = syncedByTarget([skill('a'), skill('b')], [
      entry('a', 'claude', 'synced'),
      entry('b', 'claude', 'synced'),
      entry('a', 'cursor', 'not_included'),
      entry('gone', 'codex', 'synced'),
    ]);

    expect([...index.keys()]).toEqual(['claude']);
    expect([...index.get('claude')!]).toEqual(['a', 'b']);
  });
});

/* -- Tree view ----------------------------------- */

const at = (relPath: string, extra: Partial<Skill> = {}): Skill => ({
  name: relPath.split('/').pop()!, kind: 'skill', flatName: relPath.replace(/\//g, '__'), relPath, sourcePath: '',
  isInRepo: relPath.startsWith('_'), ...extra,
});

const SKILLS = [
  at('_repo/plugins/demo/skills/alpha'),
  at('_repo/plugins/demo/skills/beta'),
  at('_repo/skills/gamma'),
  at('local/one'),
  at('local/two', { disabled: true }),
];

function mount() {
  render(
    <MemoryRouter initialEntries={['/skills']}>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <I18nProvider><ToastProvider><ResourcesPage kind="skill" /></ToastProvider></I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

/** The tree row whose name reads `name` ("plugins/demo/skills" for a merged row). */
async function row(name: string) {
  await screen.findAllByRole('treeitem');
  const found = screen.getAllByRole('treeitem').find((el) => el.querySelector('.nm')?.textContent === name);
  if (!found) throw new Error(`no tree row ${name}`);
  return found;
}

describe('Skills tree view', () => {
  beforeEach(() => {
    localStorage.clear();
    localStorage.setItem('skillshare:skills-view', 'tree');
    vi.clearAllMocks();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: SKILLS } as Awaited<ReturnType<typeof api.listSkills>>);
    vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as unknown as Awaited<ReturnType<typeof api.diff>>);
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 0 });
    vi.mocked(api.listTrash).mockResolvedValue({ items: [] } as unknown as Awaited<ReturnType<typeof api.listTrash>>);
    vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [] } as unknown as Awaited<ReturnType<typeof api.getSyncMatrix>>);
    vi.mocked(api.batchToggleResources).mockResolvedValue({ results: [], summary: { updated: 1, unchanged: 0, failed: 0 } } as Awaited<ReturnType<typeof api.batchToggleResources>>);
    vi.mocked(api.batchSetTargets).mockResolvedValue({ updated: 2, skipped: 0, errors: [] });
    vi.mocked(api.availableTargets).mockResolvedValue({ targets: [{ name: 'claude', installed: true }] } as Awaited<ReturnType<typeof api.availableTargets>>);
  });

  it('turns every skill of a partly enabled folder on from its switch', async () => {
    mount();
    fireEvent.click(await row('local'));
    fireEvent.click(screen.getByRole('switch', { name: 'local' }));
    await waitFor(() => expect(api.batchToggleResources).toHaveBeenCalledWith(['local__one', 'local__two'], true, 'skill'));
  });

  it('turns every skill of a fully enabled folder off, without asking first', async () => {
    mount();
    fireEvent.click(await row('repo'));
    const sw = screen.getByRole('switch', { name: 'repo' });
    expect(sw).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(sw);
    await waitFor(() => expect(api.batchToggleResources).toHaveBeenCalledWith(
      ['_repo__plugins__demo__skills__alpha', '_repo__plugins__demo__skills__beta', '_repo__skills__gamma'], false, 'skill',
    ));
  });

  it('shows only the latest toast after quick off/on clicks', async () => {
    mount();
    fireEvent.click(await row('repo'));
    const sw = screen.getByRole('switch', { name: 'repo' });
    fireEvent.click(sw);
    await waitFor(() => expect(sw).not.toBeDisabled());
    fireEvent.click(sw);
    await waitFor(() => expect(api.batchToggleResources).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(sw).not.toBeDisabled());
    const toasts = [...document.querySelectorAll('[data-toast-container] .ss-toast')].map((el) => el.textContent);
    const lastEnable = vi.mocked(api.batchToggleResources).mock.calls[1][1];
    expect(toasts).toEqual([expect.stringMatching(lastEnable ? /^Enabled/ : /^Disabled/)]);
  });

  it('marks a disabled skill with its icon instead of hover text', async () => {
    mount();
    expect(within(await row('two')).getByRole('img', { name: 'Disabled' })).toBeInTheDocument();
    expect(within(await row('one')).queryByRole('img', { name: 'Disabled' })).toBeNull();
    expect(within(await row('two')).queryByText('Disabled', { ignore: 'title' })).toBeNull();
  });

  it('collapses every folder from one button, which then expands them again', async () => {
    mount();
    await row('one');
    fireEvent.click(screen.getByRole('button', { name: 'Collapse all' }));
    expect(screen.getAllByRole('treeitem').map((el) => el.querySelector('.nm')?.textContent)).toEqual(['repo', 'local']);
    fireEvent.click(screen.getByRole('button', { name: 'Expand all' }));
    expect(await row('one')).toBeInTheDocument();
  });

  it('keeps a plain folder name as is in the detail pane path', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [at('notes__2024/deep/x'), at('notes__2024/deep/y')] } as Awaited<ReturnType<typeof api.listSkills>>);
    mount();
    fireEvent.click(await row('x'));
    expect(await screen.findByText('notes__2024 / deep /')).toBeInTheDocument();
  });

  it('selects the visible range on Shift-click', async () => {
    mount();
    fireEvent.click(await row('gamma'));
    fireEvent.click(await row('one'), { shiftKey: true });
    expect(screen.getAllByRole('treeitem', { selected: true }).map((el) => el.querySelector('.nm')?.textContent)).toEqual(['gamma', 'local', 'one']);
    expect(screen.getByRole('heading', { name: '3 skills selected' })).toBeInTheDocument();
  });

  it('sets targets for a folder inside a tracked repo', async () => {
    const user = userEvent.setup();
    mount();
    await user.click(await row('plugins/demo/skills'));
    await user.click(screen.getByRole('button', { name: 'Set targets' }));
    await user.click(within(await screen.findByRole('menu')).getByRole('menuitem', { name: 'claude' }));
    await waitFor(() => expect(api.batchSetTargets).toHaveBeenCalledWith('_repo/plugins/demo/skills', 'claude'));
  });

  it('remembers the tree width set with the divider', async () => {
    mount();
    const divider = await screen.findByRole('separator');
    fireEvent.keyDown(divider, { key: 'ArrowRight' });
    expect(divider).toHaveAttribute('aria-valuenow', '396');
    expect(localStorage.getItem('skillshare:tree-width')).toBe('396');
  });
});

/* -- Folders ------------------------------------- */

describe('Skills list folders', () => {
  const FOLDERED = [at('frontend/react/hooks'), at('frontend/react/router'), at('frontend/vue'), at('solo'), at('_repo/skills/gamma')];

  beforeEach(() => {
    localStorage.clear();
    localStorage.setItem('skillshare:skills-view', 'list');
    vi.clearAllMocks();
    HTMLElement.prototype.scrollIntoView = vi.fn();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: FOLDERED } as Awaited<ReturnType<typeof api.listSkills>>);
    vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as unknown as Awaited<ReturnType<typeof api.diff>>);
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 0 });
    vi.mocked(api.listTrash).mockResolvedValue({ items: [] } as unknown as Awaited<ReturnType<typeof api.listTrash>>);
    vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [] } as unknown as Awaited<ReturnType<typeof api.getSyncMatrix>>);
  });

  /** Opens the toolbar select whose prefix reads `prefix` and picks `option`. */
  async function choose(prefix: string, option: string) {
    const user = userEvent.setup();
    const box = (await screen.findAllByRole('combobox')).find((el) => el.textContent?.startsWith(prefix));
    if (!box) throw new Error(`no ${prefix} select`);
    await user.click(box);
    await user.click(await screen.findByRole('option', { name: option }));
  }

  const names = () => [...document.querySelectorAll('.ss-r .nm')].map((el) => el.textContent);

  it('shows only the chosen folder\'s items', async () => {
    mount();
    await choose('Folder', 'frontend/react (2)');
    expect(names()).toEqual(['hooks', 'router']);
  });

  it('shows an unset filter by its name alone', async () => {
    mount();
    expect((await screen.findAllByRole('combobox')).map((el) => el.textContent)).toEqual(['Source', 'Status', 'Folder']);
  });

  it('clears a filter from its chip', async () => {
    mount();
    await choose('Folder', 'frontend/react (2)');
    fireEvent.click(screen.getByRole('button', { name: 'Clear Folder' }));
    expect(names()).toHaveLength(FOLDERED.length);
  });

  it('groups by folder with the root first, then folders A to Z', async () => {
    const user = userEvent.setup();
    mount();
    await user.click(await screen.findByRole('button', { name: 'Group and sort' }));
    await user.click(screen.getByRole('menuitemradio', { name: 'Folder' }));
    expect([...document.querySelectorAll('.ss-gh b')].map((el) => el.textContent)).toEqual(['Root', 'frontend', 'frontend/react', 'repo']);
  });
});
