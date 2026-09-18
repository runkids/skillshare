import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mcpApi } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPServerDialog from './MCPServerDialog';

vi.mock('../CopyButton', () => ({ default: () => null }));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { save: vi.fn(), render: vi.fn() } }));

const renderDialog = (props: Partial<Parameters<typeof MCPServerDialog>[0]> = {}) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPServerDialog defaultTargets={['claude']} existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} {...props} /></I18nProvider></QueryClientProvider>);

describe('MCP server dialog', () => {
  it('requires a Pi extension and persists the explicit selection', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://example.com/mcp', targets: ['pi'] } } });
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    await user.click(screen.getByRole('combobox', { name: 'MCP extension installed in Pi' }));
    await user.click(screen.getByRole('option', { name: 'pi-mcp-extension' }));
    expect(screen.getByText('pi install npm:pi-mcp-extension')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://example.com/mcp', targets: ['pi'], piExtension: 'pi-mcp-extension' } })));
  });

  it('limits new selections to clients available in this scope', () => {
    renderDialog({ availableTargets: ['claude', 'amp', 'gemini'] });
    expect(screen.getByRole('checkbox', { name: 'Amp' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Gemini CLI' })).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: /Claude Desktop/ })).not.toBeInTheDocument();
  });
  beforeEach(() => {
    vi.clearAllMocks(); localStorage.clear();
    HTMLElement.prototype.scrollIntoView = vi.fn();
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

  it('edits a header, keeping a secret as a reference to the environment', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://docs.example/mcp', headers: { 'X-Team': 'core' }, targets: ['claude'] } } });
    await user.click(screen.getByRole('button', { name: 'Add header' }));
    await user.type(screen.getAllByLabelText('Header name')[1], 'X-Api-Key');
    await user.type(screen.getAllByLabelText('From environment')[0], 'DOCS_KEY');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining({ headers: { 'X-Team': 'core', 'X-Api-Key': { fromEnv: 'DOCS_KEY' } } }) })));
  });

  it('shows what the chosen Agent would get in place of the form, and brings the form back', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [{ target: 'claude', path: '/home/me/.claude.json', content: '{\n  "mcpServers": {}\n}\n' }] });
    renderDialog({ initial: { name: 'docs', server: { command: 'npx', targets: ['claude'] } } });
    await user.click(screen.getByRole('button', { name: 'View them' }));
    expect(await screen.findByText(/"mcpServers"/)).toBeInTheDocument();
    // The fields come back as they were: the view replaces the form, it does not reset it.
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(screen.getByLabelText('Command')).toHaveValue('npx');
    expect(mcpApi.render).toHaveBeenCalledWith({ name: 'docs', server: { command: 'npx', targets: ['claude'] } });
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
