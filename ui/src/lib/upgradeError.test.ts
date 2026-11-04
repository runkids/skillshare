import { describe, expect, it } from 'vitest';
import { ApiError } from '../api/client';
import { nixUpdateNote, upgradeErrorMessage } from './upgradeError';

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

describe('nixUpdateNote', () => {
  const base = { cliVersion: '1.0.0', cliUpdateAvailable: true, skillVersion: '1.0.0', skillUpdateAvailable: false };
  it('names the nix profile command for a newer CLI', () => {
    expect(nixUpdateNote({ ...base, nixUpgradeCommand: 'nix profile upgrade skillshare' }, t)).toBe('updateDialog.nixProfile:nix profile upgrade skillshare');
  });
  it('points a Nix configuration at a rebuild', () => {
    expect(nixUpdateNote({ ...base, nixUpgradeCommand: '' }, t)).toBe('updateDialog.nixConfig');
  });
  it('stays quiet when only the skill is outdated', () => {
    expect(nixUpdateNote({ ...base, cliUpdateAvailable: false, skillUpdateAvailable: true, nixUpgradeCommand: '' }, t)).toBeNull();
  });
  it('stays quiet for installs skillshare upgrade replaces', () => {
    expect(nixUpdateNote({ ...base, nixUpgradeCommand: null }, t)).toBeNull();
  });
});
