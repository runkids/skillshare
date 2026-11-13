import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import { api } from '../api/client';
import ExtrasPage from './ExtrasPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: true }) }));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listExtras: vi.fn().mockResolvedValue({
        extras: [
          { name: 'team-rules', file: 'AGENTS.md', source_dir: '/p/.skillshare/extras/team-rules', source_type: 'per-extra', file_count: 1, source_exists: true,
            targets: [{ path: '.', mode: 'symlink', flatten: false, status: 'synced' }] },
          { name: 'rules', source_dir: '/p/.skillshare/extras/rules', source_type: 'per-extra', file_count: 2, source_exists: true,
            targets: [{ path: '.claude/rules', mode: 'merge', flatten: false, status: 'synced' }] },
          { name: 'conventions', file: 'CONVENTIONS.md', source_dir: '/p/.skillshare/extras/conventions', source_type: 'per-extra', file_count: 1, source_exists: true,
            targets: [{ path: '.cursor', mode: 'copy', flatten: false, as: 'rules.md', status: 'synced' }] },
          { name: 'pi-prompt', file: 'system.md', source_dir: String.raw`C:\Users\me\prompts`, source_type: 'custom', file_count: 1, source_exists: true,
            targets: [{ path: String.raw`C:\Users\me\.pi\agent`, mode: 'copy', flatten: false, as: 'APPEND_SYSTEM.md', status: 'synced' }] },
        ],
      }),
      deleteExtra: vi.fn().mockResolvedValue({ success: true, remote: 'example-org/hub', deleted_dir: true }),
      createExtra: vi.fn().mockResolvedValue({ success: true }),
      previewExtraFilter: vi.fn().mockResolvedValue({ files: [{ file: 'index.md', status: 'synced' }, { file: 'draft.md', status: 'synced' }], unmatched: null }),
      editExtraTarget: vi.fn().mockResolvedValue({ success: true }),
      editExtra: vi.fn().mockResolvedValue({ success: true }),
      listExtraExtensions: vi.fn().mockResolvedValue({ extensions: [] }),
      availableTargets: vi.fn().mockResolvedValue({ targets: [] }),
      getOverview: vi.fn().mockResolvedValue({}),
    },
  };
});

const renderPage = (url = '/extras') => render(
  <QueryClientProvider client={new QueryClient()}>
    <I18nProvider><ToastProvider><MemoryRouter initialEntries={[url]}><ExtrasPage /></MemoryRouter></ToastProvider></I18nProvider>
  </QueryClientProvider>,
);

