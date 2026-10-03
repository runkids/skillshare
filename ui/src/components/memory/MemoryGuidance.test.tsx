import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import MemoryGuidance from './MemoryGuidance';

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, getMemoryGuidance: vi.fn(), planMemoryGuidance: vi.fn(), applyMemoryGuidance: vi.fn() } };
});
const renderGuidance = () => render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
  <I18nProvider><ToastProvider><MemoryRouter><MemoryGuidance initialized instructions="Read the notes" /></MemoryRouter></ToastProvider></I18nProvider>
</QueryClientProvider>);

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getMemoryGuidance).mockResolvedValue({ scope: 'global', instructions: 'Read the notes', targets: [{ name: 'codex', state: 'unconfigured' }] });
  vi.mocked(api.planMemoryGuidance).mockResolvedValue({ token: 'reviewed', changes: [{ path: '/shared/AGENTS.md', before: '# Existing rules', after: '# Existing rules\nMemory block', targets: ['codex'], created: false }], skipped: [], warnings: [] });
  vi.mocked(api.applyMemoryGuidance).mockResolvedValue({ success: true, applied: ['/shared/AGENTS.md'], errors: [], targets: [{ name: 'codex', state: 'configured' }] });
});

it('reviews the exact before/after text and applies only after confirmation', async () => {
  const user = userEvent.setup(); renderGuidance();
  await user.click(await screen.findByRole('button', { name: 'Connect to agents' }));
  await user.click(screen.getByRole('checkbox', { name: /codex/ }));
  await user.click(screen.getByRole('button', { name: 'Review changes' }));
  expect(api.applyMemoryGuidance).not.toHaveBeenCalled();
  expect(await screen.findByRole('button', { name: 'Apply changes' })).toBeInTheDocument();
  expect(screen.getByText('Before')).toBeInTheDocument();
  expect(screen.getByText('After')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Apply changes' }));
  expect(api.applyMemoryGuidance).toHaveBeenCalledWith(['codex'], 'reviewed');
});

it('returns to selection after a stale preview instead of reapplying silently', async () => {
  vi.mocked(api.applyMemoryGuidance).mockRejectedValue(new Error('Files changed since preview'));
  const user = userEvent.setup(); renderGuidance();
  await user.click(await screen.findByRole('button', { name: 'Connect to agents' }));
  await user.click(screen.getByRole('checkbox', { name: /codex/ }));
  await user.click(screen.getByRole('button', { name: 'Review changes' }));
  await user.click(await screen.findByRole('button', { name: 'Apply changes' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('Files changed since preview');
  expect(within(screen.getByRole('dialog')).getByRole('button', { name: 'Review changes' })).toBeInTheDocument();
  expect(api.applyMemoryGuidance).toHaveBeenCalledTimes(1);
});

it('rechecks target states when the dialog opens', async () => {
  vi.mocked(api.getMemoryGuidance).mockResolvedValueOnce({ scope: 'global', instructions: 'Read the notes', targets: [{ name: 'codex', state: 'configured' }] });
  const user = userEvent.setup(); renderGuidance();
  await user.click(await screen.findByRole('button', { name: 'Connect to agents' }));
  expect(await screen.findByRole('checkbox', { name: /codex/ })).toBeEnabled();
});
