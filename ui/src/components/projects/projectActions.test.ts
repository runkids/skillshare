import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { mcpApi } from '../../api/mcp';
import { hooksApi } from '../../api/hooks';
import { removeProject } from './projectActions';

vi.mock('../../api/client', () => ({ api: { removeProject: vi.fn() } }));
vi.mock('../../api/mcp', () => ({ mcpApi: { save: vi.fn() } }));
vi.mock('../../api/hooks', () => ({ hooksApi: { save: vi.fn() } }));

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.removeProject).mockResolvedValue({ success: true, root: '', origin: 'local', originPath: '' });
});

describe('project removal ownership', () => {
  it('removes a registration on this machine without deleting shared MCP or hooks', async () => {
    await removeProject({ declared: true, root: '~/work/app', path: '/work/app' }, { mcp: true, hooks: true });
    expect(api.removeProject).toHaveBeenCalledWith('~/work/app', false);
    expect(mcpApi.save).not.toHaveBeenCalled();
    expect(hooksApi.save).not.toHaveBeenCalled();
  });

  it('keeps the explicit shared-only removal for projects without a registration', async () => {
    await removeProject({ declared: false, root: '/work/app', path: '/work/app' }, { mcp: true, hooks: true });
    expect(api.removeProject).not.toHaveBeenCalled();
    expect(mcpApi.save).toHaveBeenCalledWith({ project: '/work/app', remove: true });
    expect(hooksApi.save).toHaveBeenCalledWith({ project: '/work/app', remove: true });
  });

  it('removes config.yaml\'s copy only when asked, and reports that the shared copy stays otherwise', async () => {
    vi.mocked(api.removeProject).mockResolvedValueOnce({ success: true, root: '~/work/app', origin: 'local', originPath: '', remaining: true });
    expect(await removeProject({ declared: true, root: '~/work/app', path: '/work/app' }, { mcp: false, hooks: false })).toBe(true);
    expect(api.removeProject).toHaveBeenLastCalledWith('~/work/app', false);
    vi.mocked(api.removeProject).mockResolvedValueOnce({ success: true, root: '~/work/app', origin: 'local', originPath: '' });
    expect(await removeProject({ declared: true, root: '~/work/app', path: '/work/app' }, { mcp: false, hooks: false }, true)).toBe(false);
    expect(api.removeProject).toHaveBeenLastCalledWith('~/work/app', true);
  });
});
