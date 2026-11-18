import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { api } from '../api/client';
import { hubDrafts, hubRefs } from '../api/hubDrafts';
import type { HubDraft } from '../api/hubDrafts';
import HubPage from './HubPage';

vi.mock('../api/hubDrafts', async (original) => ({
  ...(await original<typeof import('../api/hubDrafts')>()),
  hubDrafts: {
    list: vi.fn(),
    get: vi.fn(),
    candidates: vi.fn(),
    create: vi.fn(),
    save: vi.fn(),
    remove: vi.fn(),
    import: vi.fn(),
    export: vi.fn(),
  },
  hubRefs: vi.fn(),
}));
vi.mock('../api/client', async (original) => {
  const mod = await original<typeof import('../api/client')>();
  return {
    ...mod,
    api: {
      ...mod.api,
      getHubConfig: vi.fn(),
      putHubConfig: vi.fn(),
      listSkills: vi.fn(),
      searchHub: vi.fn(),
      discover: vi.fn(),
    },
  };
});

const draft: HubDraft = {
  id: 'a',
  revision: 'r1',
  name: 'Team',
  description: '',
  updatedAt: '',
  fields: { publishUrl: 'https://host/team/skillshare-hub.json' },
  entries: [{ id: 'entry', data: { name: 'Review', source: '/local/review', future: 42 } }],
};

function renderPage() {
  const router = createMemoryRouter(
    [
      {
        path: '*',
        element: (
          <I18nProvider>
            <HubPage />
          </I18nProvider>
        ),
      },
    ],
    { initialEntries: ['/hubs'] },
  );
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
  HTMLElement.prototype.scrollIntoView = vi.fn();
  vi.mocked(hubDrafts.list).mockResolvedValue([draft]);
  vi.mocked(hubDrafts.get).mockResolvedValue({
    draft,
    problems: [{ entryId: 'entry', code: 'local_source' }],
    refs: { entry: 'v1.2.0' },
  });
  vi.mocked(api.getHubConfig).mockResolvedValue({
    hubs: [{ label: 'team-copy', url: 'https://host/team/skillshare-hub.json/' }],
    default: '',
  });
  vi.mocked(api.listSkills).mockResolvedValue({ resources: [] } as unknown as Awaited<
    ReturnType<typeof api.listSkills>
  >);
  vi.mocked(api.searchHub).mockResolvedValue({ results: [] });
});

it('saves in-place edits, keeps unknown fields and refreshes the hub list', async () => {
  const user = userEvent.setup();
  renderPage();
  await user.click(await screen.findByRole('button', { name: 'Edit' }));
  expect(
    screen.getByText('This source is local. Enter a remote repository before exporting.'),
  ).toBeInTheDocument();
  const source = screen.getByLabelText('Source');
  await user.clear(source);
  await user.type(source, 'acme/review');
  vi.mocked(hubDrafts.save).mockImplementation(async (d) => ({
    draft: { ...d, revision: 'r2' },
    problems: [],
  }));
  await user.click(screen.getByRole('button', { name: 'Save' }));
  expect(await screen.findByRole('button', { name: 'Edit' })).toBeInTheDocument();
  expect(vi.mocked(hubDrafts.save).mock.calls[0][0].entries[0].data.future).toBe(42);
  expect(hubDrafts.list).toHaveBeenCalledTimes(2);
});

it('confirms deletion', async () => {
  const user = userEvent.setup();
  renderPage();
  await user.click(await screen.findByRole('button', { name: 'More actions' }));
  await user.click(screen.getByRole('menuitem', { name: 'Delete Hub' }));
  expect(hubDrafts.remove).not.toHaveBeenCalled();
  expect(screen.getByRole('dialog')).toHaveTextContent('Delete this Hub permanently?');
});

