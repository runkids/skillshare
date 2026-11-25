import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { mcpApi, type MCPOverride, type MCPServer } from '../../api/mcp';
import { mcpCheckApi } from '../../api/mcpCheck';
import { ToastProvider } from '../Toast';
import MCPOverrideDialog from './MCPOverrideDialog';

vi.mock('../../api/mcp', async (load) => ({
  ...(await load<typeof import('../../api/mcp')>()),
  mcpApi: { save: vi.fn().mockResolvedValue({}) },
}));
vi.mock('../../api/mcpCheck', () => ({ mcpCheckApi: { probe: vi.fn().mockResolvedValue({}) } }));

type Source = Parameters<typeof MCPOverrideDialog>[0]['source'];

const docs: MCPServer = {
  url: 'https://docs.example/mcp',
  global: false,
  targets: ['claude', 'pi'],
};

const open = (server: MCPServer, override?: MCPOverride, targets: string[] | null = null) => {
  const source = {
    path: '',
    configPath: '',
    targets,
    servers: { docs: server },
    projects: { '/work/billing': { use: ['docs'], overrides: override && { docs: override } } },
  } as Source;
  render(
    <QueryClientProvider client={new QueryClient()}>
      <I18nProvider>
        <ToastProvider>
          <MCPOverrideDialog
            name="docs"
            root="/work/billing"
            source={source}
            paths={{ claude: '/work/billing/.mcp.json', pi: '/work/billing/.pi/mcp.json' }}
            onClose={vi.fn()}
            onSaved={vi.fn()}
          />
        </ToastProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
};

describe('MCP override dialog', () => {
  beforeEach(() => {
    // jsdom has no layout, and the Select scrolls its focused option into view.
    HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it('keeps the settings it does not show when Pi gets another tool exposure', async () => {
    const user = userEvent.setup();
    open(docs, { piOptions: { timeout: 30 }, options: { claude: { alwaysLoad: true } } });
    await user.click(screen.getByRole('combobox', { name: 'Tool exposure' }));
    await user.click(screen.getByRole('option', { name: /^direct\b/ }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: {
          piOptions: { timeout: 30, exposure: 'direct' },
          options: { claude: { alwaysLoad: true } },
        },
      }),
    );
  });

  it('clears the override when its only setting is reset', async () => {
    const user = userEvent.setup();
    open(docs, { piOptions: { exposure: 'direct' } });
    await user.click(screen.getByRole('button', { name: 'Reset to default' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: {},
      }),
    );
  });

  it('keeps saved Agents this page does not offer when another setting changes', async () => {
    const user = userEvent.setup();
    open(docs, { targets: ['cursor', 'pi'] });
    await user.click(screen.getByRole('combobox', { name: 'Tool exposure' }));
    await user.click(screen.getByRole('option', { name: /^direct\b/ }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: { targets: ['pi', 'cursor'], piOptions: { exposure: 'direct' } },
      }),
    );
  });

  it('saves an override that sends the server to no Agent in this project', async () => {
    const user = userEvent.setup();
    open(docs, { targets: [] });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: { targets: [] },
      }),
    );
  });

  it('saves the Agents chosen for this project and keeps the settings it does not show', async () => {
    const user = userEvent.setup();
    open(docs, { piOptions: { timeout: 30 } });
    await user.click(screen.getByRole('combobox', { name: 'Targets' }));
    await user.click(screen.getByRole('option', { name: 'Choose Agents for this project' }));
    await user.click(screen.getByRole('checkbox', { name: 'Claude' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: { targets: ['pi'], piOptions: { timeout: 30 } },
      }),
    );
  });

  it('starts the chosen Agents from the inherited ones this project offers', async () => {
    const user = userEvent.setup();
    open({ url: 'https://docs.example/mcp', global: false }, undefined, ['claude', 'claude-work']);
    await user.click(screen.getByRole('combobox', { name: 'Targets' }));
    await user.click(screen.getByRole('option', { name: 'Choose Agents for this project' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: { targets: ['claude'] },
      }),
    );
  });

  it('leaves out an inherited Agent that has no file in this project', async () => {
    const user = userEvent.setup();
    open({ ...docs, targets: ['claude', 'windsurf'] });
    await user.click(screen.getByRole('combobox', { name: 'Targets' }));
    await user.click(screen.getByRole('option', { name: 'Choose Agents for this project' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: { targets: ['claude'] },
      }),
    );
  });

  it("loads this project's tools from the project's folder", async () => {
    const user = userEvent.setup();
    open({ command: 'node', args: ['./scripts/mcp.js'], global: false });
    await user.click(screen.getByRole('combobox', { name: 'Tools' }));
    await user.click(screen.getByRole('option', { name: 'Choose tools for this project' }));
    await user.click(screen.getByRole('button', { name: 'Load tools' }));
    await waitFor(() =>
      expect(mcpCheckApi.probe).toHaveBeenCalledWith(
        expect.objectContaining({ project: '/work/billing' }),
      ),
    );
  });

  it("starts this project's tool policy from the MCP page's and saves it", async () => {
    const user = userEvent.setup();
    open({ ...docs, tools: { deny: ['delete_page'] } });
    await user.click(screen.getByRole('combobox', { name: 'Tools' }));
    await user.click(screen.getByRole('option', { name: 'Choose tools for this project' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        project: '/work/billing',
        name: 'docs',
        override: { tools: { deny: ['delete_page'] } },
      }),
    );
  });
});
