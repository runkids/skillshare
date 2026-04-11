import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
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
      getSyncMatrix: vi.fn(),
      deleteResource: vi.fn(),
      disableResource: vi.fn(),
      enableResource: vi.fn(),
      deleteRepo: vi.fn(),
      setSkillTargets: vi.fn(),
      batchSetTargets: vi.fn(),
    },
  };
});

describe('ResourcesPage', () => {
  const listSkills = vi.mocked(api.listSkills);
  const getSyncMatrix = vi.mocked(api.getSyncMatrix);
  const getItem = vi.fn();
  const setItem = vi.fn();
  const observe = vi.fn();
  const disconnect = vi.fn();
  const unobserve = vi.fn();

  beforeEach(() => {
    listSkills.mockReset();
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

  it('switches the header create action between skills and agents', async () => {
    renderPage();

    const skillButton = await screen.findByRole('button', { name: /new skill/i });
    expect(skillButton.closest('a')).toHaveAttribute('href', '/resources/new');

    await userEvent.click(screen.getByRole('tab', { name: /agents/i }));

    const agentButton = await screen.findByRole('button', { name: /new agent/i });
    expect(agentButton.closest('a')).toHaveAttribute('href', '/resources/new?kind=agent');
  });
});
