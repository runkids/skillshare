import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import HooksPage from './HooksPage';
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

describe('HooksPage', () => {
  const listHooks = vi.mocked(api.listHooks);
  const listManagedHooks = vi.mocked(api.managedHooks.list);
  const collectManagedHooks = vi.mocked(api.managedHooks.collect);

  beforeEach(() => {
    listHooks.mockReset();
    listManagedHooks.mockReset();
    collectManagedHooks.mockReset();
    listManagedHooks.mockResolvedValue({ hooks: [] });
    listHooks.mockResolvedValue({ warnings: [], hooks: [] });
  });

  function renderPage(initialEntry = '/hooks') {
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
              <Route path="/hooks" element={<HooksPage />} />
              <Route path="/hooks/discovered/:groupRef" element={<DiscoveredHookDetailPage />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders warnings and grouped hook actions including non-command Claude hooks', async () => {
    listHooks.mockResolvedValue({
      warnings: ['Skipped malformed hook in legacy config'],
      hooks: [
        {
          sourceTool: 'claude',
          scope: 'project',
          event: 'PreToolUse',
          matcher: 'Bash',
          actionType: 'command',
          command: './scripts/check.sh',
          path: '/tmp/project/.claude/settings.json',
        },
        {
          sourceTool: 'claude',
          scope: 'project',
          event: 'PreToolUse',
          matcher: 'Bash',
          actionType: 'http',
          url: 'https://example.com/hook',
          path: '/tmp/project/.claude/settings.json',
        },
        {
          sourceTool: 'claude',
          scope: 'project',
          event: 'PreToolUse',
          matcher: 'Write',
          actionType: 'prompt',
          prompt: 'Review the pending write',
          path: '/tmp/project/.claude/settings.local.json',
        },
        {
          sourceTool: 'claude',
          scope: 'project',
          event: 'PreToolUse',
          matcher: 'Edit',
          actionType: 'agent',
          prompt: 'Delegate to the review agent',
          path: '/tmp/project/.claude/settings.local.json',
        },
        {
          sourceTool: 'gemini',
          scope: 'user',
          event: 'BeforeTool',
          matcher: 'Read',
          actionType: 'command',
          command: './scripts/gemini.sh',
          path: '/tmp/home/.gemini/settings.json',
        },
      ],
    });

    renderPage('/hooks?mode=discovered');

    expect(await screen.findByRole('heading', { name: 'Hooks' })).toBeInTheDocument();
    expect(screen.getByText('Skipped malformed hook in legacy config')).toBeInTheDocument();

    expect(screen.getByRole('heading', { name: 'claude project PreToolUse Bash' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'gemini user BeforeTool Read' })).toBeInTheDocument();

    expect(screen.getByText('Bash')).toBeInTheDocument();
    expect(screen.getByText('./scripts/check.sh')).toBeInTheDocument();
    expect(screen.getByText('https://example.com/hook')).toBeInTheDocument();
    expect(screen.getByText('Review the pending write')).toBeInTheDocument();
    expect(screen.getByText('Delegate to the review agent')).toBeInTheDocument();
    expect(screen.getAllByText('/tmp/project/.claude/settings.local.json')).toHaveLength(2);
    expect(screen.getByText('./scripts/gemini.sh')).toBeInTheDocument();
  });

  it('links discovered hook groups to the dedicated detail page', async () => {
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
          command: './scripts/check.sh',
          path: '/tmp/project/.claude/settings.json',
          collectible: true,
        },
      ],
    });

    renderPage('/hooks?mode=discovered');

    await userEvent.click(await screen.findByRole('link', { name: 'claude project PreToolUse Bash' }));

    expect(await screen.findByRole('heading', { name: 'claude project PreToolUse Bash' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /collect & edit/i })).toBeInTheDocument();
  });

  it('shows an empty state when no hooks are returned', async () => {
    listManagedHooks.mockResolvedValue({
      hooks: [],
    });
    listHooks.mockResolvedValue({
      warnings: [],
      hooks: [],
    });

    renderPage();

    expect(await screen.findByText('No hooks found')).toBeInTheDocument();
  });

  it('shows managed and discovered modes and collects discovered hook groups', async () => {
    listManagedHooks.mockResolvedValue({
      hooks: [
        {
          id: 'claude/pre-tool-use/bash.yaml',
          tool: 'claude',
          event: 'PreToolUse',
          matcher: 'Bash',
          handlers: [
            {
              type: 'command',
              command: './bin/check',
            },
          ],
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
    collectManagedHooks.mockResolvedValue({
      created: ['claude/pre-tool-use/bash.yaml'],
      overwritten: [],
      skipped: [],
    });

    renderPage();

    expect(await screen.findByRole('tab', { name: /hooks \(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /discovered \(1\)/i })).toBeInTheDocument();
    expect(screen.getByText('claude/pre-tool-use/bash.yaml')).toBeInTheDocument();
    expect(listHooks).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByRole('tab', { name: /discovered/i }));
    await userEvent.click(await screen.findByRole('checkbox', { name: /collect bash/i }));
    await userEvent.click(screen.getByRole('button', { name: /collect selected/i }));

    expect(collectManagedHooks).toHaveBeenCalledWith({
      groupIds: ['claude:project:PreToolUse:Bash'],
      strategy: 'overwrite',
    });
  });
});
