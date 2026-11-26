import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import MCPServerList from './MCPServerList';

const renderList = (onProjects = vi.fn()) =>
  render(
    <I18nProvider>
      <MCPServerList
        rows={[
          {
            name: 'docs',
            server: { url: 'https://docs.example/mcp', targets: ['claude'], global: false },
            cells: {},
          },
        ]}
        targets={['claude']}
        targetsOf={() => ['claude']}
        onToggle={vi.fn()}
        onMenu={vi.fn()}
        users={{ docs: ['/work/api'] }}
        roots={['/work/api', '/work/web']}
        onProjects={onProjects}
      />
    </I18nProvider>,
  );

describe('MCP server list', () => {
  it("saves the projects ticked in a shared server's row together", async () => {
    const user = userEvent.setup();
    const onProjects = vi.fn();
    renderList(onProjects);
    await user.click(screen.getByRole('button', { name: 'Choose which agents get docs' }));
    await user.click(screen.getByRole('checkbox', { name: '/work/api' }));
    await user.click(screen.getByRole('checkbox', { name: '/work/web' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(onProjects).toHaveBeenCalledWith('docs', ['/work/web']);
  });

  it('waits to open any project while another row has unsaved ticks', async () => {
    const user = userEvent.setup();
    const onUnsavedChange = vi.fn();
    const server = { url: 'https://docs.example/mcp', targets: ['claude'], global: false };
    render(
      <I18nProvider>
        <MCPServerList
          rows={[
            { name: 'docs', server, cells: {} },
            { name: 'wiki', server, cells: {} },
          ]}
          targets={['claude']}
          targetsOf={() => ['claude']}
          onToggle={vi.fn()}
          onMenu={vi.fn()}
          users={{ docs: ['/work/api'], wiki: ['/work/api'] }}
          roots={['/work/api', '/work/web']}
          onProjects={vi.fn()}
          onOpenProject={vi.fn()}
          onUnsavedChange={onUnsavedChange}
        />
      </I18nProvider>,
    );
    await user.click(screen.getByRole('button', { name: 'Choose which agents get docs' }));
    await user.click(screen.getByRole('button', { name: 'Choose which agents get wiki' }));
    await user.click(screen.getAllByRole('checkbox', { name: '/work/web' })[0]);
    for (const button of screen.getAllByRole('button', { name: /MCP tab$/ }))
      expect(button).toBeDisabled();
    expect(onUnsavedChange).toHaveBeenLastCalledWith(true);
  });

  it('keeps Save off until a tick changes', async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(screen.getByRole('button', { name: 'Choose which agents get docs' }));
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });
});
