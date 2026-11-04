import { ApiError } from '../api/client';
import type { useT } from '../i18n';

/** Turns a failed POST /api/upgrade into the message to show. */
export function upgradeErrorMessage(err: unknown, t: ReturnType<typeof useT>): string {
  if (err instanceof ApiError && err.code === 'upgrade.needs_sudo') return t('updateDialog.needsSudo');
  if (err instanceof ApiError && err.code === 'upgrade.nix') {
    const command = typeof err.params?.command === 'string' ? err.params.command : '';
    return command ? t('updateDialog.nixProfile', { command }) : t('updateDialog.nixConfig');
  }
  return (err as Error).message;
}
