import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { ApiError, api } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import { useState } from 'react';
import MemoryNotes from './MemoryNotes';

function Notes() {
  const [creating, setCreating] = useState(false);
  return <><button type="button" onClick={() => setCreating(true)}>New note</button><MemoryNotes creating={creating} setCreating={setCreating} /></>;
}

vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, listMemoryNotes: vi.fn(), initMemory: vi.fn(), readMemoryNote: vi.fn(), writeMemoryNote: vi.fn(), deleteMemoryNote: vi.fn(), moveMemoryNote: vi.fn(), getMemoryGuidance: vi.fn(), planMemoryGuidance: vi.fn(), linkMemoryIndex: vi.fn() } };
});
vi.mock('../instructions/InstructionsEditorDialog', () => ({ default: ({ content, onSave }: { content: string; onSave: (value: string) => Promise<void> }) => (
  <button onClick={() => void onSave(content + '\nupdated')}>Save note</button>
) }));

const noGuidance = { passive: '', active: '' };

const renderNotes = () => render(
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <I18nProvider><ToastProvider><MemoryRouter><Notes /></MemoryRouter></ToastProvider></I18nProvider>
  </QueryClientProvider>,
);

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.getMemoryGuidance).mockResolvedValue({ scope: 'global', instructions: noGuidance, targets: [] });
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [{ path: 'build.md', title: 'Build notes', version: 'old' }], instructions: { passive: 'Read the shared notes.', active: 'Save lasting facts.' } });
  vi.mocked(api.readMemoryNote).mockResolvedValue({ path: 'build.md', title: 'Build notes', content: '# Build', version: 'old' });
  vi.mocked(api.writeMemoryNote).mockResolvedValue({ path: 'build.md', title: 'Build notes', content: '# Build\nupdated', version: 'new' });
});

it('keeps valid notes available when another note is invalid', async () => {
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [
    { path: 'large.md', title: 'Large note', version: '', invalid: 'Too large to read' },
    { path: 'build.md', title: 'Build notes', version: 'old' },
  ], instructions: noGuidance });
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'Large note large.md' }));
  expect(await screen.findByText(/Too large to read/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: 'Build notes build.md' }));
  expect(await screen.findByRole('heading', { name: 'Build' })).toBeInTheDocument();
});

it('keeps a newly created note when updating its index conflicts', async () => {
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [], instructions: noGuidance,
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
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: false, notes: [], instructions: noGuidance });
  vi.mocked(api.initMemory).mockResolvedValue({ success: true, root: '/shared/extras/memory' });
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'Create memory' }));
  expect(api.initMemory).toHaveBeenCalledOnce();
});

it('keeps the search box when no note matches', async () => {
  vi.mocked(api.listMemoryNotes).mockImplementation(async (search) => ({ root: '/shared/extras/memory', initialized: true, instructions: noGuidance,
    notes: search ? [] : [{ path: 'build.md', title: 'Build notes', version: 'old' }] }));
  const user = userEvent.setup();
  renderNotes();
  await user.type(await screen.findByRole('searchbox', { name: 'Search names and content' }), 'zzz');
  expect(await screen.findByText('No matching notes')).toBeInTheDocument();
  expect(screen.getByRole('searchbox', { name: 'Search names and content' })).toHaveValue('zzz');
});

it('moves a note with its read version, clears search and selects the new path', async () => {
  vi.mocked(api.moveMemoryNote).mockImplementation(async () => {
    vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, instructions: noGuidance, notes: [{ path: 'wiki/build.md', title: 'Moved build', version: 'v2' }] });
    vi.mocked(api.readMemoryNote).mockResolvedValue({ path: 'wiki/build.md', title: 'Moved build', content: '# Moved build', version: 'v2' });
    return { path: 'wiki/build.md', title: 'Moved build', version: 'v2' };
  });
  const user = userEvent.setup();
  renderNotes();
  await user.type(await screen.findByRole('searchbox', { name: 'Search names and content' }), 'build');
  await user.click(await screen.findByRole('button', { name: 'More actions' }));
  await user.click(await screen.findByRole('menuitem', { name: 'Move or rename' }));
  const field = within(screen.getByRole('dialog')).getByLabelText('New path');
  await user.clear(field);
  await user.type(field, 'wiki/build.md');
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Move' }));
  expect(api.moveMemoryNote).toHaveBeenCalledWith('build.md', 'wiki/build.md', 'old');
  expect(await screen.findByRole('heading', { name: 'Moved build' })).toBeInTheDocument();
  expect(screen.getByRole('searchbox', { name: 'Search names and content' })).toHaveValue('');
  expect(screen.getByRole('link', { name: 'Restore in Backup Files' })).toHaveAttribute('href', '/backup?tab=files&path=%2Fshared%2Fextras%2Fmemory%2Fbuild.md');
});

it('explains a destination that already exists', async () => {
  vi.mocked(api.moveMemoryNote).mockRejectedValue(new ApiError(409, 'exists', { code: 'memory_destination_exists' }));
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'More actions' }));
  await user.click(await screen.findByRole('menuitem', { name: 'Move or rename' }));
  const field = within(screen.getByRole('dialog')).getByLabelText('New path');
  await user.clear(field);
  await user.type(field, 'taken.md');
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Move' }));
  expect(await screen.findByText('A note already exists at that path.')).toBeInTheDocument();
  expect(screen.getByRole('dialog')).toBeInTheDocument();
});

it('copies the guidance of the chosen update mode', async () => {
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: /Copy guidance/ }));
  await user.click(screen.getByRole('menuitem', { name: /^active/ }));
  expect(await navigator.clipboard.readText()).toBe('Save lasting facts.');
});

