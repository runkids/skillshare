import { fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, expect, it, vi } from 'vitest';
import PluginList from './PluginList';
import { pluginsApi, type PluginInventory } from '../../api/plugins';

vi.mock('../../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../../i18n', async (original) => ({ ...await original<typeof import('../../i18n')>(), useT: () => (key: string) => key }));
vi.mock('../../api/plugins', async (original) => ({ ...await original<typeof import('../../api/plugins')>(), pluginsApi: { discover: vi.fn() } }));
beforeEach(() => vi.clearAllMocks());
const show = (inventory: PluginInventory, onAdd = vi.fn()) => {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <PluginList inventory={inventory} names={['demo']} busy={false} working="" onToggle={vi.fn()} onMenu={vi.fn()} onAdd={onAdd} onBlocked={vi.fn()} />
  </QueryClientProvider>);
  fireEvent.click(screen.getByRole('button', { name: 'mcp.chooseAgents' }));
};
it('adds another Pi account from an explicit npm recipe without Git discovery', () => {
  const onAdd = vi.fn();
  show({ targetDefinitions: [
    { target: 'pi', label: 'Pi', project: true, operations: ['add'], npm: true },
    { target: 'pi-work', label: 'Pi work', project: false, operations: ['add'], npm: true },
  ], packages: { demo: { source: 'npm:demo@1.2.3', bindings: { pi: { id: '' } } } }, hosts: [] }, onAdd);
  expect(pluginsApi.discover).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('checkbox', { name: 'Pi work' }));
  expect(onAdd).toHaveBeenCalledWith({ action: 'add', source: 'npm:demo@1.2.3', name: 'demo', targets: ['pi-work'] }, 'demo:pi-work');
});
