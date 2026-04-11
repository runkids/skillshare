import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import ResourcesPage from './ResourcesPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      listSkills: vi.fn(),
      listRules: vi.fn(),
      listHooks: vi.fn(),
      getSyncMatrix: vi.fn(),
      deleteResource: vi.fn(),
      disableResource: vi.fn(),
      enableResource: vi.fn(),
      deleteRepo: vi.fn(),
      setSkillTargets: vi.fn(),
      batchSetTargets: vi.fn(),
      managedRules: {
        list: vi.fn(),
      },
      managedHooks: {
        list: vi.fn(),
      },
    },
  };
});

describe('ResourcesPage', () => {
  const listSkills = vi.mocked(api.listSkills);
  const listRules = vi.mocked(api.listRules);
  const listHooks = vi.mocked(api.listHooks);
  const listManagedRules = vi.mocked(api.managedRules.list);
  const listManagedHooks = vi.mocked(api.managedHooks.list);
  const getSyncMatrix = vi.mocked(api.getSyncMatrix);
  const getItem = vi.fn();
  const setItem = vi.fn();
  const observe = vi.fn();
  const disconnect = vi.fn();
  const unobserve = vi.fn();

  beforeEach(() => {
    listSkills.mockReset();
    listRules.mockReset();
    listHooks.mockReset();
    listManagedRules.mockReset();
    listManagedHooks.mockReset();
    getSyncMatrix.mockReset();
    getItem.mockReset();
    setItem.mockReset();
    observe.mockReset();
    disconnect.mockReset();
    unobserve.mockReset();

    listSkills.mockResolvedValue({
      resources: [
        {
          name: 'writer',
          kind: 'skill',
          flatName: 'writer',
          relPath: 'writer',
          sourcePath: '/tmp/skills/writer',
          isInRepo: false,
          disabled: false,
        },
        {
          name: 'reviewer',
          kind: 'agent',
          flatName: 'reviewer.md',
          relPath: 'reviewer.md',
          sourcePath: '/tmp/agents/reviewer.md',
          isInRepo: false,
          disabled: false,
        },
      ],
    });
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
          content: '# Backend rule',
          size: 14,
          isScoped: false,
          collectible: true,
        },
      ],
    });
    listManagedHooks.mockResolvedValue({
      hooks: [
        {
          id: 'claude/pre-tool-use/bash.yaml',
          tool: 'claude',
          event: 'PreToolUse',
          matcher: 'Bash',
          handlers: [{ type: 'command', command: './bin/check' }],
        },
      ],
    });
    listHooks.mockResolvedValue({
      warnings: [],
      hooks: [
        {
          groupId: 'claude:project:PreToolUse:Bash',
          sourceTool: 'claude',
          scope: 'project',
          event: 'PreToolUse',
          matcher: 'Bash',
          actionType: 'command',
          command: './bin/check',
          path: '/tmp/project/.claude/settings.json',
          collectible: true,
        },
      ],
    });
    getSyncMatrix.mockResolvedValue({ entries: [] });
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: {
        getItem,
        setItem,
      },
    });
    Object.defineProperty(window, 'ResizeObserver', {
      configurable: true,
      value: class ResizeObserver {
        observe = observe;
        disconnect = disconnect;
        unobserve = unobserve;
      },
    });
  });

  function renderPage(initialEntry = '/resources') {
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
              <Route path="/resources" element={<ResourcesPage />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders rules and hooks tabs inside the shared shell', async () => {
    renderPage('/resources?tab=rules&mode=managed');

    const ruleButton = await screen.findByRole('button', { name: /new rule/i });
    const resourceTabs = within(screen.getAllByRole('tablist')[0]);

    expect(await resourceTabs.findByRole('tab', { name: /skills/i })).toBeInTheDocument();
    expect(resourceTabs.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    expect(resourceTabs.getByRole('tab', { name: /^rules/i })).toBeInTheDocument();
    expect(resourceTabs.getByRole('tab', { name: /^hooks/i })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /resources/i })).toBeInTheDocument();
    expect(ruleButton.closest('a')).toHaveAttribute('href', '/rules/new');
    await waitFor(() => {
      expect(listManagedRules).toHaveBeenCalledTimes(1);
    });
    expect(listRules).not.toHaveBeenCalled();
    expect(listManagedHooks).not.toHaveBeenCalled();
    expect(listHooks).not.toHaveBeenCalled();

    await userEvent.click(resourceTabs.getByRole('tab', { name: /^hooks/i }));

    const hookButton = await screen.findByRole('button', { name: /new hook/i });
    expect(hookButton.closest('a')).toHaveAttribute('href', '/hooks/new');
    await waitFor(() => {
      expect(listManagedHooks).toHaveBeenCalledTimes(1);
    });
    expect(listHooks).not.toHaveBeenCalled();
  });
});
