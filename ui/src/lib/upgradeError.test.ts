import { describe, expect, it } from 'vitest';
import { ApiError } from '../api/client';
import { upgradeErrorMessage } from './upgradeError';

const t = ((key: string, params?: Record<string, unknown>) => (params?.command ? `${key}:${params.command}` : key)) as Parameters<typeof upgradeErrorMessage>[1];

describe('upgradeErrorMessage', () => {
  it('names the nix profile command', () => {
    const err = new ApiError(409, 'x', { code: 'upgrade.nix', params: { command: 'nix profile upgrade skillshare' } });
    expect(upgradeErrorMessage(err, t)).toBe('updateDialog.nixProfile:nix profile upgrade skillshare');
  });
  it('points a Nix configuration at a rebuild', () => {
    const err = new ApiError(409, 'x', { code: 'upgrade.nix', params: { command: '' } });
    expect(upgradeErrorMessage(err, t)).toBe('updateDialog.nixConfig');
  });
  it('keeps the sudo hint and other messages', () => {
    expect(upgradeErrorMessage(new ApiError(403, 'x', { code: 'upgrade.needs_sudo' }), t)).toBe('updateDialog.needsSudo');
    expect(upgradeErrorMessage(new Error('boom'), t)).toBe('boom');
  });
});