it('keeps unsaved edits after a revision conflict', async () => {
  const { ApiError } = await import('../api/client');
  vi.mocked(hubDrafts.save).mockRejectedValue(new ApiError(409, 'stale'));
  const user = userEvent.setup();
  renderPage();
  await user.click(await screen.findByRole('button', { name: 'Edit' }));
  const name = screen.getByLabelText('Name', { selector: '#hub-name' });
  await user.clear(name);
  await user.type(name, 'My edit');
  await user.click(screen.getByRole('button', { name: 'Save' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('another window');
  expect(screen.getByDisplayValue('My edit')).toBeInTheDocument();
});

it('warns before navigating away from edits', async () => {
  const user = userEvent.setup();
  renderPage();
  await user.click(await screen.findByRole('button', { name: 'Edit' }));
  await user.type(screen.getByLabelText('Name', { selector: '#hub-name' }), ' edits');
  await user.click(screen.getByRole('link', { name: 'Back' }));
  const dialog = await screen.findByRole('dialog');
  expect(dialog).toHaveTextContent('Discard unsaved changes?');
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
  expect(screen.getByDisplayValue('Team edits')).toBeInTheDocument();
});

it('adds skills found at a URL, pinned to the chosen version', async () => {
  vi.mocked(hubDrafts.get).mockResolvedValue({
    draft: {
      ...draft,
      entries: [
        {
          id: 'lint',
          data: { name: 'lint', source: 'github.com/acme/skills/tree/v1.0.0/skills/lint' },
        },
      ],
    },
    problems: [],
  });
  const refs = {
    pinnable: true,
    current: '',
    defaultBranch: 'main',
    branches: ['main'],
    tags: ['v1.0.0'],
  };
  vi.mocked(hubRefs).mockImplementation(async (_source, ref) =>
    ref === undefined
      ? refs
      : { ...refs, source: ref ? `github.com/acme/skills/tree/${ref}` : 'github.com/acme/skills' },
  );
  const skills = [
    { name: 'review', path: 'skills/review', description: 'Reviews diffs' },
    { name: 'lint', path: 'skills/lint' },
  ];
  vi.mocked(api.discover).mockResolvedValue({ needsSelection: true, skills, agents: [] });

  const user = userEvent.setup();
  renderPage();
  await user.click(await screen.findByRole('button', { name: 'Edit' }));
  await user.click(screen.getByRole('button', { name: 'Add skill' }));
  const dialog = screen.getByRole('dialog');
  await user.type(within(dialog).getByLabelText('Git URL'), 'github.com/acme/skills');
  await user.click(within(dialog).getByRole('button', { name: 'Find' }));
  await user.click(await within(dialog).findByRole('combobox', { name: 'Version' }));
  await user.click(screen.getByRole('option', { name: 'v1.0.0' }));
  await waitFor(() =>
    expect(api.discover).toHaveBeenLastCalledWith('github.com/acme/skills/tree/v1.0.0', 'v1.0.0'),
  );
  expect(within(dialog).getByText('Already in this Hub')).toBeInTheDocument();
  await user.click(within(dialog).getByRole('button', { name: 'Add 1' }));
  expect(
    screen.getByDisplayValue('github.com/acme/skills/tree/v1.0.0/skills/review'),
  ).toBeInTheDocument();
});

it('keeps a GitLab repo root apart from the skill paths added under it', async () => {
  const refs = { pinnable: true, current: '', defaultBranch: 'main', branches: ['main'], tags: [] };
  vi.mocked(hubRefs).mockImplementation(async (_source, ref) =>
    ref === undefined ? refs : { ...refs, source: 'https://gitlab.com/g/r.git' },
  );
  vi.mocked(api.discover).mockResolvedValue({
    needsSelection: true,
    skills: [{ name: 'lint', path: 'skills/lint' }],
    agents: [],
  });

  const user = userEvent.setup();
  renderPage();
  await user.click(await screen.findByRole('button', { name: 'Edit' }));
  await user.click(screen.getByRole('button', { name: 'Add skill' }));
  const dialog = screen.getByRole('dialog');
  await user.type(within(dialog).getByLabelText('Git URL'), 'https://gitlab.com/g/r');
  await user.click(within(dialog).getByRole('button', { name: 'Find' }));
  await user.click(await within(dialog).findByRole('button', { name: 'Add 1' }));
  expect(screen.getByDisplayValue('https://gitlab.com/g/r.git/skills/lint')).toBeInTheDocument();
});
