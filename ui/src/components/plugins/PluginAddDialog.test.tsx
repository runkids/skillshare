import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PluginAddDialog from './PluginAddDialog';
import { pluginsApi } from '../../api/plugins';

const context = vi.hoisted(() => ({ isProjectMode: false }));
vi.mock('../../context/AppContext', () => ({ useAppContext: () => context }));
vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../i18n')>()),
  useT: () => (key: string) => key,
}));
vi.mock('../../api/plugins', async (original) => ({
  ...(await original<typeof import('../../api/plugins')>()),
  pluginsApi: { discover: vi.fn() },
}));

describe('PluginAddDialog targets', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    context.isProjectMode = false;
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      targetDefinitions: [
        { target: 'cursor', label: 'Cursor', project: false, operations: ['add'] },
        { target: 'antigravity', label: 'Antigravity Desktop', project: true, operations: ['add'] },
        { target: 'pi', label: 'Pi', project: true, operations: ['add'] },
        { target: 'opencode', label: 'OpenCode', project: true, operations: ['add'] },
      ],
      source: '/demo',
      digest: 'abc',
      candidates: [
        {
          name: 'demo',
          description: '',
          version: '1',
          components: ['skills'],
          targets: ['cursor', 'antigravity', 'pi', 'opencode'],
        },
      ],
    });
  });
  it('offers all compatible new targets and submits Antigravity identity', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    const agy = await screen.findByRole('checkbox', { name: 'Antigravity Desktop' });
    for (const name of ['Cursor', 'Antigravity Desktop', 'Pi', 'OpenCode'])
      expect(screen.getByRole('checkbox', { name })).toBeEnabled();
    expect(screen.queryByRole('checkbox', { name: 'Gemini' })).not.toBeInTheDocument();
    fireEvent.click(agy);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() =>
      expect(preview).toHaveBeenCalledWith(expect.objectContaining({ targets: ['antigravity'] })),
    );
  });
  it('ticks every Agent that can take the plugin from select all', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    fireEvent.click(await screen.findByRole('button', { name: 'resources.select.selectAll' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() =>
      expect(preview).toHaveBeenCalledWith(
        expect.objectContaining({ targets: ['cursor', 'antigravity', 'pi', 'opencode'] }),
      ),
    );
  });
  it('keeps discovery-only formats out of the picker and sends the Git ref with the source', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      source: 'https://github.com/example/plugin.git',
      digest: 'abc',
      sourceRef: 'v1',
      commit: 'abc123',
      targetDefinitions: [
        { target: 'copilot', label: 'GitHub Copilot CLI', project: false, operations: ['add'] },
        {
          target: 'kimi',
          label: 'Kimi Code',
          project: false,
          operations: [],
          reason: 'Use native Kimi plugins',
        },
      ],
      candidates: [
        {
          name: 'demo',
          description: '',
          version: '1',
          components: [],
          targets: ['copilot', 'kimi'],
        },
      ],
    });
    const preview = vi.fn().mockResolvedValue(undefined);
    render(
      <PluginAddDialog initialSource="example/plugin" onClose={() => {}} onPreview={preview} />,
    );
    fireEvent.change(screen.getByLabelText(/plugins\.sourceRef/), { target: { value: 'v1' } });
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByRole('checkbox', { name: 'GitHub Copilot CLI' })).toBeEnabled();
    expect(screen.queryByRole('checkbox', { name: 'Kimi Code' })).not.toBeInTheDocument();
    expect(screen.queryByText('Use native Kimi plugins')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /plugins\.targetsBlocked/ }));
    expect(screen.getByText('Use native Kimi plugins')).toBeInTheDocument();
    expect(pluginsApi.discover).toHaveBeenCalledWith('example/plugin', 'v1', undefined);
    fireEvent.click(screen.getByRole('checkbox', { name: 'GitHub Copilot CLI' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() =>
      expect(preview).toHaveBeenCalledWith(
        expect.objectContaining({ sourceRef: 'v1', targets: ['copilot'] }),
      ),
    );
  });
  it('shows why other Agents cannot take a plugin that is being extended', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      source: '/demo',
      digest: 'abc',
      targetDefinitions: [
        { target: 'cursor', label: 'Cursor', project: false, operations: ['add'] },
        {
          target: 'kimi',
          label: 'Kimi Code',
          project: false,
          operations: [],
          reason: 'Use native Kimi plugins',
        },
      ],
      candidates: [
        {
          name: 'demo',
          description: '',
          version: '1',
          components: [],
          targets: ['cursor', 'kimi'],
        },
      ],
    });
    render(
      <PluginAddDialog
        initialSource="/demo"
        initialName="demo"
        bound={{ cursor: { id: 'demo' } }}
        recorded={{ plugin: 'demo' }}
        onClose={() => {}}
        onPreview={vi.fn()}
      />,
    );
    expect(await screen.findByText('Use native Kimi plugins')).toBeInTheDocument();
  });
  it('sends a Herdr launcher entry with the first discovery', async () => {
    render(
      <PluginAddDialog initialSource="owner/lantern" onClose={() => {}} onPreview={vi.fn()} />,
    );
    fireEvent.change(screen.getByLabelText(/plugins\.launcherEntry/), {
      target: { value: 'bin/lantern-app' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    await waitFor(() =>
      expect(pluginsApi.discover).toHaveBeenCalledWith(
        'owner/lantern',
        undefined,
        'bin/lantern-app',
      ),
    );
  });
  it('keeps an OpenCode entry out of the launcher field after going back', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      source: 'owner/tool',
      digest: 'abc',
      targetDefinitions: [
        { target: 'opencode', label: 'OpenCode', project: true, operations: ['add'] },
      ],
      candidates: [
        {
          name: 'tool',
          version: '',
          description: '',
          targets: ['opencode'],
          components: [],
          targetInfo: {
            opencode: { manifest: 'package.json', components: [], problem: 'No entry found' },
          },
        },
      ],
    });
    render(<PluginAddDialog initialSource="owner/tool" onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.entrySet' }));
    fireEvent.change(screen.getByLabelText('plugins.entry'), {
      target: { value: 'dist/index.js' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'common.back' }));
    expect(screen.getByLabelText(/plugins\.launcherEntry/)).toHaveValue('');
  });
  it('previews the linked Herdr mod with its only target and no launcher entry', async () => {
    const source = 'https://github.com/owner/repo/blob/main/gallery/mods/demo/main.js';
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      source,
      digest: 'abc',
      targetDefinitions: [
        { target: 'herdr-app', label: 'Herdr app', project: false, operations: ['add'] },
      ],
      candidates: [
        {
          name: 'demo',
          version: '',
          description: '',
          targets: ['herdr-app'],
          components: ['mods'],
        },
      ],
    });
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource={source} onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    const next = await screen.findByRole('button', { name: 'plugins.preview' });
    expect(preview).not.toHaveBeenCalled();
    fireEvent.click(next);
    await waitFor(() =>
      expect(preview).toHaveBeenCalledWith(
        expect.objectContaining({
          source,
          plugin: 'demo',
          targets: ['herdr-app'],
          entry: undefined,
        }),
      ),
    );
  });
  it('previews installation and display together for a mod that declares a dock', async () => {
    const source = 'https://github.com/owner/repo/tree/main/herdr/demo';
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      source,
      digest: 'abc',
      targetDefinitions: [
        { target: 'herdr-app', label: 'Herdr app', project: false, operations: ['add'] },
      ],
      candidates: [
        {
          name: 'demo',
          version: '',
          description: '',
          targets: ['herdr-app'],
          components: ['mods'],
          targetInfo: {
            'herdr-app': { manifest: '', components: ['mods'], herdrDock: { place: 'right' } },
          },
        },
      ],
    });
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource={source} onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByRole('checkbox', { name: 'plugins.herdrDockAttach' })).toBeChecked();
    expect(preview).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() =>
      expect(preview).toHaveBeenCalledWith(
        expect.objectContaining({
          source,
          plugin: 'demo',
          targets: ['herdr-app'],
          herdrPanel: true,
        }),
      ),
    );
  });
  it('reviews selected Herdr mods together without installing unselected mods or layouts', async () => {
    const source = 'https://github.com/owner/repo/tree/main';
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      source,
      sourceRef: 'main',
      digest: 'abc',
      targetDefinitions: [
        { target: 'herdr-app', label: 'Herdr app', project: false, operations: ['add'] },
      ],
      candidates: ['one', 'two', 'three'].map((name) => ({
        name,
        version: '',
        description: '',
        targets: ['herdr-app'],
        components: ['mods'],
      })),
    });
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource={source} onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    const next = await screen.findByRole('button', { name: 'plugins.preview' });
    expect(next).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'resources.select.selectAll' }));
    expect(screen.getByRole('checkbox', { name: 'three' })).toBeChecked();
    fireEvent.click(screen.getByRole('checkbox', { name: 'three' }));
    expect(preview).not.toHaveBeenCalled();
    fireEvent.click(next);
    await waitFor(() =>
      expect(preview).toHaveBeenCalledExactlyOnceWith({
        action: 'add',
        source,
        sourceRef: 'main',
        plugins: ['one', 'two'],
        targets: ['herdr-app'],
      }),
    );
  });
  const piDefinitions = [
    { target: 'pi', label: 'Pi', project: true, operations: ['add'], npm: true },
    { target: 'pi-work', label: 'pi-work', project: false, operations: ['add'], npm: true },
    { target: 'opencode', label: 'OpenCode', project: true, operations: ['add'] },
  ];
  it('adds an npm package to Pi targets without discovering it, after saying Pi runs its install scripts', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(
      <PluginAddDialog
        initialSource="npm:@team/tools"
        definitions={piDefinitions}
        onClose={() => {}}
        onPreview={preview}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'plugins.npmContinue' }));
    expect(screen.getByText('plugins.npmNotice')).toBeInTheDocument();
    expect(screen.getByLabelText('resources.col.name')).toHaveAttribute('placeholder', 'tools');
    expect(screen.queryByRole('checkbox', { name: 'OpenCode' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Pi' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() =>
      expect(preview).toHaveBeenCalledWith({
        action: 'add',
        source: 'npm:@team/tools',
        name: undefined,
        targets: ['pi'],
      }),
    );
    expect(pluginsApi.discover).not.toHaveBeenCalled();
  });
  it('keeps the npm source of a pasted pi install command', () => {
    render(<PluginAddDialog definitions={piDefinitions} onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('plugins.source'), {
      target: { value: 'pi install npm:pi-cc-extensions' },
    });
    expect(screen.getByLabelText('plugins.source')).toHaveValue('npm:pi-cc-extensions');
  });
  it('reads the npm package of a pi.dev page', () => {
    render(<PluginAddDialog definitions={piDefinitions} onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('plugins.source'), {
      target: { value: 'https://pi.dev/packages/@vanillagreen/pi-tool-renderer' },
    });
    expect(screen.getByLabelText('plugins.source')).toHaveValue(
      'npm:@vanillagreen/pi-tool-renderer',
    );
  });
  it('leaves a pi.dev address it cannot decode as it was typed', () => {
    render(<PluginAddDialog definitions={piDefinitions} onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('plugins.source'), {
      target: { value: 'https://pi.dev/packages/%E0x' },
    });
    expect(screen.getByLabelText('plugins.source')).toHaveValue('https://pi.dev/packages/%E0x');
  });
});
