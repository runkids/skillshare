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
    context.isProjectMode = false;
    vi.mocked(pluginsApi.discover).mockResolvedValue({ source: '/demo', digest: 'abc', candidates: [{ name: 'demo', description: '', version: '1', components: ['skills'], targets: ['cursor', 'antigravity', 'pi', 'opencode'] }] });
  });
  it('offers all compatible new targets and submits Antigravity identity', async () => {
    const preview = vi.fn().mockResolvedValue(undefined);
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={preview} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    const agy = await screen.findByRole('checkbox', { name: 'Antigravity' });
    for (const name of ['Cursor', 'Antigravity', 'Pi', 'OpenCode']) expect(screen.getByRole('checkbox', { name })).toBeEnabled();
    expect(screen.queryByRole('checkbox', { name: 'Gemini' })).not.toBeInTheDocument();
    fireEvent.click(agy);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.preview' }));
    await waitFor(() => expect(preview).toHaveBeenCalledWith(expect.objectContaining({ targets: ['antigravity'] })));
  });
  it('excludes global-only Cursor in project mode without excluding Antigravity', async () => {
    context.isProjectMode = true;
    render(<PluginAddDialog initialSource="/demo" onClose={() => {}} onPreview={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'plugins.discover' }));
    expect(await screen.findByRole('checkbox', { name: 'Cursor' })).toBeDisabled();
    expect(screen.getByRole('checkbox', { name: 'Antigravity' })).toBeEnabled();
    expect(screen.getByRole('checkbox', { name: 'Pi' })).toBeEnabled();
  });
});
