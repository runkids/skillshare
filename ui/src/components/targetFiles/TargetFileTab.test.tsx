import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import type { TargetFile } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { ToastProvider } from '../Toast';
import TargetFileTab from './TargetFileTab';

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
  return { ...actual, api: { ...actual.api, getTargetFile: vi.fn(), removeTargetFile: vi.fn() } };
});

const file = (path: string, extra: Partial<TargetFile> = {}) => ({
  path,
  abs: `/home/me/.pi/agent/${path}`,
  builtin: true,
  exists: true,
  size: 5,
  content: 'hello',
  ...extra,
});

// A data router, as the app uses: the editor guards unsaved edits with useBlocker.
const renderTab = (data: ReturnType<typeof file>) => {
  vi.mocked(api.getTargetFile).mockResolvedValue(data);
  const router = createMemoryRouter(
    [
      {
        path: '*',
        element: (
          <QueryClientProvider client={new QueryClient()}>
            <I18nProvider>
              <ToastProvider>
                <TargetFileTab target="pi" path={data.path} project={false} />
              </ToastProvider>
            </I18nProvider>
          </QueryClientProvider>
        ),
      },
    ],
    { initialEntries: [`/targets/pi?tab=file&path=${encodeURIComponent(data.path)}`] },
  );
  render(<RouterProvider router={router} />);
  return router;
};

describe('Target file tab', () => {
  beforeEach(() => vi.clearAllMocks());

  it('removes a file the user added after asking, then opens the instruction tab', async () => {
    vi.mocked(api.removeTargetFile).mockResolvedValue({
      target: 'pi',
      project: false,
      root: '/home/me/.pi/agent',
      files: [],
    });
    const user = userEvent.setup();
    const router = renderTab(file('SYSTEM.md', { builtin: false }));
    await user.click(await screen.findByRole('button', { name: 'Remove from tabs' }));
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText(/~\/.pi\/agent\/SYSTEM.md is not deleted/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Remove tab' }));
    await waitFor(() => expect(api.removeTargetFile).toHaveBeenCalledWith('pi', 'SYSTEM.md'));
    await waitFor(() => expect(router.state.location.search).toBe('?tab=instructions'));
  });

  it('shares a file through a new single-file extra for its folder', async () => {
    renderTab(file('APPEND_SYSTEM.md'));
    expect(await screen.findByRole('link', { name: /Share with Extras/ })).toHaveAttribute(
      'href',
      '/extras?add=file&target=~%2F.pi%2Fagent&file=APPEND_SYSTEM.md',
    );
  });
});
