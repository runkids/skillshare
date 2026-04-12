import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import ResourcesPage from './ResourcesPage';

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      listSkills: vi.fn(),
      listRules: vi.fn(),
      listHooks: vi.fn(),
      getSyncMatrix: vi.fn(),
      deleteResource: vi.fn(),
      disableResource: vi.fn(),
      enableResource: vi.fn(),
      deleteRepo: vi.fn(),
      setSkillTargets: vi.fn(),
      batchSetTargets: vi.fn(),
      availableTargets: vi.fn(),
      managedRules: {
        list: vi.fn(),
        setTargets: vi.fn(),
        setDisabled: vi.fn(),
        remove: vi.fn(),
      },
      managedHooks: {
        list: vi.fn(),
        setTargets: vi.fn(),
        setDisabled: vi.fn(),
        remove: vi.fn(),
      },
    },
  };
});

describe('ResourcesPage', () => {
  const listSkills = vi.mocked(api.listSkills);
  const listRules = vi.mocked(api.listRules);
  const listHooks = vi.mocked(api.listHooks);
  const listManagedRules = vi.mocked(api.managedRules.list);
  const listManagedHooks = vi.mocked(api.managedHooks.list);
  const setManagedRuleTargets = vi.mocked(api.managedRules.setTargets);
  const setManagedRuleDisabled = vi.mocked(api.managedRules.setDisabled);
  const removeManagedRule = vi.mocked(api.managedRules.remove);
  const setManagedHookTargets = vi.mocked(api.managedHooks.setTargets);
  const setManagedHookDisabled = vi.mocked(api.managedHooks.setDisabled);
  const removeManagedHook = vi.mocked(api.managedHooks.remove);
  const getSyncMatrix = vi.mocked(api.getSyncMatrix);
  const availableTargets = vi.mocked(api.availableTargets);
  const getItem = vi.fn();
  const setItem = vi.fn();
  const observe = vi.fn();
  const disconnect = vi.fn();
  const unobserve = vi.fn();

  beforeEach(() => {
    listSkills.mockReset();
    listRules.mockReset();
    listHooks.mockReset();
    listManagedRules.mockReset();
    listManagedHooks.mockReset();
    setManagedRuleTargets.mockReset();
    setManagedRuleDisabled.mockReset();
    removeManagedRule.mockReset();
    setManagedHookTargets.mockReset();
    setManagedHookDisabled.mockReset();
    removeManagedHook.mockReset();
    getSyncMatrix.mockReset();
    availableTargets.mockReset();
    getItem.mockReset();
    setItem.mockReset();
    observe.mockReset();
    disconnect.mockReset();
    unobserve.mockReset();

    listSkills.mockResolvedValue({
      resources: [
        {
          name: 'writer',
          kind: 'skill',
          flatName: 'writer',
          relPath: 'writer',
          sourcePath: '/tmp/skills/writer',
          isInRepo: false,
          disabled: false,
        },
        {
          name: 'reviewer',
          kind: 'agent',
          flatName: 'reviewer.md',
          relPath: 'reviewer.md',
          sourcePath: '/tmp/agents/reviewer.md',
          isInRepo: false,
          disabled: false,
        },
      ],
    });
    listManagedRules.mockResolvedValue({
      rules: [
        {
          id: 'claude/manual.md',
          tool: 'claude',
          name: 'manual.md',
          relativePath: 'claude/manual.md',
          content: '# Manual rule',
          targets: ['claude'],
          sourceType: 'local',
          disabled: false,
        },
      ],
    });
    listRules.mockResolvedValue({
      warnings: [],
      rules: [
        {
          id: 'claude:project:backend',
          name: 'backend.md',
          sourceTool: 'claude',
          scope: 'project',
          path: '/tmp/project/.claude/rules/backend.md',
          exists: true,
          content: '# Backend rule',
          size: 14,
          isScoped: false,
          collectible: true,
        },
      ],
    });
    listManagedHooks.mockResolvedValue({
      hooks: [
        {
          id: 'claude/pre-tool-use/bash.yaml',
          tool: 'claude',
          event: 'PreToolUse',
          matcher: 'Bash',
          handlers: [{ type: 'command', command: './bin/check' }],
          targets: ['claude'],
          sourceType: 'local',
          disabled: false,
        },
      ],
    });
    listHooks.mockResolvedValue({
      warnings: [],
      hooks: [
        {
          groupId: 'claude:project:PreToolUse:Bash',
          sourceTool: 'claude',
          scope: 'project',
          event: 'PreToolUse',
          matcher: 'Bash',
          actionType: 'command',
          command: './bin/check',
          path: '/tmp/project/.claude/settings.json',
          collectible: true,
        },
      ],
    });
    availableTargets.mockResolvedValue({
      targets: [
        { name: 'claude', path: '/tmp/targets/claude', installed: true, detected: true },
        { name: 'cursor', path: '/tmp/targets/cursor', installed: true, detected: true },
      ],
    });
    setManagedRuleTargets.mockResolvedValue({
      rule: {
        id: 'claude/manual.md',
        tool: 'claude',
        name: 'manual.md',
        relativePath: 'claude/manual.md',
        content: '# Manual rule',
        targets: ['cursor'],
        sourceType: 'local',
        disabled: false,
      },
      previews: [],
    });
    setManagedRuleDisabled.mockResolvedValue({
      rule: {
        id: 'claude/manual.md',
        tool: 'claude',
        name: 'manual.md',
        relativePath: 'claude/manual.md',
        content: '# Manual rule',
        targets: ['claude'],
        sourceType: 'local',
        disabled: true,
      },
      previews: [],
    });
    removeManagedRule.mockResolvedValue({ success: true });
    setManagedHookTargets.mockResolvedValue({
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        handlers: [{ type: 'command', command: './bin/check' }],
        targets: ['cursor'],
        sourceType: 'local',
        disabled: false,
      },
      previews: [],
    });
    setManagedHookDisabled.mockResolvedValue({
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        handlers: [{ type: 'command', command: './bin/check' }],
        targets: ['claude'],
        sourceType: 'local',
        disabled: true,
      },
      previews: [],
    });
    removeManagedHook.mockResolvedValue({ success: true });
    getSyncMatrix.mockResolvedValue({ entries: [] });
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: {
        getItem,
        setItem,
      },
    });
    Object.defineProperty(window, 'ResizeObserver', {
      configurable: true,
      value: class ResizeObserver {
        observe = observe;
        disconnect = disconnect;
        unobserve = unobserve;
      },
    });
  });

  function renderPage(initialEntry = '/resources') {
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
              <Route path="/resources" element={<ResourcesPage />} />
              <Route path="/hooks/manage/:id" element={<div>Hook detail page</div>} />
              <Route path="/rules/manage/:id" element={<div>Rule detail page</div>} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>,
    );
  }

  it('renders rules and hooks tabs inside the shared shell with managed-only filters', async () => {
    renderPage('/resources?tab=rules&mode=discovered');

    const ruleButton = await screen.findByRole('button', { name: /new rule/i });
    const resourceTabs = within(screen.getAllByRole('tablist')[0]);
    const orderedTabs = resourceTabs.getAllByRole('tab').map((tab) => tab.textContent?.replace(/\s+/g, ' ').trim());

    expect(await resourceTabs.findByRole('tab', { name: /skills/i })).toBeInTheDocument();
    expect(resourceTabs.getByRole('tab', { name: /agents/i })).toBeInTheDocument();
    expect(resourceTabs.getByRole('tab', { name: /^hooks/i })).toBeInTheDocument();
    expect(resourceTabs.getByRole('tab', { name: /^rules/i })).toBeInTheDocument();
    expect(orderedTabs).toEqual([
      expect.stringMatching(/^Skills/i),
      expect.stringMatching(/^Agents/i),
      expect.stringMatching(/^Hooks/i),
      expect.stringMatching(/^Rules/i),
    ]);
    expect(screen.getByRole('heading', { name: /resources/i })).toBeInTheDocument();
    expect(ruleButton.closest('a')).toHaveAttribute('href', '/rules/new');
    expect(screen.getByRole('button', { name: /grid view/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /grouped view/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /table view/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /managed/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /discovered/i })).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /all\s*1/i })).toHaveLength(1);
    expect(screen.getByRole('button', { name: /tracked\s*0/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /github\s*0/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /local\s*1/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^claude\s*1$/i })).not.toBeInTheDocument();
    await waitFor(() => {
      expect(listManagedRules).toHaveBeenCalledTimes(1);
    });
    expect(listRules).not.toHaveBeenCalled();
    expect(listManagedHooks).not.toHaveBeenCalled();
    expect(listHooks).not.toHaveBeenCalled();

    await userEvent.click(resourceTabs.getByRole('tab', { name: /^hooks/i }));

    const hookButton = await screen.findByRole('button', { name: /new hook/i });
    expect(hookButton.closest('a')).toHaveAttribute('href', '/hooks/new');
    expect(screen.getByRole('button', { name: /local\s*1/i })).toBeInTheDocument();
    await waitFor(() => {
      expect(listManagedHooks).toHaveBeenCalledTimes(1);
    });
    expect(listHooks).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole('button', { name: /table view/i }));

    expect(screen.getByRole('columnheader', { name: /hook/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /type/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /available in/i })).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /matcher/i })).not.toBeInTheDocument();
  });

  it('shows managed rule context-menu actions in resources', async () => {
    renderPage('/resources?tab=rules');

    const cardTitle = await screen.findByRole('heading', { name: 'manual.md' });
    fireEvent.contextMenu(cardTitle);

    expect(await screen.findByRole('menuitem', { name: /available in/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /view detail/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /disable/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /uninstall/i })).toBeInTheDocument();

    fireEvent.mouseEnter(screen.getByRole('menuitem', { name: /available in/i }));
    fireEvent.mouseDown(await screen.findByRole('menuitem', { name: 'cursor' }));

    await waitFor(() => {
      expect(setManagedRuleTargets).toHaveBeenCalledWith('claude/manual.md', 'cursor');
    });

    fireEvent.contextMenu(cardTitle);
    fireEvent.mouseDown(await screen.findByRole('menuitem', { name: /disable/i }));

    await waitFor(() => {
      expect(setManagedRuleDisabled).toHaveBeenCalledWith('claude/manual.md', true);
    });
  });

  it('shows managed hook context-menu actions in resources', async () => {
    renderPage('/resources?tab=hooks');

    const cardTitle = await screen.findByRole('heading', { name: 'claude/pre-tool-use/bash.yaml' });
    fireEvent.contextMenu(cardTitle);

    expect(await screen.findByRole('menuitem', { name: /available in/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /view detail/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /disable/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /uninstall/i })).toBeInTheDocument();

    fireEvent.mouseEnter(screen.getByRole('menuitem', { name: /available in/i }));
    fireEvent.mouseDown(await screen.findByRole('menuitem', { name: 'cursor' }));

    await waitFor(() => {
      expect(setManagedHookTargets).toHaveBeenCalledWith('claude/pre-tool-use/bash.yaml', 'cursor');
    });

    fireEvent.contextMenu(cardTitle);
    fireEvent.mouseDown(await screen.findByRole('menuitem', { name: /disable/i }));

    await waitFor(() => {
      expect(setManagedHookDisabled).toHaveBeenCalledWith('claude/pre-tool-use/bash.yaml', true);
    });
  });

  it('navigates from hook cards and renders hooks in the shared folder tree view', async () => {
    const firstRender = renderPage('/resources?tab=hooks');

    expect(await screen.findByRole('link', { name: /claude\/pre-tool-use\/bash\.yaml/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /edit hook/i })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('link', { name: /claude\/pre-tool-use\/bash\.yaml/i }));
    expect(await screen.findByText('Hook detail page')).toBeInTheDocument();

    firstRender.unmount();

    renderPage('/resources?tab=hooks');
    await userEvent.click(await screen.findByRole('button', { name: /grouped view/i }));

    await screen.findByText(/1 item in 2 folders/i);
    expect(screen.getByRole('button', { name: /expand all/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /collapse all/i })).toBeInTheDocument();
  });

  it('navigates from rule cards without a dedicated edit button', async () => {
    renderPage('/resources?tab=rules');

    expect(await screen.findByRole('link', { name: /manual\.md/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /edit rule/i })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('link', { name: /manual\.md/i }));
    expect(await screen.findByText('Rule detail page')).toBeInTheDocument();
  });

  it('uses the same actions menu for hook table rows and overflow buttons', async () => {
    renderPage('/resources?tab=hooks');

    await userEvent.click(await screen.findByRole('button', { name: /table view/i }));

    const actionsButton = screen.getByTitle('Actions');
    await userEvent.click(actionsButton);
    expect(await screen.findByRole('menuitem', { name: /available in/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /view detail/i })).toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Escape' });

    const hookRowLink = screen.getByRole('link', { name: /claude\/pre-tool-use\/bash\.yaml/i });
    fireEvent.contextMenu(hookRowLink.closest('tr')!);

    expect(await screen.findByRole('menuitem', { name: /available in/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /view detail/i })).toBeInTheDocument();
  });

  it('uses the same actions menu for skill table rows and overflow buttons', async () => {
    renderPage('/resources');

    await userEvent.click(await screen.findByRole('button', { name: /table view/i }));

    const actionsButton = screen.getByTitle('Actions');
    await userEvent.click(actionsButton);
    expect(await screen.findByRole('menuitem', { name: /view detail/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /disable/i })).toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Escape' });

    const skillRowLink = screen.getByRole('link', { name: /writer/i });
    fireEvent.contextMenu(skillRowLink.closest('tr')!);

    expect(await screen.findByRole('menuitem', { name: /view detail/i })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /disable/i })).toBeInTheDocument();
  });

  it('shows rules table with type and available-in columns', async () => {
    renderPage('/resources?tab=rules');

    await userEvent.click(await screen.findByRole('button', { name: /table view/i }));

    expect(screen.getByRole('columnheader', { name: /rule/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /type/i })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /available in/i })).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: /location/i })).not.toBeInTheDocument();
  });
});
