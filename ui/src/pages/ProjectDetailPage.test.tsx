import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { api, type ProjectList } from '../api/client';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import ProjectDetailPage from './ProjectDetailPage';

const idle = { data: undefined, isPending: false, error: null };
vi.mock('../hooks/useSharedQueries', async (load) => ({ ...await load<typeof import('../hooks/useSharedQueries')>(), useMcpQuery: () => idle, useHooksQuery: () => idle, useAvailableTargetsQuery: () => idle }));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return { ...actual, api: { ...actual.api, listProjects: vi.fn(), listTargets: vi.fn().mockResolvedValue({ targets: [], sourceSkillCount: 0 }), previewSyncMatrix: vi.fn().mockResolvedValue({ entries: [] }), removeProject: vi.fn(), saveProject: vi.fn() } };
});

const everything = { mode: 'merge', include: [], exclude: [] };
const tools = [{ name: 'claude', skillsPath: '.claude/skills', agentsPath: '' }, { name: 'cursor', skillsPath: '.cursor/skills', agentsPath: '' }];
const list = (origin: 'shared' | 'local', targets: string[]): ProjectList => ({
  projects: [{ root: '/work/app', path: '/work/app', name: 'app', origin, sharedCopy: origin === 'local', targets, skills: everything, agents: null, groups: [], missing: false, hasOwnConfig: false }],
  convertible: [], tools,
});

describe('Project detail page', () => {
  // A kept draft of the removed copy would be saved into config.yaml for every machine.
  it("drops this machine's settings once only its copy is removed", async () => {
    const user = userEvent.setup();
    vi.mocked(api.listProjects).mockResolvedValueOnce(list('local', ['claude', 'cursor'])).mockResolvedValue(list('shared', ['claude']));
    vi.mocked(api.removeProject).mockResolvedValue({ success: true, root: '/work/app', origin: 'local', originPath: '', remaining: true });
    render(
      <MemoryRouter initialEntries={['/projects/%2Fwork%2Fapp']}>
        <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider>
          <Routes><Route path="/projects/:root" element={<ProjectDetailPage />} /></Routes>
        </ToastProvider></I18nProvider></QueryClientProvider>
      </MemoryRouter>,
    );
    await user.click(await screen.findByRole('button', { name: 'Remove project' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Remove project' }));
    await waitFor(() => expect(api.listProjects).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled());
  });
});
