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
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { import: vi.fn(), save: vi.fn(), render: vi.fn().mockResolvedValue({ rendered: [] }) } }));

const renderDialog = (props: Partial<Parameters<typeof MCPImportDialog>[0]> = {}, order: readonly string[] = mcpTargets) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPTargetOrder.Provider value={order}><MCPImportDialog source="target" servers={{ github: { command: 'npx' } }} defaultTargets={['claude', 'cursor']} paths={{ claude: '/home/me/.claude.json', codex: '/home/me/.codex/config.toml' }} detected={['claude']} onClose={vi.fn()} onImported={vi.fn()} {...props} /></MCPTargetOrder.Provider></ToastProvider></I18nProvider></QueryClientProvider>);

describe('MCP import dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks(); localStorage.clear();
    HTMLElement.prototype.scrollIntoView = vi.fn();
    vi.mocked(mcpApi.save).mockResolvedValue({ applied: [], backupIds: [] });
  });

  it("reads each of Pi's files as picked and resets selections between files", async () => {
    vi.mocked(mcpApi.import).mockImplementation(async (body) => ({ candidates: [{ name: 'docs', server: { command: body.piExtension === 'pi-mcp-adapter' ? 'adapter' : 'native' }, problems: [], warnings: [], from: 'pi' }] }));
    const user = userEvent.setup();
    renderDialog({ servers: {}, defaultTargets: ['pi'], paths: { pi: '/project/.pi/mcp.json' }, project: '/project', detected: ['pi'], importSources: [
      { target: 'pi', path: '/project/.pi/mcp.json', piExtension: 'builtin' },
      { target: 'pi', path: '/project/.pi/mcp-adapter.json', piExtension: 'pi-mcp-adapter' },
    ] });
    await waitFor(() => expect(mcpApi.import).toHaveBeenCalledWith({ from: 'pi', root: '/project', piExtension: 'builtin' }));
    await user.click(await screen.findByRole('checkbox', { name: /docs/ }));
    await user.click(screen.getByRole('combobox', { name: 'Target' }));
    expect(screen.getByRole('option', { name: 'Pi /project/.pi/mcp.json' })).toBeInTheDocument();
    await user.click(screen.getByRole('option', { name: 'Pi /project/.pi/mcp-adapter.json' }));
    await waitFor(() => expect(mcpApi.import).toHaveBeenLastCalledWith({ from: 'pi', root: '/project', piExtension: 'pi-mcp-adapter' }));
    await user.click(await screen.findByRole('button', { name: 'Import 1 server' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ project: '/project', server: { command: 'adapter' }, resolutions: [{ target: 'pi', name: 'docs', action: 'adopt' }] })));
  });

  it('retains builtin custom settings and previews the project destination', async () => {
    const server = { command: 'docs', piOptions: { exposure: 'hidden', toolExposure: { 'get_*': 'direct' }, custom: { flag: true } } };
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server, problems: [], warnings: [], from: 'pi' }] });
    const user = userEvent.setup();
    renderDialog({ servers: {}, defaultTargets: ['pi'], paths: { pi: '/project/.pi/mcp.json' }, project: '/project', detected: ['pi'] });
    const add = await screen.findByRole('button', { name: 'Import 1 server' });
    expect(add).toBeEnabled();
    await waitFor(() => expect(mcpApi.render).toHaveBeenCalledWith(expect.objectContaining({ project: '/project', server: expect.objectContaining(server) })));
    await user.click(add);
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ project: '/project', server })));
  });

  it('opens the exact file of an unmanaged Pi account', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [] });
    renderDialog({ servers: {}, defaultTargets: [], paths: { 'pi-work': '/work/pi/mcp.json' }, defaultFrom: 'pi-work', defaultPath: '/work/pi/mcp-adapter.json', importSources: [
      { target: 'pi-work', path: '/work/pi/mcp.json', piExtension: 'builtin' },
      { target: 'pi-work', path: '/work/pi/mcp-adapter.json', piExtension: 'pi-mcp-adapter' },
    ] }, [...mcpTargets, 'pi-work']);
    await waitFor(() => expect(mcpApi.import).toHaveBeenCalledWith({ from: 'pi-work', piExtension: 'pi-mcp-adapter' }));
    expect(screen.getByRole('combobox', { name: 'Target' })).toHaveTextContent('pi-work /work/pi/mcp-adapter.json');
  });

  it('imports into Pi without a Pi mode', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server: { url: 'https://example.com/mcp' }, problems: [], warnings: [], from: 'claude' }] });
    const user = userEvent.setup();
    renderDialog({ servers: { other: { command: 'other' } }, defaultTargets: ['pi'], paths: { claude: '/home/me/.claude.json' }, detected: ['claude'] });
    await user.click(await screen.findByRole('button', { name: 'Import 1 server' }));
    expect(screen.queryByRole('combobox', { name: 'Pi MCP mode' })).not.toBeInTheDocument();
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://example.com/mcp' } })));
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

  // An account of an Agent has its own file, so it is an import source under its own name.
  it('offers an account of an Agent as an import source', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [] });
    const user = userEvent.setup();
    renderDialog(
      { paths: { claude: '/home/me/.claude.json', 'claude-work': '/home/me/.claude-work/.claude.json' } },
      [...mcpTargets, 'claude-work'],
    );
    await user.click(screen.getByRole('combobox'));
    await user.click(await screen.findByRole('option', { name: 'claude-work ~/.claude-work/.claude.json' }));
    await waitFor(() => expect(mcpApi.import).toHaveBeenLastCalledWith({ from: 'claude-work' }));
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

  it('offers no Pi settings for several pasted servers', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [
      { name: 'docs', server: { command: 'docs' }, problems: [], warnings: [] },
      { name: 'wiki', server: { command: 'wiki' }, problems: [], warnings: [] },
    ] });
    const user = userEvent.setup();
    renderDialog({ source: 'paste', servers: {}, defaultTargets: ['pi'] });
    await user.click(screen.getByLabelText('Server snippet'));
    await user.paste('{"mcpServers":{"docs":{"command":"docs"},"wiki":{"command":"wiki"}}}');
    expect(await screen.findByRole('checkbox', { name: /wiki/ })).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Tool exposure' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Other Pi settings')).not.toBeInTheDocument();
  });

  it('says in the dashboard language that a pasted snippet is not valid JSON', async () => {
    vi.mocked(mcpApi.import).mockRejectedValue(new Error('invalid JSON/JSONC; target was not changed'));
    const user = userEvent.setup();
    renderDialog({ source: 'paste' });
    await user.click(screen.getByLabelText('Server snippet'));
    await user.paste('ready (global mode, hot-reload)');
    expect(await screen.findByText('Not valid JSON or JSONC. Check the syntax.')).toBeInTheDocument();
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
