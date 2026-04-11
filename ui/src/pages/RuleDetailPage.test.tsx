import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api, type ManagedRuleDetailResponse } from '../api/client';
import RuleDetailPage from './RuleDetailPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
      api: {
        ...actual.api,
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

describe('RuleDetailPage', () => {
  const createManagedRule = vi.mocked(api.managedRules.create);
  const getManagedRule = vi.mocked(api.managedRules.get);
  const updateManagedRule = vi.mocked(api.managedRules.update);
  const removeManagedRule = vi.mocked(api.managedRules.remove);

  beforeEach(() => {
    createManagedRule.mockReset();
    getManagedRule.mockReset();
    updateManagedRule.mockReset();
    removeManagedRule.mockReset();
  });

  function renderPage(initialEntry = '/rules/new') {
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
              <Route path="/rules" element={<div>Rules</div>} />
              <Route path="/rules/new" element={<RuleDetailPage />} />
              <Route path="/rules/manage/*" element={<RuleDetailPage />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('creates and saves a managed rule with compiled previews', async () => {
    const createdRule: ManagedRuleDetailResponse = {
      rule: {
        id: 'claude/backend.md',
        tool: 'claude',
        name: 'backend.md',
        relativePath: 'claude/backend.md',
        content: '# Backend',
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/rules/backend.md',
              content: '# Backend',
              format: 'markdown',
            },
          ],
        },
      ],
    };
    createManagedRule.mockResolvedValue(createdRule);
    getManagedRule.mockResolvedValue(createdRule);
    updateManagedRule.mockResolvedValue({
      rule: {
        id: 'claude/backend.md',
        tool: 'claude',
        name: 'backend.md',
        relativePath: 'claude/backend.md',
        content: '# Backend updated',
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/rules/backend.md',
              content: '# Backend updated',
              format: 'markdown',
            },
          ],
        },
      ],
    });

    renderPage();

    await userEvent.type(await screen.findByLabelText(/tool/i), 'claude');
    await userEvent.type(screen.getByLabelText(/relative path/i), 'claude/backend.md');
    await userEvent.type(screen.getByLabelText(/content/i), '# Backend');
    await userEvent.click(screen.getByRole('button', { name: /save rule/i }));

    expect(createManagedRule).toHaveBeenCalledWith({
      tool: 'claude',
      relativePath: 'claude/backend.md',
      content: '# Backend',
    });
    expect(await screen.findByRole('button', { name: /delete rule/i }, { timeout: 3000 })).toBeInTheDocument();
    expect(screen.getByText('/tmp/home/.claude/rules/backend.md')).toBeInTheDocument();
    expect(screen.getByText(/compiled preview/i)).toBeInTheDocument();

    await userEvent.clear(screen.getByLabelText(/content/i));
    await userEvent.type(screen.getByLabelText(/content/i), '# Backend updated');
    await userEvent.click(screen.getByRole('button', { name: /save rule/i }));

    expect(updateManagedRule).toHaveBeenCalledWith('claude/backend.md', {
      tool: 'claude',
      relativePath: 'claude/backend.md',
      content: '# Backend updated',
    });
  });

  it('loads an existing managed rule, updates it, and deletes it', async () => {
    getManagedRule.mockResolvedValue({
      rule: {
        id: 'claude/backend.md',
        tool: 'claude',
        name: 'backend.md',
        relativePath: 'claude/backend.md',
        content: '# Backend',
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/rules/backend.md',
              content: '# Backend',
              format: 'markdown',
            },
          ],
          warnings: [],
        },
      ],
    });
    updateManagedRule.mockResolvedValue({
      rule: {
        id: 'claude/backend.md',
        tool: 'claude',
        name: 'backend.md',
        relativePath: 'claude/backend.md',
        content: '# Backend updated',
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/rules/backend.md',
              content: '# Backend updated',
              format: 'markdown',
            },
          ],
          warnings: [],
        },
      ],
    });
    removeManagedRule.mockResolvedValue({ success: true });

    renderPage('/rules/manage/claude/backend.md');

    expect(await screen.findByLabelText(/tool/i)).toHaveValue('claude');

    await userEvent.clear(screen.getByLabelText(/content/i));
    await userEvent.type(screen.getByLabelText(/content/i), '# Backend updated');
    await userEvent.click(screen.getByRole('button', { name: /save rule/i }));

    expect(updateManagedRule).toHaveBeenCalledWith('claude/backend.md', {
      tool: 'claude',
      relativePath: 'claude/backend.md',
      content: '# Backend updated',
    });
    expect(await screen.findByText(/saved rule/i)).toBeInTheDocument();
    expect(screen.getByText('/tmp/home/.claude/rules/backend.md')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /delete rule/i }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByRole('button', { name: /delete rule/i }));

    expect(removeManagedRule).toHaveBeenCalledWith('claude/backend.md');
  });

  it('redirects an empty manage route back to the rules list', async () => {
    renderPage('/rules/manage');

    expect(await screen.findByText('Rules')).toBeInTheDocument();
  });
});
