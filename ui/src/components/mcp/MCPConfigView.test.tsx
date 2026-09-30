import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { mcpApi, type MCPMutation } from '../../api/mcp';
import { I18nProvider } from '../../i18n';
import MCPConfigView from './MCPConfigView';

vi.mock('../CopyButton', () => ({ default: () => null }));
vi.mock('../../api/mcp', async (load) => ({ ...await load<typeof import('../../api/mcp')>(), mcpApi: { render: vi.fn(), list: vi.fn(() => new Promise(() => {})) } }));

it('keeps the previous native preview while a changed draft is rendering', async () => {
  const client = new QueryClient();
  const initial = { rendered: [{ target: 'pi', path: '/pi/mcp.json', content: 'previous native preview' }] };
  let resolve!: (value: typeof initial) => void;
  vi.mocked(mcpApi.render).mockResolvedValueOnce(initial).mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
  const view = (command: string) => {
    const mutation: MCPMutation = { name: 'docs', server: { command, targets: ['pi'] } };
    return <QueryClientProvider client={client}><I18nProvider><MCPConfigView mutation={mutation} /></I18nProvider></QueryClientProvider>;
  };
  const result = render(view('old'));
  expect(await screen.findByText('previous native preview')).toBeInTheDocument();
  result.rerender(view('new'));
  await waitFor(() => expect(mcpApi.render).toHaveBeenCalledTimes(2));
  expect(screen.getByText('previous native preview')).toBeInTheDocument();
  resolve({ rendered: [{ target: 'pi', path: '/pi/mcp.json', content: 'updated native preview' }] });
  expect(await screen.findByText('updated native preview')).toBeInTheDocument();
});

const rendered = { rendered: [
  { target: 'claude', path: '/home/me/.claude.json', content: 'claude file' },
  { target: 'codex', path: '/home/me/.codex/config.toml', content: 'codex file' },
  { target: 'pi', path: '/home/me/.pi/agent/mcp.json', error: 'Pi is too old' },
] };
const showAll = () => {
  vi.mocked(mcpApi.render).mockResolvedValue(rendered);
  const mutation: MCPMutation = { name: 'docs', server: { command: 'docs', targets: ['claude', 'codex', 'pi'] } };
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPConfigView mutation={mutation} /></I18nProvider></QueryClientProvider>);
};

it('lists the source and every target Agent', async () => {
  showAll();
  await screen.findByText('claude file');
  expect(screen.getAllByRole('button').map((b) => b.textContent)).toEqual([
    expect.stringContaining('Skillshare source'), expect.stringContaining('Claude'), expect.stringContaining('Codex'), expect.stringContaining('Pi'),
  ]);
});

it('shows the first Agent file by default', async () => {
  showAll();
  expect(await screen.findByText('claude file')).toBeInTheDocument();
});

it('shows the file of the item picked from the list', async () => {
  showAll();
  await screen.findByText('claude file');
  fireEvent.click(screen.getByRole('button', { name: /Codex/ }));
  expect(screen.getByText('codex file')).toBeInTheDocument();
});

it('shows the source as JSON when it is picked', async () => {
  showAll();
  await screen.findByText('claude file');
  fireEvent.click(screen.getByRole('button', { name: /Skillshare source/ }));
  expect(document.querySelector('.ss-code')).toHaveTextContent('"command": "docs"');
});

it('marks only the Agent whose render failed', async () => {
  showAll();
  await screen.findByText('claude file');
  expect(within(screen.getByRole('button', { name: /Codex/ })).queryByText('Pi is too old')).not.toBeInTheDocument();
  expect(within(screen.getByRole('button', { name: /Pi/ })).getByText('Pi is too old')).toBeInTheDocument();
});

it('counts only the files Sync writes', async () => {
  showAll();
  expect(await screen.findByText('Sync writes 2 files')).toBeInTheDocument();
});

it('tags a JSONC file with its format', async () => {
  vi.mocked(mcpApi.render).mockResolvedValue({ rendered: [{ target: 'kilo', path: '/home/me/.config/kilo/kilo.jsonc', content: 'kilo file' }] });
  const mutation: MCPMutation = { name: 'docs', server: { command: 'docs', targets: ['kilo'] } };
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPConfigView mutation={mutation} /></I18nProvider></QueryClientProvider>);
  await screen.findByText('kilo file');
  expect(screen.getByText('JSONC')).toBeInTheDocument();
});
