import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type Project } from '../api/client';
import { hooksApi } from '../api/hooks';
import { mcpApi } from '../api/mcp';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import ProjectDetailPage from './ProjectDetailPage';

vi.mock('../api/client', async (load) => ({
  ...await load<typeof import('../api/client')>(),
  api: { listProjects: vi.fn(), listTargets: vi.fn(), availableTargets: vi.fn(), previewSyncMatrix: vi.fn() },
}));
vi.mock('../api/mcp', async (load) => ({ ...await load<typeof import('../api/mcp')>(), mcpApi: { list: vi.fn() } }));
vi.mock('../api/hooks', async (load) => ({ ...await load<typeof import('../api/hooks')>(), hooksApi: { list: vi.fn() } }));
vi.mock('../components/targets/TargetPiExtensions', () => ({ default: ({ name }: { name: string }) => <p>pi extensions of {name}</p> }));

const project = (targets: string[]): Project => ({
  root: '/home/me/acme', path: '/home/me/acme', name: 'acme', targets, skills: { mode: 'merge', include: [], exclude: [] }, agents: null, groups: [], missing: false, hasOwnConfig: false,
});

const view = (targets: string[], tab = '') => {
  vi.mocked(api.listProjects).mockResolvedValue({ projects: [project(targets)], convertible: [], tools: [{ name: 'pi', skillsPath: '.pi/skills', agentsPath: '' }, { name: 'claude', skillsPath: '.claude/skills', agentsPath: '.claude/agents' }] });
  render(
    <MemoryRouter initialEntries={[`/projects/${encodeURIComponent('/home/me/acme')}${tab}`]}>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><I18nProvider><ToastProvider>
        <Routes><Route path="/projects/:root" element={<ProjectDetailPage />} /></Routes>
      </ToastProvider></I18nProvider></QueryClientProvider>
    </MemoryRouter>,
  );
};

describe('project Extensions tab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 0 });
    vi.mocked(api.availableTargets).mockResolvedValue({ targets: [] });
    vi.mocked(api.previewSyncMatrix).mockResolvedValue({ entries: [] } as unknown as Awaited<ReturnType<typeof api.previewSyncMatrix>>);
    vi.mocked(mcpApi.list).mockRejectedValue(new Error('no mcp'));
    vi.mocked(hooksApi.list).mockRejectedValue(new Error('no hooks'));
  });

  it("shows the project's Pi extensions on its Extensions tab", async () => {
    view(['pi'], '?tab=extensions');
    expect(await screen.findByText('pi extensions of acme@pi')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
  });

  it('gives a project without Pi no Extensions tab', async () => {
    view(['claude'], '?tab=extensions');
    expect(await screen.findByRole('link', { name: 'Skills' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Extensions' })).not.toBeInTheDocument();
    expect(screen.queryByText(/pi extensions of/)).not.toBeInTheDocument();
  });
});