describe('Extras page in a project', () => {
  it('creates a single-file extra with its file and target file name', async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Add extra' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText('Name'), 'notes');
    await user.click(within(dialog).getByRole('radio', { name: 'Single file' }));
    const [fileInput, asInput] = within(dialog).getAllByRole('textbox', { name: 'File name' });
    await user.type(fileInput, 'NOTES.md');
    await user.type(within(dialog).getByRole('textbox', { name: 'Folder' }), '.claude');
    await user.type(asInput, 'CLAUDE-notes.md');
    await user.click(within(dialog).getByRole('button', { name: 'Create' }));

    expect(api.createExtra).toHaveBeenCalledWith({
      name: 'notes',
      file: 'NOTES.md',
      targets: [{ path: '.claude', mode: 'merge', as: 'CLAUDE-notes.md' }],
    });
  });

  // Issue #300: several single files can share one folder of the shared extras folder.
  it('creates a single-file extra in a source folder named differently from the extra', async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'Add extra' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText('Name'), 'review');
    await user.click(within(dialog).getByRole('radio', { name: 'Single file' }));
    const [fileInput] = within(dialog).getAllByRole('textbox', { name: 'File name' });
    await user.type(fileInput, 'review.md');
    await user.type(within(dialog).getByRole('textbox', { name: 'Source folder' }), 'prompts');
    await user.type(within(dialog).getByRole('textbox', { name: 'Folder' }), '.claude/commands');
    await user.click(within(dialog).getByRole('button', { name: 'Create' }));

    expect(api.createExtra).toHaveBeenCalledWith({
      name: 'review',
      folder: 'prompts',
      file: 'review.md',
      targets: [{ path: '.claude/commands', mode: 'merge' }],
    });
  });

  // Issue #435: a click on a preview row excludes that file from the target.
  it('saves a file excluded from a target in Edit target', async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'More actions for .claude/rules' }));
    await user.click(screen.getByRole('menuitem', { name: 'Edit target…' }));
    const dialog = screen.getByRole('dialog');
    await user.click(await within(dialog).findByRole('button', { name: /draft\.md/ }));
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));

    expect(api.editExtraTarget).toHaveBeenCalledWith('rules', '.claude/rules', {
      path: '.claude/rules', mode: 'merge', flatten: false, extension: '', as: '', include: [], exclude: ['draft.md'],
    });
  });

  // Issue #490: Edit target clears flatten on symlink, like Add target.
  it('keeps flatten off after Edit target switches to symlink and back', async () => {
    const user = userEvent.setup();
    // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
    HTMLElement.prototype.scrollIntoView = vi.fn();
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'More actions for .claude/rules' }));
    await user.click(screen.getByRole('menuitem', { name: 'Edit target…' }));
    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('switch', { name: 'Flatten' }));
    const mode = within(dialog).getAllByRole('combobox')[1];
    await user.click(mode);
    await user.click(screen.getByRole('option', { name: /^symlink/ }));
    await user.click(mode);
    await user.click(screen.getByRole('option', { name: /^merge/ }));
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));

    expect(api.editExtraTarget).toHaveBeenLastCalledWith('rules', '.claude/rules', expect.objectContaining({ mode: 'merge', flatten: false }));
  });

  it('renames an extra in Edit extra', async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole('button', { name: 'More actions for rules' }));
    await user.click(screen.getByRole('menuitem', { name: 'Edit extra…' }));
    const name = within(screen.getByRole('dialog')).getByLabelText('Name');
    await user.clear(name);
    await user.type(name, 'docs');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(api.editExtra).toHaveBeenCalledWith('rules', { name: 'docs' });
  });

});


describe('remote extra removal', () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('renders the explicit restore command as code within its localized guidance', async () => {
    vi.mocked(api.listExtras).mockResolvedValueOnce({ extras: [{
      name: 'missing-rules', remote: 'example-org/hub/rules', branch: 'main', state: 'missing',
      source_dir: '/fixture/extras/missing-rules', source_type: 'default', source_exists: false, file_count: 0, targets: [],
    }] });
    renderPage();
    const command = await screen.findByText('skillshare install example-org/hub/rules --kind extra --name missing-rules --branch main -p');
    expect(command.tagName).toBe('CODE');
    expect(command.parentElement).toHaveTextContent('Restore or repair with: skillshare install');
  });

  it('requires confirmation before removing a managed directory and allows cancel', async () => {
    vi.mocked(api.listExtras).mockResolvedValueOnce({ extras: [{
      name: 'remote-rules', remote: 'example-org/hub', branch: 'main', commit: 'a'.repeat(40), state: 'intact',
      source_dir: '/fixture/extras/remote-rules', source_type: 'default', source_exists: true, file_count: 1, targets: [],
    }] });
    const user = userEvent.setup();
    renderPage();
    const openRemoval = async () => {
      await user.click(await screen.findByRole('button', { name: 'More actions for remote-rules' }));
      await user.click(screen.getByRole('menuitem', { name: 'Remove extra' }));
    };
    await openRemoval();
    expect(api.deleteExtra).not.toHaveBeenCalled();
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText(/unedited.*deleted.*edited.*kept/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(api.deleteExtra).not.toHaveBeenCalled();
    await openRemoval();
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Remove' }));
    expect(api.deleteExtra).toHaveBeenCalledExactlyOnceWith('remote-rules');
  });
});
