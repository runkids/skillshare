import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { expect, it, vi } from 'vitest';
import { api, type Skill } from '../api/client';
import { I18nProvider } from '../i18n';
import { ToastProvider } from '../components/Toast';
import ResourceDetailPage from './ResourceDetailPage';

vi.mock('../api/client', async (load) => ({ ...await load<typeof import('../api/client')>(), api: {
  getSyncMatrix: vi.fn(), listTargets: vi.fn(), getResource: vi.fn(), listSkills: vi.fn(),
  auditSkill: vi.fn(), diff: vi.fn(), batchUninstall: vi.fn(), moveResources: vi.fn(),
} }));

it('uninstalls only the linked skill from its detail page', async () => {
  const user = userEvent.setup();
  const linked: Skill = {
    name: 'foo', kind: 'skill', flatName: '_dev__foo', relPath: '_dev/foo',
    sourcePath: '/skills/_dev/foo', isInRepo: true, linkName: '_dev', linkTarget: '/checkout',
  };
  vi.mocked(api.getResource).mockResolvedValue({ resource: linked, skillMdContent: '', files: [] });
  vi.mocked(api.listSkills).mockResolvedValue({ resources: [linked, { ...linked, name: 'bar', flatName: '_dev__bar', relPath: '_dev/bar' }] });
  vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 2 });
  vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [] } as never);
  vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as never);
  vi.mocked(api.auditSkill).mockResolvedValue({ result: { findings: [] } } as never);
  vi.mocked(api.batchUninstall).mockResolvedValue({ results: [], summary: { succeeded: 1, failed: 0 } });
  render(<MemoryRouter initialEntries={['/skills/_dev__foo']}>
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <I18nProvider><ToastProvider><Routes>
        <Route path="/skills/:name" element={<ResourceDetailPage />} />
        <Route path="/skills" element={<div>Skills</div>} />
      </Routes></ToastProvider></I18nProvider>
    </QueryClientProvider>
  </MemoryRouter>);
  await user.click(await screen.findByRole('button', { name: 'More actions' }));
  expect(screen.queryByRole('menuitem', { name: 'Uninstall Repo' })).toBeNull();
  expect(screen.queryByRole('menuitem', { name: 'Move to folder…' })).toBeNull();
  await user.click(screen.getByRole('menuitem', { name: 'Uninstall' }));
  const dialog = screen.getByRole('dialog');
  expect(within(dialog).getByText('Selected skills are moved out of the linked folder to the trash.')).toBeInTheDocument();
  expect(within(dialog).getByText('foo')).toBeInTheDocument();
  expect(within(dialog).queryByText('bar')).toBeNull();
  await user.click(within(dialog).getByRole('button', { name: 'Uninstall' }));
  await waitFor(() => expect(api.batchUninstall).toHaveBeenCalledWith({ names: ['_dev__foo'], kind: 'skill', force: false }));
});

it('offers Move to folder for a plain skill and opens the dialog', async () => {
  const user = userEvent.setup();
  const plain: Skill = { name: 'demo', kind: 'skill', flatName: 'grp__demo', relPath: 'grp/demo', sourcePath: '/skills/grp/demo', isInRepo: false };
  vi.mocked(api.getResource).mockResolvedValue({ resource: plain, skillMdContent: '', files: [] });
  vi.mocked(api.listSkills).mockResolvedValue({ resources: [plain] });
  vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 1 });
  vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [] } as never);
  vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as never);
  vi.mocked(api.auditSkill).mockResolvedValue({ result: { findings: [] } } as never);
  render(<MemoryRouter initialEntries={['/skills/grp__demo']}>
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <I18nProvider><ToastProvider><Routes>
        <Route path="/skills/:name" element={<ResourceDetailPage />} />
      </Routes></ToastProvider></I18nProvider>
    </QueryClientProvider>
  </MemoryRouter>);
  await user.click(await screen.findByRole('button', { name: 'More actions' }));
  await user.click(screen.getByRole('menuitem', { name: 'Move to folder…' }));
  expect(await screen.findByRole('dialog', { name: 'Move to folder' })).toBeInTheDocument();
});

it('keeps the move result open after the old name stops loading, and goes to the new name on close', async () => {
  const user = userEvent.setup();
  HTMLElement.prototype.scrollIntoView = vi.fn();
  const plain: Skill = { name: 'demo', kind: 'skill', flatName: 'grp__demo', relPath: 'grp/demo', sourcePath: '/skills/grp/demo', isInRepo: false };
  const moved: Skill = { ...plain, flatName: 'archive__demo', relPath: 'archive/demo' };
  vi.mocked(api.getResource).mockImplementation(async (name) => {
    if (name === 'archive__demo') return { resource: moved, skillMdContent: '', files: [] };
    if (vi.mocked(api.moveResources).mock.calls.some(([o]) => !o.dryRun)) throw new Error('not found');
    return { resource: plain, skillMdContent: '', files: [] };
  });
  vi.mocked(api.listSkills).mockResolvedValue({ resources: [plain, { ...plain, name: 'old', flatName: 'archive__old', relPath: 'archive/old' }] });
  vi.mocked(api.listTargets).mockResolvedValue({ targets: [], sourceSkillCount: 1 });
  vi.mocked(api.getSyncMatrix).mockResolvedValue({ entries: [] } as never);
  vi.mocked(api.diff).mockResolvedValue({ diffs: [] } as never);
  vi.mocked(api.auditSkill).mockResolvedValue({ result: { findings: [] } } as never);
  vi.mocked(api.moveResources).mockImplementation(async (o) => ({
    results: [{ name: 'grp__demo', success: true, from: 'grp/demo', to: 'archive/demo', flatName: 'archive__demo', record: true }],
    summary: { succeeded: 1, failed: 0 }, warnings: [], dryRun: !!o.dryRun,
  }));
  render(<MemoryRouter initialEntries={['/skills/grp__demo']}>
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <I18nProvider><ToastProvider><Routes>
        <Route path="/skills/:name" element={<ResourceDetailPage />} />
      </Routes></ToastProvider></I18nProvider>
    </QueryClientProvider>
  </MemoryRouter>);
  await user.click(await screen.findByRole('button', { name: 'More actions' }));
  await user.click(screen.getByRole('menuitem', { name: 'Move to folder…' }));
  await user.click(await screen.findByRole('combobox', { name: 'Destination' }));
  await user.click(await screen.findByRole('option', { name: /^archive/ }));
  await user.click(await screen.findByRole('button', { name: /^Move 1 skill$/ }));

  expect(await screen.findByText('Moved 1 skill')).toBeInTheDocument();
  await waitFor(() => expect(screen.getByText(/Failed to load/i)).toBeInTheDocument());
  expect(screen.getByText('Moved 1 skill')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: 'Later' }));
  await waitFor(() => expect(api.getResource).toHaveBeenCalledWith('archive__demo', undefined));
});
