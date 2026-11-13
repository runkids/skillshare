import type { Extra, ExtraRemoveResult } from '../api/client';
import type { TFunc } from './extrasSyncToast';

// Same POSIX shell quoting as the CLI's config.ExtraRestoreCommand.
const shellWord = (value: string) => /^[a-zA-Z0-9_./:@+-]+$/.test(value) ? value : `'${value.replaceAll("'", "'\"'\"'")}'`;

export function extraRestoreCommand(extra: Extra, project: boolean): string {
  return ['skillshare', 'install', shellWord(extra.remote ?? ''), '--kind', 'extra', '--name', shellWord(extra.name),
    ...(extra.branch ? ['--branch', shellWord(extra.branch)] : []), project ? '-p' : '-g'].join(' ');
}

export function extraRemovalMessage(extra: Extra, result: ExtraRemoveResult, t: TFunc): string {
  const lines = [t('extras.toast.removed', { name: extra.name })];
  if (result.remote) {
    if (result.deleted_dir) lines.push(t('extras.remote.deleted', { path: extra.source_dir }));
    if (result.kept_edited_dir) lines.push(t('extras.remote.retained', { path: extra.source_dir }));
  }
  if (result.kept_outputs?.length) lines.push(t('extras.remote.keptOutputs', { paths: result.kept_outputs.join('\n') }));
  return lines.join('\n');
}
