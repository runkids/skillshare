import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PluginAddDialog from './PluginAddDialog';
import { pluginsApi } from '../../api/plugins';

const context = vi.hoisted(() => ({ isProjectMode: false }));
vi.mock('../../context/AppContext', () => ({ useAppContext: () => context }));
vi.mock('../../i18n', () => ({ useT: () => (key: string) => key }));
vi.mock('../../api/plugins', async (original) => ({ ...await original<typeof import('../../api/plugins')>(), pluginsApi: { discover: vi.fn() } }));

describe('PluginAddDialog targets', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    context.isProjectMode = false;
    vi.mocked(pluginsApi.discover).mockResolvedValue({ targetDefinitions: [{target:'cursor',label:'Cursor',project:false,operations:['add']},{target:'antigravity',label:'Antigravity Desktop',project:true,operations:['add']},{target:'pi',label:'Pi',project:true,operations:['add']},{target:'opencode',label:'OpenCode',project:true,operations:['add']}], source: '/demo', digest: 'abc', candidates: [{ name: 'demo', description: '', version: '1', components: ['skills'], targets: ['cursor', 'antigravity', 'pi', 'opencode'] }] });
  });
  it('offers all compatible new targets and submits Antigravity identity', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    const agy = await screen.findByRole('checkbox', { name: 'Antigravity Desktop' });
    for (const name of ['Cursor', 'Antigravity Desktop', 'Pi', 'OpenCode']) expect(screen.getByRole('checkbox', { name })).toBeEnabled();
    expect(screen.queryByRole('checkbox', { name: 'Gemini' })).not.toBeInTheDocument();
    fireEvent.click(agy);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith(expect.objectContaining({ targets: ['antigravity'] })));
  });
  it('previews with no Agent chosen, so the plugin is only added to Skillshare', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    await screen.findByRole('checkbox', { name: 'Pi' });
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith(expect.objectContaining({ plugin: 'demo', targets: [] })));
  });
  it('moves global-only Cursor out of the picker in project mode, with its reason', async () => {
    context.isProjectMode = true;
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByRole('checkbox', { name: 'Antigravity Desktop' })).toBeEnabled();
    expect(screen.queryByRole('checkbox', { name: 'Cursor' })).not.toBeInTheDocument();
    expect(screen.getByText('plugins.globalOnly')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Pi' })).toBeEnabled();
  });
  it('keeps discovery-only formats out of the picker and sends the Git ref with the source', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue({
      source: 'https://github.com/example/plugin.git', digest: 'abc', sourceRef: 'v1', commit: 'abc123',
      targetDefinitions: [
        { target: 'copilot', label: 'GitHub Copilot CLI', project: false, operations: ['add'] },
        { target: 'kimi', label: 'Kimi Code', project: false, operations: [], reason: 'Use native Kimi plugins' },
      ],
      candidates: [{ name: 'demo', description: '', version: '1', components: [], targets: ['copilot', 'kimi'] }],
    });
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="example/plugin" onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.advanced' }));
    fireEvent.change(screen.getByLabelText('plugins.sourceRef'), { target: { value: 'v1' } });
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByRole('checkbox', { name: 'GitHub Copilot CLI' })).toBeEnabled();
    expect(screen.queryByRole('checkbox', { name: 'Kimi Code' })).not.toBeInTheDocument();
    expect(screen.getByText('Use native Kimi plugins')).toBeInTheDocument();
    expect(pluginsApi.discover).toHaveBeenCalledWith('example/plugin', 'v1', undefined);
    fireEvent.click(screen.getByRole('checkbox', { name: 'GitHub Copilot CLI' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith(expect.objectContaining({ sourceRef: 'v1', targets: ['copilot'] })));
  });
  const piDefinitions = [
    { target: 'pi', label: 'Pi', project: true, operations: ['add'], npm: true },
    { target: 'pi-work', label: 'pi-work', project: false, operations: ['add'], npm: true },
    { target: 'opencode', label: 'OpenCode', project: true, operations: ['add'] },
  ];
  it('adds an npm package to Pi targets without discovering it, after saying Pi runs its install scripts', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="npm:@team/tools" definitions={piDefinitions} onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.npmContinue' }));
    expect(screen.getByText('plugins.npmNotice')).toBeInTheDocument();
    expect(screen.getByLabelText('resources.col.name')).toHaveAttribute('placeholder', 'tools');
    expect(screen.queryByRole('checkbox', { name: 'OpenCode' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Pi' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith({ action: 'add', source: 'npm:@team/tools', name: undefined, targets: ['pi'] }));
    expect(pluginsApi.discover).not.toHaveBeenCalled();
  });
  it('keeps the npm source of a pasted pi install command', () => {
    render(<PluginAddDialog definitions={piDefinitions} onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('plugins.source'), { target: { value: 'pi install npm:pi-cc-extensions' } });
    expect(screen.getByLabelText('plugins.source')).toHaveValue('npm:pi-cc-extensions');
  });
  it('reads the npm package of a pi.dev page', () => {
    render(<PluginAddDialog definitions={piDefinitions} onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('plugins.source'), { target: { value: 'https://pi.dev/packages/@vanillagreen/pi-tool-renderer' } });
    expect(screen.getByLabelText('plugins.source')).toHaveValue('npm:@vanillagreen/pi-tool-renderer');
  });
  it('leaves a pi.dev address it cannot decode as it was typed', () => {
    render(<PluginAddDialog definitions={piDefinitions} onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('plugins.source'), { target: { value: 'https://pi.dev/packages/%E0x' } });
    expect(screen.getByLabelText('plugins.source')).toHaveValue('https://pi.dev/packages/%E0x');
  });
  it('ticks the target the dialog was opened for', async () => {
    render(<PluginAddDialog initialSource="npm:demo" initialTargets={['pi-work']} definitions={piDefinitions} onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.npmContinue' }));
    expect(screen.getByRole('checkbox', { name: 'pi-work' })).toHaveAttribute('aria-checked', 'true');
  });
  it('leaves out a preselected target the discovered plugin cannot use', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" initialTargets={['pi', 'pi-work']} onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByRole('checkbox', { name: 'Pi' })).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith(expect.objectContaining({ targets: ['pi'] })));
  });
  it('top-aligns the radio beside a candidate with a long description', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue({ source: '/demo', digest: 'abc', candidates: [{ name: 'demo', description: 'A long plugin description that wraps onto multiple lines.', version: '1', components: ['skills'], targets: ['pi'] }] });
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    const radio = await screen.findByRole('radio');
    expect(radio.querySelector('.ss-chk.rad')).toHaveClass('self-start');
  });

  it('does not pick the only candidate when it cannot be installed', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue({ targetDefinitions: [], source: '/demo', digest: 'abc', candidates: [{ name: 'demo', description: '', version: '1', components: [], targets: ['codex'], problem: 'blocked', problemKey: 'plugins.problem.noManifest' }] });
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByRole('radio')).toHaveAttribute('aria-checked', 'false');
  });
  const found = (targets: string[]) => ({
    source: '/demo', digest: 'abc',
    targetDefinitions: [
      { target: 'copilot', label: 'GitHub Copilot CLI', project: false, operations: ['add'] },
      { target: 'opencode', label: 'OpenCode', project: true, operations: ['add'] },
    ],
    candidates: [{ name: 'demo', description: '', version: '1', components: [], targets }],
  });

  it('asks for the OpenCode entry where OpenCode is blocked, and keeps the choice made so far', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValueOnce(found(['copilot'])).mockResolvedValueOnce(found(['copilot', 'opencode']));
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={preview} />);
    expect(screen.queryByLabelText('plugins.entry')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    fireEvent.click(await screen.findByRole('checkbox', { name: 'GitHub Copilot CLI' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.entrySet' }));
    fireEvent.change(screen.getByLabelText('plugins.entry'), { target: { value: 'dist/main.js' } });
    fireEvent.click(screen.getByRole('button', { name: 'plugins.rediscover' }));
    expect(await screen.findByRole('checkbox', { name: 'OpenCode' })).toBeEnabled();
    expect(pluginsApi.discover).toHaveBeenLastCalledWith('/demo', undefined, 'dist/main.js');
    expect(screen.getByRole('checkbox', { name: 'GitHub Copilot CLI' })).toBeChecked();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith(expect.objectContaining({ entry: 'dist/main.js', targets: ['copilot'] })));
  });

  it('shows a blocked reason through its message key, not the backend English', async () => {
    const d = found(['copilot']);
    vi.mocked(pluginsApi.discover).mockResolvedValue({ ...d, candidates: [{ ...d.candidates[0], targetInfo: { opencode: { manifest: 'package.json', components: [], problem: 'OpenCode package entry index.js is missing', problemKey: 'plugins.problem.opencodeEntryMissing', problemArgs: { entry: 'index.js' } } } }] });
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByText('plugins.problem.opencodeEntryMissing')).toBeInTheDocument();
    expect(screen.queryByText(/is missing/)).not.toBeInTheDocument();
  });

  it('opens on the Agent choice when adding Agents to a plugin it already knows, with the bound ones locked', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue(found(['copilot', 'opencode']));
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" initialName="demo" bound={{ copilot: { id: 'demo', plugin: 'demo', source: '/demo' } }} onClose={() => {}} onPreview={preview} />);
    fireEvent.click(await screen.findByRole('checkbox', { name: 'OpenCode' }));
    expect(screen.getByRole('checkbox', { name: 'GitHub Copilot CLI' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith(expect.objectContaining({ name: 'demo', targets: ['opencode'] })));
  });

  it('forgets the OpenCode entry when going back, since step 1 cannot show it', async () => {
    vi.mocked(pluginsApi.discover).mockResolvedValue(found(['copilot']));
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    fireEvent.click(await screen.findByRole('button', { name: 'plugins.entrySet' }));
    fireEvent.change(screen.getByLabelText('plugins.entry'), { target: { value: 'dist/main.js' } });
    fireEvent.click(screen.getByRole('button', { name: 'common.back' }));
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    await waitFor(() => expect(pluginsApi.discover).toHaveBeenLastCalledWith('/demo', undefined, undefined));
  });
});
