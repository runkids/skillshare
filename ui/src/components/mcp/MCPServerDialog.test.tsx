import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mcpApi } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPServerDialog from './MCPServerDialog';

vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { save: vi.fn() } }));

const renderDialog = (props: Partial<Parameters<typeof MCPServerDialog>[0]> = {}) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPServerDialog defaultTargets={['claude']} existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} {...props} /></I18nProvider></QueryClientProvider>);

describe('MCP server dialog', () => {
  it('limits new selections to clients available in this scope', () => {
    renderDialog({ availableTargets: ['claude', 'amp', 'gemini'] });
    expect(screen.getByRole('checkbox', { name: 'Amp' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Gemini CLI' })).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: /Claude Desktop/ })).not.toBeInTheDocument();
  });
  beforeEach(() => {
    vi.clearAllMocks(); localStorage.clear();
    vi.mocked(mcpApi.save).mockResolvedValue({ applied: [], backupIds: [] });
  });

  it('saves a stdio server to the source only, splitting the command and keeping inherited targets', async () => {
    const user = userEvent.setup();
    const saved = vi.fn();
    renderDialog({ onSaved: saved });
    await user.type(screen.getByLabelText('Name'), 'notes');
    await user.type(screen.getByLabelText('Command'), `npx -y @scope/notes "~/My Notes"`);
    await user.click(screen.getByRole('button', { name: 'Add variable' }));
    await user.type(screen.getByLabelText('Variable name'), 'NOTES_TOKEN');
    await user.type(screen.getByLabelText('From environment'), 'NOTES_TOKEN');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(saved).toHaveBeenCalled());
    expect(mcpApi.save).toHaveBeenCalledWith({
      name: 'notes',
      server: { command: 'npx', args: ['-y', '@scope/notes', '~/My Notes'], env: { NOTES_TOKEN: { fromEnv: 'NOTES_TOKEN' } } },
      replace: false,
    });
  });

  it('pins targets when the selection differs from the inherited default', async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.type(screen.getByLabelText('Name'), 'docs');
    await user.click(screen.getByRole('button', { name: 'streamable-http' }));
    await user.type(screen.getByLabelText('URL'), 'https://docs.example/mcp');
    await user.click(screen.getByRole('checkbox', { name: 'Codex' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://docs.example/mcp', targets: ['claude', 'codex'] } })));
  });

  it('keeps explicit targets and existing headers when editing', async () => {
    const user = userEvent.setup();
    const server = { url: 'https://docs.example/mcp', headers: { 'X-Team': 'core' }, targets: ['claude'] };
    renderDialog({ initial: { name: 'docs', server } });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith({ name: 'docs', server, replace: true }));
  });

  it('refuses a name that is already taken', async () => {
    const user = userEvent.setup();
    renderDialog({ existingNames: ['docs'] });
    await user.type(screen.getByLabelText('Name'), 'docs');
    await user.type(screen.getByLabelText('Command'), 'npx docs');
    expect(screen.getByText('A server with this name already exists.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });
});
