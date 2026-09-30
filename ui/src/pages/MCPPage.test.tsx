import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { mcpApi } from '../api/mcp';
import { ToastProvider } from '../components/Toast';
import MCPPage from './MCPPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../api/mcp', async (load) => ({ ...await load<typeof import('../api/mcp')>(), mcpApi: { list: vi.fn(), preview: vi.fn(), import: vi.fn().mockResolvedValue({ candidates: [] }), save: vi.fn() } }));

describe('MCP page', () => {
  it('includes removals from a previous Pi file in the global server list', async () => {
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['pi'], servers: {} },
      projectConfigs: [], paths: { pi: '/.pi/agent/mcp.json' }, detected: ['pi'], previewError: '', backups: [], unmanaged: [],
      plan: { revision: '', sourcePath: '', blocked: false, changes: [{ target: 'pi', path: '/.pi/agent/mcp-adapter.json', name: 'old', action: 'remove' }] },
    } as Awaited<ReturnType<typeof mcpApi.list>>);
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    expect(await screen.findByText('Removed from source')).toBeInTheDocument();
  });

  it('offers no check while there are no servers', async () => {
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['claude'], servers: {} },
      projectConfigs: [], paths: {}, detected: ['claude'], previewError: '', backups: [], unmanaged: [], plan: null,
    });
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    expect(await screen.findAllByRole('button', { name: /Import from/ })).not.toHaveLength(0);
    expect(screen.queryByRole('button', { name: 'Check' })).not.toBeInTheDocument();
  });

  it('keeps only the two ways to add in the header, and Check and Backups in the sync card', async () => {
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['claude'], servers: { docs: { command: 'npx' } } },
      projectConfigs: [], paths: { claude: '/.claude.json' }, detected: ['claude'], previewError: '', unmanaged: [],
      backups: [{ id: 'b1', target: 'claude', path: '/.claude.json' }],
      plan: { revision: '', sourcePath: '', blocked: false, changes: [{ target: 'claude', path: '/.claude.json', name: 'docs', action: 'unchanged' }] },
    } as Awaited<ReturnType<typeof mcpApi.list>>);
    const { container } = render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    const card = (await screen.findByRole('heading', { name: 'Sync' })).closest('.ss-box') as HTMLElement;
    const header = container.querySelector('[data-tour="mcp-actions"]') as HTMLElement;
    expect(within(header).getAllByRole('button').map((b) => b.textContent)).toEqual(['Import from a target', 'Add server']);
    expect(within(card).getByRole('button', { name: 'Check' })).toBeInTheDocument();
    expect(within(card).getByRole('button', { name: 'Backups and restore' })).toBeInTheDocument();
  });

  it('previews an adapter-file removal even when builtin owns the Pi path label', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['pi'], servers: { native: { command: 'n' }, X: { command: 'x' } } },
      projectConfigs: [], paths: { pi: '/.pi/agent/mcp.json' }, detected: ['pi'], previewError: '', backups: [], unmanaged: [], plan: null,
    });
    vi.mocked(mcpApi.preview).mockResolvedValue({ revision: '', sourcePath: '', blocked: false, changes: [
      { target: 'pi', path: '/.pi/agent/mcp-adapter.json', name: 'X', action: 'remove' },
      { target: 'pi', path: '/work/project/.pi/mcp.json', root: '/work/project', name: 'X', action: 'unchanged' },
    ] });
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: 'More actions for X' }));
    await user.click(screen.getByRole('menuitem', { name: 'Remove' }));
    const dialog = await screen.findByRole('dialog', { name: 'Remove X?' });
    expect(await within(dialog).findByText('/.pi/agent/mcp-adapter.json')).toBeInTheDocument();
    expect(within(dialog).queryByText('/work/project/.pi/mcp.json')).not.toBeInTheDocument();
  });

  it("imports a project's conflicting entry from that project's Agent file", async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['claude'], servers: {}, projects: { '/work/app': { targets: ['cursor'], servers: { docs: { command: 'npx' } } } } },
      projectConfigs: [], paths: { claude: '/.claude.json', cursor: '/.cursor/mcp.json' }, detected: ['claude', 'cursor'], previewError: '', backups: [], unmanaged: [],
      plan: { revision: '', sourcePath: '', blocked: true, changes: [{ target: 'cursor', path: '/work/app/.cursor/mcp.json', name: 'docs', root: '/work/app', action: 'conflict', message: 'existing entry is not managed; import it to explicitly adopt it' }] },
    } as Awaited<ReturnType<typeof mcpApi.list>>);
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: 'Import from Cursor' }));
    await waitFor(() => expect(mcpApi.import).toHaveBeenCalledWith({ from: 'cursor', root: '/work/app' }));
  });

  // Each save sends the revision it previewed; a second one racing it would be refused.
  it('holds the other toggles while a save is on its way', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['claude'], servers: { a: { command: 'npx' }, b: { command: 'npx' } } },
      projectConfigs: [], paths: { claude: '/.claude.json' }, detected: ['claude'], previewError: '', backups: [], unmanaged: [], plan: null,
    } as Awaited<ReturnType<typeof mcpApi.list>>);
    vi.mocked(mcpApi.save).mockReturnValue(new Promise(() => {}));
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: 'Choose which agents get a' }));
    await user.click(screen.getByRole('button', { name: 'Choose which agents get b' }));
    const [first, second] = screen.getAllByRole('checkbox', { name: /Claude/ });
    await user.click(first);
    expect(second).toBeDisabled();
  });
});
