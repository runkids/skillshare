import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import { I18nProvider } from '../i18n';
import InstallDialog from './InstallDialog';
import { ToastProvider } from './Toast';

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listSkills: vi.fn(),
      getHubConfig: vi.fn(),
      putHubConfig: vi.fn(),
      searchHub: vi.fn(),
      discover: vi.fn(),
      install: vi.fn(),
      installBatch: vi.fn(),
    },
  };
});

function renderDialog(initialTab: 'search' | 'url') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <ToastProvider>
          <InstallDialog kind="skill" initialTab={initialTab} onClose={() => undefined} />
        </ToastProvider>
      </I18nProvider>
    </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('InstallDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listSkills).mockResolvedValue({ resources: [] });
    vi.mocked(api.getHubConfig).mockResolvedValue({ hubs: [], default: '' });
  });

  it('installs only the skills left selected after discovery', async () => {
    vi.mocked(api.discover).mockResolvedValue({ needsSelection: true, skills: [{ name: 'pdf', path: 'pdf' }, { name: 'docx', path: 'docx' }], agents: [] });
    vi.mocked(api.installBatch).mockResolvedValue({ results: [{ name: 'docx', action: 'installed' }], summary: 'Installed 1' });
    const user = userEvent.setup();
    renderDialog('url');

    await user.type(screen.getByLabelText(/git url/i), 'anthropics/skills');
    await user.click(screen.getByRole('button', { name: /find skills/i }));
    await user.click(await screen.findByRole('checkbox', { name: 'pdf' }));
    await user.click(screen.getByRole('button', { name: /install 1 skill$/i }));

    await waitFor(() => expect(api.installBatch).toHaveBeenCalledWith(
      expect.objectContaining({ source: 'anthropics/skills', skills: [expect.objectContaining({ name: 'docx' })] }),
    ));
  });

  it('force installs only the skill the audit blocked', async () => {
    vi.mocked(api.discover).mockResolvedValue({ needsSelection: true, skills: [{ name: 'deploy', path: 'deploy' }], agents: [] });
    vi.mocked(api.installBatch)
      .mockResolvedValueOnce({
        results: [{
          name: 'deploy',
          error: 'security audit failed — findings at/above HIGH detected:\n  HIGH: Downloads and runs a remote script (scripts/setup.sh:14)\n    "curl x | sh"\n\nUse --force to override',
        }],
        summary: '',
      })
      .mockResolvedValueOnce({ results: [{ name: 'deploy', action: 'installed' }], summary: 'Installed 1' });
    const user = userEvent.setup();
    renderDialog('url');

    await user.type(screen.getByLabelText(/git url/i), 'acme/ops-skills');
    await user.click(screen.getByRole('button', { name: /find skills/i }));
    await user.click(await screen.findByRole('button', { name: /install 1 skill$/i }));
    expect(await screen.findByText('Downloads and runs a remote script')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /force install/i }));

    await waitFor(() => expect(api.installBatch).toHaveBeenLastCalledWith(
      expect.objectContaining({ force: true, skills: [expect.objectContaining({ name: 'deploy' })] }),
    ));
  });

  it('reports an empty source instead of installing it when Find skills discovers nothing', async () => {
    vi.mocked(api.discover).mockResolvedValue({ needsSelection: false, skills: [], agents: [] });
    const user = userEvent.setup();
    renderDialog('url');

    await user.type(screen.getByLabelText(/git url/i), 'acme/notes');
    await user.click(screen.getByRole('button', { name: /find skills/i }));

    expect(await screen.findByText('No skills or agents found in acme/notes. Nothing was installed.')).toBeInTheDocument();
    expect(api.install).not.toHaveBeenCalled();
  });

  it('installs just the skill a hub entry names', async () => {
    vi.mocked(api.getHubConfig).mockResolvedValue({ hubs: [{ label: 'Acme', url: 'https://acme.dev/hub.json' }], default: 'Acme' });
    vi.mocked(api.searchHub).mockResolvedValue({
      results: [{ name: 'pdf', description: 'Read PDFs', source: 'anthropics/skills', skill: 'pdf', stars: 0, owner: 'anthropics', repo: 'skills' }],
    });
    vi.mocked(api.discover).mockResolvedValue({
      needsSelection: true,
      skills: [{ name: 'pdf', path: 'pdf' }, { name: 'docx', path: 'docx' }],
      agents: [],
      warnings: ['skipped git submodule "vendor/up"'],
    });
    vi.mocked(api.installBatch).mockResolvedValue({ results: [{ name: 'pdf', action: 'installed' }], summary: 'Installed 1' });
    const user = userEvent.setup();
    renderDialog('search');

    await screen.findByText('Acme');
    await user.type(screen.getByLabelText(/search skills/i), 'pdf{Enter}');
    await user.click(await screen.findByRole('button', { name: /^install$/i }));

    await waitFor(() => expect(api.installBatch).toHaveBeenCalledWith(
      expect.objectContaining({ source: 'anthropics/skills', skills: [expect.objectContaining({ name: 'pdf' })] }),
    ));
    expect(api.searchHub).toHaveBeenCalledWith('pdf', 'https://acme.dev/hub.json');
    expect(screen.getByText(/skipped git submodule "vendor\/up"/)).toBeInTheDocument();
  });

  it('sends the folder picked from the existing ones as `into`', async () => {
    vi.mocked(api.listSkills).mockResolvedValue({
      resources: [{ name: 'demo', kind: 'skill', flatName: 'frontend__demo', relPath: 'frontend/demo', sourcePath: '/s/frontend/demo', isInRepo: false }],
    });
    // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
    HTMLElement.prototype.scrollIntoView = vi.fn();
    vi.mocked(api.install).mockResolvedValue({ skillName: 'team' } as Awaited<ReturnType<typeof api.install>>);
    const user = userEvent.setup();
    renderDialog('url');

    await user.type(screen.getByLabelText(/git url/i), 'acme/team-skills');
    await user.click(screen.getByRole('switch', { name: /track this repo/i }));
    await user.click(screen.getByRole('button', { name: /advanced/i }));
    await user.click(await screen.findByRole('combobox', { name: /into folder/i }));
    await user.click(await screen.findByRole('option', { name: 'frontend' }));
    await user.click(screen.getByRole('button', { name: /install repo/i }));

    await waitFor(() => expect(api.install).toHaveBeenCalledWith(expect.objectContaining({ source: 'acme/team-skills', into: 'frontend' })));
  });

  it('blocks install while a new folder has no name', async () => {
    HTMLElement.prototype.scrollIntoView = vi.fn();
    const user = userEvent.setup();
    renderDialog('url');

    await user.type(screen.getByLabelText(/git url/i), 'acme/team-skills');
    await user.click(screen.getByRole('switch', { name: /track this repo/i }));
    await user.click(screen.getByRole('button', { name: /advanced/i }));
    await user.click(await screen.findByRole('combobox', { name: /into folder/i }));
    await user.click(await screen.findByRole('option', { name: /new folder/i }));

    expect(screen.getByRole('button', { name: /install repo/i })).toBeDisabled();
  });
});
