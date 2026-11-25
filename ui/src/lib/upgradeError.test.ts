import { describe, expect, it } from 'vitest';
import { ApiError } from '../api/client';
import { nixUpdateNote, pypiUpdateNote, upgradeErrorMessage } from './upgradeError';

const t = ((key: string, params?: Record<string, unknown>) =>
  typeof params?.command === 'string' ? `${key}:${params.command}` : key) as Parameters<
  typeof upgradeErrorMessage
>[1];

describe('upgradeErrorMessage', () => {
  it('names the nix profile command', () => {
    const err = new ApiError(409, 'x', {
      code: 'upgrade.nix',
      params: { command: 'nix profile upgrade skillshare' },
    });
    expect(upgradeErrorMessage(err, t)).toBe(
      'updateDialog.nixProfile:nix profile upgrade skillshare',
    );
  });
  it('points a Nix configuration at a rebuild', () => {
    const err = new ApiError(409, 'x', { code: 'upgrade.nix', params: { command: '' } });
    expect(upgradeErrorMessage(err, t)).toBe('updateDialog.nixConfig');
  });
  it('names the PyPI command', () => {
    const err = new ApiError(409, 'x', {
      code: 'upgrade.pypi',
      params: { command: 'uv tool upgrade skillshare-cli' },
    });
    expect(upgradeErrorMessage(err, t)).toBe('updateDialog.pypi:uv tool upgrade skillshare-cli');
  });
  it('keeps the sudo hint and other messages', () => {
    expect(upgradeErrorMessage(new ApiError(403, 'x', { code: 'upgrade.needs_sudo' }), t)).toBe(
      'updateDialog.needsSudo',
    );
    expect(upgradeErrorMessage(new Error('boom'), t)).toBe('boom');
  });
});

describe('nixUpdateNote', () => {
  const base = {
    cliVersion: '1.0.0',
    cliUpdateAvailable: true,
    skillVersion: '1.0.0',
    skillUpdateAvailable: false,
  };
  it('names the nix profile command for a newer CLI', () => {
    expect(nixUpdateNote({ ...base, nixUpgradeCommand: 'nix profile upgrade skillshare' }, t)).toBe(
      'updateDialog.nixProfile:nix profile upgrade skillshare',
    );
  });
  it('points a Nix configuration at a rebuild', () => {
    expect(nixUpdateNote({ ...base, nixUpgradeCommand: '' }, t)).toBe('updateDialog.nixConfig');
  });
  it('stays quiet when only the skill is outdated', () => {
    expect(
      nixUpdateNote(
        { ...base, cliUpdateAvailable: false, skillUpdateAvailable: true, nixUpgradeCommand: '' },
        t,
      ),
    ).toBeNull();
  });
  it('stays quiet for installs skillshare upgrade replaces', () => {
    expect(nixUpdateNote({ ...base, nixUpgradeCommand: null }, t)).toBeNull();
  });
});

describe('pypiUpdateNote', () => {
  const base = {
    cliVersion: '1.0.0',
    cliUpdateAvailable: true,
    skillVersion: '1.0.0',
    skillUpdateAvailable: false,
  };
  it('names the pip, uv or pipx command for a newer CLI', () => {
    expect(pypiUpdateNote({ ...base, pypiUpgradeCommand: 'pipx upgrade skillshare-cli' }, t)).toBe(
      'updateDialog.pypi:pipx upgrade skillshare-cli',
    );
  });
  it('stays quiet when only the skill is outdated', () => {
    expect(
      pypiUpdateNote(
        {
          ...base,
          cliUpdateAvailable: false,
          skillUpdateAvailable: true,
          pypiUpgradeCommand: 'pip install --upgrade skillshare-cli',
        },
        t,
      ),
    ).toBeNull();
  });
  it('stays quiet for other installs', () => {
    expect(
      pypiUpdateNote({ ...base, nixUpgradeCommand: 'nix profile upgrade skillshare' }, t),
    ).toBeNull();
  });
});
