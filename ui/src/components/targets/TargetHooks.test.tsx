import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { hooksApi, type HookInventory } from '../../api/hooks';
import { I18nProvider } from '../../i18n';
import { LOCALE_STORAGE_KEY, messagesByLocale, supportedLocales } from '../../i18n/locales';
import { hookAgents } from '../../api/hooks';
import { ToastProvider } from '../Toast';
import TargetHooks from './TargetHooks';

vi.mock('../../api/hooks', async (load) => ({ ...await load<typeof import('../../api/hooks')>(), hooksApi: { syncProject: vi.fn().mockResolvedValue({ applied: [], backupIds: [] }), configure: vi.fn().mockResolvedValue({ applied: [], backupIds: [] }), preview: vi.fn(), save: vi.fn(), catalog: vi.fn().mockResolvedValue({}) } }));

const codex = { bindings: { codex: { events: { Stop: [{ hooks: [{ type: 'command', command: 'true' }] }] } } } };
const data = {
  source: { path: '', configPath: '', entries: { 'global-lint': codex }, projects: { '/work/app': { entries: { 'app-fmt': codex } } } },
  targets: [{ name: 'codex', kind: 'command' }], paths: { codex: '/home/me/.codex/hooks.json' },
  plan: { revision: 'r1', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [
    { target: 'codex', path: '/home/me/.codex/hooks.json', name: 'global-lint', action: 'add' },
    { target: 'codex', path: '/work/app/.codex/hooks.json', name: 'app-fmt', root: '/work/app', action: 'add' },
  ] },
  previewError: '', backups: [], unmanaged: [],
} as unknown as HookInventory;

const view = (project?: string) => render(
  <MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><TargetHooks agent="codex" data={data} project={project} /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>,
);

