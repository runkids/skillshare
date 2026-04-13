import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import SyncPage from './SyncPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      sync: vi.fn(),
      diffStream: vi.fn(),
    },
  };
});

describe('SyncPage', () => {
  const sync = vi.mocked(api.sync);
  const diffStream = vi.mocked(api.diffStream);

  beforeEach(() => {
    sync.mockReset();
    diffStream.mockReset();
    diffStream.mockImplementation((_onDiscovering, _onStart, _onResult, onDone) => {
      onDone({
        diffs: [],
        ignored_count: 0,
        ignored_skills: [],
        ignore_root: '',
        ignore_repos: [],
      });
      return {
        close: vi.fn(),
      } as unknown as EventSource;
    });
  });

  function renderPage() {
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
          <MemoryRouter>
            <SyncPage />
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('shows resource-grouped sync results', async () => {
    sync.mockResolvedValue({
      results: [
        {
          resource: 'skills',
          target: 'claude',
          linked: ['a'],
          updated: [],
          skipped: [],
          pruned: [],
        },
        {
          resource: 'rules',
          target: 'claude',
          linked: ['claude/backend.md'],
          updated: [],
          skipped: [],
          pruned: [],
        },
        {
          resource: 'hooks',
          target: 'codex',
          linked: ['codex/pre-tool-use/bash.yaml'],
          updated: [],
          skipped: [],
          pruned: [],
        },
      ],
      ignored_count: 0,
      ignored_skills: [],
      ignore_root: '',
      ignore_repos: [],
    });

    renderPage();

    await userEvent.click(await screen.findByRole('button', { name: /sync now/i }));

    expect(await screen.findByText(/rules/i)).toBeInTheDocument();
    expect(screen.getByText(/hooks/i)).toBeInTheDocument();
  });
});
