import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { X } from 'lucide-react';
import { api } from '../../api/client';
import type { DiscoveredSkill } from '../../api/client';
import { hubDrafts, hubRefs } from '../../api/hubDrafts';
import type { HubEntry, HubRefs } from '../../api/hubDrafts';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { Checkbox, Select } from '../Input';
import { inHub, refOptions, skillSource } from './hubShared';
import { queryKeys } from '../../lib/queryKeys';
import { useT, plural } from '../../i18n';

interface Props {
  hubName: string;
  /** What the hub already holds, so nothing is added twice. */
  entries: HubEntry[];
  onAdd: (data: HubEntry['data'][]) => void;
  /** Adds an empty row to fill in by hand. */
  onManual: () => void;
  onClose: () => void;
}

interface Found {
  /** What the user pasted. */
  source: string;
  refs: HubRefs;
  version: string;
  /** The pasted source at the chosen version; each skill's source starts from it. */
  root: string;
  skills: DiscoveredSkill[];
}

/** Adds skills from a Git URL, at a chosen version, or from what is installed here. */
export default function AddSkillDialog({ hubName, entries, onAdd, onManual, onClose }: Props) {
  const t = useT();
  const [tab, setTab] = useState<'url' | 'installed'>('url');
  const [url, setURL] = useState('');
  const [found, setFound] = useState<Found | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [picked, setPicked] = useState<string[]>([]);
  const [query, setQuery] = useState('');
  const [installedPicked, setInstalledPicked] = useState<string[]>([]);

  const candidates = useQuery({
    queryKey: queryKeys.hub.candidates,
    queryFn: () => hubDrafts.candidates(),
    enabled: tab === 'installed',
  });

  const foundSource = (skill: DiscoveredSkill) => skillSource(found!.root, skill.path);
  const addable = (skills: DiscoveredSkill[], root: string) =>
    skills.filter((s) => !inHub(entries, skillSource(root, s.path), '', s.name)).map((s) => s.path);

  async function find() {
    const source = url.trim();
    if (!source) return;
    setBusy(true);
    setError('');
    try {
      const [refs, discovered] = await Promise.all([hubRefs(source), api.discover(source)]);
      // A pasted GitLab or Bitbucket root has no marker between repo and path;
      // the server's rewrite keeps the skill paths joined onto it apart.
      const root = refs.pinnable
        ? ((await hubRefs(source, refs.current)).source ?? source)
        : source;
      setFound({ source, refs, version: refs.current, root, skills: discovered.skills });
      setPicked(addable(discovered.skills, root));
    } catch (e) {
      setFound(null);
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function changeVersion(version: string) {
    if (!found) return;
    setBusy(true);
    setError('');
    try {
      const res = await hubRefs(found.source, version);
      const root = res.source ?? found.source;
      const discovered = await api.discover(root, version || undefined);
      setFound({ ...found, version, root, skills: discovered.skills });
      setPicked(addable(discovered.skills, root));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const shownCandidates = (candidates.data ?? []).filter((c) =>
    `${c.data.name ?? ''} ${c.id}`.toLowerCase().includes(query.toLowerCase()),
  );

  const count = tab === 'url' ? picked.length : installedPicked.length;
  const add = () => {
    if (tab === 'url' && found) {
      onAdd(
        found.skills
          .filter((s) => picked.includes(s.path))
          .map((s) => ({
            name: s.name,
            ...(s.description ? { description: s.description } : {}),
            source: foundSource(s),
          })),
      );
    } else {
      onAdd(
        (candidates.data ?? []).filter((c) => installedPicked.includes(c.id)).map((c) => c.data),
      );
    }
  };

  const toggle = (list: string[], id: string, on: boolean) =>
    on ? [...list, id] : list.filter((x) => x !== id);

  return (
    <DialogShell
      open
      onClose={onClose}
      maxWidth="xl"
      padding="none"
      ariaLabel={t('hubs.addSkill.title', { name: hubName })}
    >
      <div className="dh">
        <h2 className="ss-h2">{t('hubs.addSkill.title', { name: hubName })}</h2>
        <button type="button" className="ss-ib" onClick={onClose} aria-label={t('common.close')}>
          <X size={16} />
        </button>
      </div>
      <div className="db">
        <div className="ss-tabs" role="tablist" aria-label={t('hubs.addSkill.how')}>
          {(['url', 'installed'] as const).map((k) => (
            <button
              key={k}
              type="button"
              role="tab"
              aria-selected={tab === k}
              className={tab === k ? 'on' : ''}
              onClick={() => setTab(k)}
            >
              {t(k === 'url' ? 'hubs.addSkill.tab.url' : 'hubs.addSkill.tab.installed')}
            </button>
          ))}
        </div>

        {tab === 'url' ? (
          <>
            <form
              className="ss-fld"
              onSubmit={(e) => {
                e.preventDefault();
                void find();
              }}
            >
              <label htmlFor="hub-skill-url">{t('hubs.addSkill.url')}</label>
              <div className="flex gap-2">
                <span className="ss-inp min-w-0 flex-1 font-mono">
                  <input
                    id="hub-skill-url"
                    value={url}
                    placeholder="github.com/owner/repo"
                    onChange={(e) => setURL(e.target.value)}
                  />
                </span>
                <Button type="submit" variant="secondary" disabled={!url.trim() || busy}>
                  {t('hubs.addSkill.find')}
                </Button>
              </div>
              <span className="hp">{t('hubs.addSkill.urlHint')}</span>
            </form>

            {error && (
              <div role="alert" className="ss-note bad">
                <span className="flex-1 break-words">{error}</span>
              </div>
            )}
            {busy && (
              <div className="flex items-center gap-2 text-[13px] text-ink-2">
                <Spinner size="sm" />
                {t('hubs.browse.loading')}
              </div>
            )}

            {found && (
              <>
                <div className="ss-fld">
                  <Select
                    label={t('hubs.view.version')}
                    className="w-full"
                    value={found.version}
                    onChange={(v) => void changeVersion(v)}
                    options={refOptions(t, found.refs, found.version)}
                    disabled={busy || !found.refs.pinnable}
                  />
                  <span className="hp">
                    {found.refs.pinnable ? t('hubs.addSkill.versionHint') : t('hubs.ref.fixed')}
                  </span>
                </div>

                <div className="flex flex-col gap-2">
                  <div className="flex items-center gap-2">
                    <span className="text-[13px] font-semibold">
                      {t(plural('hubs.addSkill.found', found.skills.length), {
                        count: found.skills.length,
                      })}
                    </span>
                    {found.skills.length > 1 && (
                      <Button
                        variant="link"
                        onClick={() => setPicked(addable(found.skills, found.root))}
                      >
                        {t('hubs.addSkill.selectAll')}
                      </Button>
                    )}
                  </div>
                  {found.skills.length > 0 && (
                    <div className="ss-list max-h-72 overflow-auto">
                      {found.skills.map((s) => {
                        const already = inHub(entries, foundSource(s), '', s.name);
                        return (
                          <div
                            key={s.path}
                            className={`ss-r !min-h-11 !items-start ${already ? 'dim' : ''}`}
                          >
                            <Checkbox
                              label={s.name}
                              hideLabel
                              checked={picked.includes(s.path)}
                              disabled={already}
                              onChange={(on) => setPicked((l) => toggle(l, s.path, on))}
                              className="mt-0.5"
                            />
                            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                              <span className="nm m font-mono">{s.name}</span>
                              {s.description && (
                                <span className="text-[12.5px] text-ink-2">{s.description}</span>
                              )}
                            </span>
                            {already && (
                              <span className="whitespace-nowrap text-xs text-ink-3">
                                {t('hubs.addSkill.inHub')}
                              </span>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </>
            )}
          </>
        ) : (
          <>
            <span className="ss-inp">
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t('hubBuilder.filter')}
                aria-label={t('hubBuilder.filter')}
              />
            </span>
            {candidates.isPending && <Spinner size="sm" />}
            {candidates.isError && (
              <div className="ss-note bad">
                <span className="flex-1">{(candidates.error as Error).message}</span>
              </div>
            )}
            {candidates.data?.length === 0 && (
              <p className="text-[13px] text-ink-2">{t('hubBuilder.noInstalled')}</p>
            )}
            {shownCandidates.length > 0 && (
              <div className="ss-list max-h-80 overflow-auto">
                {shownCandidates.map((c) => {
                  const already = inHub(
                    entries,
                    c.data.source ?? '',
                    c.data.skill ?? '',
                    c.data.name ?? '',
                  );
                  return (
                    <div
                      key={c.id}
                      className={`ss-r !min-h-11 !items-start ${already ? 'dim' : ''}`}
                    >
                      <Checkbox
                        label={c.data.name || c.id}
                        hideLabel
                        checked={installedPicked.includes(c.id)}
                        disabled={already}
                        onChange={(on) => setInstalledPicked((l) => toggle(l, c.id, on))}
                        className="mt-0.5"
                      />
                      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                        <span className="nm m font-mono">{c.data.name || c.id}</span>
                        <span className="truncate font-mono text-[11.5px] text-ink-3">
                          {c.data.source || c.id}
                        </span>
                      </span>
                      {already && (
                        <span className="whitespace-nowrap text-xs text-ink-3">
                          {t('hubs.addSkill.inHub')}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </>
        )}
      </div>
      <div className="df">
        <Button
          variant="link"
          className="mr-auto !text-ink-2 underline underline-offset-4"
          onClick={onManual}
        >
          {t('hubs.addSkill.manual')}
        </Button>
        <Button variant="secondary" onClick={onClose}>
          {t('common.cancel')}
        </Button>
        <Button onClick={add} disabled={count === 0 || busy}>
          {t('hubs.addSkill.add', { count })}
        </Button>
      </div>
    </DialogShell>
  );
}
