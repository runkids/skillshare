import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mcpApi, mcpTargets } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import MCPImportDialog from './MCPImportDialog';
import { MCPTargetOrder } from './targetOrder';

// CodeMirror needs a real layout engine; a textarea stands in for it
vi.mock('../CodeEditor', () => ({
  default: ({ value, onChange, ariaLabel }: { value: string; onChange: (v: string) => void; ariaLabel: string }) => <textarea aria-label={ariaLabel} value={value} onChange={(e) => onChange(e.target.value)} />,
}));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { import: vi.fn(), save: vi.fn(), preview: vi.fn(), render: vi.fn().mockResolvedValue({ rendered: [] }) } }));

const renderDialog = (props: Partial<Parameters<typeof MCPImportDialog>[0]> = {}, order: readonly string[] = mcpTargets) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPTargetOrder.Provider value={order}><MCPImportDialog source="target" servers={{ github: { command: 'npx' } }} defaultTargets={['claude', 'cursor']} paths={{ claude: '/home/me/.claude.json', codex: '/home/me/.codex/config.toml' }} detected={['claude']} onClose={vi.fn()} onImported={vi.fn()} {...props} /></MCPTargetOrder.Provider></ToastProvider></I18nProvider></QueryClientProvider>);

describe('MCP import dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks(); localStorage.clear();
    HTMLElement.prototype.scrollIntoView = vi.fn();
    vi.mocked(mcpApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    vi.mocked(mcpApi.preview).mockResolvedValue({ revision: '', sourcePath: '', blocked: false, changes: [] });
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

  // Refs: #289. A pasted snippet is a new server, so like the form it may have no Agent yet.
  it('adds a pasted server with no Agent selected as an explicit empty list', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server: { url: 'https://example.com/mcp' }, problems: [], warnings: [] }] });
    const user = userEvent.setup();
    renderDialog({ source: 'paste', defaultTargets: [] });
    await user.click(screen.getByLabelText('Server snippet'));
    await user.paste('{"mcpServers":{"docs":{"url":"https://example.com/mcp"}}}');
    expect(await screen.findByText('With no Agent selected, it is only kept in Skillshare, not written to any config file')).toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: 'Add 1 server' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({
      server: { url: 'https://example.com/mcp', targets: [] },
    })));
  });

  it('offers the Pi settings for one pasted server and saves the edits', async () => {
    const server = { command: 'docs', piOptions: { exposure: 'hidden', toolExposure: { 'get_*': 'direct' } } };
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server, problems: [], warnings: [] }] });
    const user = userEvent.setup();
    renderDialog({ source: 'paste', servers: {}, defaultTargets: ['pi'] });
    await user.click(screen.getByLabelText('Server snippet'));
    await user.paste('{"mcpServers":{"docs":{"command":"docs"}}}');
    const exposure = await screen.findByRole('combobox', { name: 'Tool exposure' });
    expect(exposure).toHaveTextContent('hidden');
    await user.click(exposure);
    await user.click(screen.getByRole('option', { name: /^direct\b/ }));
    const saved = { ...server, piOptions: { exposure: 'direct', toolExposure: { 'get_*': 'direct' } } };
    await waitFor(() => expect(mcpApi.render).toHaveBeenLastCalledWith(expect.objectContaining({ server: expect.objectContaining(saved) })));
    await user.click(screen.getByRole('button', { name: 'Add 1 server' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining(saved) })));
  });

  it('blocks adding a pasted server whose Pi settings are not valid', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server: { command: 'docs' }, problems: [], warnings: [] }] });
    const user = userEvent.setup();
    renderDialog({ source: 'paste', servers: {}, defaultTargets: ['pi'] });
    await user.click(screen.getByLabelText('Server snippet'));
    await user.paste('{"mcpServers":{"docs":{"command":"docs"}}}');
    await user.click(await screen.findByLabelText('Other Pi settings'));
    await user.paste('{"exposure":"loud"}');
    expect(screen.getByRole('button', { name: 'Add 1 server' })).toBeDisabled();
  });

  it('reports a file read failure and retains the snippet', async () => {
    const user = userEvent.setup();
    renderDialog({ source: 'paste' });
    await user.type(screen.getByLabelText('Server snippet'), 'previous snippet');
    const file = new File(['new snippet'], 'mcp.json', { type: 'application/json' });
    Object.defineProperty(file, 'text', { value: vi.fn().mockRejectedValue(new Error('File unreadable')) });
    await user.upload(screen.getByLabelText('Load a file'), file);
    expect(await screen.findByText('File unreadable')).toBeInTheDocument();
    expect(screen.getByLabelText('Server snippet')).toHaveValue('previous snippet');
  });

});
