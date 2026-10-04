import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { mcpApi } from '../../api/mcp';
import { mcpCheckApi } from '../../api/mcpCheck';
import type { MCPCheckFinding } from '../../api/mcpCheck';
import { ToastProvider } from '../Toast';
import MCPProjectView from './MCPProjectView';

vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { save: vi.fn().mockResolvedValue({}), import: vi.fn().mockResolvedValue({ candidates: [] }) } }));
vi.mock('../../api/mcpCheck', () => ({ mcpCheckApi: { run: vi.fn() } }));

type Data = Parameters<typeof MCPProjectView>[0]['data'];

const view = (servers: Data['source']['servers'], project: NonNullable<Data['source']['projects']>[string], unmanaged: Data['unmanaged'] = []) => {
  const data = {
    source: { path: '', configPath: '', targets: null, servers, projects: { '/work/app': project } },
    projectConfigs: [], paths: {}, detected: [], plan: null, previewError: '', backups: [], unmanaged,
  } as Data;
  render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><MCPProjectView data={data} root="/work/app" offered={['claude', 'cursor', 'opencode', 'pi']} onChanged={vi.fn()} onRemoved={vi.fn()} /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
};
const context7 = { command: 'npx', targets: ['claude', 'cursor', 'opencode'] };

describe('MCP project view', () => {
  it('turns a global server off without storing targets, so the switch follows the project', async () => {
    const user = userEvent.setup();
    view({ context7 }, { targets: ['opencode'] });
    await user.click(screen.getByRole('switch', { name: 'context7' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenCalledWith({ name: 'context7', replace: true, server: { disabled: true }, project: '/work/app' }));
  });

  it('names the Agents that keep loading a server turned off here', () => {
    view({ context7 }, { targets: ['claude', 'cursor'], servers: { context7: { disabled: true } } });
    expect(screen.getByText('Still loads in Cursor, which has no per-project switch.')).toBeInTheDocument();
  });

  it('shows where a server is off as Agents, not as the fields written', () => {
    view({ context7 }, { targets: ['claude', 'opencode'], servers: { context7: { disabled: true } } });
    expect(screen.getByRole('img', { name: 'Off in Claude, OpenCode' })).toBeInTheDocument();
    expect(screen.queryByText(/disabledMcpServers/)).not.toBeInTheDocument();
  });

  it('offers to make a switch saved with its own Agents follow the project again', async () => {
    const user = userEvent.setup();
    view({ context7 }, { targets: ['opencode'], servers: { context7: { disabled: true, targets: ['claude', 'opencode'] } } });
    expect(screen.getByText('Off for other Agents than this project uses.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Match the project' }));
    await waitFor(() => expect(mcpApi.save).toHaveBeenLastCalledWith({ name: 'context7', replace: true, server: { disabled: true }, project: '/work/app' }));
  });

  // Pi's switch is its project override, so Pi turns it off here like the others.
  it('turns a server off here for Pi too', () => {
    view({ docs: { command: 'npx', targets: ['opencode', 'pi'] } }, { targets: ['opencode', 'pi'], servers: { docs: { disabled: true } } });
    expect(screen.getByRole('img', { name: 'Off in OpenCode, Pi' })).toBeInTheDocument();
    expect(screen.queryByText(/Still loads in/)).not.toBeInTheDocument();
  });

  it("offers to import servers found in this project's Agent files, reading that project's file", async () => {
    const user = userEvent.setup();
    view({}, {}, [
      { target: 'claude', path: '/.claude.json', names: ['global'] },
      { target: 'cursor', project: '/work/app', path: '/work/app/.cursor/mcp.json', names: ['a', 'b'] },
    ]);
    expect(screen.getByText('Found 2 servers not managed by skillshare in Cursor')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Import' }));
    await waitFor(() => expect(mcpApi.import).toHaveBeenCalledWith({ from: 'cursor', root: '/work/app' }));
  });

  it('shows no import note when every server in the project is managed', () => {
    view({}, {}, [{ target: 'claude', path: '/.claude.json', names: ['global'] }]);
    expect(screen.queryByText(/not managed by skillshare/)).not.toBeInTheDocument();
  });

  it('counts Pi for a switch, as sync does', () => {
    view({}, { targets: ['opencode', 'pi'], servers: { gone: { disabled: true } } });
    expect(screen.getByRole('button', { name: 'Choose which agents get gone' })).toHaveTextContent('2/3');
  });

  const env: MCPCheckFinding = { level: 'error', check: 'env', target: '', subject: 'TOKEN', message: 'TOKEN is not set' };

  it("shows only this project's check findings", async () => {
    const user = userEvent.setup();
    vi.mocked(mcpCheckApi.run).mockResolvedValueOnce({ summary: { errors: 3, warnings: 0 }, servers: [
      { name: 'global', ok: false, findings: [env] },
      { name: 'api', project: '/work/app', ok: false, findings: [env] },
      { name: 'other', project: '/work/other', ok: false, findings: [env] },
    ] });
    view({}, { targets: ['claude'], servers: { api: { command: 'npx' } } });
    await user.click(screen.getByRole('button', { name: 'Check' }));
    expect(await screen.findByText('1 server has problems')).toBeInTheDocument();
    expect(screen.getByText('1 error')).toBeInTheDocument();
  });

  it('never marks a project server for a global server of the same name', async () => {
    const user = userEvent.setup();
    vi.mocked(mcpCheckApi.run).mockResolvedValueOnce({ summary: { errors: 1, warnings: 0 }, servers: [
      { name: 'docs', ok: false, findings: [env] },
      { name: 'docs', project: '/work/app', ok: true, findings: [] },
    ] });
    view({}, { targets: ['claude'], servers: { docs: { command: 'npx' } } });
    await user.click(screen.getByRole('button', { name: 'Check' }));
    expect(await screen.findByText('1 server has no problems')).toBeInTheDocument();
    expect(screen.queryByText('1 error')).not.toBeInTheDocument();
  });
});
