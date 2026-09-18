import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Library, Link2, Plus, Search, Star, Trash2 } from 'lucide-react';
import { api, type HubSavedEntry, type SearchResult } from '../../api/client';
import { hubDrafts, type HubEntry } from '../../api/hubDrafts';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import InstallDialog from '../InstallDialog';
import Spinner from '../Spinner';
import { Select } from '../Select';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import { useT } from '../../i18n';

/** Shipped with skillshare, so it is offered even when the user has saved nothing. */
const COMMUNITY: HubSavedEntry = {
  label: 'Skillshare Hub',
  url: 'https://raw.githubusercontent.com/runkids/skillshare-hub/main/skillshare-hub.json',
  builtIn: true,
};

/** Prefix that marks a source as a local draft rather than a hosted index. */
const DRAFT = 'draft:';

/** A draft entry already carries everything a hub row shows; nothing is fetched. */
const draftResult = (entry: HubEntry): SearchResult => ({
  name: entry.data.name ?? '',
  description: entry.data.description ?? '',
  source: entry.data.source ?? '',
  skill: entry.data.skill,
  stars: 0,
  owner: '',
  repo: '',
  tags: entry.data.tags,
});

const sameURL = (a: string, b: string) => a.trim().replace(/\/+$/, '') === b.trim().replace(/\/+$/, '');

interface Props {
  /** Set when arriving from a draft's "preview as recipient"; not saved as a subscription. */
  previewURL?: string;
  previewLabel?: string;
}

/**
 * The consuming half of hubs: pick a saved hub, see what is inside it, install from it.
 * Installing hands the source to the install dialog so the audit gate stays in one place.
 */
