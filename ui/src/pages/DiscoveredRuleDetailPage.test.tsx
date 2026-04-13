import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import DiscoveredRuleDetailPage from './DiscoveredRuleDetailPage';

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

describe('DiscoveredRuleDetailPage', () => {
  const listRules = vi.mocked(api.listRules);
  const collectManagedRules = vi.mocked(api.managedRules.collect);

  beforeEach(() => {
    listRules.mockReset();
    collectManagedRules.mockReset();
    listRules.mockResolvedValue({
      warnings: [],
      rules: [
        {
          id: 'claude:project:alpha',
          name: 'alpha-rule',
          sourceTool: 'claude',
          scope: 'project',
          path: '/tmp/rules/alpha-rule.md',
          exists: true,
          collectible: true,
          collectReason: 'Ready to import',
          content:
            '# Alpha Rule\n\n- first\n- [docs](https://example.com)\n- ![remote pixel](https://attacker.example/pixel)',
          size: 48,
          isScoped: false,
          stats: {
            wordCount: 99,
            lineCount: 12,
            tokenCount: 42,
          },
        },
      ],
    });
  });

  function renderPage(initialEntry = '/rules/discovered/claude%3Aproject%3Aalpha') {
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
              <Route path="/rules/discovered/:ruleRef" element={<DiscoveredRuleDetailPage />} />
              <Route path="/rules/manage/*" element={<div>Managed Rule Editor</div>} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders a full discovered rule detail page and collects into the editor flow', async () => {
    collectManagedRules.mockResolvedValue({
      created: ['claude/alpha-rule.md'],
      overwritten: [],
      skipped: [],
    });

    renderPage();

    expect(await screen.findByRole('heading', { name: 'alpha-rule' })).toBeInTheDocument();
    expect(screen.getByText('Alpha Rule')).toBeInTheDocument();
    expect(screen.getByText('docs')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'docs' })).not.toBeInTheDocument();
    expect(screen.getByText('[image: remote pixel]')).toBeInTheDocument();
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(screen.getByText('48 bytes')).toBeInTheDocument();
    expect(screen.getByText('99 words')).toBeInTheDocument();
    expect(screen.getByText('12 lines')).toBeInTheDocument();
    expect(screen.getByText('42 tokens')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /collect & edit/i }));

    expect(collectManagedRules).toHaveBeenCalledWith({
      ids: ['claude:project:alpha'],
      strategy: 'overwrite',
    });
    expect(await screen.findByText('Managed Rule Editor')).toBeInTheDocument();
  });
});
