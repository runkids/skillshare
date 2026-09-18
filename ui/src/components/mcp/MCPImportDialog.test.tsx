import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mcpApi } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import MCPImportDialog from './MCPImportDialog';

// CodeMirror needs a real layout engine; a textarea stands in for it
vi.mock('../CodeEditor', () => ({
  default: ({ value, onChange, ariaLabel }: { value: string; onChange: (v: string) => void; ariaLabel: string }) => <textarea aria-label={ariaLabel} value={value} onChange={(e) => onChange(e.target.value)} />,
}));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { import: vi.fn(), save: vi.fn() } }));

const renderDialog = (props: Partial<Parameters<typeof MCPImportDialog>[0]> = {}) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPImportDialog source="target" servers={{ github: { command: 'npx' } }} defaultTargets={['claude', 'cursor']} paths={{ claude: '/home/me/.claude.json', codex: '/home/me/.codex/config.toml' }} detected={['claude']} onClose={vi.fn()} onImported={vi.fn()} {...props} /></ToastProvider></I18nProvider></QueryClientProvider>);

describe('MCP import dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks(); localStorage.clear();
    HTMLElement.prototype.scrollIntoView = vi.fn();
    vi.mocked(mcpApi.save).mockResolvedValue({ applied: [], backupIds: [] });
  });

  it('requires a Pi extension when importing into Pi', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server: { url: 'https://example.com/mcp' }, problems: [], warnings: [], from: 'pi' }] });
    const user = userEvent.setup();
    renderDialog({ servers: {}, defaultTargets: ['pi'], paths: { pi: '/home/me/.pi/agent/mcp.json' }, detected: ['pi'] });
    expect(await screen.findByRole('button', { name: 'Import 1 server' })).toBeDisabled();
    await user.click(screen.getByRole('combobox', { name: 'MCP extension installed in Pi' }));
    await user.click(screen.getByRole('option', { name: 'pi-mcp-adapter' }));
    await user.click(screen.getByRole('button', { name: 'Import 1 server' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://example.com/mcp', piExtension: 'pi-mcp-adapter' }, resolutions: [{ target: 'pi', name: 'docs', action: 'adopt' }] })));
  });

  it('imports every new server from a target and adopts the entries that target keeps', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [
      { name: 'sentry', server: { url: 'https://mcp.sentry.dev/mcp' }, problems: [], warnings: [], from: 'claude' },
      { name: 'github', server: { command: 'npx' }, problems: [], warnings: [], from: 'claude' },
    ] });
    const user = userEvent.setup();
    const imported = vi.fn();
    renderDialog({ onImported: imported });
    expect(await screen.findByText('Already added')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Import 1 server' }));
    await waitFor(() => expect(imported).toHaveBeenCalled());
    expect(mcpApi.import).toHaveBeenCalledWith({ from: 'claude' });
    expect(mcpApi.save).toHaveBeenCalledOnce();
    expect(mcpApi.save).toHaveBeenCalledWith({
      name: 'sentry', server: { url: 'https://mcp.sentry.dev/mcp' }, replace: false,
      resolutions: [{ target: 'claude', name: 'sentry', action: 'adopt' }],
    });
  });

  it('lets a conflicting entry replace the source server of the same name', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'github', server: { command: 'uvx' }, problems: [], warnings: [], from: 'claude' }] });
    const user = userEvent.setup();
    renderDialog({ conflict: { target: 'claude', name: 'github' } });
    await user.click(await screen.findByRole('button', { name: 'Import 1 server' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ name: 'github', replace: true })));
  });

  it('needs a target picked before importing when nothing is inherited', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'sentry', server: { url: 'https://mcp.sentry.dev/mcp' }, problems: [], warnings: [], from: 'claude' }] });
    const user = userEvent.setup();
    renderDialog({ defaultTargets: [] });
    const button = await screen.findByRole('button', { name: 'Import 1 server' });
    expect(button).toBeDisabled();
    expect(screen.getByText('Pick at least one target.')).toBeInTheDocument();
    await user.click(screen.getByRole('checkbox', { name: 'Codex' }));
    await user.click(button);
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({
      server: { url: 'https://mcp.sentry.dev/mcp', targets: ['codex'] },
    })));
  });

  it('asks which client pasted TOML comes from', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server: { url: 'https://example.com/mcp' }, problems: [], warnings: [] }] });
    const user = userEvent.setup();
    renderDialog({ source: 'paste' });
    await user.click(screen.getByLabelText('Server snippet'));
    await user.paste('[mcp_servers.docs]\nurl = "https://example.com/mcp"');
    expect(await screen.findByText('TOML detected · 1 server')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'grok' }));
    await waitFor(() => expect(mcpApi.import).toHaveBeenLastCalledWith(expect.objectContaining({ from: 'grok' })));
  });

  it('reads a picked file into the snippet editor', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server: { url: 'https://example.com/mcp' }, problems: [], warnings: [] }] });
    const user = userEvent.setup();
    renderDialog({ source: 'paste' });
    const snippet = '{"mcpServers":{"docs":{"url":"https://example.com/mcp"}}}';
    await user.upload(screen.getByLabelText('Load a file'), new File([snippet], 'mcp.json', { type: 'application/json' }));
    await waitFor(() => expect(screen.getByLabelText('Server snippet')).toHaveValue(snippet));
    await waitFor(() => expect(mcpApi.import).toHaveBeenCalledWith({ content: snippet }));
  });
});