export default function HubBrowser({ previewURL, previewLabel }: Props) {
  const t = useT();
  const queryClient = useQueryClient();
  const [picked, setPicked] = useState<string | null>(previewURL ?? null);
  const [filter, setFilter] = useState('');
  const [tag, setTag] = useState('');
  const [url, setURL] = useState('');
  const [label, setLabel] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState<HubSavedEntry | null>(null);
  const [installing, setInstalling] = useState<string | null>(null);

  const { data: config } = useQuery({
    queryKey: queryKeys.hubConfig,
    queryFn: () => api.getHubConfig(),
    staleTime: staleTimes.config,
  });

  const { data: drafts = [] } = useQuery({ queryKey: ['hub', 'drafts'], queryFn: () => hubDrafts.list() });

  const hubs = useMemo(() => {
    const saved = (config?.hubs ?? []).filter((h) => !sameURL(h.url, COMMUNITY.url));
    const list = [COMMUNITY, ...saved];
    if (previewURL && !previewURL.startsWith(DRAFT) && !list.some((h) => sameURL(h.url, previewURL))) {
      list.unshift({ label: previewLabel || t('hubs.preview.label'), url: previewURL });
    }
    return list;
  }, [config, previewURL, previewLabel, t]);

  const isDraft = Boolean(picked?.startsWith(DRAFT));
  const pickedDraft = isDraft ? drafts.find((d) => DRAFT + d.id === picked) : undefined;
  const current = pickedDraft
    ? { label: pickedDraft.name || t('hubBuilder.untitled'), url: picked!, builtIn: false }
    : hubs.find((h) => h.url === picked) ?? null;
  const isDefault = (hub: HubSavedEntry) =>
    Boolean(config?.default) && hub.label.toLowerCase() === config!.default.toLowerCase();

  const contents = useQuery({
    queryKey: ['hub', 'contents', picked],
    // An empty query returns every entry of a hosted index.
    queryFn: async () => {
      if (picked!.startsWith(DRAFT)) {
        const { draft } = await hubDrafts.get(picked!.slice(DRAFT.length));
        return { results: draft.entries.map(draftResult) };
      }
      return api.searchHub('', picked!);
    },
    enabled: Boolean(picked),
    staleTime: staleTimes.skills,
  });

  const { data: skillsData } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const installed = useMemo(
    () => new Set((skillsData?.resources ?? []).map((r) => r.name)),
    [skillsData],
  );

  /** Every tag the picked hub actually uses, so the dropdown never offers a dead option. */
  const tags = useMemo(() => {
    const seen = new Set<string>();
    for (const r of contents.data?.results ?? []) for (const name of r.tags ?? []) seen.add(name);
    return [...seen].sort();
  }, [contents.data]);

  const shown = (contents.data?.results ?? []).filter(
    (r: SearchResult) =>
      (!tag || (r.tags ?? []).includes(tag)) &&
      `${r.name} ${r.description} ${(r.tags ?? []).join(' ')}`.toLowerCase().includes(filter.toLowerCase()),
  );

  const save = async (next: HubSavedEntry[], defaultLabel: string) => {
    const body = { hubs: next.filter((h) => !h.builtIn).map(({ label: l, url: u }) => ({ label: l, url: u })), default: defaultLabel };
    await api.putHubConfig(body);
    queryClient.setQueryData(queryKeys.hubConfig, body);
  };

  const add = async () => {
    const trimmed = url.trim();
    if (!trimmed) { setError(t('install.hubs.urlRequired')); return; }
    if (hubs.some((h) => sameURL(h.url, trimmed))) { setError(t('install.hubs.urlExists')); return; }
    setBusy(true);
    setError('');
    try {
      const name = label.trim() || trimmed.split('/').filter(Boolean).pop() || trimmed;
      await save([...hubs, { label: name, url: trimmed }], config?.default ?? '');
      setURL('');
      setLabel('');
      setPicked(trimmed);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (hub: HubSavedEntry) => {
    const next = hubs.filter((h) => h.url !== hub.url);
    await save(next, isDefault(hub) ? '' : config?.default ?? '');
    if (picked === hub.url) setPicked(null);
  };

  return (
    <div className="grid grid-cols-[260px_minmax(0,1fr)] items-start gap-6">
      <div className="sticky top-6 flex flex-col gap-3">
        <div className="ss-sec !mb-0">
          <h2>{t('hubs.browse.sources')}</h2>
          <span className="ss-cnt">{hubs.length}</span>
        </div>
        <div className="ss-list">
          {hubs.map((hub) => (
            <div key={hub.url} className={`ss-r !min-h-[52px] ${hub.url === picked ? 'sel' : ''}`}>
              <button
                type="button"
                aria-current={hub.url === picked}
                className="flex min-w-0 flex-1 flex-col items-start gap-0.5 text-start"
                onClick={() => { setPicked(hub.url); setFilter(''); setTag(''); }}
              >
                <span className="flex items-center gap-1.5 font-semibold text-[13.5px]">
                  {hub.label}
                  {isDefault(hub) && <Star size={12} className="fill-current text-warn" />}
                </span>
                <span className="w-full truncate font-mono text-[11.5px] text-ink-3">{hub.url}</span>
              </button>
              {!hub.builtIn && (
                <button type="button" className="ss-ib" aria-label={t('install.hubs.remove')} onClick={() => setRemoving(hub)}>
                  <Trash2 size={15} />
                </button>
              )}
            </div>
          ))}
        </div>

        {drafts.length > 0 && (
          <>
            <div className="ss-sec !mb-0 !mt-1">
              <h2>{t('hubs.browse.mine')}</h2>
              <span className="ss-cnt">{drafts.length}</span>
            </div>
            <div className="ss-list">
              {drafts.map((d) => (
                <div key={d.id} className={`ss-r !min-h-[52px] ${DRAFT + d.id === picked ? 'sel' : ''}`}>
                  <button
                    type="button"
                    aria-current={DRAFT + d.id === picked}
                    className="flex w-full min-w-0 flex-1 flex-col items-start gap-0.5 text-start"
                    onClick={() => { setPicked(DRAFT + d.id); setFilter(''); setTag(''); }}
                  >
                    <span className="font-semibold text-[13.5px]">{d.name || t('hubBuilder.untitled')}</span>
                    <span className="text-xs text-ink-3">{t('hubBuilder.count', { count: d.entries.length })}</span>
                  </button>
                </div>
              ))}
            </div>
          </>
        )}

        <div className="ss-box flex flex-col gap-2.5">
          <span className="text-[13px] font-semibold">{t('install.hubs.add')}</span>
          <span className="ss-inp">
            <Link2 size={14} className="shrink-0 text-ink-3" />
            <input
              value={url}
              onChange={(e) => { setURL(e.target.value); setError(''); }}
              placeholder={t('install.hubs.urlPlaceholder')}
              aria-label={t('install.hubs.urlPlaceholder')}
              disabled={busy}
            />
          </span>
          <span className="ss-inp">
            <input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder={t('install.hubs.labelPlaceholder')}
              aria-label={t('install.hubs.labelPlaceholder')}
              disabled={busy}
            />
          </span>
          {error && <span className="text-[12.5px] text-bad">{error}</span>}
          <Button size="sm" onClick={() => void add()} loading={busy} disabled={busy}>
            <Plus size={14} />
            {t('install.hubs.addButton')}
          </Button>
          <span className="text-[12.5px] leading-relaxed text-ink-3">{t('install.hubs.hint')}</span>
        </div>
      </div>

      <div className="flex min-w-0 flex-col gap-3">
        {!current ? (
          <EmptyState icon={Library} title={t('hubs.browse.pickTitle')} description={t('hubs.browse.pickHint')} />
        ) : (
          <>
            {/* The filter has to stay reachable: this list runs to hundreds of rows.
                The band runs wider than the content so the list's offset shadow scrolls under it too. */}
            <div className="ss-sec !mb-0 sticky top-0 z-10 -mx-2 -mt-6 bg-bg px-2 pt-6 pb-3">
              <h2>{current.label}</h2>
              {contents.data && <span className="ss-cnt">{contents.data.results.length}</span>}
              <span className="ml-auto flex items-center gap-3">
                {!isDraft && !current.builtIn && !isDefault(current) && (
                  <button type="button" className="ss-btn sm" onClick={() => void save(hubs, current.label)}>
                    {t('hubs.browse.makeDefault')}
                  </button>
                )}
                {tags.length > 0 && (
                  <Select
                    size="sm"
                    className="w-[150px] shrink-0"
                    prefix={t('hubs.browse.tag')}
                    value={tag}
                    onChange={setTag}
                    options={[{ value: '', label: t('hubs.browse.allTags') }, ...tags.map((v) => ({ value: v, label: v }))]}
                  />
                )}
                <span className="ss-inp sm !h-8 w-[200px]">
                  <Search size={14} className="shrink-0 text-ink-3" />
                  <input
                    value={filter}
                    onChange={(e) => setFilter(e.target.value)}
                    placeholder={t('hubs.browse.filter')}
                    aria-label={t('hubs.browse.filter')}
                  />
                </span>
              </span>
            </div>

            {isDraft && (
              <div className="ss-note">
                <span className="flex-1">{t('hubs.browse.localNote')}</span>
              </div>
            )}

            {contents.isPending && (
              <div className="ss-box flex items-center gap-2.5 text-[13px] text-ink-2">
                <Spinner size="sm" />
                {t('hubs.browse.loading')}
              </div>
            )}
            {contents.isError && (
              <div className="ss-note bad">
                <span className="flex-1">{(contents.error as Error).message}</span>
              </div>
            )}
            {contents.data && shown.length === 0 && (
              <EmptyState icon={Search} title={t('hubs.browse.noMatch')} />
            )}
            {shown.length > 0 && (
              <div className="ss-list">
                {shown.map((r) => (
                  <div key={`${r.source}:${r.name}`} className="ss-r">
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="flex items-center gap-2">
                        <span className="nm m">{r.name}</span>
                        {(r.tags ?? []).slice(0, 3).map((name) => (
                          <span key={name} className="ss-tag">{name}</span>
                        ))}
                      </span>
                      {r.description && <span className="truncate text-[13px] text-ink-2">{r.description}</span>}
                      <span className="truncate font-mono text-[11.5px] text-ink-3">{r.source}</span>
                    </span>
                    {!r.source ? (
                      <span className="text-[12.5px] text-ink-3">{t('hubBuilder.noSource')}</span>
                    ) : installed.has(r.name) ? (
                      <span className="ss-st ok">{t('hubs.browse.installed')}</span>
                    ) : (
                      <button type="button" className="ss-btn sm" onClick={() => setInstalling(r.source)}>
                        {t('hubs.browse.install')}
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      {installing && (
        <InstallDialog
          kind="skill"
          initialTab="url"
          initialSource={installing}
          onClose={() => {
            setInstalling(null);
            void queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
          }}
        />
      )}

      <ConfirmDialog
        open={removing !== null}
        title={t('install.hubs.remove')}
        message={t('hubs.browse.removeHint', { name: removing?.label ?? '' })}
        variant="danger"
        onCancel={() => setRemoving(null)}
        onConfirm={() => { const hub = removing; setRemoving(null); if (hub) void remove(hub); }}
      />
    </div>
  );
}

