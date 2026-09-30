import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { MCPPlan } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import { MCP_CHANGED, runSync } from '../sync/syncView';
import MCPSyncBox from './MCPSyncBox';

vi.mock('../sync/syncView', async (load) => ({ ...await load<typeof import('../sync/syncView')>(), runSync: vi.fn() }));

const own = { target: 'claude', path: '/work/app/.mcp.json', root: '/work/app', name: 'docs', action: 'add' };
const other = { target: 'cursor', path: '/home/me/.cursor/mcp.json', name: 'notes', action: 'update' };
const plan: MCPPlan = { revision: 'r1', sourcePath: '', blocked: false, changes: [own, other] };

// The project view passes only its own changes; the plan still covers every file.
const box = (p: MCPPlan = plan) =>
  render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSyncBox changes={[own]} roots={['/work', '/work/app']} plan={p} /></I18nProvider></QueryClientProvider></MemoryRouter>);

describe('MCP sync box', () => {
  beforeEach(() => { vi.mocked(runSync).mockReset(); });

  it('localizes every pending action in the sync rail', () => {
    const previous = localStorage.getItem('skillshare:locale');
    localStorage.setItem('skillshare:locale', 'zh-TW');
    try {
      const changes = ['add', 'remove', 'adopt', 'update'].map((action) => ({ ...own, name: `docs-${action}`, action }));
      render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><MCPSyncBox changes={changes} roots={[]} plan={{ ...plan, changes }} /></I18nProvider></QueryClientProvider></MemoryRouter>);
      for (const label of ['新增', '移除', '接管', '更新']) expect(screen.getByText(label)).toBeInTheDocument();
      for (const raw of ['add', 'remove', 'adopt', 'update']) expect(screen.queryByText(raw)).not.toBeInTheDocument();
    } finally {
      if (previous === null) localStorage.removeItem('skillshare:locale');
      else localStorage.setItem('skillshare:locale', previous);
    }
  });

  it('confirms every pending change, then writes only MCP with the reviewed plan', async () => {
    const user = userEvent.setup();
    vi.mocked(runSync).mockResolvedValue({ resources: undefined, failures: [] });
    box();
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    expect(screen.getByText('Also writes 1 change outside this project.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Sync Now' }));
    await waitFor(() => expect(runSync).toHaveBeenCalledWith({ resources: null, extras: false, mcp: plan, force: false }));
    expect(await screen.findByText('The MCP config files are written.')).toBeInTheDocument();
  });

  // Laid out like the Skills sync dialog, but a server is added or removed, never linked or pruned.
  it('lists a row per Agent file with the MCP change counts', async () => {
    const user = userEvent.setup();
    box();
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    expect(screen.getByText('1 to update')).toBeInTheDocument();
  });

  // Claude's off list for a project sits in ~/.claude.json: adding to it turns a server off, it adds none.
  it('counts a project switch as turning a server off', async () => {
    const user = userEvent.setup();
    box({ ...plan, changes: [own, { target: 'claude', path: '/home/me/.claude.json', name: 'context7', root: '/work/app', switch: true, action: 'add' }] });
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    expect(screen.getByText('1 to turn off')).toBeInTheDocument();
  });

  // Under nested roots the path fits both; the plan's root says which project the file is for.
  it("names a file's project by the plan's root", async () => {
    const user = userEvent.setup();
    box();
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    expect(screen.getByText('app · docs')).toBeInTheDocument();
  });

  it('spells out projects whose folders share a name', async () => {
    const user = userEvent.setup();
    const at = (root: string) => ({ target: 'cursor', path: `${root}/.cursor/mcp.json`, root, name: 'docs', action: 'add' });
    box({ ...plan, changes: [at('/a/app'), at('/b/app')] });
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    expect(screen.getByText('/b/app · docs')).toBeInTheDocument();
  });

  it('says so when the plan moved before it was written', async () => {
    const user = userEvent.setup();
    vi.mocked(runSync).mockRejectedValue(new Error(MCP_CHANGED));
    box();
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    await user.click(screen.getByRole('button', { name: 'Sync Now' }));
    expect(await screen.findByText(/The MCP changes shifted during the sync/)).toBeInTheDocument();
  });

  it('shows the plan notices, worded for the dashboard, before writing', async () => {
    const user = userEvent.setup();
    box({ ...plan, notices: ["Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP"] });
    await user.click(screen.getByRole('button', { name: 'Sync MCP' }));
    expect(screen.getByText(/On older Pi these servers stop loading until Pi is updated\./)).toBeInTheDocument();
  });

  it('sends a blocked plan to the Sync page instead of offering to write it', () => {
    box({ ...plan, blocked: true });
    expect(screen.queryByRole('button', { name: 'Sync MCP' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Review in Sync' })).toBeInTheDocument();
  });
});
