import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type Project, type Target } from '../api/client';
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
vi.mock('../components/targets/TargetOmpExtensions', () => ({ default: ({ name }: { name: string }) => <p>omp extensions of {name}</p> }));

const project = (targets: string[]): Project => ({
  root: '/home/me/acme', path: '/home/me/acme', name: 'acme', targets, skills: { mode: 'merge', include: [], exclude: [] }, agents: null, groups: [], missing: false, hasOwnConfig: false,
});

const piTarget: Target = {
  name: 'acme@pi', project: '/home/me/acme', path: '/home/me/acme/.pi/skills', mode: 'merge', targetNaming: 'flatten',
  status: 'not exist', linkedCount: 0, localCount: 0, include: [], exclude: [], expectedSkillCount: 0, skillsEnabled: true,
};

const view = (targets: string[], tab = '', overrides: Partial<Project> = {}) => {
  vi.mocked(api.listProjects).mockResolvedValue({ projects: [{ ...project(targets), ...overrides }], convertible: [], tools: [{ name: 'pi', skillsPath: '.pi/skills', agentsPath: '' }, { name: 'claude', skillsPath: '.claude/skills', agentsPath: '.claude/agents' }] });
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
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [piTarget], sourceSkillCount: 0 });
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

  it.each([
    { reason: 'missing directory', overrides: { missing: true } },
    { reason: 'agents only', overrides: { skills: null, agents: { mode: 'merge', include: [], exclude: [] } } },
    { reason: 'no resolved Pi target', overrides: {} },
  ])('hides Extensions for $reason even when Pi is declared', async ({ overrides }) => {
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 0 });
    view(['pi'], '?tab=extensions', overrides);
    expect(await screen.findByRole('link', { name: 'Skills' })).toHaveAttribute('aria-current', 'true');
    expect(screen.queryByRole('link', { name: 'Extensions' })).not.toBeInTheDocument();
    expect(screen.queryByText(/pi extensions of/)).not.toBeInTheDocument();
  });

  it('does not use another project\'s resolved Pi target', async () => {
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [{ ...piTarget, name: 'other@pi', project: '/home/me/other' }], sourceSkillCount: 0 });
    view(['pi'], '?tab=extensions');
    expect(await screen.findByRole('link', { name: 'Skills' })).toHaveAttribute('aria-current', 'true');
    expect(screen.queryByRole('link', { name: 'Extensions' })).not.toBeInTheDocument();
    expect(screen.queryByText(/pi extensions of/)).not.toBeInTheDocument();
  });

  it("shows the project's Oh My Pi extensions read-only when only omp is declared", async () => {
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [{ ...piTarget, name: 'acme@omp', path: '/home/me/acme/.omp/skills' }], sourceSkillCount: 0 });
    view(['omp'], '?tab=extensions');
    expect(await screen.findByText('omp extensions of acme@omp')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Oh My Pi' })).not.toBeInTheDocument();
  });

  it('lets a project with both Pi and Oh My Pi pick which extensions to show', async () => {
    const user = userEvent.setup();
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [piTarget, { ...piTarget, name: 'acme@omp', path: '/home/me/acme/.omp/skills' }], sourceSkillCount: 0 });
    view(['pi', 'omp'], '?tab=extensions');
    expect(await screen.findByText('pi extensions of acme@pi')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Oh My Pi' }));
    expect(screen.getByText('omp extensions of acme@omp')).toBeInTheDocument();
    expect(screen.queryByText(/pi extensions of/)).not.toBeInTheDocument();
  });

  it('gives a project without Pi no Extensions tab', async () => {
    view(['claude'], '?tab=extensions');
    expect(await screen.findByRole('link', { name: 'Skills' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Extensions' })).not.toBeInTheDocument();
    expect(screen.queryByText(/pi extensions of/)).not.toBeInTheDocument();
  });
});
