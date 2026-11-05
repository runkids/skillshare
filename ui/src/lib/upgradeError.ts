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

/** Turns a failed POST /api/upgrade into the message to show. */
export function upgradeErrorMessage(err: unknown, t: T): string {
  if (err instanceof ApiError && err.code === 'upgrade.needs_sudo') return t('updateDialog.needsSudo');
  if (err instanceof ApiError && err.code === 'upgrade.nix') {
    const command = typeof err.params?.command === 'string' ? err.params.command : '';
    return nixAdvice(command, t);
  }
  return (err as Error).message;
}
