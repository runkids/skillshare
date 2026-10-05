import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Search, Tag } from 'lucide-react';
import { api } from '../../api/client';
import type { SearchResult } from '../../api/client';
import { hubDrafts } from '../../api/hubDrafts';
import EmptyState from '../EmptyState';
import InstallDialog from '../InstallDialog';
import Spinner from '../Spinner';
import { Select } from '../Select';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import { useT } from '../../i18n';
import { useSkillsQuery } from '../../hooks/useSharedQueries';

interface Props {
  title: string;
  /** Set for the user's own hub; its rows come from the saved draft instead of a hosted index. */
  draftId?: string;
  url?: string;
  actions: ReactNode;
}

/**
 * A hub exactly as a recipient sees it. Installing hands the source to the
 * install dialog so the audit gate stays in one place.
 */
export default function HubView({ title, draftId, url, actions }: Props) {
  const t = useT();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState('');
  const [tag, setTag] = useState('');
  const [installing, setInstalling] = useState<string | null>(null);

  const draft = useQuery({
    queryKey: queryKeys.hub.draft(draftId ?? ''),
    queryFn: () => hubDrafts.get(draftId!),
    enabled: Boolean(draftId),
  });
  const hosted = useQuery({
    queryKey: queryKeys.hub.contents(url ?? ''),
    // An empty query returns every entry of a hosted index.
    queryFn: () => api.searchHub('', url!),
    enabled: !draftId && Boolean(url),
    staleTime: staleTimes.skills,
  });
  const contents = draftId ? draft : hosted;
  const results = useMemo<SearchResult[] | undefined>(() => {
    if (!draftId) return hosted.data?.results;
    const response = draft.data;
    return response?.draft.entries.map((e) => ({
      name: e.data.name ?? '',
      description: e.data.description ?? '',
      source: e.data.source ?? '',
      skill: e.data.skill,
      tags: e.data.tags,
      ref: response.refs?.[e.id],
      stars: 0,
      owner: '',
      repo: '',
    }));
  }, [draftId, draft.data, hosted.data]);

  const { data: skillsData } = useSkillsQuery();
  const installed = useMemo(() => new Set((skillsData?.resources ?? []).map((r) => r.name)), [skillsData]);

  /** Every tag the hub actually uses, so the dropdown never offers a dead option. */
  const tags = useMemo(() => {
    const seen = new Set<string>();
    for (const r of results ?? []) for (const name of r.tags ?? []) seen.add(name);
    return [...seen].sort();
  }, [results]);

  const shown = (results ?? []).filter(
    (r) =>
      (!tag || (r.tags ?? []).includes(tag)) &&
      `${r.name} ${r.description} ${(r.tags ?? []).join(' ')}`.toLowerCase().includes(filter.toLowerCase()),
  );

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {/* The filter has to stay reachable: a hosted hub runs to hundreds of rows.
          The band runs wider than the content so the list's offset shadow scrolls under it too. */}
      <div className="sticky top-0 z-10 -mx-2 flex flex-col gap-3 bg-bg px-2">
        <div className="ss-sec !mb-0 !items-center">
          <h2>{title}</h2>
          {results && <span className="ss-cnt">{results.length}</span>}
          {draftId && <span className="ss-tag inf">{t('hubs.mine')}</span>}
          <span className="ml-auto flex items-center gap-2.5">
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
            <span className="ss-inp sm !h-8 w-[220px]">
              <Search size={14} className="shrink-0 text-ink-3" />
              <input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder={t('hubs.browse.filter')} aria-label={t('hubs.browse.filter')} />
            </span>
            {actions}
          </span>
        </div>
        {draftId && <span className="text-[12.5px] text-ink-3">{t('hubs.view.mineNote')}</span>}
      </div>

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
      {results && shown.length === 0 && (
        <EmptyState icon={Search} title={results.length === 0 ? t('hubBuilder.noEntries') : t('hubs.browse.noMatch')} />
      )}
      {shown.length > 0 && (
        <div className="ss-list">
          <div className="ss-lh">
            <span className="flex-1">Skill</span>
            <span className="w-[120px] shrink-0">{t('hubs.view.version')}</span>
            <span className="w-[96px] shrink-0" />
          </div>
          {shown.map((r, i) => (
            <div key={`${i}:${r.source}:${r.name}`} className="ss-r">
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex items-center gap-2">
                  <span className="nm m font-mono">{r.name}</span>
                  {(r.tags ?? []).slice(0, 3).map((name) => (
                    <span key={name} className="ss-tag">{name}</span>
                  ))}
                </span>
                {r.description && <span className="truncate text-[13px] text-ink-2">{r.description}</span>}
                <span className="truncate font-mono text-[11.5px] text-ink-3">{r.source}</span>
              </span>
              <span className="flex w-[120px] shrink-0 items-center">
                {r.ref ? (
                  <span className="ss-tag max-w-full"><Tag size={11} className="shrink-0" /><span className="truncate">{r.ref}</span></span>
                ) : (
                  <span className="text-xs text-ink-3">{t('hubs.ref.default')}</span>
                )}
              </span>
              <span className="flex w-[96px] shrink-0 justify-end">
                {!r.source ? (
                  <span className="text-[12.5px] text-ink-3">{t('hubBuilder.noSource')}</span>
                ) : installed.has(r.name) ? (
                  <span className="ss-st ok">{t('hubs.browse.installed')}</span>
                ) : (
                  <button type="button" className="ss-btn sm" onClick={() => setInstalling(r.source)}>
                    {t('hubs.browse.install')}
                  </button>
                )}
              </span>
            </div>
          ))}
        </div>
      )}

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
    </div>
  );
}
