import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import DiscoveredHookDetailPage from './DiscoveredHookDetailPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      listHooks: vi.fn(),
      managedHooks: {
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

describe('DiscoveredHookDetailPage', () => {
  const listHooks = vi.mocked(api.listHooks);
  const collectManagedHooks = vi.mocked(api.managedHooks.collect);

  beforeEach(() => {
    listHooks.mockReset();
    collectManagedHooks.mockReset();
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
          collectReason: 'Ready to import',
        },
        {
          groupId: 'claude:project:PreToolUse:Bash',
          sourceTool: 'claude',
          scope: 'project',
          event: 'PreToolUse',
          matcher: 'Bash',
          actionType: 'http',
          url: 'https://example.com/hook',
          path: '/tmp/project/.claude/settings.local.json',
          collectible: true,
        },
      ],
    });
  });

  function renderPage(initialEntry = '/hooks/discovered/claude%3Aproject%3APreToolUse%3ABash') {
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
              <Route path="/hooks/discovered/:groupRef" element={<DiscoveredHookDetailPage />} />
              <Route path="/hooks/manage/*" element={<div>Managed Hook Editor</div>} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders a discovered hook group and collects it into the managed editor flow', async () => {
    collectManagedHooks.mockResolvedValue({
      created: ['claude/pre-tool-use/bash.yaml'],
      overwritten: [],
      skipped: [],
    });

    renderPage();

    expect(await screen.findByRole('heading', { name: 'claude project PreToolUse Bash' })).toBeInTheDocument();
    expect(screen.getByText('Ready to import')).toBeInTheDocument();
    expect(screen.getByText('./bin/check')).toBeInTheDocument();
    expect(screen.getByText('https://example.com/hook')).toBeInTheDocument();
    expect(screen.getAllByText('/tmp/project/.claude/settings.json')).toHaveLength(2);
    expect(screen.getAllByText('/tmp/project/.claude/settings.local.json')).toHaveLength(2);
    expect(screen.queryByLabelText(/tool/i)).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /collect & edit/i }));

    expect(collectManagedHooks).toHaveBeenCalledWith({
      groupIds: ['claude:project:PreToolUse:Bash'],
      strategy: 'overwrite',
    });
    expect(await screen.findByText('Managed Hook Editor')).toBeInTheDocument();
  });
});
