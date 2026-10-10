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

function mount(kind: 'skill' | 'agent' = 'skill') {
  render(
    <MemoryRouter initialEntries={['/skills']}>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <I18nProvider><ToastProvider><ResourcesPage kind={kind} /></ToastProvider></I18nProvider>
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

  it('offers to move a plain folder whole, but not a tracked repo', async () => {
    mount();
    fireEvent.contextMenu(await row('local'));
    expect(await screen.findByRole('menuitem', { name: 'Move whole folder…' })).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });
    fireEvent.contextMenu(await row('repo'));
    expect(await screen.findByRole('menuitem', { name: 'Update repo' })).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', { name: 'Move whole folder…' })).toBeNull();
  });

  it('sets targets for a folder inside a tracked repo', async () => {
    const user = userEvent.setup();
    mount();
    await user.click(await row('plugins/demo/skills'));
    await user.click(screen.getByRole('button', { name: 'Set targets' }));
    await user.click(within(await screen.findByRole('menu')).getByRole('menuitem', { name: 'claude' }));
    await waitFor(() => expect(api.batchSetTargets).toHaveBeenCalledWith('_repo/plugins/demo/skills', 'claude'));
  });

});

describe('Move to folder', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [at('local/one'), at('_repo/skills/gamma')] } as Awaited<ReturnType<typeof api.listSkills>>);
    vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as unknown as Awaited<ReturnType<typeof api.diff>>);
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 0 });
    vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [] } as unknown as Awaited<ReturnType<typeof api.getSyncMatrix>>);
  });

  it('opens from the selection bar with the skills that can move', async () => {
    mount();
    fireEvent.click(await screen.findByRole('checkbox', { name: 'one' }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'gamma' }));
    fireEvent.click(screen.getByRole('button', { name: 'Move to folder…' }));
    const dialog = await screen.findByRole('dialog', { name: 'Move to folder' });
    expect(within(dialog).getByText('one')).toBeInTheDocument();
    expect(within(dialog).queryByText('gamma')).toBeNull();
    expect(within(dialog).getByText(/1 selected items stay where they are/)).toBeInTheDocument();
  });

  it('stays disabled while only a tracked-repo skill is selected', async () => {
    mount();
    fireEvent.click(await screen.findByRole('checkbox', { name: 'gamma' }));
    expect(screen.getByRole('button', { name: 'Move to folder…' })).toBeDisabled();
  });
});

/** Link folder sits in the menu beside Install. */
async function openLinkFolder() {
  fireEvent.click(await screen.findByRole('button', { name: 'More ways to add' }));
  fireEvent.mouseDown(await screen.findByRole('menuitem', { name: /^Link folder/ }));
}

describe('Link folder', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [] });
    vi.mocked(api.getConfig).mockResolvedValue({ config: { FollowSourceLinks: false }, raw: '' });
    vi.mocked(api.createSourceLink).mockResolvedValue({ path: '/source/_team', target: '/work/team', kind: 'symlink', warning: '' });
  });

  it('posts the path, optional name and explicit enable flag and refreshes the list', async () => {
    mount();
    await openLinkFolder();
    fireEvent.change(screen.getByLabelText('Folder path'), { target: { value: '/work/team' } });
    fireEvent.change(screen.getByLabelText('Link name (optional)'), { target: { value: '_team' } });
    fireEvent.click(await screen.findByRole('checkbox', { name: /Enable follow_source_links/ }));
    const calls = vi.mocked(api.listSkills).mock.calls.length;
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Link folder' }));
    await waitFor(() => expect(api.createSourceLink).toHaveBeenCalledWith({ path: '/work/team', name: '_team', enable: true }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(vi.mocked(api.listSkills).mock.calls.length).toBeGreaterThan(calls);
  });

});

/* -- Folders ------------------------------------- */

