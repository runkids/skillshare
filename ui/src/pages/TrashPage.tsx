import { useState, useMemo } from 'react';
import {
  Trash2,
  Clock,
  RotateCcw,
  X,
  RefreshCw,
  Puzzle,
  Bot,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { TrashedSkill } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { formatSize } from '../lib/format';
import Card from '../components/Card';
import PageHeader from '../components/PageHeader';
import Button from '../components/Button';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import KindBadge from '../components/KindBadge';
import { useT } from '../i18n';

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d ago`;
  return `${Math.floor(days / 30)}mo ago`;
}

export default function TrashPage() {
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const t = useT();

  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.trash,
    queryFn: () => api.listTrash(),
    staleTime: staleTimes.trash,
  });

  const [restoreItem, setRestoreItem] = useState<TrashedSkill | null>(null);
  const [restoring, setRestoring] = useState(false);
  const [deleteItem, setDeleteItem] = useState<TrashedSkill | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [emptyOpen, setEmptyOpen] = useState(false);
  const [emptying, setEmptying] = useState(false);

  const allItems = data?.items ?? [];

  // Tab state
  type ResourceTab = 'skills' | 'agents';
  const [activeTab, setActiveTab] = useState<ResourceTab>('skills');
  const skillCount = useMemo(() => allItems.filter((i) => (i.kind ?? 'skill') !== 'agent').length, [allItems]);
  const agentCount = useMemo(() => allItems.filter((i) => i.kind === 'agent').length, [allItems]);
  const items = useMemo(
    () => activeTab === 'agents'
      ? allItems.filter((i) => i.kind === 'agent')
      : allItems.filter((i) => (i.kind ?? 'skill') !== 'agent'),
    [allItems, activeTab],
  );

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.trash });
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
  };

  const handleRestore = async () => {
    if (!restoreItem) return;
    setRestoring(true);
    try {
      await api.restoreTrash(restoreItem.name, restoreItem.kind ?? 'skill');
      toast(t('trash.toast.restored', { name: restoreItem.name }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setRestoring(false);
      setRestoreItem(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteItem) return;
    setDeleting(true);
    try {
      await api.deleteTrash(deleteItem.name, deleteItem.kind ?? 'skill');
      toast(t('trash.toast.deleted', { name: deleteItem.name }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setDeleting(false);
      setDeleteItem(null);
    }
  };

  const handleEmpty = async () => {
    setEmptying(true);
    try {
      const res = await api.emptyTrash('all');
      toast(t('trash.toast.emptied', { count: res.removed, s: res.removed !== 1 ? 's' : '' }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setEmptying(false);
      setEmptyOpen(false);
    }
  };

  if (isPending) return <PageSkeleton />;

  if (error) {
    return (
      <Card>
        <p className="text-danger">{error.message}</p>
      </Card>
    );
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<Trash2 size={24} strokeWidth={2.5} />}
        title={t('trash.title')}
        subtitle={isProjectMode
          ? t('trash.subtitle.project')
          : t('trash.subtitle.global')}
        className="mb-4!"
        actions={
          <>
            <Button onClick={handleRefresh} variant="secondary" size="sm">
              <RefreshCw size={16} /> {t('trash.refresh')}
            </Button>
            {allItems.length > 0 && (
              <Button variant="danger" size="sm" onClick={() => setEmptyOpen(true)}>
                <Trash2 size={16} strokeWidth={2.5} /> {t('trash.emptyButton')}
              </Button>
            )}
          </>
        }
      />

      {/* Resource type tabs (Skills / Agents) */}
      <nav className="ss-resource-tabs flex items-center gap-6 border-b-2 border-muted -mx-4 px-4 md:-mx-8 md:px-8" role="tablist">
        {([
          { key: 'skills' as ResourceTab, icon: <Puzzle size={16} strokeWidth={2.5} />, label: t('trash.tab.skills'), count: skillCount },
          { key: 'agents' as ResourceTab, icon: <Bot size={16} strokeWidth={2.5} />, label: t('trash.tab.agents'), count: agentCount },
        ]).map((tab) => (
          <button
            key={tab.key}
            role="tab"
            aria-selected={activeTab === tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`
              ss-resource-tab
              inline-flex items-center gap-1.5 px-1 pb-2.5 text-sm font-semibold cursor-pointer
              transition-all duration-150 border-b-[3px] -mb-[2px]
              ${activeTab === tab.key
                ? 'border-pencil text-pencil'
                : 'border-transparent text-pencil-light hover:text-pencil hover:border-muted-dark'
              }
            `}
          >
            {tab.icon}
            {tab.label}
            <span className={`
              text-[11px] font-medium px-1.5 py-0.5 rounded-[var(--radius-sm)]
              ${activeTab === tab.key ? 'bg-pencil/10 text-pencil' : 'bg-muted text-pencil-light'}
            `}>
              {tab.count}
            </span>
          </button>
        ))}
      </nav>

      {/* Summary line */}
      {items.length > 0 && (
        <p className="text-sm text-pencil-light">
          {t('trash.itemCount', { count: items.length, s: items.length !== 1 ? 's' : '' })}
          {data && data.totalSize > 0 && ` · ${formatSize(data.totalSize)}`}
        </p>
      )}

      {/* Content */}
      {items.length === 0 ? (
        <EmptyState
          icon={Trash2}
          title={activeTab === 'agents' ? t('trash.emptyState.agents.title') : t('trash.emptyState.skills.title')}
          description={activeTab === 'agents'
            ? t('trash.emptyState.agents.description')
            : t('trash.emptyState.skills.description')}
        />
      ) : (
        <div className="space-y-4">
          {items.map((item) => (
            <TrashCard
              key={`${item.name}-${item.timestamp}`}
              item={item}
              onRestore={() => setRestoreItem(item)}
              onDelete={() => setDeleteItem(item)}
            />
          ))}
        </div>
      )}

      {/* Restore Dialog */}
      <ConfirmDialog
        open={restoreItem !== null}
        title={restoreItem?.kind === 'agent' ? t('trash.confirm.restore.titleAgent') : t('trash.confirm.restore.titleSkill')}
        message={
          restoreItem ? (
            <span>
              {t('trash.confirm.restore.message', {
                name: restoreItem.name,
                dir: restoreItem.kind === 'agent' ? 'agents' : 'skills',
              })}
            </span>
          ) : <span />
        }
        confirmText={t('trash.actions.restore')}
        variant="default"
        loading={restoring}
        onConfirm={handleRestore}
        onCancel={() => setRestoreItem(null)}
      />

      {/* Delete Dialog */}
      <ConfirmDialog
        open={deleteItem !== null}
        title={t('trash.confirm.delete.title')}
        message={
          deleteItem ? (
            <span>
              {t('trash.confirm.delete.message', { name: deleteItem.name })}
            </span>
          ) : <span />
        }
        confirmText={t('trash.confirm.delete.confirmText')}
        variant="danger"
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setDeleteItem(null)}
      />

      {/* Empty Trash Dialog */}
      <ConfirmDialog
        open={emptyOpen}
        title={t('trash.confirm.empty.title')}
        message={
          <span>
            {t('trash.confirm.empty.message', { count: items.length, s: items.length !== 1 ? 's' : '' })}
          </span>
        }
        confirmText={t('trash.confirm.empty.confirmText')}
        variant="danger"
        loading={emptying}
        onConfirm={handleEmpty}
        onCancel={() => setEmptyOpen(false)}
      />
    </div>
  );
}

function TrashCard({
  item,
  onRestore,
  onDelete,
}: {
  item: TrashedSkill;
  onRestore: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  return (
    <Card>
      <div className="space-y-3">
        {/* Name + time */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-pencil">
            <Trash2 size={16} strokeWidth={2.5} />
            <span className="font-medium">{item.name}</span>
            <KindBadge kind={item.kind ?? 'skill'} />
            <span className="text-sm text-pencil-light">
              {timeAgo(item.date)}
            </span>
          </div>
          <Badge variant="default">{formatSize(item.size)}</Badge>
        </div>

        {/* Deleted at */}
        <div className="flex items-center gap-2 text-sm text-pencil-light">
          <Clock size={14} strokeWidth={2.5} />
          <span>{t('trash.deletedAt', { date: new Date(item.date).toLocaleString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: 'numeric',
            minute: '2-digit',
          }) })}</span>
        </div>

        {/* Actions */}
        <div className="border-t border-dashed border-pencil-light/30 pt-3 flex gap-2">
          <Button variant="secondary" size="sm" onClick={onRestore}>
            <RotateCcw size={14} strokeWidth={2.5} /> {t('trash.actions.restore')}
          </Button>
          <Button variant="ghost" size="sm" onClick={onDelete}>
            <X size={14} strokeWidth={2.5} /> {t('trash.actions.delete')}
          </Button>
        </div>
      </div>
    </Card>
  );
}
