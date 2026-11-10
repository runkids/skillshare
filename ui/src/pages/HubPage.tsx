import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Library, Pencil, Share, Star, Trash2 } from 'lucide-react';
import { api } from '../api/client';
import type { HubSavedEntry } from '../api/client';
import { hubDrafts } from '../api/hubDrafts';
import type { DraftResponse, HubDraft } from '../api/hubDrafts';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import HubEditor from '../components/hub/HubEditor';
import HubList from '../components/hub/HubList';
import type { HubListItem } from '../components/hub/HubList';
import HubView from '../components/hub/HubView';
import MoreMenu from '../components/hub/MoreMenu';
import ShareDialog from '../components/hub/ShareDialog';
import { COMMUNITY, DRAFT, sameURL } from '../components/hub/hubShared';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useT, plural } from '../i18n';
import { invalidate } from '../lib/queryEvents';

/**
 * Every hub in one list. The user's own hubs show exactly what a recipient sees,
 * and are edited and shared from the same place.
 */
export default function HubPage() {
  const t = useT();
  const queryClient = useQueryClient();
  const [picked, setPicked] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [removing, setRemoving] = useState<HubSavedEntry | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState('');

  const { data: config } = useQuery({
    queryKey: queryKeys.hubConfig,
    queryFn: () => api.getHubConfig(),
    staleTime: staleTimes.config,
  });
  const { data: drafts = [], isPending: draftsPending } = useQuery({ queryKey: queryKeys.hub.drafts, queryFn: () => hubDrafts.list() });

  // A subscription to one of the user's own published hubs is the same hub; show it once.
  const saved = useMemo(() => {
    const own = drafts.map((d) => String(d.fields?.publishUrl ?? '')).filter(Boolean);
    const rest = (config?.hubs ?? []).filter((h) => !sameURL(h.url, COMMUNITY.url) && !own.some((u) => sameURL(u, h.url)));
    return [COMMUNITY, ...rest];
  }, [config, drafts]);
  // With no default saved, `search --hub` falls back to the built-in hub, so that one is the default.
  const isDefault = (hub: HubSavedEntry) =>
    config?.default ? hub.label.toLowerCase() === config.default.toLowerCase() : hub.builtIn === true;

  const items: HubListItem[] = [
    ...saved.map((h) => ({ key: h.url, label: h.label, sub: h.url, mine: false, isDefault: isDefault(h) })),
    ...drafts.map((d) => ({
      key: DRAFT + d.id,
      label: d.name || t('hubBuilder.untitled'),
      sub: t(plural('hubBuilder.count', d.entries.length), { count: d.entries.length })
        + (String(d.fields?.publishUrl ?? '').trim() ? ` · ${t('hubs.shared')}` : ''),
      mine: true,
      isDefault: false,
    })),
  ];

  // Open on the user's own hub when there is one; that is the one they came to work on.
  // Otherwise open on the hub installs come from, once it is known there is no own hub.
  const fallback = draftsPending ? null : (items.find((i) => i.isDefault) ?? items[0])?.key ?? null;
  const selected = picked && items.some((i) => i.key === picked) ? picked : drafts[0] ? DRAFT + drafts[0].id : fallback;
  const draftId = selected?.startsWith(DRAFT) ? selected.slice(DRAFT.length) : undefined;
  const hub = draftId ? undefined : saved.find((h) => h.url === selected);
  const current = items.find((i) => i.key === selected);
  const draft = useQuery({
    queryKey: queryKeys.hub.draft(draftId ?? ''),
    queryFn: () => hubDrafts.get(draftId!),
    enabled: Boolean(draftId),
  });

  const pick = (key: string) => { setPicked(key); setEditing(false); setError(''); };

  const saveConfig = async (next: HubSavedEntry[], defaultLabel: string) => {
    const body = { hubs: next.filter((h) => !h.builtIn).map(({ label, url }) => ({ label, url })), default: defaultLabel };
    await api.putHubConfig(body);
    queryClient.setQueryData(queryKeys.hubConfig, body);
  };

  const addHub = async (raw: string) => {
    const url = raw.trim();
    if (!url) throw new Error(t('install.hubs.urlRequired'));
    if ((config?.hubs ?? []).some((h) => sameURL(h.url, url)) || sameURL(url, COMMUNITY.url)) throw new Error(t('install.hubs.urlExists'));
    // The default hub is kept by label, so two hubs must never share one.
    const base = url.split('/').filter(Boolean).pop() || url;
    const taken = new Set([COMMUNITY, ...(config?.hubs ?? [])].map((h) => h.label.toLowerCase()));
    let label = base;
    for (let n = 2; taken.has(label.toLowerCase()); n++) label = `${base} (${n})`;
    await saveConfig([...(config?.hubs ?? []), { label, url }], config?.default ?? '');
    pick(url);
  };

  const opened = (res: DraftResponse, edit: boolean) => {
    queryClient.setQueryData(queryKeys.hub.draft(res.draft.id), res);
    // In the list before it is picked, or the page falls back to another draft until the refetch.
    queryClient.setQueryData<HubDraft[]>(queryKeys.hub.drafts, (list = []) => [res.draft, ...list.filter((d) => d.id !== res.draft.id)]);
    void invalidate(queryClient, 'hubDraftsChanged');
    pick(DRAFT + res.draft.id);
    setEditing(edit);
  };

  const run = async (action: () => Promise<void>) => {
    setError('');
    try { await action(); } catch (e) { setError((e as Error).message); }
  };

  const create = () => run(async () => {
    opened(await hubDrafts.create({ name: t('hubBuilder.untitled'), description: '', entries: [], fields: {} }), true);
  });

  const importFile = (file: File) => run(async () => {
    if (file.size > 4 * 1024 * 1024) throw new Error(t('hubBuilder.fileTooLarge'));
    opened(await hubDrafts.import(await file.text()), false);
  });

  const deleteDraft = () => run(async () => {
    setDeleting(false);
    if (!draft.data) return;
    await hubDrafts.remove(draft.data.draft);
    queryClient.removeQueries({ queryKey: queryKeys.hub.draft(draft.data.draft.id) });
    await invalidate(queryClient, 'hubDraftsChanged');
    setPicked(null);
  });

  const removeHub = (target: HubSavedEntry) => run(async () => {
    await saveConfig((config?.hubs ?? []).filter((h) => h.url !== target.url), isDefault(target) ? '' : config?.default ?? '');
    setPicked(null);
  });

  let actions = null;
  if (draftId) {
    actions = (
      <>
        <Button variant="secondary" disabled={!draft.data} onClick={() => setEditing(true)}><Pencil size={14} />{t('hubs.view.edit')}</Button>
        <Button disabled={!draft.data} onClick={() => setSharing(true)}><Share size={14} />{t('hubs.view.share')}</Button>
        <MoreMenu label={t('hubs.view.more')} items={[{ label: t('hubs.view.delete'), icon: Trash2, danger: true, onClick: () => setDeleting(true) }]} />
      </>
    );
  } else if (hub && !hub.builtIn) {
    actions = (
      <>
        {!isDefault(hub) && (
          <Button variant="secondary" onClick={() => void run(() => saveConfig(config?.hubs ?? [], hub.label))}><Star size={14} />{t('hubs.browse.makeDefault')}</Button>
        )}
        <MoreMenu label={t('hubs.view.more')} items={[{ label: t('install.hubs.remove'), icon: Trash2, danger: true, onClick: () => setRemoving(hub) }]} />
      </>
    );
  }

  return (
    <div className="ss-wrap animate-page-in">
      <PageHeader title={t('hubs.title')} subtitle={t('hubs.subtitle')} backTo="/skills" />
      <div className="grid grid-cols-[280px_minmax(0,1fr)] items-start gap-7">
        <HubList
          items={items}
          selected={selected}
          onPick={pick}
          locked={editing}
          onAdd={addHub}
          onCreate={() => void create()}
          onImport={(file) => void importFile(file)}
        />
        <div className="flex min-w-0 flex-col gap-3">
          {error && <div role="alert" className="ss-note bad"><span className="flex-1 break-words">{error}</span></div>}
          {!current ? (
            <EmptyState icon={Library} title={t('hubs.browse.pickTitle')} description={t('hubs.browse.pickHint')} />
          ) : editing && draft.data ? (
            <HubEditor key={draft.data.draft.id} response={draft.data} onDone={() => setEditing(false)} />
          ) : (
            <HubView key={current.key} title={current.label} draftId={draftId} url={hub?.url} actions={actions} />
          )}
        </div>
      </div>

      {sharing && draft.data && <ShareDialog response={draft.data} onClose={() => setSharing(false)} />}
      <ConfirmDialog
        open={deleting}
        title={t('hubs.view.delete')}
        message={t('hubs.view.deleteHint')}
        variant="danger"
        onCancel={() => setDeleting(false)}
        onConfirm={() => void deleteDraft()}
      />
      <ConfirmDialog
        open={removing !== null}
        title={t('install.hubs.remove')}
        message={t('hubs.browse.removeHint', { name: removing?.label ?? '' })}
        variant="danger"
        onCancel={() => setRemoving(null)}
        onConfirm={() => { const target = removing; setRemoving(null); if (target) void removeHub(target); }}
      />
    </div>
  );
}
