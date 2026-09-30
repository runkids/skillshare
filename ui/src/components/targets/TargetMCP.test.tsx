import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { mcpApi } from '../../api/mcp';
import { queryKeys } from '../../lib/queryKeys';
import { ToastProvider } from '../Toast';
import TargetMCP from './TargetMCP';

vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { save: vi.fn().mockResolvedValue({}) } }));

type Data = Parameters<typeof TargetMCP>[0]['data'];

const view = (name: string, servers: Data['source']['servers'], plan: Data['plan'] = null, targets: string[] | null = null) => {
  const data = {
    source: { path: '', configPath: '', targets, servers },
    projectConfigs: [], paths: { claude: '/work/app/.mcp.json', cursor: '/work/app/.cursor/mcp.json' }, detected: [], plan, previewError: '', backups: [], unmanaged: [],
  } as Data;
  // The switch reads the server from the shared MCP query, as the page that owns it does.
  const client = new QueryClient();
  client.setQueryData(queryKeys.mcp, data);
  render(<MemoryRouter><QueryClientProvider client={client}><I18nProvider><ToastProvider><TargetMCP name={name} data={data} /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
};

describe('Target MCP tab', () => {
  it('takes this Agent out of a server and keeps the others', async () => {
    const user = userEvent.setup();
    view('claude', { context7: { command: 'npx', targets: ['claude', 'cursor'] } });
    await user.click(screen.getByRole('switch', { name: 'context7' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith({ name: 'context7', replace: true, server: { command: 'npx', targets: ['cursor'] } }));
  });

  // Sync sends a switch that names no targets to the project's Agents that have a switch;
  // listing Cursor would be refused.
  it('adds an Agent to a switch that names no targets without listing the others', async () => {
    const user = userEvent.setup();
    view('opencode', { docs: { disabled: true } }, null, ['claude', 'cursor']);
    await user.click(screen.getByRole('switch', { name: 'docs' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith({ name: 'docs', replace: true, server: { disabled: true, targets: ['claude', 'opencode'] } }));
  });

  it("shows a switch that names no targets where the plan sends it", () => {
    view('opencode', { docs: { disabled: true } }, {
      revision: 'r', sourcePath: '/work/app/.skillshare/config.yaml', blocked: false,
      changes: [{ target: 'opencode', path: '/work/app/opencode.json', name: 'docs', switch: true, action: 'unchanged' }],
    }, ['claude', 'opencode']);
    expect(screen.getByRole('switch', { name: 'docs' })).toHaveAttribute('aria-checked', 'true');
  });

  // Each save sends the revision it previewed; a second one racing it would be refused.
  it('holds the other rows while a save is on its way', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.save).mockReturnValueOnce(new Promise(() => {}));
    view('claude', { a: { command: 'npx', targets: ['claude'] }, b: { command: 'npx', targets: ['claude'] } });
    await user.click(screen.getByRole('switch', { name: 'a' }));
    expect(screen.getByRole('switch', { name: 'b' })).toBeDisabled();
  });

  it('keeps a row for a server the plan still takes out of this Agent', () => {
    view('claude', {}, { revision: 'r', sourcePath: '/c.yaml', blocked: false, changes: [{ target: 'claude', path: '/c.json', name: 'old', action: 'remove' }] });
    expect(screen.getByText('Removed from source')).toBeInTheDocument();
  });

  it('counts only servers it writes, not a switch that turns one off', () => {
    view('claude', { docs: { disabled: true, targets: ['claude'] }, ctx: { command: 'npx', targets: ['claude'] } });
    expect(screen.getByText('1 of 1 MCP server goes to claude.')).toBeInTheDocument();
  });

  it('does not call an Agent with only a conflict synced', () => {
    view('claude', { docs: { command: 'npx', targets: ['claude'] } }, { revision: 'r', sourcePath: '/c.yaml', blocked: true, changes: [{ target: 'claude', path: '/c.json', name: 'docs', action: 'conflict' }] });
    expect(screen.getByText('Conflict')).toBeInTheDocument();
  });

  it('lists a project switch that turns a global server off for Claude', () => {
    view('claude', { docs: { disabled: true, targets: ['claude'] } });
    expect(screen.getByText('Off in this project')).toBeInTheDocument();
  });

  it('leaves the project switch out for an Agent that has no per-project switch', () => {
    view('cursor', { docs: { disabled: true, targets: ['claude'] } });
    expect(screen.queryByRole('switch', { name: 'docs' })).not.toBeInTheDocument();
  });

  // Claude's off list for a -p project sits in ~/.claude.json and carries the project root.
  it("counts a -p project's Claude off list as pending", () => {
    view('claude', { docs: { disabled: true, targets: ['claude'] } }, {
      revision: 'r', sourcePath: '/work/app/.skillshare/config.yaml', blocked: false,
      changes: [{ target: 'claude', path: '/home/.claude.json', name: 'docs', root: '/work/app', switch: true, action: 'add' }],
    });
    expect(screen.getByText('Pending sync')).toBeInTheDocument();
  });

  // Sync writes the whole plan, so the tab must not suggest it writes only this Agent.
  it("says Sync writes every target's changes, not only this one's", () => {
    view('claude', { docs: { command: 'npx', targets: ['claude', 'cursor'] } }, {
      revision: 'r', sourcePath: '/c.yaml', blocked: false,
      changes: [{ target: 'claude', path: '/c.json', name: 'docs', action: 'add' }, { target: 'cursor', path: '/k.json', name: 'docs', action: 'add' }],
    });
    expect(screen.getByText(/every target at once: 2 changes in all/)).toBeInTheDocument();
  });
});
