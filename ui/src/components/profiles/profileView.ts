import { ApiError, type ProfileEntryChange, type ProfilesResponse, type ProfileTarget } from '../../api/client';
import type { ChangeGroup, ChangeRow } from '../sync/syncView';
import type { SelectOption } from '../Select';
import type { useT } from '../../i18n';

export const profileVisible = (data?: ProfilesResponse) => !!data && (data.profiles.length > 0 || data.state !== 'none' || !!data.active || data.pending);

export function profileOptions(data: ProfilesResponse, t: ReturnType<typeof useT>): SelectOption[] {
  const options: SelectOption[] = [{ value: '', label: t('profiles.all') }];
  for (const p of [...data.profiles].sort((a, b) => a.name.localeCompare(b.name))) {
    options.push({ value: p.name, label: p.name, mono: true, description: p.usable ? p.description : profileReason(p.reason ?? '', t, p.name), detail: t('profiles.count', { count: p.selected }), disabled: !p.usable });
  }
  if (data.active && !options.some((o) => o.value === data.active)) options.push({ value: data.active, label: data.active, mono: true, description: t('profiles.unavailable'), disabled: true });
  return options;
}

/** Only decodes the existing error envelope; the server owns classification and permission. */
export function blockedEntries(error: unknown): ProfileEntryChange[] {
  if (!(error instanceof ApiError) || error.code !== 'profile_blocked' || typeof error.params?.blocked !== 'string') return [];
  try {
    const rows: unknown = JSON.parse(error.params.blocked);
    return Array.isArray(rows) && rows.every((r) => r && ['target', 'entry', 'step', 'class', 'reason'].every((k) => typeof r[k] === 'string')) ? rows : [];
  } catch { return []; }
}

export function profileGroups(targets: ProfileTarget[] | null): ChangeGroup[] {
  return (targets ?? []).map((target) => {
    const rows: ChangeRow[] = [];
    for (const [field, icon, text] of [['added', 'add', 'sync.row.newCopy'], ['refreshed', 'update', 'sync.row.updateCopy'], ['removed', 'remove', 'sync.row.prune'], ['converted', 'update', 'sync.row.copyToLink']] as const) {
      for (const name of target[field] ?? []) rows.push({ key: `${field}/${name}`, part: 'skill', name, icon, text, counts: true });
    }
    for (const entry of target.kept_modified ?? []) rows.push({ key: `kept/${entry.entry}`, part: 'skill', name: entry.entry, icon: 'kept', text: null, detail: entry.reason, counts: false });
    return { key: target.name, part: 'target', name: target.name, rows };
  });
}

/** Display translations only: unknown diagnostics retain the server's original text. */
export function profileReason(reason: string, t: ReturnType<typeof useT>, name = ''): string {
  const failed = reason.match(/^(.+): sync failed: (target root .+ is an unmanaged link; kept)$/s);
  if (failed) return `${failed[1]}: ${t('sync.last.failed')}: ${profileReason(failed[2], t)}`;
  const root = reason.match(/^target root (.+) is an unmanaged link; kept$/s);
  if (root) return t('profiles.targetRoot', { path: root[1] });
  const modified = reason.match(/^(.+): kept modified (.+) during (refresh|prune|convert): (.+); use collect or --discard-edits$/s);
  const unmanaged = reason.match(/^(.+): kept unmanaged (.+) during (refresh|prune|convert): (.+)$/s);
  const kept = modified ?? unmanaged;
  if (kept) return t(modified ? 'profiles.keptModified' : 'profiles.keptUnmanaged', {
    target: kept[1], entry: kept[2], step: t(`profiles.step.${kept[3]}`), reason: profileReason(kept[4], t),
  });
  const q = JSON.stringify(name);
  const blocked = reason.match(/^profile ("(?:[^"\\]|\\.)*"): skill ("(?:[^"\\]|\\.)*") is blocked by exclude pattern ("(?:[^"\\]|\\.)*")$/s);
  if (blocked) return t('profileEdit.skillBlocked', { name: blocked[1], skill: blocked[2], pattern: blocked[3] });
  const unknown = reason.match(/^unknown (?:profile|skills) key (".+")$/s);
  if (unknown) return t('profileEdit.unknownKey', { key: unknown[1] });
  const unusable = `profile ${q} is unusable: `;
  if (reason.startsWith(unusable)) return t('profileEdit.unusable', { name, reason: profileReason(reason.slice(unusable.length), t, name) });
  if (reason === `profile ${q} already exists`) return t('profileEdit.exists', { name });
  if (reason === `profile ${q} does not exist`) return t('profileEdit.missing', { name });
  if (reason === `cannot delete active profile ${q}`) return t('profileEdit.activeDelete', { name });
  const known: Record<string, string> = {
    'copy was edited or could not be verified': 'profiles.edited',
    'entry is not a recorded copy': 'profiles.unmanaged',
    'link is not a recorded link into the skills source': 'profiles.link',
    'legacy prefixed copy cannot be verified without its source': 'profiles.legacy',
    'profile name must be a string': 'profiles.nameString',
    'duplicate profile name': 'profiles.duplicateName',
    'skills must be a mapping': 'profiles.skillsMapping',
    'skills.include must be a list of strings': 'profiles.listStrings',
    'skills.exclude must be a list of strings': 'profiles.listStrings',
    'description must be a string': 'profiles.description',
    [`profile ${q} must be a mapping`]: 'profiles.mapping',
    [`profile ${q}: description must be one line`]: 'profiles.description',
    [`profile ${q}: exclude requires an include; add include: ["*"]`]: 'profiles.exclude',
    [`invalid profile name ${q}: use lowercase letters, digits, underscores or hyphens (1–64 characters); none is reserved`]: 'profiles.nameInvalid',
  };
  return known[reason] ? t(known[reason]) : reason;
}

export function profileError(error: unknown, t: ReturnType<typeof useT>): string {
  if (error instanceof ApiError && error.code === 'profile_refused') return `${t('profiles.refused')} ${profileReason(error.message, t)}`;
  const codes: Record<string, string> = { profile_locked: 'profiles.locked', profile_empty: 'profiles.empty', profile_blocked: 'profiles.blocked', profile_unavailable: 'profiles.unavailable' };
  return error instanceof ApiError && codes[error.code ?? ''] ? t(codes[error.code!]) : error instanceof Error ? error.message : String(error);
}