describe('Target hooks tab', () => {
  it("lists the global hooks on a global target, not a project's", () => {
    view();
    expect(screen.getByText('global-lint')).toBeInTheDocument();
    expect(screen.queryByText('app-fmt')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Hooks page' })).toHaveAttribute('href', '/hooks');
  });

  it("lists only the project's hooks on a project target and links to its Hooks tab", () => {
    view('/work/app');
    expect(screen.getByText('app-fmt')).toBeInTheDocument();
    expect(screen.queryByText('global-lint')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Hooks page' })).toHaveAttribute('href', '/projects/%2Fwork%2Fapp?tab=hooks');
  });

  it('counts every write the global sync makes while this Agent keeps its own state', () => {
    view();
    expect(screen.getByText(/2 changes in all/)).toBeInTheDocument();
    expect(screen.getByText('1 change not written yet')).toBeInTheDocument();
    expect(screen.getByText('1 hook is configured for Codex.')).toBeInTheDocument();
  });

  it("counts only the project's root on a project target", () => {
    view('/work/app');
    expect(screen.getByText(/1 change in all/)).toBeInTheDocument();
  });

  it('names the Agent in the empty state instead of leaving the placeholder', () => {
    view('/work/none');
    expect(screen.getByText('No hooks for Codex yet')).toBeInTheDocument();
    expect(screen.queryByText(/\{name\}/)).not.toBeInTheDocument();
  });

  it("syncs a project target through that project's root only", async () => {
    vi.mocked(hooksApi.preview).mockResolvedValue({ revision: 'r2', fingerprint: 'fp', sourcePath: '', blocked: false, changes: [{ target: 'codex', path: '/work/app/.codex/hooks.json', name: 'app-fmt', root: '/work/app', action: 'add' }] });
    const user = userEvent.setup();
    view('/work/app');
    await user.click(screen.getByRole('button', { name: 'Sync all Agents' }));
    await user.click(await screen.findByRole('button', { name: 'Sync Now' }));
    await waitFor(() => expect(hooksApi.syncProject).toHaveBeenCalledWith('/work/app', 'r2'));
    expect(hooksApi.configure).not.toHaveBeenCalled();
  });
});

describe('Target hooks editing', () => {
  it("keeps the hook's account bindings when it is saved from an Agent's tab", async () => {
    vi.mocked(hooksApi.save).mockResolvedValue({ applied: [], backupIds: [] });
    const both = { bindings: { codex: codex.bindings.codex, 'codex-2': codex.bindings.codex } };
    const inventory = { ...data, source: { ...data.source, entries: { 'global-lint': both } }, targets: [{ name: 'codex', kind: 'command' }, { name: 'codex-2', agent: 'codex', kind: 'command' }] } as unknown as HookInventory;
    const user = userEvent.setup();
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><TargetHooks agent="codex" data={inventory} /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    await user.click(screen.getByRole('button', { name: 'Edit global-lint' }));
    await user.click(await screen.findByRole('button', { name: 'Save' }));
    await waitFor(() => expect(hooksApi.save).toHaveBeenCalledWith({ name: 'global-lint', entry: both }));
  });
});

describe('Target hooks conflicts', () => {
  const blocked = (changes: object[]) => ({ ...data, plan: { revision: 'r1', fingerprint: 'fp', sourcePath: '', blocked: true, changes } }) as unknown as HookInventory;
  const show = (inventory: HookInventory) => render(
    <MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><TargetHooks agent="codex" data={inventory} /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>,
  );

  it("words this Agent's unmanaged-hook conflict for the dashboard", () => {
    show(blocked([{ target: 'codex', path: '/home/me/.codex/hooks.json', name: 'global-lint', action: 'conflict', message: 'an identical hook exists that Skillshare does not manage; import it or explicitly replace it' }]));
    expect(screen.getByText(/The same hook already exists/)).toBeInTheDocument();
    expect(screen.queryByText(/explicitly replace it/)).not.toBeInTheDocument();
  });

  it("explains a sync held by another Agent's conflict and opens the review", async () => {
    vi.mocked(hooksApi.preview).mockResolvedValue(blocked([]).plan!);
    const user = userEvent.setup();
    show(blocked([
      { target: 'codex', path: '/home/me/.codex/hooks.json', name: 'global-lint', action: 'add' },
      { target: 'claude', path: '/home/me/.claude/settings.json', name: 'other', action: 'conflict' },
    ]));
    expect(screen.getByText(/Nothing is written until/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'View conflicts' }));
    expect(await screen.findByRole('button', { name: 'Sync Now' })).toBeDisabled();
    expect(hooksApi.configure).not.toHaveBeenCalled();
  });
});

describe('Target hooks native guidance', () => {
  it('uses the account binding key but its native Agent for code and guidance', () => {
    const inventory = { ...data, source: { ...data.source, entries: { guard: { bindings: { 'omp-work': { code: 'export default () => {}' } } } } }, targets: [{ name: 'omp-work', agent: 'omp', kind: 'code' }] } as HookInventory;
    render(<MemoryRouter><QueryClientProvider client={new QueryClient()}><I18nProvider><ToastProvider><TargetHooks agent="omp-work" data={inventory} /></ToastProvider></I18nProvider></QueryClientProvider></MemoryRouter>);
    expect(screen.getByText('1 hook is configured for omp-work (Oh My Pi).')).toBeInTheDocument();
    expect(screen.getByText('code')).toBeInTheDocument();
    expect(screen.getByText(/asks for no project trust/)).toBeInTheDocument();
  });

  it('shows the Codex guidance in the dashboard language and keeps the native terms', async () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, 'zh-TW');
    view();
    const note = await screen.findByText(/Codex 會一併載入/);
    expect(note.textContent).not.toMatch(/Codex loads hooks\.json/);
    for (const term of ['hooks.json', 'config.toml', '[hooks]', '/hooks', '.codex']) expect(note.textContent).toContain(term);
    localStorage.clear();
  });
});

describe('Native guidance translations', () => {
  const placeholders = (v: string) => [...v.matchAll(/\{([\w.-]+)\}/g)].map((m) => m[1]).sort();
  it('has guidance for every hook Agent in every locale, with the English placeholders', () => {
    for (const { code } of supportedLocales) {
      for (const agent of hookAgents) {
        const key = `hooks.native.${agent}`;
        const text = (messagesByLocale[code] as Record<string, string>)[key];
        expect(text, `${code} ${key}`).toBeTruthy();
        expect(placeholders(text), `${code} ${key}`).toEqual(placeholders((messagesByLocale.en as Record<string, string>)[key]));
      }
    }
  });
});
