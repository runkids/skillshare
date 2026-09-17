import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Bot, CircleX, Puzzle, Trash2, Undo2 } from 'lucide-react';
import { api } from '../api/client';
import type { Skill, TrashedSkill } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { formatRelativeTime, useI18n, useT, type Locale } from '../i18n';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';

const HOUR = 60 * 60 * 1000;
const DAY = 24 * HOUR;
// ponytail: mirrors defaultMaxAge in internal/trash; send it from the API if it becomes configurable.
const TRASH_TTL = 7 * DAY;

export default function TrashPage({ kind }: { kind: Skill['kind'] }) {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, error } = useQuery({
    queryKey: queryKeys.trash,
    queryFn: () => api.listTrash(),
    staleTime: staleTimes.trash,
  });
  const items = useMemo(
    () => (data?.items ?? [])
      .filter((i) => (i.kind ?? 'skill') === kind)
      .sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime()),
    [data, kind],
  );

  const [restoring, setRestoring] = useState<string | null>(null);
  const [deleteItem, setDeleteItem] = useState<TrashedSkill | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [emptyOpen, setEmptyOpen] = useState(false);
  const [emptying, setEmptying] = useState(false);

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.trash });
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    queryClient.invalidateQueries({ queryKey: ['sync-matrix'] });
  };

  const restore = async (item: TrashedSkill) => {
    setRestoring(itemKey(item));
    try {
      await api.restoreTrash(item.name, kind);
      toast(t('trash.toast.restored', { name: item.name }), 'success');
      refresh();
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setRestoring(null);
    }
  };

  const remove = async () => {
    if (!deleteItem) return;
    setDeleting(true);
    try {
      await api.deleteTrash(deleteItem.name, kind);
      toast(t('trash.toast.deleted', { name: deleteItem.name }), 'success');
      refresh();
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setDeleting(false);
      setDeleteItem(null);
    }
  };

  const empty = async () => {
    setEmptying(true);
    try {
      const res = await api.emptyTrash(kind);
      toast(t('trash.toast.emptied', { count: res.removed, s: res.removed !== 1 ? 's' : '' }), 'success');
      refresh();
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setEmptying(false);
      setEmptyOpen(false);
    }
  };

  if (error) {
    return (
      <div className="ss-note bad">
        <CircleX size={16} />
        <div className="flex-1">{error.message}</div>
      </div>
    );
  }
  if (!data) return null;

  return (
    <>
      <div className="flex flex-wrap items-center gap-2 -mt-2">
        <span className="flex-1 text-[13px] text-ink-2">{t('trash.summary')}</span>
        {items.length > 0 && (
          <Button variant="danger" onClick={() => setEmptyOpen(true)}>
            <Trash2 size={15} />
            {t('trash.emptyButton')}
          </Button>
        )}
      </div>

      <div className="-mt-3">
        {items.length === 0 ? (
          <EmptyState
            icon={Trash2}
            title={t(kind === 'agent' ? 'trash.emptyState.agents.title' : 'trash.emptyState.skills.title')}
            description={t(kind === 'agent' ? 'trash.emptyState.agents.description' : 'trash.emptyState.skills.description')}
          />
        ) : (
          <div className="ss-list">
            <div className="ss-lh">
              <span className="w-[26px]" />
              <span className="flex-1">{t('resources.col.name')}</span>
              <span className="w-[130px]">{t('trash.col.uninstalled')}</span>
              <span className="w-[130px]">{t('trash.col.deletedIn')}</span>
              <span className="w-[132px]" />
            </div>
            {items.map((item) => {
              const left = timeLeft(item.date, locale);
              return (
                <div key={itemKey(item)} className="ss-r">
                  <span className={`ss-cat sm ${kind}`}>{kind === 'agent' ? <Bot size={14} /> : <Puzzle size={14} />}</span>
                  <span className="nm m flex-1 truncate">{item.name}</span>
                  <span className="w-[130px] text-[13px] text-ink-2">{formatRelativeTime(item.date, locale)}</span>
                  <span className={`w-[130px] text-[13px] ${left.soon ? 'text-warn' : 'text-ink-2'}`}>{left.text}</span>
                  <span className="w-[132px] flex items-center justify-end gap-1">
                    <Button variant="secondary" size="sm" loading={restoring === itemKey(item)} disabled={restoring !== null} onClick={() => restore(item)}>
                      <Undo2 size={14} />
                      {t('trash.actions.restore')}
                    </Button>
                    <button
                      type="button"
                      className="ss-ib"
                      title={t('trash.confirm.delete.confirmText')}
                      aria-label={t('trash.confirm.delete.confirmText')}
                      onClick={() => setDeleteItem(item)}
                    >
                      <Trash2 size={16} />
                    </button>
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <ConfirmDialog
        open={deleteItem !== null}
        title={t('trash.confirm.delete.title')}
        message={t('trash.confirm.delete.message', { name: deleteItem?.name ?? '' })}
        confirmText={t('trash.confirm.delete.confirmText')}
        variant="danger"
        loading={deleting}
        onConfirm={remove}
        onCancel={() => setDeleteItem(null)}
      />
      <ConfirmDialog
        open={emptyOpen}
        title={t('trash.confirm.empty.title')}
        message={t('trash.confirm.empty.message', { count: items.length, s: items.length !== 1 ? 's' : '' })}
        confirmText={t('trash.confirm.empty.confirmText')}
        variant="danger"
        loading={emptying}
        onConfirm={empty}
        onCancel={() => setEmptyOpen(false)}
      />
    </>
  );
}

function itemKey(item: TrashedSkill) {
  return `${item.name}-${item.timestamp}`;
}

function timeLeft(date: string, locale: Locale) {
  const ms = new Date(date).getTime() + TRASH_TTL - Date.now();
  const unit = ms >= DAY ? 'day' : 'hour';
  const n = Math.max(1, Math.ceil(ms / (unit === 'day' ? DAY : HOUR)));
  return {
    text: new Intl.NumberFormat(locale, { style: 'unit', unit, unitDisplay: 'long' }).format(n),
    soon: ms <= 2 * DAY,
  };
}
