import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { hooksApi, type HookEntry } from '../../api/hooks';
import { HooksConfigDialog } from './HooksConfigView';

vi.mock('../CopyButton', () => ({ default: () => null }));
vi.mock('../../api/hooks', async (load) => ({ ...await load<typeof import('../../api/hooks')>(), hooksApi: { render: vi.fn() } }));

const entry: HookEntry = { bindings: { codex: { events: { Stop: [{ hooks: [{ type: 'command', command: 'true' }] }] } } } };
const view = (mutation = { name: 'lint', entry }) => render(
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><I18nProvider><HooksConfigDialog mutation={mutation} sourcePath="/home/u/.config/skillshare/config.yaml" onClose={vi.fn()} /></I18nProvider></QueryClientProvider>,
);

describe('Hooks native config view', () => {
  beforeEach(() => vi.resetAllMocks());

  it('asks for a project hook under its root and names the project', async () => {
    vi.mocked(hooksApi.render).mockResolvedValue({ rendered: [{ target: 'codex', path: '/work/app/.codex/hooks.json', content: '{}' }] });
    view({ name: 'lint', entry, project: '/work/app' } as never);
    expect(await screen.findByText('Sync writes 1 file')).toBeInTheDocument();
    expect(hooksApi.render).toHaveBeenCalledWith(expect.objectContaining({ project: '/work/app', name: 'lint' }));
    expect(screen.getByTitle('/work/app')).toBeInTheDocument();
  });

  it('shows an Agent that cannot be rendered with its error, and other files still show', async () => {
    vi.mocked(hooksApi.render).mockResolvedValue({ rendered: [{ target: 'codex', path: '/h/.codex/hooks.json', error: 'unsupported event' }, { target: 'pi', path: '/h/.pi/agent/extensions/skillshare-lint.ts', content: 'export default {}' }] });
    view();
    expect((await screen.findAllByText('unsupported event')).length).toBeGreaterThan(0);
    expect(screen.getByText('Sync writes 1 file')).toBeInTheDocument();
  });

  it('lists an Agent whose error has no path, and shows the global scope', async () => {
    vi.mocked(hooksApi.render).mockResolvedValue({ rendered: [{ target: 'codex', error: 'cannot resolve config directory' }] });
    view();
    expect((await screen.findAllByText('cannot resolve config directory')).length).toBeGreaterThan(0);
    expect(screen.getByText('Global')).toBeInTheDocument();
    // Selected row has no path: the header names the Agent, not the Skillshare source.
    expect(screen.getByRole('dialog').querySelector('[title=""], span.font-mono.text-xs')).toHaveTextContent('Codex');
  });

  it('says a disabled hook writes nothing and a hook without Agents writes nothing', async () => {
    vi.mocked(hooksApi.render).mockResolvedValue({ rendered: [] });
    view({ name: 'lint', entry: { enabled: false, bindings: {} } });
    expect(await screen.findByText('Sync writes 0 files')).toBeInTheDocument();
    expect(await screen.findByText(/This hook is disabled/)).toBeInTheDocument();
    expect(screen.getByText(/no targets selected/)).toBeInTheDocument();
    expect(screen.getByRole('dialog')).toHaveTextContent('"enabled": false');
  });

  it('surfaces a render failure', async () => {
    vi.mocked(hooksApi.render).mockRejectedValue(new Error('boom'));
    view();
    expect(await screen.findByText('boom')).toBeInTheDocument();
  });

  it('switches from a native command file to a script, raw Pi TypeScript and the source, showing each one', async () => {
    const user = userEvent.setup();
    vi.mocked(hooksApi.render).mockResolvedValue({ rendered: [
      { target: 'claude', path: '/h/.claude/settings.json', content: '{"hooks":{"Stop":[{"command":"claudeCommandJson"}]}}' },
      { target: 'claude', path: '/h/.claude/hooks/skillshare/lint/check.sh', content: 'scriptBodyLine' },
      { target: 'pi', path: '/h/.pi/agent/extensions/skillshare-lint.ts', content: 'export default function piExtensionSource() {}' },
    ] });
    view();
    const dialog = await screen.findByRole('dialog');
    await within(dialog).findByText('Sync writes 3 files');
    expect(dialog).toHaveTextContent('claudeCommandJson');
    await user.click(within(dialog).getByRole('button', { name: /check\.sh/ }));
    expect(dialog).toHaveTextContent('scriptBodyLine');
    expect(dialog).not.toHaveTextContent('claudeCommandJson');
    await user.click(within(dialog).getByRole('button', { name: /skillshare-lint\.ts/ }));
    expect(dialog).toHaveTextContent('piExtensionSource');
    expect(dialog).not.toHaveTextContent('scriptBodyLine');
    await user.click(within(dialog).getByRole('button', { name: /Skillshare source/ }));
    expect(dialog).toHaveTextContent('"command": "true"');
    expect(dialog).not.toHaveTextContent('piExtensionSource');
  });

  it('shows the loading spinner while the source is the initial selection', () => {
    vi.mocked(hooksApi.render).mockReturnValue(new Promise(() => {}));
    view();
    expect(screen.getByRole('dialog').querySelector('[role="status"], .animate-spin')).not.toBeNull();
  });
});
