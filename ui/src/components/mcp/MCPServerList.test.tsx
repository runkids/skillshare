import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import MCPServerList from './MCPServerList';

describe('MCP server list', () => {
  it("shows a server's Pi exposure on its row, and only whether other Pi settings exist", () => {
    const server = { command: 'npx', targets: ['pi'], piOptions: { exposure: 'direct', toolExposure: { 'secret_*': 'hidden' } } };
    render(<I18nProvider><MCPServerList rows={[{ name: 'docs', server, cells: {} }]} targets={['pi']} targetsOf={() => ['pi']} onToggle={vi.fn()} onMenu={vi.fn()} /></I18nProvider>);
    const line = screen.getByText('· direct').parentElement!;
    expect(line).toHaveTextContent('Other Pi settings');
    expect(line).not.toHaveTextContent('secret_');
  });

  it('marks a server that has a tool policy with a short summary', () => {
    const server = { command: 'npx', targets: ['pi'], tools: { expose: 'direct' as const, allow: ['a', 'b', 'c'], deny: ['d'] } };
    render(<I18nProvider><MCPServerList rows={[{ name: 'docs', server, cells: {} }]} targets={['pi']} targetsOf={() => ['pi']} onToggle={vi.fn()} onMenu={vi.fn()} /></I18nProvider>);
    expect(screen.getByText('Tools: direct · 3 allowed · 1 denied')).toBeInTheDocument();
    // Pi's exposure comes from the policy.
    expect(screen.getByText('· direct')).toBeInTheDocument();
  });

  it('opens and closes the target toggles from the target count', async () => {
    const user = userEvent.setup();
    render(<I18nProvider><MCPServerList rows={[{ name: 'docs', server: { command: 'npx' }, cells: {} }]} targets={['claude', 'cursor']} targetsOf={() => ['claude']} onToggle={vi.fn()} onMenu={vi.fn()} /></I18nProvider>);
    const edit = screen.getByRole('button', { name: 'Choose which agents get docs' });
    await user.click(edit);
    expect(screen.getByRole('checkbox', { name: /Cursor/ })).toBeInTheDocument();
    await user.click(edit);
    expect(screen.queryByRole('checkbox', { name: /Cursor/ })).not.toBeInTheDocument();
  });
});
