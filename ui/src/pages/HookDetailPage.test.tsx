import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitForElementToBeRemoved, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api, type ManagedHookDetailResponse } from '../api/client';
import HookDetailPage from './HookDetailPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      managedHooks: {
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

describe('HookDetailPage', () => {
  const createManagedHook = vi.mocked(api.managedHooks.create);
  const getManagedHook = vi.mocked(api.managedHooks.get);
  const updateManagedHook = vi.mocked(api.managedHooks.update);
  const removeManagedHook = vi.mocked(api.managedHooks.remove);

  beforeEach(() => {
    createManagedHook.mockReset();
    getManagedHook.mockReset();
    updateManagedHook.mockReset();
    removeManagedHook.mockReset();
  });

  function renderPage(initialEntry = '/hooks/manage/claude/pre-tool-use/bash.yaml') {
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
              <Route path="/hooks" element={<div>Hooks</div>} />
              <Route path="/hooks/new" element={<HookDetailPage />} />
              <Route path="/hooks/manage/*" element={<HookDetailPage />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders managed hook groups and compiled previews', async () => {
    getManagedHook.mockResolvedValue({
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        targets: ['claude-work'],
        sourceType: 'tracked',
        disabled: true,
        handlers: [
          {
            type: 'command',
            command: './bin/check',
          },
        ],
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/settings.json',
              content: '{"hooks":{}}',
              format: 'json',
            },
          ],
        },
      ],
    });

    renderPage();

    expect(await screen.findByText(/compiled preview/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/event/i)).toHaveValue('PreToolUse');
    expect(screen.getByText('/tmp/home/.claude/settings.json')).toBeInTheDocument();
  });

  it('creates a managed hook and shows the compiled preview', async () => {
    const createdHook: ManagedHookDetailResponse = {
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        targets: ['claude-work'],
        sourceType: 'tracked',
        disabled: true,
        handlers: [
          {
            type: 'command',
            command: './bin/check',
          },
        ],
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/settings.json',
              content: '{"hooks":{}}',
              format: 'json',
            },
          ],
          warnings: [],
        },
      ],
    };
    createManagedHook.mockResolvedValue(createdHook);
    getManagedHook.mockResolvedValue(createdHook);
    renderPage('/hooks/new');

    await userEvent.type(await screen.findByLabelText(/tool/i), 'claude');
    await userEvent.type(screen.getByLabelText(/event/i), 'PreToolUse');
    await userEvent.type(screen.getByLabelText(/matcher/i), 'Bash');
    await userEvent.type(screen.getByLabelText(/command/i), './bin/check');
    await userEvent.click(screen.getByRole('button', { name: /save hook/i }));

    expect(createManagedHook).toHaveBeenCalledWith({
      tool: 'claude',
      event: 'PreToolUse',
      matcher: 'Bash',
      handlers: [
        {
          type: 'command',
          command: './bin/check',
          url: undefined,
          prompt: undefined,
          timeout: undefined,
          timeoutSec: undefined,
          statusMessage: undefined,
        },
      ],
    });
    expect(screen.getByText('/tmp/home/.claude/settings.json')).toBeInTheDocument();
    expect(screen.getByText(/compiled preview/i)).toBeInTheDocument();
  });

  it('uses the custom handler type select with a single timeout field for claude hooks', async () => {
    const createdHook: ManagedHookDetailResponse = {
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        targets: ['claude-work'],
        sourceType: 'tracked',
        disabled: true,
        handlers: [
          {
            type: 'prompt',
            prompt: 'Review the action',
          },
        ],
      },
      previews: [],
    };
    createManagedHook.mockResolvedValue(createdHook);
    getManagedHook.mockResolvedValue(createdHook);

    renderPage('/hooks/new');

    await userEvent.type(screen.getByLabelText(/tool/i), 'claude');
    await userEvent.type(screen.getByLabelText(/event/i), 'PreToolUse');
    await userEvent.type(screen.getByLabelText(/matcher/i), 'Bash');

    const typeControl = screen.getByRole('combobox', { name: /type/i });
    expect(typeControl.tagName).toBe('BUTTON');
    await userEvent.click(typeControl);
    await userEvent.click(screen.getByRole('option', { name: /^prompt$/i }));

    expect(typeControl).toHaveTextContent('prompt');
    expect(screen.getByLabelText(/prompt/i).tagName).toBe('TEXTAREA');
    expect(screen.getByLabelText(/prompt/i)).toHaveAttribute('rows', '5');
    expect(screen.getByLabelText(/^timeout$/i)).not.toHaveAttribute('inputmode');
    expect(screen.queryByLabelText(/timeout sec/i)).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText(/prompt/i), 'Review the action');
    await userEvent.type(screen.getByLabelText(/^timeout$/i), '15s');
    await userEvent.click(screen.getByRole('button', { name: /save hook/i }));

    expect(createManagedHook).toHaveBeenCalledWith({
      tool: 'claude',
      event: 'PreToolUse',
      matcher: 'Bash',
      handlers: [
        {
          type: 'prompt',
          command: undefined,
          url: undefined,
          prompt: 'Review the action',
          timeout: '15s',
          timeoutSec: undefined,
          statusMessage: undefined,
        },
      ],
    });
  });

  it('uses a single numeric timeout field for codex hooks', async () => {
    getManagedHook.mockResolvedValue({
      hook: {
        id: 'codex/pre-tool-use/bash.yaml',
        tool: 'codex',
        event: 'PreToolUse',
        matcher: 'Bash',
        targets: ['codex-work'],
        sourceType: 'tracked',
        disabled: false,
        handlers: [
          {
            type: 'command',
            command: './bin/check',
            timeout: '30',
          },
        ],
      },
      previews: [],
    });
    updateManagedHook.mockResolvedValue({
      hook: {
        id: 'codex/pre-tool-use/bash.yaml',
        tool: 'codex',
        event: 'PreToolUse',
        matcher: 'Bash',
        targets: ['codex-work'],
        sourceType: 'tracked',
        disabled: false,
        handlers: [
          {
            type: 'command',
            command: './bin/check',
            timeoutSec: 45,
          },
        ],
      },
      previews: [],
    });

    renderPage('/hooks/manage/codex/pre-tool-use/bash.yaml');

    const timeoutInput = await screen.findByLabelText(/timeout sec/i);
    expect(timeoutInput).toHaveValue('30');
    expect(timeoutInput).toHaveAttribute('type', 'text');
    expect(timeoutInput).toHaveAttribute('inputmode', 'numeric');
    expect(timeoutInput).toHaveAttribute('pattern', '[0-9]*');
    expect(timeoutInput).toHaveAttribute('placeholder', '30');
    expect(screen.queryByLabelText(/^timeout$/i)).not.toBeInTheDocument();

    await userEvent.clear(timeoutInput);
    await userEvent.type(timeoutInput, '45');
    await userEvent.click(screen.getByRole('button', { name: /save hook/i }));

    expect(updateManagedHook).toHaveBeenCalledWith('codex/pre-tool-use/bash.yaml', {
      tool: 'codex',
      event: 'PreToolUse',
      matcher: 'Bash',
      targets: ['codex-work'],
      sourceType: 'tracked',
      disabled: false,
      handlers: [
        {
          type: 'command',
          command: './bin/check',
          url: undefined,
          prompt: undefined,
          timeout: undefined,
          timeoutSec: 45,
          statusMessage: undefined,
        },
      ],
    });
  });

  it('animates added handlers in and removed handlers out', async () => {
    renderPage('/hooks/new');

    await userEvent.click(screen.getByRole('button', { name: /add handler/i }));

    const secondHeading = await screen.findByRole('heading', { name: /handler 2/i });
    const secondCard = secondHeading.closest('div.relative.p-4');
    expect(secondCard).toHaveClass('animate-sketch-in');

    await userEvent.click(within(secondCard as HTMLElement).getByRole('button', { name: /remove/i }));
    expect(screen.getByRole('heading', { name: /handler 2/i })).toBeInTheDocument();
    expect(secondCard).toHaveClass('opacity-0');

    await waitForElementToBeRemoved(() => screen.getByRole('heading', { name: /handler 2/i }));
  });

  it('updates and deletes an existing managed hook', async () => {
    getManagedHook.mockResolvedValue({
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        targets: ['claude-work'],
        sourceType: 'tracked',
        disabled: true,
        handlers: [
          {
            type: 'command',
            command: './bin/check',
          },
        ],
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/settings.json',
              content: '{"hooks":{}}',
              format: 'json',
            },
          ],
          warnings: [],
        },
      ],
    });
    updateManagedHook.mockResolvedValue({
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        handlers: [
          {
            type: 'command',
            command: './bin/check --updated',
          },
        ],
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/settings.json',
              content: '{"hooks":{"updated":true}}',
              format: 'json',
            },
          ],
          warnings: [],
        },
      ],
    });
    removeManagedHook.mockResolvedValue({ success: true });

    renderPage();

    expect(await screen.findByLabelText(/tool/i)).toHaveValue('claude');
    await userEvent.clear(screen.getByLabelText(/command/i));
    await userEvent.type(screen.getByLabelText(/command/i), './bin/check --updated');
    await userEvent.click(screen.getByRole('button', { name: /save hook/i }));

    expect(updateManagedHook).toHaveBeenCalledWith('claude/pre-tool-use/bash.yaml', {
      tool: 'claude',
      event: 'PreToolUse',
      matcher: 'Bash',
      targets: ['claude-work'],
      sourceType: 'tracked',
      disabled: true,
      handlers: [
        {
          type: 'command',
          command: './bin/check --updated',
          url: undefined,
          prompt: undefined,
          timeout: undefined,
          timeoutSec: undefined,
          statusMessage: undefined,
        },
      ],
    });
    expect(await screen.findByText(/saved hook/i)).toBeInTheDocument();
    expect(screen.getByText('/tmp/home/.claude/settings.json')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /delete hook/i }));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByRole('button', { name: /delete hook/i }));

    expect(removeManagedHook).toHaveBeenCalledWith('claude/pre-tool-use/bash.yaml');
  });

  it('navigates to the renamed managed hook when save returns a new id', async () => {
    getManagedHook.mockResolvedValue({
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        targets: ['claude-work'],
        sourceType: 'tracked',
        disabled: true,
        handlers: [
          {
            type: 'command',
            command: './bin/check',
          },
        ],
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/settings.json',
              content: '{"hooks":{}}',
              format: 'json',
            },
          ],
        },
      ],
    });
    updateManagedHook.mockResolvedValue({
      hook: {
        id: 'claude/post-tool-use/edit.yaml',
        tool: 'claude',
        event: 'PostToolUse',
        matcher: 'Edit',
        targets: ['claude-work'],
        sourceType: 'tracked',
        disabled: true,
        handlers: [
          {
            type: 'command',
            command: './bin/check',
          },
        ],
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/settings.json',
              content: '{"hooks":{"renamed":true}}',
              format: 'json',
            },
          ],
        },
      ],
    });

    renderPage();

    const eventInput = await screen.findByLabelText(/event/i);
    await userEvent.clear(eventInput);
    await userEvent.type(eventInput, 'PostToolUse');
    const matcherInput = screen.getByLabelText(/matcher/i);
    await userEvent.clear(matcherInput);
    await userEvent.type(matcherInput, 'Edit');
    await userEvent.click(screen.getByRole('button', { name: /save hook/i }));

    expect(updateManagedHook).toHaveBeenCalledWith('claude/pre-tool-use/bash.yaml', {
      tool: 'claude',
      event: 'PostToolUse',
      matcher: 'Edit',
      targets: ['claude-work'],
      sourceType: 'tracked',
      disabled: true,
      handlers: [
        {
          type: 'command',
          command: './bin/check',
          url: undefined,
          prompt: undefined,
          timeout: undefined,
          timeoutSec: undefined,
          statusMessage: undefined,
        },
      ],
    });
    expect(screen.getByLabelText(/matcher/i)).toHaveValue('Edit');
  });

  it('redirects an empty manage route back to the hooks list', async () => {
    renderPage('/hooks/manage');

    expect(await screen.findByText('Hooks')).toBeInTheDocument();
  });
});
