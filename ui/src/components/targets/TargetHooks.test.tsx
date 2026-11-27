import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { hooksApi, type HookInventory } from '../../api/hooks';
import { I18nProvider } from '../../i18n';
import { messagesByLocale, supportedLocales } from '../../i18n/locales';
import { hookAgents } from '../../api/hooks';
import { ToastProvider } from '../Toast';
import TargetHooks from './TargetHooks';

vi.mock('../../api/hooks', async (load) => ({
  ...(await load<typeof import('../../api/hooks')>()),
  hooksApi: {
    syncProject: vi.fn().mockResolvedValue({ applied: [], backupIds: [] }),
    configure: vi.fn().mockResolvedValue({ applied: [], backupIds: [] }),
    preview: vi.fn(),
    save: vi.fn(),
    catalog: vi.fn().mockResolvedValue({}),
  },
}));

const codex = {
  bindings: { codex: { events: { Stop: [{ hooks: [{ type: 'command', command: 'true' }] }] } } },
};
const data = {
  source: {
    path: '',
    configPath: '',
    entries: { 'global-lint': codex },
    projects: { '/work/app': { entries: { 'app-fmt': codex } } },
  },
  targets: [{ name: 'codex', kind: 'command' }],
  paths: { codex: '/home/me/.codex/hooks.json' },
  plan: {
    revision: 'r1',
    fingerprint: 'fp',
    sourcePath: '',
    blocked: false,
    changes: [
      { target: 'codex', path: '/home/me/.codex/hooks.json', name: 'global-lint', action: 'add' },
      {
        target: 'codex',
        path: '/work/app/.codex/hooks.json',
        name: 'app-fmt',
        root: '/work/app',
        action: 'add',
      },
    ],
  },
  previewError: '',
  backups: [],
  unmanaged: [],
} as unknown as HookInventory;

const view = (project?: string) =>
  render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient()}>
        <I18nProvider>
          <ToastProvider>
            <TargetHooks agent="codex" data={data} project={project} />
          </ToastProvider>
        </I18nProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );

describe('Target hooks tab', () => {
  it("syncs a project target through that project's root only", async () => {
    vi.mocked(hooksApi.preview).mockResolvedValue({
      revision: 'r2',
      fingerprint: 'fp',
      sourcePath: '',
      blocked: false,
      changes: [
        {
          target: 'codex',
          path: '/work/app/.codex/hooks.json',
          name: 'app-fmt',
          root: '/work/app',
          action: 'add',
        },
      ],
    });
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
    const inventory = {
      ...data,
      source: { ...data.source, entries: { 'global-lint': both } },
      targets: [
        { name: 'codex', kind: 'command' },
        { name: 'codex-2', agent: 'codex', kind: 'command' },
      ],
    } as unknown as HookInventory;
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <QueryClientProvider client={new QueryClient()}>
          <I18nProvider>
            <ToastProvider>
              <TargetHooks agent="codex" data={inventory} />
            </ToastProvider>
          </I18nProvider>
        </QueryClientProvider>
      </MemoryRouter>,
    );
    await user.click(screen.getByRole('button', { name: 'Edit global-lint' }));
    await user.click(await screen.findByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(hooksApi.save).toHaveBeenCalledWith({ name: 'global-lint', entry: both }),
    );
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
        expect(placeholders(text), `${code} ${key}`).toEqual(
          placeholders((messagesByLocale.en as Record<string, string>)[key]),
        );
      }
    }
  });
});
