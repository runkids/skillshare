import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { expect, it, vi } from 'vitest';
import { api } from '../api/client';
import AgentIcon from './AgentIcon';
import Layout from './Layout';

vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return { ...actual, api: { ...actual.api, listTargets: vi.fn(), diffExtras: vi.fn(() => new Promise(() => {})), gitStatus: vi.fn(() => new Promise(() => {})) } };
});
vi.mock('../hooks/useSharedQueries', async (load) => ({
  ...await load<typeof import('../hooks/useSharedQueries')>(),
  useOverviewQuery: () => ({}), useDiffQuery: () => ({}), useMcpQuery: () => ({}), useHooksQuery: () => ({}),
}));
vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../hooks/useGlobalShortcuts', () => ({ useGlobalShortcuts: () => ({ modifierHeld: false }) }));
vi.mock('../i18n', () => ({ useT: () => (key: string) => key }));
vi.mock('./tour', () => ({ useTour: () => ({ startTour: vi.fn() }) }));
vi.mock('./KeyboardShortcutsModal', () => ({ default: () => null }));
vi.mock('./DialogShell', () => ({ default: () => null }));
vi.mock('./ThemePopover', () => ({ default: () => null }));
vi.mock('./LanguagePopover', () => ({ default: () => null }));
vi.mock('./ScrollToTop', () => ({ default: () => null }));
vi.mock('./ShortcutHUD', () => ({ default: () => null }));
vi.mock('./UpdateDialog', () => ({ default: () => null }));

it('shows account logos from default targets while all targets are pending', async () => {
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }));
  let resolveAll!: (value: Awaited<ReturnType<typeof api.listTargets>>) => void;
  vi.mocked(api.listTargets).mockImplementation((scope) => scope === 'all'
    ? new Promise((resolve) => { resolveAll = resolve; })
    : Promise.resolve({ targets: [{ name: 'codex-2', agent: 'codex' }], sourceSkillCount: 0 } as Awaited<ReturnType<typeof api.listTargets>>));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <Routes><Route element={<Layout />}><Route path="/" element={<>
          <div data-testid="account"><AgentIcon target="codex-2" /></div>
          <div data-testid="project"><AgentIcon target="project@codex-2" /></div>
          <div data-testid="codex"><AgentIcon target="codex" /></div>
          <div data-testid="other"><AgentIcon target="project@other-account" /></div>
          <div data-testid="claude"><AgentIcon target="claude" /></div>
        </>} /></Route></Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  const logo = screen.getByTestId('codex').querySelector('img')!.getAttribute('src');
  await waitFor(() => expect(screen.getByTestId('account').querySelector('img')).toHaveAttribute('src', logo));
  expect(screen.getByTestId('project').querySelector('img')).toHaveAttribute('src', logo);
  expect(api.listTargets).toHaveBeenCalledWith('all');
  await act(async () => resolveAll({ targets: [{ name: 'other-account', agent: 'claude' }], sourceSkillCount: 0 } as Awaited<ReturnType<typeof api.listTargets>>));
  const claudeLogo = screen.getByTestId('claude').querySelector('img')!.getAttribute('src');
  await waitFor(() => expect(screen.getByTestId('other').querySelector('img')).toHaveAttribute('src', claudeLogo));
  expect(screen.getByTestId('account').querySelector('img')).toHaveAttribute('src', logo);
  vi.unstubAllGlobals();
});
