import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import type { SharedInstructionsFile } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import SharedInstructions from './SharedInstructions';

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
      listSharedInstructions: vi.fn(),
      listExtras: vi.fn().mockResolvedValue({ extras: [] }),
      getSharedInstructionsContent: vi.fn(async (name: string) => ({
        name,
        path: `/h/extras/${name}/AGENTS.md`,
        exists: true,
        content: `# ${name}\n`,
      })),
      createSharedInstructions: vi.fn().mockResolvedValue({ success: true, path: '' }),
      assignSharedInstructions: vi.fn(),
      resolveSharedInstructions: vi.fn().mockResolvedValue({ success: true }),
    },
  };
});

const shared = (name: string): SharedInstructionsFile => ({
  name,
  file: 'AGENTS.md',
  path: `/h/extras/${name}/AGENTS.md`,
  exists: true,
  size: 1,
  chars: 1,
  targets: 0,
});

function Page({ creating: initial }: { creating: boolean }) {
  const [creating, setCreating] = useState(initial);
  return <SharedInstructions creating={creating} setCreating={setCreating} />;
}

const renderAt = (url: string, creating = false) => {
  const router = createMemoryRouter(
    [
      {
        path: '*',
        element: (
          <QueryClientProvider client={new QueryClient()}>
            <I18nProvider>
              <ToastProvider>
                <Page creating={creating} />
              </ToastProvider>
            </I18nProvider>
          </QueryClientProvider>
        ),
      },
    ],
    { initialEntries: [url] },
  );
  render(<RouterProvider router={router} />);
  return router;
};

describe('Other locations', () => {
  it('collects an edited location by its path', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue({
      files: [
        {
          ...shared('personal'),
          locations: [
            { path: '/h/notes', file: '/h/notes/AGENTS.md', mode: 'symlink', status: 'modified' },
          ],
        },
      ],
      targets: [],
      file_links: true,
    });
    renderAt('/extras?tab=instructions&file=personal');

    await userEvent.click(await screen.findByRole('button', { name: 'Collect into personal' }));
    await userEvent.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: 'Collect into personal' }),
    );

    await waitFor(() =>
      expect(api.resolveSharedInstructions).toHaveBeenCalledWith(
        'personal',
        { path: '/h/notes' },
        'collect',
      ),
    );
  });
});
