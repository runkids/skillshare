import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { api, type Target } from '../api/client';
import { hooksApi } from '../api/hooks';
import { mcpApi } from '../api/mcp';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import TargetsPage from './TargetsPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../api/client', async (load) => ({
  ...await load<typeof import('../api/client')>(),
  api: { listTargets: vi.fn(), availableTargets: vi.fn(), listSharedInstructions: vi.fn() },
}));
vi.mock('../api/mcp', async (load) => ({ ...await load<typeof import('../api/mcp')>(), mcpApi: { list: vi.fn() } }));
vi.mock('../api/hooks', async (load) => ({ ...await load<typeof import('../api/hooks')>(), hooksApi: { list: vi.fn() } }));

const target = (over: Partial<Target>) => ({
  path: '/home/me/.claude/skills', mode: 'merge', targetNaming: 'flat', status: 'merged', linkedCount: 4, localCount: 0,
  include: [], exclude: [], expectedSkillCount: 4, skillsEnabled: true, ...over,
}) as Target;

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.availableTargets).mockResolvedValue({ targets: [] });
  vi.mocked(mcpApi.list).mockResolvedValue({ paths: { claude: '/home/me/.claude.json' }, source: { targets: ['claude'], servers: { a: {}, b: {} } } } as never);
  vi.mocked(hooksApi.list).mockResolvedValue({ paths: {}, source: { path: '/s', configPath: '', entries: {} }, plan: null } as never);
  vi.mocked(api.listSharedInstructions).mockResolvedValue({
    files: [], file_links: true,
    targets: [
      { name: 'claude', path: '/home/me/.claude/CLAUDE.md', import: true, exists: true, assigned: [{ name: 'general', mode: 'import', status: 'synced' }] },
      { name: 'codex', path: '/home/me/.codex/AGENTS.md', import: false, exists: true, assigned: [] },
    ],
  } as never);
  vi.mocked(api.listTargets).mockResolvedValue({ sourceSkillCount: 4, targets: [target({ name: 'claude' }), target({ name: 'codex', path: '/home/me/.agents/skills', skillsEnabled: false })] });
});

const view = () => render(
  <MemoryRouter initialEntries={['/targets']}>
    <QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><TargetsPage /></ToastProvider></I18nProvider></QueryClientProvider>
  </MemoryRouter>,
);

// Each thing a target gets is a link straight into the tab that manages it.
it('links each part of a target to its tab', async () => {
  view();
  const row = (await screen.findByRole('link', { name: 'claude' })).closest('.ss-r')!.parentElement!;
  expect(within(row).getByRole('link', { name: 'Skills4' })).toHaveAttribute('href', '/targets/claude');
  expect(await within(row).findByRole('link', { name: 'MCP2' })).toHaveAttribute('href', '/targets/claude?tab=mcp');
  expect(await within(row).findByRole('link', { name: 'CLAUDE.md' })).toHaveAttribute('href', '/targets/claude?tab=instructions');
});

it('shows the instruction file only when a shared file is connected', async () => {
  view();
  expect(await screen.findByRole('link', { name: 'CLAUDE.md' })).toHaveAttribute('href', '/targets/claude?tab=instructions');
  expect(screen.queryByRole('link', { name: 'AGENTS.md' })).not.toBeInTheDocument();
  // Skills off: no Skills door.
  expect(screen.queryByRole('link', { name: /^Skills/ })).toBeInTheDocument();
  expect(screen.getAllByRole('link', { name: /^Skills/ })).toHaveLength(1);
});
