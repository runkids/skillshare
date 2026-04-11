import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import RulesPage from './RulesPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      listRules: vi.fn(),
      managedRules: {
        list: vi.fn(),
        get: vi.fn(),
        create: vi.fn(),
        update: vi.fn(),
        remove: vi.fn(),
        collect: vi.fn(),
      },
    },
  };
});

describe('RulesPage', () => {
  const listRules = vi.mocked(api.listRules);
  const listManagedRules = vi.mocked(api.managedRules.list);
  const collectManagedRules = vi.mocked(api.managedRules.collect);
  const writeText = vi.fn();

  beforeEach(() => {
    listRules.mockReset();
    listManagedRules.mockReset();
    collectManagedRules.mockReset();
    writeText.mockReset();
    listManagedRules.mockResolvedValue({ rules: [] });
    listRules.mockResolvedValue({ warnings: [], rules: [] });
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function renderPage(initialEntry = '/rules') {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    return render(
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <MemoryRouter initialEntries={[initialEntry]}>
            <Routes>
              <Route path="/rules" element={<RulesPage />} />
              <Route path="/rules/discovered/*" element={<div>Discovered Rule Detail</div>} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders warnings and navigates to a full-page rule detail view', async () => {
    listRules.mockResolvedValue({
      warnings: ['Could not parse legacy rule set'],
      rules: [
        {
          name: 'alpha-rule',
          sourceTool: 'claude',
          scope: 'project',
          path: '/tmp/rules/alpha-rule.md',
          exists: true,
          content:
            '# Alpha Rule\n\n- first\n- [docs](https://example.com)\n- ![remote pixel](https://attacker.example/pixel)',
          size: 48,
          isScoped: false,
        },
      ],
    });
    writeText.mockResolvedValue(undefined);

    renderPage('/rules?mode=discovered');

    expect(await screen.findByText('Rules')).toBeInTheDocument();
    expect(screen.getByText('Could not parse legacy rule set')).toBeInTheDocument();
    expect(screen.getByText('alpha-rule')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /view rule/i }));

    expect(await screen.findByText('Discovered Rule Detail')).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('shows an empty state when no rules are returned', async () => {
    listManagedRules.mockResolvedValue({
      rules: [],
    });
    listRules.mockResolvedValue({
      warnings: [],
      rules: [],
    });

    renderPage();

    expect(await screen.findByText('No rules found')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /view rule/i })).not.toBeInTheDocument();
    });
  });

  it('shows managed and discovered modes and collects selected discovered rules', async () => {
    listManagedRules.mockResolvedValue({
      rules: [
        {
          id: 'claude/manual.md',
          tool: 'claude',
          name: 'manual.md',
          relativePath: 'claude/manual.md',
          content: '# Manual rule',
        },
      ],
    });
    listRules.mockResolvedValue({
      warnings: [],
      rules: [
        {
          id: 'claude:project:backend',
          name: 'backend.md',
          sourceTool: 'claude',
          scope: 'project',
          path: '/tmp/project/.claude/rules/backend.md',
          exists: true,
          collectible: true,
          content: '# Backend Rule',
          size: 14,
          isScoped: false,
        },
      ],
    });
    collectManagedRules.mockResolvedValue({
      created: ['claude/backend.md'],
      overwritten: [],
      skipped: [],
    });

    renderPage();

    expect(await screen.findByRole('tab', { name: /rules \(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /discovered \(1\)/i })).toBeInTheDocument();
    expect(screen.getByText('claude/manual.md')).toBeInTheDocument();
    expect(listRules).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByRole('tab', { name: /discovered/i }));
    await userEvent.click(await screen.findByRole('checkbox', { name: /collect backend\.md/i }));
    await userEvent.click(screen.getByRole('button', { name: /collect selected/i }));

    expect(collectManagedRules).toHaveBeenCalledWith({
      ids: ['claude:project:backend'],
      strategy: 'overwrite',
    });
  });
});
