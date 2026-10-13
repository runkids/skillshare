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
    projectConfigs: [], paths: { claude: '/work/app/.mcp.json', cursor: '/work/app/.cursor/mcp.json' }, detected: [], openable: [], local: false, projectPaths: {}, projectOpenable: {}, plan, previewError: '', backups: [], unmanaged: [],
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

  // Each save sends the revision it previewed; a second one racing it would be refused.
  it('holds the other rows while a save is on its way', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.save).mockReturnValueOnce(new Promise(() => {}));
    view('claude', { a: { command: 'npx', targets: ['claude'] }, b: { command: 'npx', targets: ['claude'] } });
    await user.click(screen.getByRole('switch', { name: 'a' }));
    expect(screen.getByRole('switch', { name: 'b' })).toBeDisabled();
  });

});
