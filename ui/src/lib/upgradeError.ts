import { ApiError } from '../api/client';
import type { VersionCheck } from '../api/client';
import type { useT } from '../i18n';

type T = ReturnType<typeof useT>;

/** How a Nix install gets a newer CLI: its nix profile command, or a configuration rebuild. */
function nixAdvice(command: string, t: T): string {
  return command ? t('updateDialog.nixProfile', { command }) : t('updateDialog.nixConfig');
}

/** The Nix note the update dialog adds when a newer CLI must come from Nix, or null. */
export function nixUpdateNote(data: VersionCheck, t: T): string | null {
  if (!data.cliUpdateAvailable || typeof data.nixUpgradeCommand !== 'string') return null;
  return nixAdvice(data.nixUpgradeCommand, t);
}

/** The PyPI note the update dialog adds when pip, uv or pipx must install a newer CLI, or null. */
export function pypiUpdateNote(data: VersionCheck, t: T): string | null {
  if (!data.cliUpdateAvailable || typeof data.pypiUpgradeCommand !== 'string') return null;
  return t('updateDialog.pypi', { command: data.pypiUpgradeCommand });
}

/** The pixi note the update dialog adds when pixi must install a newer CLI, or null. */
export function pixiUpdateNote(data: VersionCheck, t: T): string | null {
  if (!data.cliUpdateAvailable || typeof data.pixiUpgradeCommand !== 'string') return null;
  return t('updateDialog.pixi', { command: data.pixiUpgradeCommand });
}

/** Turns a failed POST /api/upgrade into the message to show. */
export function upgradeErrorMessage(err: unknown, t: T): string {
  if (err instanceof ApiError && err.code === 'upgrade.needs_sudo')
    return t('updateDialog.needsSudo');
  if (err instanceof ApiError && err.code === 'upgrade.nix') {
    const command = typeof err.params?.command === 'string' ? err.params.command : '';
    return nixAdvice(command, t);
  }
  if (err instanceof ApiError && err.code === 'upgrade.pypi') {
    const command = typeof err.params?.command === 'string' ? err.params.command : '';
    return t('updateDialog.pypi', { command });
  }
  if (err instanceof ApiError && err.code === 'upgrade.pixi') {
    const command = typeof err.params?.command === 'string' ? err.params.command : '';
    return t('updateDialog.pixi', { command });
  }
  return (err as Error).message;
}
