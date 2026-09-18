import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PluginsPage from './PluginsPage';
import { pluginsApi } from '../api/plugins';

vi.mock('../api/plugins', async (importOriginal) => ({ ...await importOriginal<typeof import('../api/plugins')>(), pluginsApi: { list: vi.fn(), preview: vi.fn(), apply: vi.fn() } }));
vi.mock('../i18n', () => ({ useT: () => (key: string) => key }));
vi.mock('../components/plugins/PluginAddDialog', () => ({ default: () => null }));

function mount() { return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><PluginsPage /></QueryClientProvider>); }

describe('PluginsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(pluginsApi.list).mockResolvedValue({ packages: { demo: { bindings: { codex: { id: 'demo@market' } } } }, hosts: [{ target: 'codex', version: '0.154', installed: [{ id: 'demo@market', enabled: false }] }] });
    vi.mocked(pluginsApi.preview).mockResolvedValue({ revision: 'reviewed', blocked: false, changes: [{ name: 'demo', target: 'codex', id: 'demo@market', action: 'selection' }] });
    vi.mocked(pluginsApi.apply).mockResolvedValue({ result: { results: [] }, failure: '' });
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
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.sync' }));
    await screen.findByRole('dialog');
    expect(pluginsApi.apply).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.apply' }));
    expect(await screen.findByText('Native authentication required')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('One target failed');
  });
  it('previews a sync for one agent from its own menu', async () => {
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'mcp.chooseAgents' }));
    fireEvent.click(screen.getAllByRole('button', { name: 'mcp.moreActions' })[1]);
    fireEvent.mouseDown(screen.getByRole('menuitem', { name: 'plugins.sync' }));
    await waitFor(() => expect(pluginsApi.preview).toHaveBeenCalledWith({ action: 'sync', name: 'demo', targets: ['codex'] }));
  });
});
