import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { hubDrafts, type HubDraft } from '../api/hubDrafts';
import HubPage from './HubPage';

vi.mock('../api/hubDrafts', async (original) => ({
  ...await original<typeof import('../api/hubDrafts')>(),
  hubDrafts: { list: vi.fn(), get: vi.fn(), candidates: vi.fn(), create: vi.fn(), save: vi.fn(), remove: vi.fn(), import: vi.fn(), export: vi.fn() },
}));
const draft: HubDraft = { id: 'a', revision: 'r1', name: 'Team', description: '', updatedAt: '', fields: {}, entries: [{ id: 'entry', data: { name: 'Review', source: '/local/review', future: 42 } }] };
function renderPage() {
 // The builder lives behind the "My hubs" tab; Browse hits network queries this suite does not mock.
 const router = createMemoryRouter([{ path: '*', element: <I18nProvider><HubPage /></I18nProvider> }], { initialEntries: ['/?tab=mine'] });
 return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { mutations: { retry: false } } })}><RouterProvider router={router} /></QueryClientProvider>);
}
beforeEach(() => {
 vi.clearAllMocks();
 vi.mocked(hubDrafts.list).mockResolvedValue([draft]);
 vi.mocked(hubDrafts.get).mockResolvedValue({ draft, problems: [{ entryId: 'entry', code: 'local_source' }] });
 vi.mocked(hubDrafts.candidates).mockResolvedValue([]);
});
it('resumes a draft and requires saving changed sources before export', async () => {
 const user = userEvent.setup(); renderPage();
 await user.click(await screen.findByRole('button', { name: /Team/ }));
 await user.click(await screen.findByRole('button', { name: 'Edit Review' }));
 expect(await screen.findByDisplayValue('/local/review')).toBeInTheDocument();
 expect(screen.getByRole('button', { name: 'Download index' })).toBeDisabled();
 const source = screen.getByLabelText('Install source');
 await user.clear(source); await user.type(source, 'acme/review');
 vi.mocked(hubDrafts.save).mockImplementation(async d => ({ draft: { ...d, revision: 'r2' }, problems: [] }));
 await user.click(screen.getByRole('button', { name: 'Save draft' }));
 await waitFor(() => expect(screen.getByRole('button', { name: 'Download index' })).toBeEnabled());
 expect(vi.mocked(hubDrafts.save).mock.calls[0][0].entries[0].data.future).toBe(42);
});
it('confirms deletion and keeps local-only entries visible', async () => {
 const user = userEvent.setup(); renderPage();
 await user.click(await screen.findByRole('button', { name: /Team/ }));
 expect(await screen.findByText('This source is local. Enter a remote repository before exporting.')).toBeInTheDocument();
 await user.click(screen.getByRole('button', { name: 'Delete draft' }));
 expect(hubDrafts.remove).not.toHaveBeenCalled();
 expect(screen.getByRole('dialog')).toBeInTheDocument();
});

it('keeps unsaved edits after a revision conflict', async () => {
 const { ApiError } = await import('../api/client');
 vi.mocked(hubDrafts.save).mockRejectedValue(new ApiError(409, 'stale'));
 const user = userEvent.setup(); renderPage();
 await user.click(await screen.findByRole('button', { name: /Team/ }));
 const name = await screen.findByLabelText('Hub name');
 await user.clear(name); await user.type(name, 'My edit');
 await user.click(screen.getByRole('button', { name: 'Save draft' }));
 expect(await screen.findByRole('alert')).toHaveTextContent('another window');
 expect(screen.getByDisplayValue('My edit')).toBeInTheDocument();
 expect(screen.getByRole('button', { name: 'Download index' })).toBeDisabled();
});

it('warns before navigating away from edits', async () => {
 const user = userEvent.setup(); renderPage();
 await user.click(await screen.findByRole('button', { name: /Team/ }));
 await user.type(await screen.findByLabelText('Hub name'), ' edits');
 await user.click(screen.getByRole('link', { name: 'Back' }));
 expect(await screen.findByRole('dialog')).toHaveTextContent('Discard unsaved changes?');
 await user.click(screen.getByRole('button', { name: 'Cancel' }));
 expect(screen.getByDisplayValue('Team edits')).toBeInTheDocument();
});
