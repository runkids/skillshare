import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
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

  beforeEach(() => {
    getTemplates.mockReset();
    listSkills.mockReset();

    getTemplates.mockResolvedValue({
      patterns: [
        {
          name: 'none',
          description: 'Start with just SKILL.md',
          scaffoldDirs: [],
        },
      ],
      categories: [],
    });
    listSkills.mockResolvedValue({ resources: [] });
  });

  function renderPage() {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <MemoryRouter>
            <NewSkillPage />
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('shows the frontmatter guide on the name step', async () => {
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Skill Name' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Reference' })).toBeInTheDocument();
    expect(screen.getByText(/all fields are optional/i)).toBeInTheDocument();
    expect(screen.getByText(/description is recommended/i)).toBeInTheDocument();
    expect(screen.getByText(/context:\s*fork/i)).toBeInTheDocument();
    expect(screen.getAllByText('allowed-tools').length).toBeGreaterThan(0);
    expect(screen.queryByRole('link', { name: /claude docs/i })).not.toBeInTheDocument();
  });
});
