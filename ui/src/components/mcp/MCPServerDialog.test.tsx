import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type Target } from '../../api/client';
import { mcpApi } from '../../api/mcp';
import { mcpCheckApi } from '../../api/mcpCheck';
import { I18nProvider } from '../../i18n';
import MCPServerDialog from './MCPServerDialog';
import { MCPAccountAgents } from './targetOrder';

vi.mock('../CopyButton', () => ({ default: () => null }));
// CodeMirror needs a real layout engine; a textarea stands in for it
vi.mock('../CodeEditor', () => ({
  default: ({ value, onChange, ariaLabel, placeholder }: { value: string; onChange: (v: string) => void; ariaLabel: string; placeholder?: string }) => <textarea aria-label={ariaLabel} placeholder={placeholder} value={value} onChange={(e) => onChange(e.target.value)} />,
}));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { save: vi.fn(), render: vi.fn() } }));
vi.mock('../../api/mcpCheck', () => ({ mcpCheckApi: { probe: vi.fn() } }));

const renderDialog = (props: Partial<Parameters<typeof MCPServerDialog>[0]> = {}) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPServerDialog defaultTargets={['claude']} existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} {...props} /></I18nProvider></QueryClientProvider>);

describe('MCP server dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks(); localStorage.clear();
    vi.spyOn(api, 'listTargets').mockResolvedValue({ targets: [], sourceSkillCount: 0 });
    vi.spyOn(api, 'availableTargets').mockResolvedValue({ targets: [] });
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

  it('serves every enabled skill from the Skillshare tab, with no command to type', async () => {
    const user = userEvent.setup();
    renderDialog({ serve: true });
    expect(screen.queryByLabelText('Command')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith({ name: 'skillshare', server: { command: 'skillshare', args: ['mcp', 'serve'] }, replace: false }));
  });

  // A root under mcp.projects is served from the global config, where its targets live, so no -p.
  it("serves a project's own target selection from the global config", async () => {
    const user = userEvent.setup();
    vi.mocked(api.listTargets).mockResolvedValue({
      targets: [
        { name: 'claude', skillsEnabled: true },
        { name: 'app@claude', project: '/work/app', skillsEnabled: true },
        { name: 'web@claude', project: '/work/web', skillsEnabled: true },
      ] as Target[],
      sourceSkillCount: 1,
    });
    renderDialog({ serve: true, project: '/work/app' });
    await user.click(screen.getByRole('combobox', { name: 'Skills to serve' }));
    const own = await screen.findByRole('option', { name: 'Same as target app@claude' });
    expect(screen.queryByRole('option', { name: 'Same as target web@claude' })).not.toBeInTheDocument();
    await user.click(own);
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { command: 'skillshare', args: ['mcp', 'serve', '--target', 'app@claude'] } })));
  });

  it("keeps an existing command's -p when the served target changes", async () => {
    const user = userEvent.setup();
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [{ name: 'old', skillsEnabled: true }, { name: 'new', skillsEnabled: true }] as Target[], sourceSkillCount: 1 });
    renderDialog({ serve: true, initial: { name: 'skillshare', server: { command: 'skillshare', args: ['mcp', 'serve', '-p', '--target', 'old'] } } });
    await user.click(screen.getByRole('combobox', { name: 'Skills to serve' }));
    await user.click(await screen.findByRole('option', { name: 'Same as target new' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining({ command: 'skillshare', args: ['mcp', 'serve', '--target', 'new', '-p'] }) })));
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

  it('keeps Pi options when editing something else', async () => {
    const user = userEvent.setup();
    const server = { command: 'docs', targets: ['pi'], piOptions: { timeout: 120 } };
    renderDialog({ initial: { name: 'docs', server } });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server })));
  });

  // Refs: #578. The dialog has no editor for options, so saving must not drop them.
  it('keeps other Agents\' options when editing', async () => {
    const user = userEvent.setup();
    const server = { url: 'https://docs.example/mcp', targets: ['codex'], options: { codex: { default_tools_approval_mode: 'approve' } } };
    renderDialog({ initial: { name: 'docs', server } });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server })));
  });

  it("saves the options typed for a ticked Agent and keeps another Agent's", async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://docs.example/mcp', targets: ['codex'], options: { claude: { timeout: 600000 } } } } });
    const box = screen.getByLabelText('Codex: other settings');
    await user.clear(box);
    await user.click(box);
    await user.paste('{"default_tools_approval_mode": "approve"}');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({
      server: expect.objectContaining({ options: { codex: { default_tools_approval_mode: 'approve' }, claude: { timeout: 600000 } } }),
    })));
  });

  it("saves an unticked Agent's settings cleared and retyped", async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://docs.example/mcp', targets: ['codex'], options: { claude: { timeout: 600000 } } } } });
    await user.click(screen.getByRole('combobox', { name: 'Agent' }));
    await user.click(screen.getByRole('option', { name: (name) => name.startsWith('Claude') && !name.startsWith('Claude Desktop') }));
    const box = screen.getByLabelText('Claude: other settings');
    await user.clear(box);
    await user.click(screen.getByLabelText('Claude: other settings'));
    await user.paste('{"timeout": 120000}');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({
      server: expect.objectContaining({ options: { claude: { timeout: 120000 } } }),
    })));
  });

  // Options are keyed by Agent; a ticked account opens its Agent's editor.
  it("saves an account's settings under its Agent", async () => {
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPAccountAgents.Provider value={{ 'codex-work': 'codex' }}><MCPServerDialog defaultTargets={[]} existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} initial={{ name: 'docs', server: { url: 'https://docs.example/mcp', targets: ['codex-work'] } }} /></MCPAccountAgents.Provider></I18nProvider></QueryClientProvider>);
    await user.click(screen.getByLabelText('Codex: other settings'));
    await user.paste('{"default_tools_approval_mode": "approve"}');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({
      server: expect.objectContaining({ options: { codex: { default_tools_approval_mode: 'approve' } } }),
    })));
  });

  it('blocks saving Agent settings that are not a JSON object', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://docs.example/mcp', targets: ['codex'] } } });
    await user.click(screen.getByLabelText('Codex: other settings'));
    await user.paste('[1]');
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
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

  it("turns a server off in Pi through enabled, and back on by removing it", async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['pi'], piOptions: { timeout: 120 } } } });
    const enabled = screen.getByRole('checkbox', { name: 'Turned on in Pi' });
    const json = () => JSON.parse((screen.getByLabelText('Other Pi settings') as HTMLTextAreaElement).value);
    expect(enabled).toBeChecked();
    await user.click(enabled);
    expect(json()).toEqual({ timeout: 120, enabled: false });
    await user.click(enabled);
    expect(json()).toEqual({ timeout: 120 });
    await user.click(enabled);
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining({ piOptions: { timeout: 120, enabled: false } }) })));
  });

  it('empties the JSON when an exposure is set and then cleared, and saves no piOptions', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['pi'] } } });
    const exposure = screen.getByRole('combobox', { name: 'Tool exposure' });
    await user.click(exposure);
    await user.click(screen.getByRole('option', { name: /^direct\b/ }));
    await user.click(exposure);
    await user.click(screen.getByRole('option', { name: /^Not set\b/ }));
    expect(screen.getByLabelText('Other Pi settings')).toHaveValue('');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalled());
    expect(vi.mocked(mcpApi.save).mock.calls[0][0].server).not.toHaveProperty('piOptions');
  });

  it("keeps a server's tool policy when editing something else", async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    const server = { command: 'docs', targets: ['claude'], tools: { allow: ['get_*'], deny: ['delete_issue'] } };
    renderDialog({ initial: { name: 'docs', server } });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server })));
  });

  it('ticks the tools the server reports and keeps typed rules, saving both as its policy', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 2, toolNames: ['search', 'fetch'] } });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'] } } });
    expect(screen.getByText('All tools')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    await user.click(await screen.findByRole('checkbox', { name: 'fetch' }));
    expect(screen.getByText('1 of 2 selected')).toBeInTheDocument();
    await user.type(screen.getByLabelText('Exclude rules'), 'bad name{Enter}');
    expect(screen.getByText(/^bad name is not a tool name/)).toBeInTheDocument();
    await user.clear(screen.getByLabelText('Exclude rules'));
    await user.type(screen.getByLabelText('Exclude rules'), 'delete_*{Enter}');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining({ tools: { deny: ['fetch', 'delete_*'] } }) })));
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
