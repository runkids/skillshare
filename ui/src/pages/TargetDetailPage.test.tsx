import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, type Target } from '../api/client';
import { mcpApi } from '../api/mcp';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import TargetDetailPage from './TargetDetailPage';

vi.mock('../api/client', async (load) => ({
  ...await load<typeof import('../api/client')>(),
  api: {
    listTargets: vi.fn(), updateTarget: vi.fn(), availableTargets: vi.fn(), getTargetInstructions: vi.fn(),
    previewSyncMatrix: vi.fn(), listExtraExtensions: vi.fn(), listTargetFiles: vi.fn(),
  },
}));
// The file editor needs a data router; these tests look at the page around it.
vi.mock('../components/instructions/TargetInstructions', () => ({ default: () => null }));
vi.mock('../components/targetFiles/TargetFileTab', () => ({ default: () => null }));
vi.mock('../api/mcp', async (load) => ({ ...await load<typeof import('../api/mcp')>(), mcpApi: { list: vi.fn() } }));
vi.mock('../components/targets/TargetPiExtensions', () => ({ default: ({ name }: { name: string }) => <p>pi extensions of {name}</p> }));
vi.mock('../components/targets/TargetOmpExtensions', () => ({ default: ({ name }: { name: string }) => <p>omp extensions of {name}</p> }));

const target = (over: Partial<Target>) => ({
  path: '/home/me/.gemini/skills', mode: 'merge', targetNaming: 'flat', status: 'merged', linkedCount: 0, localCount: 0,
  include: [], exclude: [], expectedSkillCount: 0, skillsEnabled: true, ...over,
}) as Target;
const view = (name: string, targets: Target[]) => {
  vi.mocked(api.listTargets).mockResolvedValue({ targets, sourceSkillCount: 9 });
  render(
    <MemoryRouter initialEntries={[`/targets/${name}`]}>
      <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider>
        <Routes><Route path="/targets/:name" element={<TargetDetailPage />} /></Routes>
      </ToastProvider></I18nProvider></QueryClientProvider>
    </MemoryRouter>,
  );
};

describe('Target detail skills switch', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.availableTargets).mockResolvedValue({ targets: [] });
    vi.mocked(api.getTargetInstructions).mockResolvedValue({ supported: true, path: '/home/me/.gemini/GEMINI.md', read_order: [] } as never);
    vi.mocked(api.previewSyncMatrix).mockResolvedValue({ entries: [] });
    vi.mocked(api.listExtraExtensions).mockResolvedValue({ extensions: [] });
    vi.mocked(mcpApi.list).mockResolvedValue({ paths: {}, source: { targets: [], servers: {} } } as never);
    vi.mocked(api.updateTarget).mockResolvedValue({ success: true });
    vi.mocked(api.listTargetFiles).mockResolvedValue({ target: 'gemini', project: false, root: '', files: [] });
  });

  it('shows a target with skills off as not synced, naming the folder it reads, and resumes it', async () => {
    const user = userEvent.setup();
    view('gemini', [
      target({ name: 'gemini', skillsEnabled: false, skillsReadFrom: ['universal'] }),
      target({ name: 'universal', path: '/home/me/.agents/skills', linkedCount: 9, skillsAlsoReadBy: ['gemini'] }),
    ]);
    expect(await screen.findByText('Skills don’t sync to gemini')).toBeInTheDocument();
    expect(screen.getByText(/gemini still reads 9 skills from universal’s ~\/.agents\/skills/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Resume syncing' }));
    await waitFor(() => expect(api.updateTarget).toHaveBeenCalledWith('gemini', { skills_enabled: true }));
  });

  it('links the targets that read this skills folder instead of syncing their own', async () => {
    view('universal', [
      target({ name: 'gemini', skillsEnabled: false, skillsReadFrom: ['universal'] }),
      target({ name: 'universal', path: '/home/me/.agents/skills', linkedCount: 9, skillsAlsoReadBy: ['gemini'] }),
    ]);
    expect(await screen.findByRole('link', { name: 'gemini' })).toHaveAttribute('href', '/targets/gemini');
    expect(screen.getByRole('button', { name: 'Stop syncing skills' })).toBeInTheDocument();
  });

  it('names the targets a tool with skills on also reads, since it sees their skills twice', async () => {
    vi.mocked(api.availableTargets).mockResolvedValue({ targets: [{ name: 'pi', path: '/home/me/.pi/agent/skills', installed: true, detected: false, readsFrom: ['universal'] }] });
    view('pi', [
      target({ name: 'pi', path: '/home/me/.pi/agent/skills' }),
      target({ name: 'universal', path: '/home/me/.agents/skills', linkedCount: 9 }),
    ]);
    expect(await screen.findByText(/This target also reads the skills of/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'universal' })).toHaveAttribute('href', '/targets/universal');
  });

  it('shows the instruction file path under the title, like the other tabs', async () => {
    vi.mocked(api.getTargetInstructions).mockResolvedValue({ supported: true, target: 'gemini', path: '/home/me/.gemini/GEMINI.md', exists: true, content: '', read_order: [], convert: [], shared: [] } as never);
    view('gemini?tab=instructions', [target({ name: 'gemini' })]);
    expect(await screen.findByRole('heading', { name: 'gemini' })).toBeInTheDocument();
    expect(await screen.findByText('~/.gemini/GEMINI.md')).toBeInTheDocument();
  });
});

