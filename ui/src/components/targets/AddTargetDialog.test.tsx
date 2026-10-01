import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import type { Target } from '../../api/client';
import { I18nProvider } from '../../i18n';
import AddTargetDialog from './AddTargetDialog';

vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { list: vi.fn().mockResolvedValue({ paths: {} }) } }));
vi.mock('../../api/client', async (load) => ({ ...await load<typeof import('../../api/client')>(), api: { addTarget: vi.fn(), addAgentConfigDir: vi.fn() } }));

const available = [
  { name: 'claude', path: '/home/me/.claude/skills', agentPath: '/home/me/.claude/agents', configDir: '/home/me/.claude', installed: true, detected: false },
  { name: 'codex', path: '/home/me/.agents/skills', configDir: '/home/me/.codex', installed: true, detected: false },
  { name: 'cursor', path: '/home/me/.cursor/skills', installed: false, detected: true },
];

describe('Add target dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.addAgentConfigDir).mockResolvedValue({ success: true });
    vi.mocked(api.addTarget).mockResolvedValue({ success: true });
  });

  it('does not sync skills by default to a tool that already reads a target\'s skills folder', async () => {
    const user = userEvent.setup();
    const reader = [...available, { name: 'gemini', path: '/home/me/.gemini/skills', installed: false, detected: true, readsFrom: ['universal'], instructionsFile: 'GEMINI.md', instructionsPath: '~/.gemini/GEMINI.md' }];
    const universal = { name: 'universal', path: '/home/me/.agents/skills', linkedCount: 9 } as Target;
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><AddTargetDialog available={reader} initial="gemini" existing={['universal']} targets={[universal]} onClose={vi.fn()} onAdded={vi.fn()} /></I18nProvider></QueryClientProvider>);
    expect(screen.getByText('Also reads universal')).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Sync skills' })).toHaveAttribute('aria-checked', 'false');
    expect(screen.queryByLabelText('Skills folder')).not.toBeInTheDocument();
    expect(screen.getByText('~/.gemini/GEMINI.md')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add gemini' }));
    await waitFor(() => expect(api.addTarget).toHaveBeenCalledWith('gemini', '/home/me/.gemini/skills', undefined, undefined, false));
  });

  it('leads with universal while it is not a target, naming the tools that read it', async () => {
    const user = userEvent.setup();
    const withShared = [...available, { name: 'universal', path: '/home/me/.agents/skills', installed: false, detected: false, readBy: ['codex', 'cursor'] }];
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><AddTargetDialog available={withShared} existing={['claude']} onClose={vi.fn()} onAdded={vi.fn()} /></I18nProvider></QueryClientProvider>);
    expect(screen.getByText('Shared folder')).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: /universal/ })).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByRole('img', { name: 'codex, cursor' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add universal' }));
    await waitFor(() => expect(api.addTarget).toHaveBeenCalledWith('universal', '/home/me/.agents/skills', undefined, undefined, true));
  });

  it('syncs skills by default to a tool that reads no other target\'s folder', async () => {
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><AddTargetDialog available={available} existing={['claude']} onClose={vi.fn()} onAdded={vi.fn()} /></I18nProvider></QueryClientProvider>);
    expect(screen.getByRole('switch', { name: 'Sync skills' })).toHaveAttribute('aria-checked', 'true');
    await user.click(screen.getByRole('button', { name: 'Add cursor' }));
    await waitFor(() => expect(api.addTarget).toHaveBeenCalledWith('cursor', '/home/me/.cursor/skills', undefined, undefined, true));
  });

  it('adds another config folder of an Agent that is already a target, showing where it writes', async () => {
    const user = userEvent.setup();
    const added = vi.fn();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><AddTargetDialog available={available} existing={['claude']} onClose={vi.fn()} onAdded={added} /></I18nProvider></QueryClientProvider>);
    await user.click(screen.getByRole('button', { name: /Another account/ }));
    expect(screen.getByRole('radio', { name: 'claude' })).toBeChecked();
    expect(screen.queryByRole('radio', { name: 'cursor' })).not.toBeInTheDocument();
    await user.type(screen.getByLabelText('Config folder'), '~/.claude-work');
    expect(screen.getByLabelText('Name')).toHaveValue('claude-work');
    expect(screen.getByText('~/.claude-work/skills')).toBeInTheDocument();
    expect(screen.getByText('~/.claude-work/agents')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add claude-work' }));
    await waitFor(() => expect(added).toHaveBeenCalledWith('claude-work'));
    expect(api.addAgentConfigDir).toHaveBeenCalledWith('claude-work', 'claude', '~/.claude-work', undefined);
  });

  // A compatible CLI such as omo can run an account's plugin commands.
  it('adds an account with another executable', async () => {
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><AddTargetDialog available={available} existing={['claude']} onClose={vi.fn()} onAdded={vi.fn()} /></I18nProvider></QueryClientProvider>);
    await user.click(screen.getByRole('button', { name: /Another account/ }));
    await user.type(screen.getByLabelText('Config folder'), '~/.claude-work');
    await user.type(screen.getByLabelText('Executable'), 'bin/claude');
    expect(screen.getByRole('button', { name: 'Add claude-work' })).toBeDisabled();
    await user.clear(screen.getByLabelText('Executable'));
    await user.type(screen.getByLabelText('Executable'), 'claude-beta');
    expect(screen.getByText('claude-beta')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Add claude-work' }));
    await waitFor(() => expect(api.addAgentConfigDir).toHaveBeenCalledWith('claude-work', 'claude', '~/.claude-work', 'claude-beta'));
  });

  // Codex reads the shared ~/.agents/skills, but an account's skills stay in its own folder.
  it('writes an account of an Agent whose skills live outside its config folder into that folder', async () => {
    const user = userEvent.setup();
    render(<QueryClientProvider client={new QueryClient()}><I18nProvider><AddTargetDialog available={available} existing={['claude', 'codex']} onClose={vi.fn()} onAdded={vi.fn()} /></I18nProvider></QueryClientProvider>);
    await user.click(screen.getByRole('button', { name: /Another account/ }));
    await user.click(screen.getByRole('radio', { name: 'codex' }));
    await user.type(screen.getByLabelText('Config folder'), '~/.codex-work');
    expect(screen.getByText('~/.codex-work/skills')).toBeInTheDocument();
  });
});
