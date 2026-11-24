import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { mcpApi } from '../../api/mcp';
import { ToastProvider } from '../Toast';
import MCPProjectView from './MCPProjectView';

vi.mock('../../api/mcp', async (load) => ({
  ...(await load<typeof import('../../api/mcp')>()),
  mcpApi: {
    save: vi.fn().mockResolvedValue({}),
    import: vi.fn().mockResolvedValue({ candidates: [] }),
  },
}));
vi.mock('../../api/mcpCheck', () => ({ mcpCheckApi: { run: vi.fn() } }));

type Data = Parameters<typeof MCPProjectView>[0]['data'];

const view = (
  servers: Data['source']['servers'],
  project: NonNullable<Data['source']['projects']>[string],
  unmanaged: Data['unmanaged'] = [],
) => {
  const data = {
    source: {
      path: '',
      configPath: '',
      targets: null,
      servers,
      projects: { '/work/app': project },
    },
    projectConfigs: [],
    paths: {},
    detected: [],
    local: false,
    openable: [],
    projectPaths: {},
    projectExisting: {},
    plan: null,
    previewError: '',
    backups: [],
    unmanaged,
  } as Data;
  render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient()}>
        <I18nProvider>
          <ToastProvider>
            <MCPProjectView
              data={data}
              root="/work/app"
              offered={['claude', 'cursor', 'opencode', 'pi']}
              onChanged={vi.fn()}
              onRemoved={vi.fn()}
            />
          </ToastProvider>
        </I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
};
const context7 = { command: 'npx', targets: ['claude', 'cursor', 'opencode'] };

describe('MCP project view', () => {
  it('turns a global server off without storing targets, so the switch follows the project', async () => {
    const user = userEvent.setup();
    view({ context7 }, { targets: ['opencode'] });
    await user.click(screen.getByRole('switch', { name: 'context7' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenCalledWith({
        name: 'context7',
        replace: true,
        server: { disabled: true },
        project: '/work/app',
      }),
    );
  });

  it("turns a shared server on by adding it to this project's use list", async () => {
    const user = userEvent.setup();
    view({ docs: { url: 'https://docs.example/mcp', global: false } }, {});
    await user.click(screen.getByRole('switch', { name: 'docs' }));
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenCalledWith({ name: 'docs', using: true, project: '/work/app' }),
    );
  });

  it('keeps the first shared server when a second is turned on before the page reloads', async () => {
    const user = userEvent.setup();
    const shared = { url: 'https://docs.example/mcp', global: false };
    view({ docs: shared, wiki: shared }, {});
    await user.click(screen.getByRole('switch', { name: 'docs' }));
    await waitFor(() => expect(screen.getByRole('switch', { name: 'wiki' })).toBeEnabled());
    await user.click(screen.getByRole('switch', { name: 'wiki' }));
    // Each switch sends only its own name, so the second save cannot undo the first.
    await waitFor(() =>
      expect(mcpApi.save).toHaveBeenLastCalledWith({
        name: 'wiki',
        using: true,
        project: '/work/app',
      }),
    );
  });
});
