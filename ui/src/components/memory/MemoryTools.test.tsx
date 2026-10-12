import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { api } from '../../api/client';
import type { Overview } from '../../api/types/resources';
import { AppProvider } from '../../context/AppContext';
import { queryKeys } from '../../lib/queryKeys';
import { ToastProvider } from '../Toast';
import MemoryTools from './MemoryTools';

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, getOverview: vi.fn() } };
});
const overview: Overview = { source: '/skills', skillCount: 0, agentCount: 0, topLevelCount: 0, targetCount: 0,
  mode: 'merge', version: 'dev', trackedRepos: [], isProjectMode: false, configDir: '/config' };
const renderTools = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><AppProvider><I18nProvider><ToastProvider>
    <MemoryTools root="/notes" disabled={false} />
  </ToastProvider></I18nProvider></AppProvider></QueryClientProvider>);
  return client;
};

beforeEach(() => {
  vi.clearAllMocks();
  sessionStorage.clear();
  vi.mocked(api.getOverview).mockReset().mockResolvedValue(overview);
});

it('offers three modes and keeps the full prompt collapsed when switching', async () => {
  const user = userEvent.setup(); renderTools();
  await waitFor(() => expect(screen.getByRole('button', { name: 'Memory tools' })).toBeEnabled());
  await user.click(screen.getByRole('button', { name: 'Memory tools' }));
  expect(screen.getByRole('button', { name: 'Try across sessions' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Dream' })).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Review this session' }));
  expect(screen.getByText('Nothing is saved until you approve')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'View full prompt' })).toHaveAttribute('aria-expanded', 'false');
  expect(screen.queryByText(/Review this session and its memory in four steps/)).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'View full prompt' }));
  expect(screen.getByText(/Review this session and its memory in four steps/)).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Dream' }));
  expect(screen.getByRole('button', { name: 'View full prompt' })).toHaveAttribute('aria-expanded', 'false');
});

it.each(['Review this session', 'Dream'])('copies the full %s prompt with project scope and protected writes', async (mode) => {
  vi.mocked(api.getOverview).mockResolvedValue({ ...overview, isProjectMode: true, projectRoot: '/project' });
  const user = userEvent.setup();
  const write = vi.spyOn(navigator.clipboard, 'writeText');
  renderTools();
  await waitFor(() => expect(screen.getByRole('button', { name: 'Memory tools' })).toBeEnabled());
  await user.click(screen.getByRole('button', { name: 'Memory tools' }));
  await user.click(screen.getByRole('button', { name: mode }));
  await user.click(screen.getByRole('button', { name: 'Copy for agent' }));
  const prompt = write.mock.calls[0][0];
  expect(prompt).toContain('/notes');
  expect(prompt).toContain('Scope: Project');
  expect(prompt).toContain('/project');
  expect(prompt).toContain('version-checked Skillshare CLI');
  expect(prompt).toContain('unavailable CLI or a conflict');
  expect(prompt).toContain('partial failures');
  expect(prompt).toContain('approval');
  expect(prompt).not.toContain('{root}');
  expect(screen.getByRole('button', { name: 'View full prompt' })).toHaveAttribute('aria-expanded', 'false');
});

it('does not disclose the trial value in the fresh-session read prompt', async () => {
  const user = userEvent.setup();
  const write = vi.spyOn(navigator.clipboard, 'writeText');
  renderTools();
  await waitFor(() => expect(screen.getByRole('button', { name: 'Memory tools' })).toBeEnabled());
  await user.click(screen.getByRole('button', { name: 'Memory tools' }));
  const buttons = screen.getAllByRole('button', { name: /^Copy for agent:/ });
  await user.click(buttons[0]); await user.click(buttons[1]); await user.click(buttons[2]);
  const [save, read, cleanup] = write.mock.calls.slice(-3).map(([prompt]) => prompt);
  const marker = save.match(/ss-\d+-\d+/)?.[0];
  expect(marker).toBeDefined();
  expect(cleanup).toContain(marker);
  expect(read).not.toContain(marker);
  expect(read).toContain('/notes');
});

it('keeps copying unavailable until the scope loads and then uses the confirmed project scope', async () => {
  let resolveOverview: ((value: Overview) => void) | undefined;
  vi.mocked(api.getOverview).mockReturnValue(new Promise((resolve) => { resolveOverview = resolve; }));
  const user = userEvent.setup();
  const write = vi.spyOn(navigator.clipboard, 'writeText');
  renderTools();
  expect(screen.getByRole('button', { name: 'Memory tools' })).toBeDisabled();
  resolveOverview?.({ ...overview, isProjectMode: true, projectRoot: '/project' });
  await waitFor(() => expect(screen.getByRole('button', { name: 'Memory tools' })).toBeEnabled());
  await user.click(screen.getByRole('button', { name: 'Memory tools' }));
  await user.click(screen.getByRole('button', { name: 'Dream' }));
  await user.click(screen.getByRole('button', { name: 'Copy for agent' }));
  expect(write.mock.calls[0][0]).toContain('Scope: Project');
  expect(write.mock.calls[0][0]).not.toContain('Scope: Global');
});

it.each(['failed overview', 'missing project root'])('does not copy a guessed scope with %s', async (state) => {
  if (state === 'failed overview') vi.mocked(api.getOverview).mockRejectedValue(new Error('Overview failed'));
  else vi.mocked(api.getOverview).mockResolvedValue({ ...overview, isProjectMode: true });
  const user = userEvent.setup();
  const write = vi.spyOn(navigator.clipboard, 'writeText');
  const client = renderTools();
  await waitFor(() => expect(client.getQueryState(queryKeys.overview)?.fetchStatus).toBe('idle'));
  expect(screen.getByRole('button', { name: 'Memory tools' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: 'Memory tools' }));
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(write).not.toHaveBeenCalled();
});
