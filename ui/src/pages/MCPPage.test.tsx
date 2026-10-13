import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { mcpApi } from '../api/mcp';
import { ToastProvider } from '../components/Toast';
import MCPPage from './MCPPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../api/mcp', async (load) => ({ ...await load<typeof import('../api/mcp')>(), mcpApi: { list: vi.fn(), preview: vi.fn(), import: vi.fn().mockResolvedValue({ candidates: [] }), save: vi.fn(), open: vi.fn().mockResolvedValue({ editor: 'code', path: '/.claude.json', pid: 1 }) } }));

describe('MCP page', () => {
  it("opens an Agent's config file by target, not by path", async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['claude'], servers: {} },
      projectConfigs: [], paths: { claude: '/.claude.json' }, detected: ['claude'], local: false, openable: ['claude'], projectPaths: {}, projectExisting: {}, previewError: '', backups: [], unmanaged: [], plan: null,
    } as Awaited<ReturnType<typeof mcpApi.list>>);
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPPage /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: /^Open in editor · Claude/ }));
    expect(mcpApi.open).toHaveBeenCalledWith('claude', {});
  });

  it("imports a project's conflicting entry from that project's Agent file", async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.list).mockResolvedValue({
      source: { path: '', configPath: '', targets: ['claude'], servers: {}, projects: { '/work/app': { targets: ['cursor'], servers: { docs: { command: 'npx' } } } } },
      projectConfigs: [], paths: { claude: '/.claude.json', cursor: '/.cursor/mcp.json' }, detected: ['claude', 'cursor'], local: false, openable: [], projectPaths: {}, projectExisting: {}, previewError: '', backups: [], unmanaged: [],
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
      projectConfigs: [], paths: { claude: '/.claude.json' }, detected: ['claude'], local: false, openable: ['claude'], projectPaths: {}, projectExisting: {}, previewError: '', backups: [], unmanaged: [], plan: null,
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
