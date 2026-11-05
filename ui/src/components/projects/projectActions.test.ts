import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import { mcpApi } from '../../api/mcp';
import { hooksApi } from '../../api/hooks';
import { removeProject } from './projectActions';

vi.mock('../../api/client', () => ({ api: { removeProject: vi.fn() } }));
vi.mock('../../api/mcp', () => ({ mcpApi: { save: vi.fn() } }));
vi.mock('../../api/hooks', () => ({ hooksApi: { save: vi.fn() } }));

beforeEach(() => vi.clearAllMocks());

describe('project removal ownership', () => {
  it('removes a registration on this machine without deleting shared MCP or hooks', async () => {
    await removeProject({ declared: true, root: '~/work/app', path: '/work/app' }, { mcp: true, hooks: true });
    expect(api.removeProject).toHaveBeenCalledWith('~/work/app');
    expect(mcpApi.save).not.toHaveBeenCalled();
    expect(hooksApi.save).not.toHaveBeenCalled();
  });

  it('keeps the explicit shared-only removal for projects without a registration', async () => {
    await removeProject({ declared: false, root: '/work/app', path: '/work/app' }, { mcp: true, hooks: true });
    expect(api.removeProject).not.toHaveBeenCalled();
    expect(mcpApi.save).toHaveBeenCalledWith({ project: '/work/app', remove: true });
    expect(hooksApi.save).toHaveBeenCalledWith({ project: '/work/app', remove: true });
  });
});
