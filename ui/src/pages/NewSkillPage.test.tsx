import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import NewSkillPage from './NewSkillPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      getTemplates: vi.fn(),
      listSkills: vi.fn(),
      createSkill: vi.fn(),
    },
  };
});

describe('NewSkillPage', () => {
  const getTemplates = vi.mocked(api.getTemplates);
  const listSkills = vi.mocked(api.listSkills);
  const createSkill = vi.mocked(api.createSkill);

  beforeEach(() => {
    getTemplates.mockReset();
    listSkills.mockReset();
    createSkill.mockReset();

    getTemplates.mockResolvedValue({
      patterns: [],
      categories: [],
    });
    listSkills.mockResolvedValue({ resources: [] });
    createSkill.mockResolvedValue({
      skill: {
        name: 'reviewer',
        kind: 'agent',
        flatName: 'reviewer.md',
        relPath: 'reviewer.md',
        sourcePath: '/tmp/agents/reviewer.md',
      },
      createdFiles: ['reviewer.md'],
    });
  });

  function renderPage(initialEntry = '/resources/new?kind=agent') {
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
              <Route path="/resources/new" element={<NewSkillPage />} />
              <Route path="/resources/:name" element={<div>Resource Detail</div>} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('creates agents from the shared new resource flow', async () => {
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Create New Agent' })).toBeInTheDocument();

    await userEvent.type(screen.getByPlaceholderText('reviewer or curriculum/reviewer'), 'reviewer');
    await userEvent.click(screen.getByRole('button', { name: /^next$/i }));
    await userEvent.click(screen.getByRole('button', { name: /create agent/i }));

    await waitFor(() => {
      expect(createSkill).toHaveBeenCalledWith({
        name: 'reviewer',
        kind: 'agent',
      });
    });

    expect(await screen.findByText('Resource Detail')).toBeInTheDocument();
  });
});
