import { fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, expect, it, vi } from 'vitest';
import PluginList from './PluginList';
import { pluginsApi, type PluginInventory } from '../../api/plugins';

vi.mock('../../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../../i18n', async (original) => ({
  ...(await original<typeof import('../../i18n')>()),
  useT: () => (key: string) => key,
}));
vi.mock('../../api/plugins', async (original) => ({
  ...(await original<typeof import('../../api/plugins')>()),
  pluginsApi: { discover: vi.fn() },
}));
beforeEach(() => vi.clearAllMocks());
const show = (inventory: PluginInventory, onAdd = vi.fn()) => {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <PluginList
        inventory={inventory}
        names={['demo']}
        busy={false}
        working=""
        onToggle={vi.fn()}
        onMenu={vi.fn()}
        onGroupMenu={vi.fn()}
        onAdd={onAdd}
        onBlocked={vi.fn()}
      />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole('button', { name: 'mcp.chooseAgents' }));
};
it('adds another Pi account from an explicit npm recipe without Git discovery', () => {
  const onAdd = vi.fn();
  show(
    {
      targetDefinitions: [
        { target: 'pi', label: 'Pi', project: true, operations: ['add'], npm: true },
        { target: 'pi-work', label: 'Pi work', project: false, operations: ['add'], npm: true },
      ],
      packages: { demo: { source: 'npm:demo@1.2.3', bindings: { pi: { id: '' } } } },
      hosts: [],
    },
    onAdd,
  );
  expect(pluginsApi.discover).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('checkbox', { name: 'Pi work' }));
  expect(onAdd).toHaveBeenCalledWith(
    { action: 'add', source: 'npm:demo@1.2.3', name: 'demo', targets: ['pi-work'] },
    'demo:pi-work',
  );
});
it('turns syncing of a Herdr mod that only Herdr can take off from its switch', () => {
  const onToggle = vi.fn();
  render(
    <QueryClientProvider client={new QueryClient()}>
      <PluginList
        inventory={{
          targetDefinitions: [
            { target: 'herdr-app', label: 'Herdr app', project: false, operations: ['add'] },
          ],
          packages: { demo: { bindings: { 'herdr-app': { id: 'demo' } } } },
          hosts: [],
        }}
        names={['demo']}
        kind="herdr"
        busy={false}
        working=""
        onToggle={onToggle}
        onMenu={vi.fn()}
        onGroupMenu={vi.fn()}
        onAdd={vi.fn()}
        onBlocked={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(screen.queryByRole('button', { name: 'mcp.chooseAgents' })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('switch', { name: 'plugins.targets Herdr app' }));
  expect(onToggle).toHaveBeenCalledWith('demo', 'herdr-app', false);
});
const repo = 'https://github.com/owner/pixel/tree/main';
const grouped = (onToggle = vi.fn(), onGroupMenu = vi.fn()) => {
  vi.mocked(pluginsApi.discover).mockResolvedValue({ source: repo, candidates: [] } as never);
  const mod = (sync?: boolean) => ({
    bindings: { 'herdr-app': { id: 'x', source: repo, ...(sync === false && { sync }) } },
  });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <PluginList
        inventory={{
          targetDefinitions: [
            { target: 'herdr-app', label: 'Herdr app', project: false, operations: ['add'] },
          ],
          packages: {
            'company-a': mod(),
            'company-b': mod(false),
            other: { bindings: { 'herdr-app': { id: 'other' } } },
          },
          hosts: [],
        }}
        names={['company-a', 'company-b', 'other']}
        kind="herdr"
        busy={false}
        working=""
        onToggle={onToggle}
        onMenu={vi.fn()}
        onGroupMenu={onGroupMenu}
        onAdd={vi.fn()}
        onBlocked={vi.fn()}
      />
    </QueryClientProvider>,
  );
};
it('opens a group to the switch of each package in it', () => {
  const onToggle = vi.fn();
  grouped(onToggle);
  fireEvent.click(screen.getByRole('button', { name: 'plugins.groupShow' }));
  expect(screen.getAllByRole('switch')).toHaveLength(3);
  fireEvent.click(screen.getAllByRole('switch')[1]);
  expect(onToggle).toHaveBeenCalledWith('company-b', 'herdr-app', true);
});
it('hands the group menu every package in the group', () => {
  const onGroupMenu = vi.fn();
  grouped(vi.fn(), onGroupMenu);
  fireEvent.click(screen.getAllByRole('button', { name: 'mcp.moreActions' })[0]);
  expect(onGroupMenu).toHaveBeenCalledWith(expect.anything(), ['company-a', 'company-b']);
});
