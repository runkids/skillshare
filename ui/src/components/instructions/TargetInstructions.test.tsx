import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import type { TargetInstructions as Data } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import TargetInstructions from './TargetInstructions';

vi.mock('../CodeEditor', () => ({
  default: ({
    value,
    onChange,
    ariaLabel,
  }: {
    value: string;
    onChange: (v: string) => void;
    ariaLabel: string;
  }) => (
    <textarea aria-label={ariaLabel} value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
vi.mock('../../api/client', async (load) => {
  const actual = await load<typeof import('../../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      getTargetInstructions: vi.fn(),
      putTargetInstructions: vi.fn().mockResolvedValue({ success: true }),
      listSharedInstructions: vi
        .fn()
        .mockResolvedValue({ files: [], targets: [], file_links: true }),
      assignSharedInstructions: vi.fn().mockResolvedValue({ success: true, errors: [] }),
    },
  };
});

const file = (target: string, path: string, extra: Partial<Data> = {}): Data => ({
  target,
  project: false,
  supported: true,
  custom: false,
  path,
  exists: true,
  content: `${target} file\n`,
  size: 10,
  import: false,
  read_order: [{ path, kind: 'main', exists: true, read: true }],
  import_lines: [],
  shared: [],
  convert: [],
  riders: [],
  read_by: [],
  ...extra,
});

// The tool is picked from the file list beside the panel.
const pickTool = async (user: ReturnType<typeof userEvent.setup>, name: RegExp) => {
  const list = await screen.findByRole('navigation', { name: 'Files on this page' });
  await user.click(within(list).getByRole('button', { name }));
};

// A data router, as the app uses: the editor guards unsaved edits with useBlocker.
const renderAt = (name: string) => {
  const router = createMemoryRouter(
    [
      {
        path: '*',
        element: (
          <QueryClientProvider client={new QueryClient()}>
            <I18nProvider>
              <ToastProvider>
                <TargetInstructions name={name} />
              </ToastProvider>
            </I18nProvider>
          </QueryClientProvider>
        ),
      },
    ],
    { initialEntries: [`/targets/${name}?tab=instructions`] },
  );
  render(<RouterProvider router={router} />);
  return router;
};
const renderTarget = (name: string) => renderAt(name);

const renderUniversal = () => {
  vi.mocked(api.getTargetInstructions).mockImplementation(async (name) =>
    name === 'codex'
      ? file('codex', '~/.codex/AGENTS.md', { rider_of: 'universal' })
      : file('universal', '~/.agents/AGENTS.md', {
          riders: [{ name: 'codex', path: '~/.codex/AGENTS.md', exists: true }],
          read_by: ['cline', 'warp'],
        }),
  );
  return renderAt('universal');
};

describe('Target instructions tab', () => {
  // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
  beforeEach(() => {
    HTMLElement.prototype.scrollIntoView = vi.fn();
    vi.mocked(api.putTargetInstructions).mockClear();
  });

  it('asks before switching away from an unsaved edit and keeps it on cancel', async () => {
    renderUniversal();
    const user = userEvent.setup();

    await user.click(await screen.findByRole('tab', { name: 'Edit' }));

    await user.type(await screen.findByRole('textbox', { name: 'AGENTS.md' }), 'draft');
    await pickTool(user, /Codex/);
    await user.click(await screen.findByRole('button', { name: 'Cancel' }));

    expect(screen.getByRole('textbox', { name: 'AGENTS.md' })).toHaveValue('universal file\ndraft');
  });

  it('keeps an unsaved edit when switching to preview and back', async () => {
    vi.mocked(api.getTargetInstructions).mockResolvedValue(file('codex', '~/.codex/AGENTS.md'));
    renderTarget('codex');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('tab', { name: 'Edit' }));

    await user.type(await screen.findByRole('textbox', { name: 'AGENTS.md' }), 'draft');
    await user.click(screen.getByRole('tab', { name: 'Preview' }));
    await user.click(screen.getByRole('tab', { name: 'Edit' }));

    expect(screen.getByRole('textbox', { name: 'AGENTS.md' })).toHaveValue('codex file\ndraft');
  });

  it('asks before leaving the page with an unsaved edit', async () => {
    vi.mocked(api.getTargetInstructions).mockResolvedValue(file('codex', '~/.codex/AGENTS.md'));
    const router = renderTarget('codex');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('tab', { name: 'Edit' }));

    await user.type(await screen.findByRole('textbox', { name: 'AGENTS.md' }), 'draft');
    act(() => {
      void router.navigate('/skills');
    });

    expect(await screen.findByRole('dialog', { name: 'Unsaved Changes' })).toBeInTheDocument();
  });

  it('switches a linked target to another shared AGENTS.md after asking', async () => {
    const user = userEvent.setup();
    const sf = (name: string) => ({
      name,
      file: 'AGENTS.md',
      path: `/x/${name}/AGENTS.md`,
      exists: true,
      size: 1,
      chars: 1,
      targets: 1,
    });
    vi.mocked(api.listSharedInstructions).mockResolvedValue({
      files: [sf('personal'), sf('work')],
      targets: [],
      file_links: true,
    });
    vi.mocked(api.getTargetInstructions).mockResolvedValue(
      file('codex', '~/.codex/AGENTS.md', {
        link_to: '/x/personal/AGENTS.md',
        link_shared: 'personal',
      }),
    );
    renderTarget('codex');

    await user.click(await screen.findByRole('button', { name: 'Change' }));
    await user.click(await screen.findByRole('menuitemradio', { name: 'work' }));
    await user.click(await screen.findByRole('button', { name: 'Switch to work' }));
    await waitFor(() =>
      expect(api.assignSharedInstructions).toHaveBeenCalledWith(['codex'], ['work']),
    );
  });
});
