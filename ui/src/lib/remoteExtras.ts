import type { Extra, ExtraRemoveResult } from '../api/client';
import type { TFunc } from './extrasSyncToast';

// Use the browser's platform for copyable commands; tests can select either shell.
const windowsShell = () => typeof navigator !== 'undefined' && /Win/i.test(
  (navigator as { userAgentData?: { platform?: string } }).userAgentData?.platform ?? navigator.platform,
);
const shellWord = (value: string, powershell: boolean) => powershell ? `'${value.replaceAll("'", "''")}'` : /^[a-zA-Z0-9_./:@+-]+$/.test(value) ? value : `'${value.replaceAll("'", "'\"'\"'")}'`;

export function extraRestoreCommand(extra: Extra, project: boolean, powershell = windowsShell()): string {
  return ['skillshare', 'install', shellWord(extra.remote ?? '', powershell), '--kind', 'extra', '--name', shellWord(extra.name, powershell),
    ...(extra.branch ? ['--branch', shellWord(extra.branch, powershell)] : []), project ? '-p' : '-g'].join(' ');
}

export function extraRemovalMessage(extra: Extra, result: ExtraRemoveResult, t: TFunc): string {
  const lines = [t('extras.toast.removed', { name: extra.name })];
  if (result.remote) {
    if (result.missing_dir) lines.push(t('extras.remote.alreadyMissing', { path: extra.source_dir }));
    if (result.deleted_dir) lines.push(t('extras.remote.deleted', { path: extra.source_dir }));
    if (result.kept_edited_dir) lines.push(t('extras.remote.retained', { path: extra.source_dir }));
  }
  if (result.kept_outputs?.length) lines.push(t('extras.remote.keptOutputs', { paths: result.kept_outputs.join('\n') }));
  return lines.join('\n');
}
