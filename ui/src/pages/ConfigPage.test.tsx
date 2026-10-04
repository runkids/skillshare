import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import type { Overview, SkillfollowResponse } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import ConfigPage from './ConfigPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
// CodeMirror does not edit under jsdom; a textarea stands in for the editor, and a stub view records
// where the page puts the cursor.
const view = { text: '', state: { doc: { get length() { return view.text.length; }, toString: () => view.text } }, dispatch: vi.fn(), focus: vi.fn() };
vi.mock('@uiw/react-codemirror', () => ({
  default: ({ value, onChange, onCreateEditor }: { value: string; onChange: (v: string) => void; onCreateEditor?: (v: unknown) => void }) => {
    view.text = value;
    onCreateEditor?.(view);
    return <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />;
  },
}));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return {
    ...actual,
    api: { ...actual.api, getOverview: vi.fn(), getConfig: vi.fn(), getSkillignore: vi.fn(), getAgentignore: vi.fn(), getSkillfollow: vi.fn(), putSkillfollow: vi.fn() },
  };
});

function followResponse(base: string, local = '', entries: SkillfollowResponse['entries'] = []): SkillfollowResponse {
  return {
    base: { path: '/src/.skillfollow', exists: base !== '', content: base },
    local: { path: '/src/.skillfollow.local', exists: local !== '', content: local },
    active: base !== '' || local !== '',
    local_active: local !== '',
    entries,
    warnings: [],
    prune_paused: entries.filter((e) => e.state === 'missing').map((e) => `prune paused: ${e.name} is missing`),
  };
}

function renderPage(query: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={[`/config?${query}`]}>
      <QueryClientProvider client={client}><I18nProvider><ToastProvider><ConfigPage /></ToastProvider></I18nProvider></QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('ConfigPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getOverview).mockResolvedValue({} as Overview);
    vi.mocked(api.getConfig).mockResolvedValue({ config: {}, raw: 'source: ~/skills\n' });
    vi.mocked(api.getSkillignore).mockResolvedValue({ exists: true, path: '', raw: 'draft-*\n' });
    vi.mocked(api.getAgentignore).mockResolvedValue({ exists: true, path: '', raw: 'old-*\n' });
  });

  it('reverts the open ignore file, not config.yaml', async () => {
    const user = userEvent.setup();
    renderPage('tab=skillignore');

    const editor = await screen.findByDisplayValue('draft-*');
    fireEvent.change(editor, { target: { value: 'draft-*\ntmp-*\n' } });
    await user.click(screen.getAllByRole('button', { name: 'Revert' })[0]);
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Revert' }));

    expect((screen.getByLabelText('editor') as HTMLTextAreaElement).value).toBe('draft-*\n');
  });

  it('opens config.yaml with the cursor on the section a page links to', async () => {
    const raw = 'source: ~/skills\nmcp:\n  servers: {}\nhooks:\n  entries: {}\n';
    vi.mocked(api.getConfig).mockResolvedValue({ config: {}, raw });
    renderPage('section=hooks');
    await screen.findByDisplayValue(/entries/);
    await waitFor(() => expect(view.dispatch).toHaveBeenCalledWith(expect.objectContaining({ selection: { anchor: raw.indexOf('hooks:') } })));
    expect(view.dispatch).toHaveBeenCalledTimes(1);
  });

  it('opens at the top when config.yaml has no such section', async () => {
    renderPage('section=hooks');
    await screen.findByDisplayValue(/skills/);
    await waitFor(() => expect(view.focus).toHaveBeenCalled());
    expect(view.dispatch).not.toHaveBeenCalled();
  });

  describe('.skillfollow tab', () => {
    const team = { name: '_team', state: 'followed', resolved_target: '/work/team', reason: 'following directory' };
    const gone = { name: '_gone', state: 'missing', reason: 'no such file or directory' };

    it('shows each declared entry with its state and the prune pause', async () => {
      vi.mocked(api.getSkillfollow).mockResolvedValue(followResponse('_team\n_gone\n', '', [team, gone]));
      renderPage('tab=skillfollow');

      const rows = await screen.findAllByRole('row');
      expect(rows.slice(1).map((row) => within(row).getAllByRole('cell').slice(0, 2).map((c) => c.textContent))).toEqual([
        ['_team', 'followed'],
        ['_gone', 'missing'],
      ]);
      expect(screen.getByText('prune paused: _gone is missing')).toBeInTheDocument();
    });

    it('shows an empty editor and no entries without declaration files', async () => {
      vi.mocked(api.getSkillfollow).mockResolvedValue(followResponse(''));
      renderPage('tab=skillfollow');

      expect(await screen.findByText('No entries declared.')).toBeInTheDocument();
      expect((screen.getByLabelText('editor') as HTMLTextAreaElement).value).toBe('');
    });

    it('shows a rejected line inline and keeps the edit', async () => {
      const user = userEvent.setup();
      vi.mocked(api.getSkillfollow).mockResolvedValue(followResponse('_team\n', '', [team]));
      vi.mocked(api.putSkillfollow).mockRejectedValue(new Error('.skillfollow:2: invalid first-level entry "a/b"'));
      renderPage('tab=skillfollow');

      fireEvent.change(await screen.findByDisplayValue('_team'), { target: { value: '_team\na/b\n' } });
      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(await screen.findByRole('alert')).toHaveTextContent('.skillfollow:2: invalid first-level entry "a/b"');
      expect((screen.getByLabelText('editor') as HTMLTextAreaElement).value).toBe('_team\na/b\n');
    });

    it('saves .skillfollow.local and shows the states the server returns', async () => {
      const user = userEvent.setup();
      vi.mocked(api.getSkillfollow).mockResolvedValue(followResponse('_team\n', '', [team]));
      vi.mocked(api.putSkillfollow).mockResolvedValue(followResponse('_team\n', '_gone\n', [team, gone]));
      renderPage('tab=skillfollow');

      await user.click(await screen.findByRole('radio', { name: '.skillfollow.local' }));
      fireEvent.change(screen.getByLabelText('editor'), { target: { value: '_gone\n' } });
      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(api.putSkillfollow).toHaveBeenCalledWith('local', '_gone\n');
      expect(await screen.findByText('prune paused: _gone is missing')).toBeInTheDocument();
    });
  });
});