it('switches every agent reading the same file to one mode', async () => {
  vi.mocked(api.getMemoryGuidance).mockResolvedValue({ scope: 'global', instructions: noGuidance, targets: [
    { name: 'antigravity-cli', state: 'configured', file: '/shared/AGENTS.md', mode: 'passive' },
    { name: 'claude', state: 'configured', file: '/home/.claude/CLAUDE.md', mode: 'passive' },
    { name: 'codex', state: 'configured', file: '/shared/AGENTS.md', mode: 'passive' },
  ] });
  vi.mocked(api.planMemoryGuidance).mockResolvedValue({ token: 't', changes: [], skipped: [], warnings: [] });
  const user = userEvent.setup();
  renderNotes();
  await user.click(await screen.findByRole('button', { name: 'Connect to agents' }));
  const dialog = await screen.findByRole('dialog');
  await user.click(within(within(dialog).getByRole('radiogroup', { name: 'Update mode for codex' })).getByRole('radio', { name: 'active' }));
  expect(within(within(dialog).getByRole('radiogroup', { name: 'Update mode for antigravity-cli' })).getByRole('radio', { name: 'active' })).toHaveAttribute('aria-checked', 'true');
  await user.click(within(dialog).getByRole('button', { name: 'Review changes' }));
  expect(api.planMemoryGuidance).toHaveBeenCalledWith(['antigravity-cli', 'codex'], { 'antigravity-cli': 'active', codex: 'active' });
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
  await user.click(await screen.findByRole('button', { name: 'More actions' }));
  await user.click(await screen.findByRole('menuitem', { name: 'Delete note' }));
  expect(api.deleteMemoryNote).not.toHaveBeenCalled();
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }));
  expect(api.deleteMemoryNote).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'More actions' }));
  await user.click(await screen.findByRole('menuitem', { name: 'Delete note' }));
  await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete note' }));
  expect(api.deleteMemoryNote).toHaveBeenCalledWith('build.md', 'old');
});

it('browses nested notes with collapsible folders and a separate preview', async () => {
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, notes: [
    { path: 'INDEX.md', title: 'Index', version: 'index' },
    { path: 'wiki/architecture notes.md', title: 'Architecture', version: 'architecture' },
  ], instructions: noGuidance });
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

it('keeps the destination draft open when the source version is stale', async () => {
 vi.mocked(api.moveMemoryNote).mockRejectedValue(new ApiError(409, 'changed', { code: 'memory_conflict' }));
 const user = userEvent.setup();
 renderNotes();
 await user.click(await screen.findByRole('button', { name: 'More actions' }));
 await user.click(await screen.findByRole('menuitem', { name: 'Move or rename' }));
 const field = within(screen.getByRole('dialog')).getByLabelText('New path');
 await user.clear(field);
 await user.type(field, 'wiki/renamed.md');
 await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Move' }));
 expect(await screen.findByText(/This note changed elsewhere/)).toBeInTheDocument();
 expect(field).toHaveValue('wiki/renamed.md');
 expect(screen.getByRole('dialog')).toBeInTheDocument();
});

it('initializes missing starters before creating and linking the first note', async () => {
  const empty = { root: '/shared/extras/memory', initialized: false, notes: [], instructions: noGuidance };
  vi.mocked(api.listMemoryNotes).mockResolvedValueOnce(empty).mockResolvedValue({ ...empty, initialized: true,
    index: { version: 'starter-index', unindexed: [], broken_links: [] } });
  vi.mocked(api.initMemory).mockResolvedValue({ success: true, root: empty.root });
  vi.mocked(api.linkMemoryIndex).mockResolvedValue({ path: 'INDEX.md', title: 'Shared memory', version: 'linked-index' });
  const user = userEvent.setup();
  renderNotes();
  await screen.findByRole('button', { name: 'Create memory' });
  await user.click(screen.getByRole('button', { name: 'New note' }));
  await user.type(screen.getByLabelText('File name'), 'decision.md');
  await user.click(screen.getByRole('button', { name: 'Create' }));
  await screen.findByRole('button', { name: 'Save note' });
  expect(api.initMemory).toHaveBeenCalledOnce();
  expect(vi.mocked(api.initMemory).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(api.writeMemoryNote).mock.invocationCallOrder[0]);
  expect(api.linkMemoryIndex).toHaveBeenCalledWith('decision.md', 'starter-index');
});

it('opens a long note collapsed and expands it on request', async () => {
  vi.mocked(api.readMemoryNote).mockResolvedValue({ path: 'build.md', title: 'Build notes', content: Array.from({ length: 12 }, (_, i) => `line ${i}`).join('\n'), version: 'old' });
  const user = userEvent.setup();
  renderNotes();
  const toggle = await screen.findByRole('button', { name: /Show all/ });
  await user.click(toggle);
  expect(screen.getByRole('button', { name: /Collapse/ })).toHaveAttribute('aria-expanded', 'true');
});

it('reloads the note list and content from disk on refresh', async () => {
  const user = userEvent.setup();
  renderNotes();
  expect(await screen.findByRole('heading', { name: 'Build' })).toBeInTheDocument();
  vi.mocked(api.listMemoryNotes).mockResolvedValue({ root: '/shared/extras/memory', initialized: true, instructions: noGuidance, notes: [
    { path: 'build.md', title: 'Build notes', version: 'v2' }, { path: 'new.md', title: 'New note', version: 'v1' },
  ] });
  vi.mocked(api.readMemoryNote).mockResolvedValue({ path: 'build.md', title: 'Build notes', content: '# Build\nedited elsewhere', version: 'v2' });
  await user.click(screen.getByRole('button', { name: 'Refresh' }));
  expect(await screen.findByText('edited elsewhere')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'New note new.md' })).toBeInTheDocument();
});
