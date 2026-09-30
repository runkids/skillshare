import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api/client';
import type { Overview } from '../api/client';
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
    api: { ...actual.api, getOverview: vi.fn(), getConfig: vi.fn(), getSkillignore: vi.fn(), getAgentignore: vi.fn() },
  };
});

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
});
