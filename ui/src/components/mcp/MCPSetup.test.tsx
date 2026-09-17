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
    render(<I18nProvider><MCPSetup defaultTargets={['claude']} onClose={vi.fn()} onSaved={saved} /></I18nProvider>);
    await user.type(screen.getByLabelText('Name'), 'docs');
    expect(screen.getByRole('checkbox', { name: 'opencode' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'grok' })).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText('https://example.com/mcp'), 'https://docs.example/mcp');
    await user.click(screen.getByRole('button', { name: 'Preview changes' }));
    await screen.findByText('/example/.claude.json');
    expect(mcpApi.configure).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Save only' }));
    await waitFor(() => expect(saved).toHaveBeenCalledWith([]));
    expect(mcpApi.configure).toHaveBeenCalledWith({ name: 'docs', server: { url: 'https://docs.example/mcp', targets: ['claude'] }, replace: false }, 'reviewed', false);
  });

  it('blocks sync on conflict while allowing the draft to be saved', async () => {
    vi.mocked(mcpApi.preview).mockResolvedValue({ ...plan, blocked: true, changes: [{ ...plan.changes[0], action: 'conflict' }] });
    const user = userEvent.setup();
    render(<I18nProvider><MCPSetup initial={{ name: 'docs', server: { url: 'https://example.com/mcp' } }} defaultTargets={['claude']} onClose={vi.fn()} onSaved={vi.fn()} /></I18nProvider>);
    await user.click(screen.getByRole('button', { name: 'Preview changes' }));
    expect(await screen.findByRole('button', { name: 'Save and sync' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Save only' })).toBeEnabled();
    expect(mcpApi.preview).toHaveBeenCalledWith(expect.objectContaining({ replace: true }));
    expect(mcpApi.configure).not.toHaveBeenCalled();
  });
});