describe('Unlink folder', () => {
  const LINKED = [
    at('team/plugins/skills/alpha', { isInRepo: false, linkName: 'team', linkTarget: '/work/team' }),
    at('plain/plugins/skills/beta', { isInRepo: false }),
  ];
  const linkedHeader = () => [...document.querySelectorAll<HTMLElement>('.ss-gh, .ss-gl')].find((g) => g.querySelector('b')?.textContent === 'team')!;
  const openUnlink = async () => {
    await screen.findByText('/work/team');
    const user = userEvent.setup();
    await user.click(within(linkedHeader()).getByRole('button', { name: 'Unlink' }));
    return screen.getByRole('dialog', { name: 'Unlink team?' });
  };

  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: LINKED });
    vi.mocked(api.removeSourceLink).mockResolvedValue({ success: true, name: 'team' });
    vi.mocked(api.batchUninstall).mockResolvedValue({ results: [], summary: { succeeded: 1, failed: 0 } });
    vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as unknown as Awaited<ReturnType<typeof api.diff>>);
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 0 });
    vi.mocked(api.listTrash).mockResolvedValue({ items: [] } as unknown as Awaited<ReturnType<typeof api.listTrash>>);
    vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [] } as unknown as Awaited<ReturnType<typeof api.getSyncMatrix>>);
  });

  it.each(['list', 'cards', 'tree'])('unlinks an empty standalone link in %s view', async (view) => {
    localStorage.setItem('skillshare:skills-view', view);
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [], sourceLinks: [{ name: 'team', target: '/work/team', available: true }] });
    mount();
    await screen.findByText('/work/team');
    const header = view === 'tree' ? await row('team') : linkedHeader();
    if (view === 'tree') fireEvent.click(header);
    expect(within(header).getByText('0 skills')).toBeInTheDocument();
    fireEvent.click(within(header).getByRole('button', { name: 'Unlink' }));
    const dialog = screen.getByRole('dialog', { name: 'Unlink team?' });
    expect(within(dialog).getByText('0 skills')).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Unlink' }));
    await waitFor(() => expect(api.removeSourceLink).toHaveBeenCalledWith('team'));
    expect(api.batchUninstall).not.toHaveBeenCalled();
  });

  it('opens unlink confirmation from the icon and preserves plain repo actions', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [{ ...LINKED[0], isInRepo: true }, at('_repo/skills/gamma')] });
    mount();
    await screen.findByText('/work/team');
    const unlink = within(linkedHeader()).getByRole('button', { name: 'Unlink' });
    await user.hover(unlink);
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Unlink');
    await user.click(unlink);
    expect(screen.getByRole('dialog', { name: 'Unlink team?' })).toBeInTheDocument();
    expect(screen.queryByRole('menu')).toBeNull();
    expect(api.removeSourceLink).not.toHaveBeenCalled();
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }));
    const plain = [...document.querySelectorAll<HTMLElement>('.ss-gh')].find((g) => g.querySelector('b')?.textContent === 'repo')!;
    await user.click(within(plain).getByRole('button', { name: 'Repo actions' }));
    expect(screen.getByRole('menuitem', { name: 'Update repo' })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: 'Uninstall repo' })).toBeInTheDocument();
  });

  it('confirms the target, affected skills and trash recovery before deleting', async () => {
    mount();
    const dialog = await openUnlink();
    expect(within(dialog).getByText('Target')).toBeInTheDocument();
    expect(within(dialog).getByText('/work/team')).toBeInTheDocument();
    expect(within(dialog).getByText('Skills')).toBeInTheDocument();
    expect(within(dialog).getByText('alpha')).toBeInTheDocument();
    expect(within(dialog).getByText('These skills leave every target on the next sync.')).toBeInTheDocument();
    expect(within(dialog).getByText('The link is removed from the skills source, and the folder it points at is not touched.')).toBeInTheDocument();
    expect(within(dialog).getByText('The link goes to trash and can be restored from the Trash page.')).toBeInTheDocument();
    expect(api.removeSourceLink).not.toHaveBeenCalled();
    const calls = vi.mocked(api.listSkills).mock.calls.length;
    fireEvent.click(within(dialog).getByRole('button', { name: 'Unlink' }));
    await waitFor(() => expect(api.removeSourceLink).toHaveBeenCalledWith('team'));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(vi.mocked(api.listSkills).mock.calls.length).toBeGreaterThan(calls);
    expect(screen.getByText('Unlinked team')).toBeInTheDocument();
  });

  it.each(['actions', 'right-click', 'cards actions', 'cards right-click', 'tree right-click'])('confirms single linked skill uninstall from %s and sends only that skill name', async (action) => {
    const user = userEvent.setup();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [
      { ...LINKED[0], isInRepo: true },
      at('team/skills/second', { isInRepo: true, linkName: 'team', linkTarget: '/work/team' }),
    ] });
    if (action === 'tree right-click') localStorage.setItem('skillshare:skills-view', 'tree');
    else if (action.startsWith('cards')) localStorage.setItem('skillshare:skills-view', 'cards');
    mount();
    if (action === 'tree right-click') fireEvent.contextMenu(await row('alpha'));
    else {
      const item = (await screen.findByRole('link', { name: 'alpha' })).closest('.ss-r, .ss-tile')!;
      if (action.includes('right-click')) fireEvent.contextMenu(item);
      else await user.click(within(item as HTMLElement).getByRole('button', { name: 'Actions' }));
    }
    expect(screen.queryByRole('menuitem', { name: 'Uninstall repo' })).toBeNull();
    await user.click(screen.getByRole('menuitem', { name: 'Uninstall' }));
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Selected skills are moved out of the linked folder to the trash.')).toBeInTheDocument();
    expect(within(dialog).getByText('alpha')).toBeInTheDocument();
    expect(within(dialog).queryByText('second')).toBeNull();
    expect(api.batchUninstall).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole('button', { name: 'Uninstall' }));
    await waitFor(() => expect(api.batchUninstall).toHaveBeenCalledWith({ names: ['team__plugins__skills__alpha'], kind: 'skill', force: false }));
  });

  it('preserves whole-repo uninstall confirmation for a plain tracked skill', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [at('_repo/skills/gamma'), at('_repo/skills/other')] });
    mount();
    const item = (await screen.findByRole('link', { name: 'gamma' })).closest('.ss-r')!;
    await user.click(within(item as HTMLElement).getByRole('button', { name: 'Actions' }));
    await user.click(screen.getByRole('menuitem', { name: 'Uninstall repo' }));
    expect(within(screen.getByRole('dialog')).getByText(/Whole repo/)).toBeInTheDocument();
    expect(api.batchUninstall).not.toHaveBeenCalled();
  });

});
