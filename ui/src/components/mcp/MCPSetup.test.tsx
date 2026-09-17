import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mcpApi, type MCPPlan } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPSetup from './MCPSetup';

vi.mock('../../api/mcp', async (load) => ({
  ...await load<typeof import('../../api/mcp')>(),
  mcpApi: { preview: vi.fn(), configure: vi.fn(), import: vi.fn() },
}));

const plan: MCPPlan = { revision: 'reviewed', sourcePath: '/example/config.yaml', blocked: false, changes: [{ target: 'claude', path: '/example/.claude.json', name: 'docs', action: 'add' }] };

describe('MCP guided setup', () => {
  beforeEach(() => {
    vi.clearAllMocks(); localStorage.clear();
    vi.mocked(mcpApi.preview).mockResolvedValue(plan);
    vi.mocked(mcpApi.configure).mockResolvedValue({ applied: [], backupIds: [] });
  });

  it('previews a simple URL before save-only and passes the exact revision', async () => {
    const user = userEvent.setup();
    const saved = vi.fn();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSetup defaultTargets={['claude']} onClose={vi.fn()} onSaved={saved} /></I18nProvider></QueryClientProvider>);
    await user.type(screen.getByLabelText('Name'), 'docs');
    expect(screen.getByRole('checkbox', { name: 'opencode' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'grok' })).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText('https://example.com/mcp'), 'https://docs.example/mcp');
    await user.click(screen.getByRole('button', { name: 'Preview changes' }));
    await screen.findByText('/example/.claude.json');
    expect(mcpApi.configure).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Save only' }));
    await waitFor(() => expect(saved).toHaveBeenCalledWith([]));
    expect(mcpApi.configure).toHaveBeenCalledWith({ name: 'docs', server: { url: 'https://docs.example/mcp' }, replace: false }, 'reviewed', false);
  });

  it('pins targets when the selection differs from the inherited default', async () => {
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSetup defaultTargets={['claude']} onClose={vi.fn()} onSaved={vi.fn()} /></I18nProvider></QueryClientProvider>);
    await user.type(screen.getByLabelText('Name'), 'docs');
    await user.type(screen.getByPlaceholderText('https://example.com/mcp'), 'https://docs.example/mcp');
    await user.click(screen.getByRole('checkbox', { name: 'codex' }));
    await user.click(screen.getByRole('button', { name: 'Preview changes' }));
    expect(mcpApi.preview).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://docs.example/mcp', targets: ['claude', 'codex'] } }));
  });

  it('keeps explicit targets even when they match the inherited default', async () => {
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSetup initial={{ name: 'docs', server: { url: 'https://docs.example/mcp', targets: ['claude'] } }} explicitTargets={['claude']} defaultTargets={['claude']} onClose={vi.fn()} onSaved={vi.fn()} /></I18nProvider></QueryClientProvider>);
    await user.click(screen.getByRole('button', { name: 'Preview changes' }));
    expect(mcpApi.preview).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://docs.example/mcp', targets: ['claude'] } }));
  });

  it('adopts the imported Agent entry and lets Replace override that resolution', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [{ name: 'docs', server: { url: 'https://docs.example/mcp' }, problems: [], warnings: [], from: 'claude' }] });
    vi.mocked(mcpApi.preview).mockResolvedValue({ ...plan, blocked: true, changes: [{ ...plan.changes[0], action: 'conflict', message: 'Agent configuration changed; import it or explicitly replace this entry' }] });
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSetup importFrom={{ target: 'claude', name: 'docs' }} defaultTargets={['claude']} onClose={vi.fn()} onSaved={vi.fn()} /></I18nProvider></QueryClientProvider>);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Preview changes' })).toBeEnabled());
    await user.click(screen.getByRole('button', { name: 'Preview changes' }));
    expect(mcpApi.preview).toHaveBeenCalledWith(expect.objectContaining({ resolutions: [{ target: 'claude', name: 'docs', action: 'adopt' }] }));
    await user.click(await screen.findByRole('button', { name: 'Replace with source' }));
    await waitFor(() => expect(mcpApi.preview).toHaveBeenLastCalledWith(expect.objectContaining({ resolutions: [{ target: 'claude', name: 'docs', action: 'replace' }] })));
  });

  it('closes from the header close button', async () => {
    const close = vi.fn();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSetup defaultTargets={['claude']} onClose={close} onSaved={vi.fn()} /></I18nProvider></QueryClientProvider>);
    await userEvent.setup().click(screen.getByRole('button', { name: 'Close' }));
    expect(close).toHaveBeenCalledOnce();
  });

  it('detects pasted JSON and only asks for the TOML source', async () => {
    vi.mocked(mcpApi.import).mockResolvedValue({ candidates: [] });
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSetup initialMode="json" defaultTargets={['claude']} onClose={vi.fn()} onSaved={vi.fn()} /></I18nProvider></QueryClientProvider>);
    expect(screen.queryByRole('group', { name: 'Source Agent' })).not.toBeInTheDocument();
    await user.click(screen.getByLabelText('Paste server JSON'));
    await user.paste('{"servers":{"docs":{"url":"https://example.com/mcp"}}}');
    await user.click(screen.getByRole('button', { name: 'Read configuration' }));
    expect(mcpApi.import).toHaveBeenCalledWith({ content: '{"servers":{"docs":{"url":"https://example.com/mcp"}}}', name: '' });
    await user.click(screen.getByLabelText('Paste server JSON'));
    await user.paste('[mcp_servers.docs]\nurl = "https://example.com/mcp"');
    await user.click(screen.getByRole('button', { name: 'grok' }));
    await user.click(screen.getByRole('button', { name: 'Read configuration' }));
    expect(mcpApi.import).toHaveBeenLastCalledWith(expect.objectContaining({ from: 'grok' }));
  });

  it('blocks sync on conflict while allowing the draft to be saved', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValue({ ...plan, blocked: true, changes: [{ ...plan.changes[0], action: 'conflict' }] });
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSetup initial={{ name: 'docs', server: { url: 'https://example.com/mcp' } }} defaultTargets={['claude']} onClose={vi.fn()} onSaved={vi.fn()} /></I18nProvider></QueryClientProvider>);
    await user.click(screen.getByRole('button', { name: 'Preview changes' }));
    expect(await screen.findByRole('button', { name: 'Save and sync' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Save only' })).toBeEnabled();
    expect(mcpApi.preview).toHaveBeenCalledWith(expect.objectContaining({ replace: true }));
    expect(mcpApi.configure).not.toHaveBeenCalled();
  });
});
