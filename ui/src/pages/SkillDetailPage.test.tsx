import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import ResourceDetailPage from './ResourceDetailPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      getResource: vi.fn(),
      listSkills: vi.fn(),
      auditSkill: vi.fn(),
      diff: vi.fn(),
      deleteSkill: vi.fn(),
      deleteRepo: vi.fn(),
      update: vi.fn(),
    },
  };
});

describe('SkillDetailPage', () => {
  const getResource = vi.mocked(api.getResource);
  const listSkills = vi.mocked(api.listSkills);
  const auditSkill = vi.mocked(api.auditSkill);
  const diff = vi.mocked(api.diff);

  beforeEach(() => {
    getResource.mockReset();
    listSkills.mockReset();
    auditSkill.mockReset();
    diff.mockReset();
    listSkills.mockResolvedValue({ resources: [] });
    auditSkill.mockResolvedValue({
      result: {
        skillName: 'cli-e2e-test',
        findings: [],
        riskScore: 0,
        riskLabel: 'low',
        threshold: 'warn',
        isBlocked: false,
      },
      summary: {
        total: 0,
        passed: 0,
        warning: 0,
        failed: 0,
        critical: 0,
        high: 0,
        medium: 0,
        low: 0,
        info: 0,
        threshold: 'warn',
        riskScore: 0,
        riskLabel: 'low',
      },
    });
    diff.mockResolvedValue({
      diffs: [],
      ignored_count: 0,
      ignored_skills: [],
      ignore_root: '',
      ignore_repos: [],
    });
  });

  function renderPage(initialEntry = '/resources/cli-e2e-test') {
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
              <Route path="/resources/:name" element={<ResourceDetailPage />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders server-provided skill stats instead of recomputing them in the browser', async () => {
    getResource.mockResolvedValue({
      resource: {
        name: 'cli-e2e-test',
        kind: 'skill',
        flatName: 'cli-e2e-test',
        relPath: 'cli-e2e-test',
        sourcePath: '/tmp/skills/cli-e2e-test',
        isInRepo: false,
      },
      skillMdContent: 'alpha beta\ncharlie delta',
      files: ['SKILL.md'],
      stats: {
        wordCount: 123,
        lineCount: 45,
        tokenCount: 67,
      },
    });

    renderPage();

    expect(await screen.findByRole('heading', { name: 'cli-e2e-test' })).toBeInTheDocument();
    expect(screen.getByText('123 words')).toBeInTheDocument();
    expect(screen.getByText('45 lines')).toBeInTheDocument();
    expect(screen.getByText('67 tokens')).toBeInTheDocument();
    expect(screen.queryByText('2 words')).not.toBeInTheDocument();
    expect(screen.queryByText('2 lines')).not.toBeInTheDocument();
    expect(screen.queryByText('4 tokens')).not.toBeInTheDocument();
  });
});
