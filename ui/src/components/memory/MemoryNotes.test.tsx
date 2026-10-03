import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import MemoryNotes from './MemoryNotes';

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, listMemoryNotes: vi.fn(), initMemory: vi.fn(), readMemoryNote: vi.fn(), writeMemoryNote: vi.fn(), deleteMemoryNote: vi.fn(), getMemoryGuidance: vi.fn(), linkMemoryIndex: vi.fn() } };
});
vi.mock('../instructions/InstructionsEditorDialog', () => ({ default: ({ content, onSave }: { content: string; onSave: (value: string) => Promise<void> }) => (
  <button onClick={() => void onSave(content + '\nupdated')}>Save note</button>
) }));

const renderNotes = () => render(
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <I18nProvider><ToastProvider><MemoryRouter><MemoryNotes /></MemoryRouter></ToastProvider></I18nProvider>
  </QueryClientProvider>,
);

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getMemoryGuidance).mockResolvedValue({ scope: 'global', instructions: '', targets: [] });
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [{ path: 'build.md', title: 'Build notes', version: 'old' }], instructions: 'Read the shared notes.' });
  vi.mocked(api.readMemoryNote).mockResolvedValue({ path: 'build.md', title: 'Build notes', content: '# Build', version: 'old' });
  vi.mocked(api.writeMemoryNote).mockResolvedValue({ path: 'build.md', title: 'Build notes', content: '# Build\nupdated', version: 'new' });
});

it('keeps valid notes available when another note is invalid', async () => {
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [
    { path: 'large.md', title: 'Large note', version: '', invalid: 'Too large to read' },
    { path: 'build.md', title: 'Build notes', version: 'old' },
  ], instructions: '' });
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'Large note large.md' }));
  expect(await screen.findByText(/Too large to read/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: 'Build notes build.md' }));
  expect(await screen.findByRole('heading', { name: 'Build' })).toBeInTheDocument();
});

it('keeps a newly created note when updating its index conflicts', async () => {
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [], instructions: '',
    index: { version: 'index-v1', unindexed: [], broken_links: [] } });
  vi.mocked(api.linkMemoryIndex).mockRejectedValue(new Error('Index changed'));
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'New note' }));
  await user.type(screen.getByLabelText('File name'), 'decisions.md');
  await user.click(screen.getByRole('button', { name: 'Create' }));
  expect(api.linkMemoryIndex).toHaveBeenCalledWith('decisions.md', 'index-v1');
  expect(await screen.findByText(/Note created.*INDEX.md/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Save note' })).toBeInTheDocument();
});

it('saves against the version read from disk and offers an instruction entry point', async () => {
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: /Build notes/ }));
  await user.click(await screen.findByRole('button', { name: 'Edit' }));
  await user.click(await screen.findByRole('button', { name: 'Save note' }));
  expect(api.writeMemoryNote).toHaveBeenCalledWith('build.md', '# Build\nupdated', 'old');
  expect(screen.getByRole('link', { name: 'Open AGENTS.md' })).toHaveAttribute('href', '/?tab=instructions');
});

it('initializes a source-only memory extra from its empty state', async () => {
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: false, notes: [], instructions: '' });
  vi.mocked(api.initMemory).mockResolvedValue({ success: true, root: '/shared/extras/memory' });
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'Create memory' }));
  expect(api.initMemory).toHaveBeenCalledOnce();
});

it('previews the copyable guidance on hover and hides it on leave', async () => {
  const user = userEvent.setup();
  renderNotes();
  const copy = await screen.findByRole('button', { name: 'Copy guidance' });
  await user.hover(copy);
  expect(await screen.findByRole('tooltip')).toHaveTextContent('Read the shared notes.');
  await user.unhover(copy);
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
});

it('previews the verification prompt on hover and hides it on leave', async () => {
  const user = userEvent.setup();
  renderNotes();
  const copy = await screen.findByRole('button', { name: 'Copy verification prompt' });
  await user.hover(copy);
  expect(await screen.findByRole('tooltip')).toHaveTextContent('read tool');
  await user.unhover(copy);
  expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
});

it('creates a Markdown note without overwriting an existing version', async () => {
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'New note' }));
  const dialog = screen.getByRole('dialog');
  await user.type(within(dialog).getByLabelText('File name'), 'decisions.md');
  await user.click(within(dialog).getByRole('button', { name: 'Create' }));
  expect(api.writeMemoryNote).toHaveBeenCalledWith('decisions.md', '# decisions\n', '');
});

it('keeps the search input mounted while the query changes', async () => {
  const user = userEvent.setup();
  renderNotes();
  const input = await screen.findByLabelText('Search names and content');
  await user.type(input, 'architecture');
  expect(api.listMemoryNotes).toHaveBeenLastCalledWith('architecture');
  expect(input).toHaveFocus();
});

it('requires confirmation and deletes the version shown in the preview', async () => {
  vi.mocked(api.deleteMemoryNote).mockResolvedValue({ success: true, path: 'build.md' });
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'Delete note' }));
  expect(api.deleteMemoryNote).not.toHaveBeenCalled();
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }));
  expect(api.deleteMemoryNote).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Delete note' }));
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete note' }));
  expect(api.deleteMemoryNote).toHaveBeenCalledWith('build.md', 'old');
});

it('browses nested notes with collapsible folders and a separate preview', async () => {
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [
    { path: 'INDEX.md', title: 'Index', version: 'index' },
    { path: 'wiki/architecture notes.md', title: 'Architecture', version: 'architecture' },
  ], instructions: '' });
  vi.mocked(api.readMemoryNote).mockImplementation(async (path) => ({ path, title: path, version: 'version', content: path === 'INDEX.md' ? '# Index\n[Architecture](wiki/architecture%20notes.md)' : '# Architecture decisions' }));
  const user = userEvent.setup();
  renderNotes();
  const folder = await screen.findByRole('button', { name: 'wiki' });
  expect(folder).toHaveAttribute('aria-expanded', 'true');
  await user.click(folder);
  expect(screen.queryByRole('button', { name: 'Architecture wiki/architecture notes.md' })).not.toBeInTheDocument();
  await user.click(folder);
  await user.click(await screen.findByRole('link', { name: 'Architecture' }));
  expect(await screen.findByRole('heading', { name: 'Architecture decisions' })).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: 'Save note' })).not.toBeInTheDocument();
});