describe('Target detail file tabs', () => {
  const file = (path: string) => ({ path, abs: `/home/me/.pi/agent/${path}`, builtin: false, exists: true, size: 1 });
  const tabs = () => screen.findByRole('navigation', { name: 'Target sections' });
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.availableTargets).mockResolvedValue({ targets: [] });
    vi.mocked(api.getTargetInstructions).mockResolvedValue({ supported: true, path: '/home/me/.pi/agent/AGENTS.md', read_order: [] } as never);
    vi.mocked(api.previewSyncMatrix).mockResolvedValue({ entries: [] });
    vi.mocked(api.listExtraExtensions).mockResolvedValue({ extensions: [] });
    vi.mocked(mcpApi.list).mockResolvedValue({ paths: {}, source: { targets: [], servers: {} } } as never);
    vi.mocked(api.listTargetFiles).mockResolvedValue({ target: 'pi', project: false, root: '/home/me/.pi/agent', files: [file('SYSTEM.md'), file('APPEND_SYSTEM.md'), file('prompts/review.md')] });
  });

  it('shows three file tabs and puts the rest in a menu', async () => {
    const user = userEvent.setup();
    view('pi', [target({ name: 'pi' })]);
    const nav = await tabs();
    expect(await within(nav).findByRole('link', { name: 'APPEND_SYSTEM.md' })).toBeInTheDocument();
    expect(within(nav).queryByRole('link', { name: 'prompts/review.md' })).not.toBeInTheDocument();
    await user.click(within(nav).getByRole('button', { name: /\+1 more file/ }));
    expect(screen.getByRole('menuitem', { name: 'prompts/review.md' })).toHaveAttribute('href', '/targets/pi?tab=file&path=prompts%2Freview.md');
  });

  it('swaps an open hidden file into the last visible slot', async () => {
    view('pi?tab=file&path=prompts%2Freview.md', [target({ name: 'pi' })]);
    const nav = await tabs();
    expect(await within(nav).findByRole('link', { name: 'prompts/review.md' })).toHaveClass('on');
    expect(within(nav).queryByRole('link', { name: 'APPEND_SYSTEM.md' })).not.toBeInTheDocument();
    expect(screen.getByText('~/.pi/agent/prompts/review.md')).toBeInTheDocument();
  });

  it('offers adding a file only when the target has a place for them', async () => {
    view('pi', [target({ name: 'pi' })]);
    expect(await screen.findByRole('button', { name: 'Add file' })).toBeInTheDocument();
  });

  it('hides the add button when the target cannot add files', async () => {
    vi.mocked(api.listTargetFiles).mockResolvedValue({ target: 'pi', project: false, root: '', files: [file('SYSTEM.md')] });
    view('pi', [target({ name: 'pi' })]);
    const nav = await tabs();
    expect(await within(nav).findByRole('link', { name: 'SYSTEM.md' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Add file' })).not.toBeInTheDocument();
  });
  // Pi's extensions belong to each Pi target: pi and a Pi account; never other Agents.
  it.each(['pi', 'pi-work'])('gives %s an Extensions tab', async (name) => {
    view(name, [target({ name, agent: name === 'pi-work' ? 'pi' : undefined, path: '/home/me/.pi/agent/skills' })]);
    expect(await screen.findByRole('link', { name: 'Extensions' })).toHaveAttribute('href', '/targets/' + name + '?tab=extensions');
  });

  // Oh My Pi lists its extensions read-only on the same tab, for omp and an omp account.
  it.each(['omp', 'omp-work'])('opens the read-only Extensions tab of %s', async (name) => {
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target({ name, agent: name === 'omp-work' ? 'omp' : undefined, path: '/home/me/.omp/agent/skills' })], sourceSkillCount: 0 });
    render(
      <MemoryRouter initialEntries={[`/targets/${name}?tab=extensions`]}>
        <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider>
          <Routes><Route path="/targets/:name" element={<TargetDetailPage />} /></Routes>
        </ToastProvider></I18nProvider></QueryClientProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByText(`omp extensions of ${name}`)).toBeInTheDocument();
    expect(screen.queryByText(/pi extensions of/)).not.toBeInTheDocument();
  });

  it('gives other targets no Extensions tab', async () => {
    view('gemini', [target({ name: 'gemini' })]);
    await screen.findByRole('link', { name: 'Skills' });
    expect(screen.queryByRole('link', { name: 'Extensions' })).not.toBeInTheDocument();
  });

  it('opens the Extensions tab of a Pi target', async () => {
    vi.mocked(api.listTargets).mockResolvedValue({ targets: [target({ name: 'pi' })], sourceSkillCount: 0 });
    render(
      <MemoryRouter initialEntries={['/targets/pi?tab=extensions']}>
        <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider>
          <Routes><Route path="/targets/:name" element={<TargetDetailPage />} /></Routes>
        </ToastProvider></I18nProvider></QueryClientProvider>
      </MemoryRouter>,
    );
    expect(await screen.findByText('pi extensions of pi')).toBeInTheDocument();
  });
});
