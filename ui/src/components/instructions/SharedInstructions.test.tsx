import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import type { SharedInstructionsFile } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import SharedInstructions from './SharedInstructions';
import ProjectSharedFiles from './ProjectSharedFiles';

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
      syncExtras: vi.fn(),
    },
  };
});

const shared = (name: string): SharedInstructionsFile => ({ name, file: 'AGENTS.md', path: `/h/extras/${name}/AGENTS.md`, exists: true, size: 1, chars: 1, targets: 0 });

function Page({ creating: initial, project }: { creating: boolean; project?: boolean }) {
  const [creating, setCreating] = useState(initial);
  if (project) return <ProjectSharedFiles creating={creating} setCreating={setCreating} />;
  return <SharedInstructions creating={creating} setCreating={setCreating} />;
}

const renderAt = (url: string, creating = false, project = false) => {
  const router = createMemoryRouter([{
    path: '*',
    element: (
      <QueryClientProvider client={new QueryClient()}>
        <I18nProvider><ToastProvider><Page creating={creating} project={project} /></ToastProvider></I18nProvider>
      </QueryClientProvider>
    ),
  }], { initialEntries: [url] });
  render(<RouterProvider router={router} />);
  return router;
};

describe('Other locations', () => {
  it('collects an edited location by its path', async () => {
    vi.mocked(api.listSharedInstructions).mockResolvedValue({
      files: [{ ...shared('personal'), locations: [{ path: '/h/notes', file: '/h/notes/AGENTS.md', mode: 'symlink', status: 'modified' }] }],
      targets: [],
      file_links: true,
    });
    const original = renderAt('/extras?tab=instructions&file=personal');

    await userEvent.click(await screen.findByRole('button', { name: 'Collect into personal' }));
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Collect into personal' }));

    await waitFor(() => expect(api.resolveSharedInstructions).toHaveBeenCalledWith('personal', { path: '/h/notes' }, 'collect'));
    cleanup();
    original.dispose();

    // The supported Config editor can retain a remote origin while selecting
    // file: AGENTS.md; /api/instructions then lists it in either shared-file view.
    for (const project of [false, true]) {
      for (const kind of ['interrupted', 'local', 'target-error'] as const) {
        vi.clearAllMocks();
        const warning = `extra "personal" has an interrupted publication; run: skillshare install example-org/hub --kind extra --name personal ${project ? '-p' : '-g'}`;
        vi.mocked(api.listSharedInstructions).mockResolvedValue({
          files: [{ ...shared('personal'), locations: [{ path: '/h/notes', file: '/h/notes/AGENTS.md', mode: 'copy', status: 'not synced' }] }],
          targets: [], file_links: true,
        });
        vi.mocked(api.syncExtras).mockResolvedValue({ extras: [{ name: 'personal',
          ...(kind === 'interrupted' ? { warnings: [warning] } : {}),
          targets: kind === 'interrupted' ? [] : [{ target: '/h/notes/AGENTS.md', mode: 'copy', synced: kind === 'local' ? 1 : 0, skipped: 0, pruned: 0,
            ...(kind === 'target-error' ? { error: 'permission denied' } : {}),
          }],
        }] });
        const router = renderAt('/extras?tab=instructions&file=personal', false, project);
        try {
          await userEvent.click(await screen.findByRole('button', { name: 'Sync' }));
          await waitFor(() => expect(document.querySelector('[data-toast-container] > .ss-toast')).not.toBeNull());
          const toast = document.querySelector('[data-toast-container] > .ss-toast')!;
          expect(api.syncExtras).toHaveBeenCalledExactlyOnceWith({ name: 'personal' });
          expect(api.resolveSharedInstructions).not.toHaveBeenCalled();
          const severity = toast.querySelector('svg')?.getAttribute('class');
          if (kind === 'interrupted') {
            expect.soft(toast, `${project ? 'project' : 'global'} shared-file warning/remedy`).toHaveTextContent(warning);
            expect.soft(severity, 'interrupted shared file must not report success').toContain('text-[#FFD37A]');
            expect.soft(toast).not.toHaveTextContent('Synced personal to');
          } else if (kind === 'target-error') {
            expect(toast).toHaveTextContent('permission denied');
            expect(severity).toContain('text-[#FF9C8F]');
            expect(toast).not.toHaveTextContent('Synced personal to');
          } else {
            expect(toast).toHaveTextContent('Synced personal to');
            expect(severity).toContain('text-[#8FD6A4]');
            expect(toast).not.toHaveTextContent(warning);
          }
        } finally {
          cleanup();
          router.dispose();
        }
      }
    }
  });
});
