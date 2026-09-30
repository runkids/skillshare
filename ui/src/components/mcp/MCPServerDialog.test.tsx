import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mcpApi } from '../../api/mcp';
import { mcpCheckApi } from '../../api/mcpCheck';
import { I18nProvider } from '../../i18n';
import MCPServerDialog from './MCPServerDialog';

vi.mock('../CopyButton', () => ({ default: () => null }));
// CodeMirror needs a real layout engine; a textarea stands in for it
vi.mock('../CodeEditor', () => ({
  default: ({ value, onChange, ariaLabel, placeholder }: { value: string; onChange: (v: string) => void; ariaLabel: string; placeholder?: string }) => <textarea aria-label={ariaLabel} placeholder={placeholder} value={value} onChange={(e) => onChange(e.target.value)} />,
}));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { save: vi.fn(), render: vi.fn() } }));
vi.mock('../../api/mcpCheck', () => ({ mcpCheckApi: { live: vi.fn() } }));

const renderDialog = (props: Partial<Parameters<typeof MCPServerDialog>[0]> = {}) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPServerDialog defaultTargets={['claude']} existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} {...props} /></I18nProvider></QueryClientProvider>);

describe('MCP server dialog', () => {
  it('offers Pi settings without a Pi mode, cleanup switch or Direct tools', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://example.com/mcp', targets: ['pi'] } } });
    expect(screen.getByRole('combobox', { name: 'Tool exposure' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Pi MCP · Official documentation' })).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Pi MCP mode' })).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: 'Remove cleared settings from Pi' })).not.toBeInTheDocument();
    expect(screen.queryByText('Direct tools')).not.toBeInTheDocument();
    expect(screen.queryByText(/pi-mcp-adapter|pi-mcp-extension/)).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://example.com/mcp', targets: ['pi'] } })));
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

  // Refs: #289. With no Agent the server is kept in Skillshare and written nowhere.
  it('saves a server with no Agent selected as an explicit empty list', async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.type(screen.getByLabelText('Name'), 'docs');
    await user.click(screen.getByRole('button', { name: 'streamable-http' }));
    await user.type(screen.getByLabelText('URL'), 'https://docs.example/mcp');
    await user.click(screen.getByRole('checkbox', { name: 'Claude' }));
    expect(screen.getByText('With no Agent selected, it is only kept in Skillshare, not written to any config file')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://docs.example/mcp', targets: [] } })));
  });

  // An empty selection equals empty defaults, but leaving targets out would inherit them and be refused.
  it('sends the empty list even when the defaults are empty too', async () => {
    const user = userEvent.setup();
    renderDialog({ defaultTargets: [] });
    await user.type(screen.getByLabelText('Name'), 'docs');
    await user.click(screen.getByRole('button', { name: 'streamable-http' }));
    await user.type(screen.getByLabelText('URL'), 'https://docs.example/mcp');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://docs.example/mcp', targets: [] } })));
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

  it('keeps Pi options when editing something else', async () => {
    const user = userEvent.setup();
    const server = { command: 'docs', targets: ['pi'], piOptions: { timeout: 120 } };
    renderDialog({ initial: { name: 'docs', server } });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server })));
  });

  // Refs: #289. Unticking Pi must not drop its settings from the source.
  it('keeps Pi settings when Pi is unticked', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { command: 'npx', targets: ['pi', 'claude'], piOptions: { timeout: 120 } } } });
    await user.click(screen.getByRole('checkbox', { name: 'Pi' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({
      server: expect.objectContaining({ targets: ['claude'], piOptions: { timeout: 120 } }),
    })));
  });

  it('takes other Pi settings as a JSON object', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['pi'] } } });
    const box = screen.getByLabelText('Other Pi settings');
    await user.click(box);
    await user.paste('["delete_*"]');
    expect(screen.getByText('Enter a JSON object.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    await user.clear(box);
    await user.paste('{"cwd": "/work"}');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({
      server: { command: 'docs', targets: ['pi'], piOptions: { cwd: '/work' } },
    })));
  });

  it('previews a new Pi server in the project scope', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [{ target: 'pi', path: '/project/.pi/mcp.json', content: '{"mcpServers":{}}' }] });
    renderDialog({ defaultTargets: ['pi'], project: '/project' });
    await user.type(screen.getByLabelText('Name'), 'docs');
    await user.type(screen.getByLabelText('Command'), 'docs');
    await user.click(screen.getByRole('button', { name: 'View them' }));
    await waitFor(() => expect(mcpApi.render).toHaveBeenCalledWith({ project: '/project', name: 'docs', server: { command: 'docs', targets: ['pi'] } }));
  });

  it('shares exposure with JSON', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['pi'], piOptions: { exposure: 'deferred', custom: { keep: true } } } } });
    const box = screen.getByLabelText('Other Pi settings');
    await user.clear(box);
    await user.click(box);
    await user.paste('{"exposure":"hidden","custom":{"keep":true}}');
    expect(screen.getByRole('combobox', { name: 'Tool exposure' })).toHaveTextContent('hidden');
    await user.click(screen.getByRole('combobox', { name: 'Tool exposure' }));
    await user.click(screen.getByRole('option', { name: /^direct\b/ }));
    expect(JSON.parse((box as HTMLTextAreaElement).value)).toEqual({ exposure: 'direct', custom: { keep: true } });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining({ piOptions: { exposure: 'direct', custom: { keep: true } } }) })));
  });

  it("keeps a server's tool policy when editing something else", async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    const server = { command: 'docs', targets: ['claude'], tools: { expose: 'deferred' as const, allow: ['get_*'], deny: ['delete_issue'] } };
    renderDialog({ initial: { name: 'docs', server } });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server })));
  });

  it('adds allowed tools from the names the saved server reports, and saves them as its policy', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    vi.mocked(mcpCheckApi.live).mockResolvedValue({ servers: [{ name: 'docs', ok: true, findings: [], live: { tools: 2, toolNames: ['search', 'fetch'] } }], summary: { errors: 0, warnings: 0 } });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'] } } });
    await user.click(screen.getByRole('button', { name: 'Tools' }));
    await user.click(screen.getByRole('button', { name: 'Load tools from server' }));
    expect(await screen.findByText('2 tools loaded. Pick them in Allow or Deny.')).toBeInTheDocument();
    expect(mcpCheckApi.live).toHaveBeenCalledWith('docs');
    expect([...document.querySelectorAll('datalist option')].map((o) => (o as HTMLOptionElement).value)).toEqual(['fetch', 'search', 'fetch', 'search']);
    await user.type(screen.getByLabelText('Allow'), 'search{Enter}get_*{Enter}');
    await user.type(screen.getByLabelText('Deny'), 'bad name{Enter}');
    expect(screen.getByText(/^bad name is not a tool name/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining({ tools: { allow: ['search', 'get_*'] } }) })));
  });

  it('says under the JSON that Tools already sets the exposure it sets, before saving', async () => {
    const overlap = "Tools already sets Pi's tool exposure for this server. Remove toolExposure here, or clear Tools.";
    renderDialog({ initial: { name: 'context7', server: { command: 'docs', targets: ['pi'], tools: { allow: ['resolve-*'] }, piOptions: { timeout: 120, toolExposure: { 'delete_*': 'hidden' } } } } });
    expect(screen.getByText(overlap)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });

  it('does not suggest toolExposure in the empty JSON while Tools has a setting', () => {
    renderDialog({ initial: { name: 'context7', server: { command: 'docs', targets: ['pi'], tools: { allow: ['resolve-*'] } } } });
    expect(screen.getByLabelText('Other Pi settings').getAttribute('placeholder')).not.toContain('toolExposure');
  });

  it('explains in the Tools section, before saving, that Deny removes every allowed tool', () => {
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'], tools: { allow: ['delete_issue'], deny: ['delete_*'] } } } });
    expect(screen.getByText('Deny removes every tool that Allow keeps, so the server would have no tools. Remove an entry from Deny, or add another tool to Allow.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });

  it('names each chosen Agent that does not apply part of the policy', async () => {
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [{ target: 'copilot', path: '/c.json', toolGaps: ['allow patterns'] }, { target: 'pi', path: '/p.json' }] });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['copilot', 'pi'], tools: { allow: ['get_*'] } } } });
    expect(await screen.findByText('Copilot CLI: * patterns in Allow')).toBeInTheDocument();
    expect(screen.queryByText(/^Pi:/)).not.toBeInTheDocument();
  });

  it('explains that a new server can load its tools once it is saved', async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.click(screen.getByRole('button', { name: 'Tools' }));
    expect(screen.getByRole('button', { name: 'Load tools from server' })).toBeDisabled();
    expect(screen.getByText('Save the server first, then load its tools here.')).toBeInTheDocument();
  });

  it('has no Tools section for an entry that only turns a server off', () => {
    renderDialog({ off: true });
    expect(screen.queryByRole('button', { name: 'Tools' })).not.toBeInTheDocument();
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
