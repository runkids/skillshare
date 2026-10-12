import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import MemoryOverview from './MemoryOverview';

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, getMemoryOverview: vi.fn(), getMemoryGuidance: vi.fn(), initMemory: vi.fn() } };
});

const instructions = { passive: '', active: '' };
const empty = { root: '/shared/memory', count: 0, initialized: false, notes: [] };
const renderOverview = () => render(
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <I18nProvider><ToastProvider><MemoryRouter><MemoryOverview /></MemoryRouter></ToastProvider></I18nProvider>
  </QueryClientProvider>,
);

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getMemoryGuidance).mockResolvedValue({ scope: 'global', instructions, targets: [
    { name: 'claude', state: 'configured' }, { name: 'codex', state: 'outdated' },
  ] });
});

it.each([
  { state: 'empty', initial: empty },
  { state: 'missing starters', initial: { ...empty, count: 1, notes: [{ path: 'decisions.md', title: 'Decisions', version: 'v', modified_at: '2026-10-12T08:00:00Z' }] } },
])('keeps initialization available after a failure and refreshes after success ($state)', async ({ initial }) => {
  vi.mocked(api.getMemoryOverview).mockResolvedValue(initial);
  vi.mocked(api.initMemory).mockRejectedValueOnce(new Error('Cannot create notes')).mockImplementationOnce(async () => {
    vi.mocked(api.getMemoryOverview).mockResolvedValue({ ...initial, initialized: true });
    return { success: true, root: '/shared/memory' };
  });
  const user = userEvent.setup();
  renderOverview();
  await user.click(await screen.findByRole('button', { name: 'Create memory' }));
  expect(await screen.findByText('Cannot create notes')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Create memory' }));
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Create memory' })).not.toBeInTheDocument());
  expect(api.initMemory).toHaveBeenCalledTimes(2);
  expect(api.getMemoryOverview).toHaveBeenCalledTimes(2);
});
