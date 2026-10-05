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
vi.mock('../../api/mcpCheck', () => ({ mcpCheckApi: { probe: vi.fn() } }));

const renderDialog = (props: Partial<Parameters<typeof MCPServerDialog>[0]> = {}) =>
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPServerDialog defaultTargets={['claude']} existingNames={[]} onClose={vi.fn()} onSaved={vi.fn()} {...props} /></I18nProvider></QueryClientProvider>);

describe('MCP server dialog', () => {
  it('offers OMP as its own client without Pi settings', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://example.com/mcp', targets: ['omp'] } }, availableTargets: ['omp'] });
    expect(screen.getByRole('checkbox', { name: 'Oh My Pi' })).toBeChecked();
    expect(screen.queryByRole('button', { name: 'About Pi settings' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: { url: 'https://example.com/mcp', targets: ['omp'] } })));
  });

  it('offers Pi settings without a Pi mode, cleanup switch or Direct tools', async () => {
    const user = userEvent.setup();
    renderDialog({ initial: { name: 'docs', server: { url: 'https://example.com/mcp', targets: ['pi'] } } });
    expect(screen.getByRole('combobox', { name: 'Tool exposure' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Pi MCP · Official documentation' })).toBeInTheDocument();
    // Help sits behind keyboard-reachable info icons, not in lines under the fields.
    expect(screen.getByRole('button', { name: 'About Pi settings' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'About tool exposure' })).toBeInTheDocument();
    expect(screen.queryByText(/Pi ≥ 0.99.0 includes MCP/)).not.toBeInTheDocument();
    screen.getByRole('button', { name: 'About Pi settings' }).focus();
    expect(await screen.findByRole('tooltip')).toHaveTextContent(/Pi ≥ 0.99.0 includes MCP.*~\/\.pi\/agent\/mcp\.json/);
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

  it('hints that Enter adds a rule only while the rule input has text', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'] } } });
    expect(screen.queryByText('Enter', { selector: 'kbd' })).not.toBeInTheDocument();
    await user.type(screen.getByLabelText('Exclude rules'), 'delete_');
    expect(screen.getByText('Enter', { selector: 'kbd' }).parentElement).toHaveTextContent('Press Enter to add');
    await user.clear(screen.getByLabelText('Exclude rules'));
    expect(screen.queryByText('Enter', { selector: 'kbd' })).not.toBeInTheDocument();
  });

  it('locks a row that a Deny pattern removes, naming the rule', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 2, toolNames: ['delete_issue', 'search'] } });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'], tools: { deny: ['delete_*'] } } } });
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    expect(await screen.findByRole('checkbox', { name: 'delete_issue' })).toBeDisabled();
  });

  it('selects none among only the rows the search shows', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 3, toolNames: ['get_issue', 'get_repo', 'search'] } });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'] } } });
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    await user.type(await screen.findByLabelText('Search'), 'get_');
    await user.click(screen.getByRole('button', { name: 'Select none' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith(expect.objectContaining({ server: expect.objectContaining({ tools: { deny: ['get_issue', 'get_repo'] } }) })));
  });

  it("leaves Pi's exposure to the Pi settings, unlocked while Tools has a setting", () => {
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['pi'], tools: { deny: ['a', 'b'] } } } });
    expect(screen.getByRole('combobox', { name: 'Tool exposure' })).toBeEnabled();
    expect(screen.getAllByRole('combobox')).toHaveLength(1);
  });

  it('sums up a policy in words before the tools are loaded', () => {
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'], tools: { deny: ['a', 'b'] } } } });
    expect(screen.getByText('2 tools excluded')).toBeInTheDocument();
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

  it('says which chosen Agents follow the policy as it is', async () => {
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [{ target: 'pi', path: '/p.json' }, { target: 'codex', path: '/c.toml' }, { target: 'claude', path: '/c.json', toolGaps: ['deny'] }] });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['pi', 'codex'], tools: { deny: ['search'] } } } });
    expect(await screen.findByText('Pi, Codex will offer tools as this list says.')).toBeInTheDocument();
  });

  it('spells out what a chosen Agent that follows part of the policy will do', async () => {
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [{ target: 'copilot', path: '/c.json', toolGaps: ['deny'] }, { target: 'codex', path: '/c.toml', toolGaps: ['deny patterns'] }] });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['copilot', 'codex'], tools: { deny: ['search', 'delete_*'] } } } });
    expect(await screen.findByText('Copilot CLI will offer the tools you unticked (it cannot exclude tools).')).toBeInTheDocument();
    expect(screen.getByText('The exclude rule delete_* has no effect on Codex (it only takes full names).')).toBeInTheDocument();
  });

  it('says which chosen Agents cannot filter tools at all', async () => {
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [{ target: 'claude', path: '/c.json', toolGaps: ['deny'] }, { target: 'cursor', path: '/m.json', toolGaps: ['deny'] }] });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude', 'cursor'], tools: { deny: ['search'] } } } });
    expect(await screen.findByText('Claude, Cursor cannot filter tools and will still offer every tool.')).toBeInTheDocument();
  });

  it('loads tools with the unsaved settings, not the saved ones', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 1, toolNames: ['search'] } });
    renderDialog({ project: '/work/app', initial: { name: 'docs', server: { url: 'http://127.0.0.1:3845/mcp', targets: ['claude'] } } });
    await user.clear(screen.getByLabelText('URL'));
    await user.type(screen.getByLabelText('URL'), 'https://docs.example/mcp');
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    expect(await screen.findByRole('checkbox', { name: 'search' })).toBeChecked();
    expect(mcpCheckApi.probe).toHaveBeenCalledWith({ project: '/work/app', server: { url: 'https://docs.example/mcp' } });
    expect(mcpApi.save).not.toHaveBeenCalled();
  });

  it('loads the tools of a new server once it has a command or URL', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 1, toolNames: ['search'] } });
    renderDialog();
    expect(screen.getByRole('button', { name: 'Load tools' })).toBeDisabled();
    expect(screen.getByText('Enter a command or URL first.')).toBeInTheDocument();
    await user.type(screen.getByLabelText('Command'), 'npx -y docs');
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    expect(await screen.findByRole('checkbox', { name: 'search' })).toBeChecked();
    expect(mcpCheckApi.probe).toHaveBeenCalledWith({ server: { command: 'npx', args: ['-y', 'docs'] } });
  });

  it('says in plain words why tools did not load, with the raw error in the Details tooltip', async () => {
    const user = userEvent.setup();
    const detail = 'Post "http://127.0.0.1:3845/mcp": dial tcp 127.0.0.1:3845: connect: connection refused';
    vi.mocked(mcpCheckApi.probe).mockResolvedValue({ errorKind: 'connect', error: detail });
    renderDialog({ initial: { name: 'docs', server: { url: 'http://127.0.0.1:3845/mcp', targets: ['claude'] } } });
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    expect(await screen.findByText('Could not connect to the server. Check the URL and that the server is running.')).toBeInTheDocument();
    expect(screen.queryByText(detail)).not.toBeInTheDocument();
    screen.getByRole('button', { name: 'Details' }).focus();
    expect(await screen.findByRole('tooltip')).toHaveTextContent(detail);
  });

  it('falls back to a general message for an error it cannot name', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpCheckApi.probe).mockRejectedValue(new Error('MCP draft requires exactly one of command or url'));
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'] } } });
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    expect(await screen.findByText("Could not load the server's tools.")).toBeInTheDocument();
    screen.getByRole('button', { name: 'Details' }).focus();
    expect(await screen.findByRole('tooltip')).toHaveTextContent('MCP draft requires exactly one of command or url');
  });

  it('drops the loaded tools when the connection settings change', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [] });
    vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 1, toolNames: ['search'] } });
    renderDialog({ initial: { name: 'docs', server: { command: 'docs', targets: ['claude'] } } });
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    await user.click(await screen.findByRole('checkbox', { name: 'search' }));
    expect(screen.getByRole('checkbox', { name: 'search' })).not.toBeChecked();
    await user.type(screen.getByLabelText('Command'), '-v2');
    expect(screen.queryByRole('checkbox', { name: 'search' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Load tools' })).toBeInTheDocument();
  });

  it('has no Tools section for an entry that only turns a server off', () => {
    renderDialog({ off: true });
    expect(screen.queryByText('Tools')).not.toBeInTheDocument();
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
