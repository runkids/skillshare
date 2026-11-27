import type { HubSavedEntry } from '../../api/client';
import type { HubEntry, HubProblem, HubRefs } from '../../api/hubDrafts';
import type { SelectOption } from '../Select';
import type { useT } from '../../i18n';
import { downloadJson } from '../../lib/downloadJson';

type T = ReturnType<typeof useT>;

/** Shipped with skillshare, so it is offered even when the user has saved nothing. */
export const COMMUNITY: HubSavedEntry = {
  label: 'Skillshare Hub',
  url: 'https://raw.githubusercontent.com/runkids/skillshare-hub/main/skillshare-hub.json',
  builtIn: true,
};

/** Prefix that marks a list key as one of the user's own hubs rather than a hosted index. */
export const DRAFT = 'draft:';

export const sameURL = (a: string, b: string) =>
  a.trim().replace(/\/+$/, '') === b.trim().replace(/\/+$/, '');

export const newEntryId = () =>
  Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('');

/** Entries a recipient could not install; the draft-wide "empty" problem has no entry. */
export const blockedEntries = (problems: HubProblem[]) =>
  new Set(problems.filter((p) => p.entryId).map((p) => p.entryId));

/**
 * Whether an entry already covers this source (and selector), so it is not added twice.
 * A skill of the same name counts too: both would install into one directory.
 */
export const inHub = (entries: HubEntry[], source: string, skill = '', name = '') =>
  (Boolean(source.trim()) &&
    entries.some((e) => sameURL(e.data.source ?? '', source) && (e.data.skill ?? '') === skill)) ||
  (Boolean(name) && entries.some((e) => e.data.name === name));

/**
 * One skill inside a discovered repository. Mirrors the server's discovery source:
 * a whole-repo SSH source marks the subdirectory with "//", every other source with "/".
 * The ref is already part of `root`, so no version is written here.
 */
export function skillSource(root: string, path: string) {
  const base = root.trim().replace(/\/+$/, '');
  if (!path || path === '.') return base;
  const ssh = /^(git@|ssh:\/\/)/.test(base) && !base.replace(/^ssh:\/\//, '').includes('//');
  return `${base}${ssh ? '//' : '/'}${path}`;
}

/** Version choices for a source: the default branch first, then branches, then tags. */
export function refOptions(
  t: T,
  refs: HubRefs | undefined,
  current: string,
  loading = false,
): SelectOption[] {
  const options: SelectOption[] = [
    {
      value: '',
      label: t('hubs.ref.default'),
      note: refs?.defaultBranch ? `· ${refs.defaultBranch}` : undefined,
    },
  ];
  const seen = new Set(['']);
  const add = (value: string, group?: string) => {
    if (seen.has(value)) return;
    seen.add(value);
    options.push({ value, label: value, group });
  };
  if (current && !(refs?.branches ?? []).includes(current) && !(refs?.tags ?? []).includes(current))
    add(current);
  for (const b of refs?.branches ?? []) add(b, 'Branch');
  for (const tag of refs?.tags ?? []) add(tag, 'Tag');
  if (loading)
    options.push({ value: '\u0000loading', label: t('hubs.ref.loading'), disabled: true });
  return options;
}

/** A hosted location a recipient can reach; local paths and control characters are not. */
export const publishable = (location: string) =>
  /^(https?:\/\/|ssh:\/\/|[^\s@]+@[^\s:]+:)/.test(location.trim()) &&
  !Array.from(location).some((char) => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127);

export function downloadIndex(data: Record<string, unknown>) {
  downloadJson('skillshare-hub.json', data);
}
