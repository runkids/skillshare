import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError } from '../../api/client';
import type { SharedInstructionsFile } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import SharedInstructions from './SharedInstructions';

vi.mock('../CodeEditor', () => ({
  default: ({ value, onChange, ariaLabel }: { value: string; onChange: (v: string) => void; ariaLabel: string }) => <textarea aria-label={ariaLabel} value={value} onChange={(e) => onChange(e.target.value)} />,
}));
vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listSharedInstructions: vi.fn(),
      listExtras: vi.fn().mockResolvedValue({ extras: [] }),
      getSharedInstructionsContent: vi.fn(async (name: string) => ({ name, path: `/h/extras/${name}/AGENTS.md`, exists: true, content: `# ${name}\n` })),
      createSharedInstructions: vi.fn().mockResolvedValue({ success: true, path: '' }),
      assignSharedInstructions: vi.fn(),
      resolveSharedInstructions: vi.fn().mockResolvedValue({ success: true }),
    },
  };
});

const shared = (name: string): SharedInstructionsFile => ({ name, file: 'AGENTS.md', path: `/h/extras/${name}/AGENTS.md`, exists: true, size: 1, chars: 1, targets: 0 });
const list = (...names: string[]) => ({ files: names.map(shared), targets: [], file_links: true });

function Page({ creating: initial }: { creating: boolean }) {
  const [creating, setCreating] = useState(initial);
  return <SharedInstructions creating={creating} setCreating={setCreating} />;
}

const renderAt = (url: string, creating = false) => {
  const router = createMemoryRouter([{
    path: '*',
    element: (
      <QueryClientProvider client={new QueryClient()}>
        <I18nProvider><ToastProvider><Page creating={creating} /></ToastProvider></I18nProvider>
      </QueryClientProvider>
    ),
  }], { initialEntries: [url] });
  render(<RouterProvider router={router} />);
  return router;
};

describe('Shared instructions', () => {
  beforeEach(() => {
    vi.mocked(api.listSharedInstructions).mockReset();
  });

  it('shows the first file for an unknown ?file= and puts its name in the address', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue(list('personal', 'work'));
    const router = renderAt('/extras?tab=instructions&file=nope');

    await waitFor(() => expect(router.state.location.search).toBe('?tab=instructions&file=personal'));
  });

  it('narrows the file list by name', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue(list('personal', 'work'));
    renderAt('/extras?tab=instructions');
    const user = userEvent.setup();

    await user.type(await screen.findByRole('searchbox', { name: 'Search files' }), 'wor');
    expect(within(screen.getByRole('navigation', { name: 'Shared AGENTS.md' })).getAllByRole('button').map((b) => b.textContent)).toEqual([expect.stringContaining('work')]);
  });

  it('replaces the unknown address instead of adding one', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue(list('personal'));
    const router = renderAt('/extras?tab=instructions&file=nope');

    await waitFor(() => expect(router.state.historyAction).toBe('REPLACE'));
  });

  // Until the refetch has it, the list holds only personal: closing then would show personal as the new file.
  it('keeps the create dialog open until the new file can be shown', async () => {
    const refetched = new Promise<ReturnType<typeof list>>(() => {});
    vi.mocked(api.listSharedInstructions).mockResolvedValueOnce(list('personal')).mockReturnValue(refetched);
    renderAt('/extras?tab=instructions', true);
    const user = userEvent.setup();

    await user.type(await screen.findByRole('textbox', { name: 'Name' }), 'fresh');
    await user.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(vi.mocked(api.listSharedInstructions).mock.calls.length).toBeGreaterThan(1));

    expect(screen.getByRole('dialog', { name: 'New shared AGENTS.md' })).toBeInTheDocument();
  });

  it('shows the new file when the create dialog closes', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValueOnce(list('personal')).mockResolvedValue(list('fresh', 'personal'));
    renderAt('/extras?tab=instructions', true);
    const user = userEvent.setup();

    await user.type(await screen.findByRole('textbox', { name: 'Name' }), 'fresh');
    await user.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New shared AGENTS.md' })).not.toBeInTheDocument());

    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('fresh');
  });

  it('says in the row why the server refused a target another shared file holds', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue({
      ...list('zz-a', 'zz-b'),
      targets: [{ name: 'commandcode', path: '/h/.commandcode/AGENTS.md', import: true, exists: true, assigned: [{ name: 'zz-b', mode: 'symlink', status: 'synced' }] }],
    });
    vi.mocked(api.assignSharedInstructions).mockRejectedValue(new ApiError(409, 'held', { code: 'instructions_target_held', params: { name: 'zz-b', target: 'commandcode' } }));
    renderAt('/extras?tab=instructions&file=zz-a');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('switch', { name: /commandcode/ }));

    expect(await screen.findByRole('alert')).toHaveTextContent("commandcode's file is zz-b (link or copy)");
  });
});

describe('Other locations', () => {
  it('lists each location by its file, with its mode and status', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue({
      files: [{ ...shared('personal'), locations: [{ path: '/h/work/notes', file: '/h/work/notes/instructions.md', as: 'instructions.md', mode: 'copy', status: 'drift' }] }],
      targets: [],
      file_links: true,
    });
    renderAt('/extras?tab=instructions&file=personal');

    expect(await screen.findByRole('combobox', { name: 'How /h/work/notes/instructions.md gets personal' })).toHaveTextContent('copy');
    expect(screen.getByText('No longer matches personal. Press Sync above to update it.')).toBeInTheDocument();
  });

  it('says there are none yet when the file has no other locations', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue({ files: [{ ...shared('personal'), locations: [] }], targets: [], file_links: true });
    renderAt('/extras?tab=instructions&file=personal');

    expect(await screen.findByText('No other locations yet')).toBeInTheDocument();
  });

  it('collects an edited location by its path', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue({
      files: [{ ...shared('personal'), locations: [{ path: '/h/notes', file: '/h/notes/AGENTS.md', mode: 'symlink', status: 'modified' }] }],
      targets: [],
      file_links: true,
    });
    renderAt('/extras?tab=instructions&file=personal');

    await userEvent.click(await screen.findByRole('button', { name: 'Collect into personal' }));
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Collect into personal' }));

    await waitFor(() => expect(api.resolveSharedInstructions).toHaveBeenCalledWith('personal', { path: '/h/notes' }, 'collect'));
  });
});

 describe('file link availability', () => {
  it.each([false, true])('shows an accessible fallback hint only when file_links is false (%s)', async (fileLinks) => {
   vi.mocked(api.listSharedInstructions).mockResolvedValue({ ...list('personal'), file_links: fileLinks });
   renderAt('/extras?tab=instructions&file=personal');
   await screen.findByText('Targets');
   const hint = screen.queryByRole('button', { name: /Without Windows Developer Mode/ });
   expect(Boolean(hint)).toBe(!fileLinks);
  });
 });
