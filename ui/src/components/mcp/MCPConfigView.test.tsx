import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { mcpApi } from '../../api/mcp';
import { ToastProvider } from '../Toast';
import MCPConfigView from './MCPConfigView';

vi.mock('../../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
vi.mock('../../api/mcp', async (load) => ({
  ...(await load<typeof import('../../api/mcp')>()),
  mcpApi: { list: vi.fn().mockResolvedValue({ source: { path: '' } }), render: vi.fn() },
}));

describe('MCP config view', () => {
  it("lists a shared server's file in each project, though two projects give one Agent", async () => {
    const user = userEvent.setup();
    vi.mocked(mcpApi.render).mockResolvedValue({
      rendered: [
        { target: 'pi', path: '/work/api/.pi/mcp.json', content: '{"exposure": "codemode"}' },
        { target: 'pi', path: '/work/billing/.pi/mcp.json', content: '{"exposure": "direct"}' },
      ],
    });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <I18nProvider>
          <ToastProvider>
            <MCPConfigView
              mutation={{
                name: 'docs',
                server: { url: 'https://docs.example/mcp', global: false },
              }}
            />
          </ToastProvider>
        </I18nProvider>
      </QueryClientProvider>,
    );
    await user.click(await screen.findByRole('button', { name: /billing/ }));
    expect(await screen.findByText(/"direct"/)).toBeInTheDocument();
  });
});
