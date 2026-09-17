import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronRight, Plus, Target as TargetIcon } from 'lucide-react';
import { api, type Target } from '../api/client';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import AddTargetDialog from '../components/targets/AddTargetDialog';
import { refreshTargets, targetHealth } from '../components/targets/targetView';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { shortenHome } from '../lib/paths';
import { useT } from '../i18n';

const TONE = { synced: 'ok', pending: 'warn', missing: 'warn', problem: 'bad', unknown: 'off' } as const;
const PROBLEM_TEXT: Record<string, string> = { 'not exist': 'targets.syncing.missing', conflict: 'targets.syncing.conflict', broken: 'targets.syncing.broken', 'has files': 'targets.syncing.hasFiles' };

export default function TargetsPage() {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.targets.all, queryFn: () => api.listTargets(), staleTime: staleTimes.targets });
  const available = useQuery({ queryKey: queryKeys.targets.available, queryFn: () => api.availableTargets(), staleTime: staleTimes.targets });
  const [adding, setAdding] = useState<{ initial?: string } | null>(null);

  // The API walks a map, so the order changes between requests.
  const targets = [...(data?.targets ?? [])].sort((a, b) => a.name.localeCompare(b.name));
  const known = available.data?.targets ?? [];
  const found = known.filter((a) => a.detected && !a.installed).sort((a, b) => a.name.localeCompare(b.name));

  const syncing = (tg: Target, pending: number) => {
    if (PROBLEM_TEXT[tg.status]) return t(PROBLEM_TEXT[tg.status]);
    if (tg.mode === 'symlink') return t('targets.syncing.folderLinked');
    return [
      t(tg.mode === 'copy' ? 'targets.syncing.copied' : 'targets.syncing.linked', { count: tg.linkedCount }),
      pending > 0 && t('targets.syncing.pending', { count: pending }),
      tg.localCount > 0 && t('targets.syncing.local', { count: tg.localCount }),
    ].filter(Boolean).join(' · ');
  };
  const addButton = <Button variant="primary" onClick={() => setAdding({})}><Plus size={15} />{t('targets.addTarget')}</Button>;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t('targets.title')} subtitle={t('targets.subtitle')} actions={<span data-tour="targets-grid">{addButton}</span>} />

      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : targets.length === 0 ? (
        <EmptyState icon={TargetIcon} title={t('targets.emptyTitle')} description={t('targets.emptyDescription')} action={addButton} />
      ) : (
        <div className="ss-list">
          <div className="ss-lh">
            <span className="w-[30px]" />
            <span className="flex-1">{t('targets.col.target')}</span>
            <span className="w-[92px]">{t('targets.col.mode')}</span>
            <span className="w-[210px]">{t('targets.col.syncing')}</span>
            <span className="w-[130px]">{t('targets.col.status')}</span>
            <span className="w-4" />
          </div>
          {targets.map((tg) => {
            const { state, pending } = targetHealth(tg);
            return (
              <Link key={tg.name} to={`/targets/${encodeURIComponent(tg.name)}`} className="ss-r link !min-h-[56px]">
                <span className="ss-at"><AgentIcon target={tg.name} size={17} /></span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="flex items-center gap-2">
                    <span className="font-semibold">{tg.name}</span>
                    {tg.name === 'universal' && <span className="ss-tag">{t('targets.sharedPath')}</span>}
                  </span>
                  <span className="truncate font-mono text-[12px] text-ink-3" title={tg.path}>{shortenHome(tg.path)}</span>
                </span>
                <span className="w-[92px] shrink-0"><span className="ss-tag">{tg.mode}</span></span>
                <span className="w-[210px] shrink-0 truncate text-[13px] text-ink-2">{syncing(tg, pending)}</span>
                <span className="w-[130px] shrink-0">
                  <span className={`ss-st ${TONE[state]}`}>
                    {state === 'pending' ? t('targets.state.pending', { count: pending }) : state === 'unknown' ? tg.status : t(`targets.state.${state}`)}
                  </span>
                </span>
                <ChevronRight size={16} className="shrink-0 text-ink-3" />
              </Link>
            );
          })}
        </div>
      )}

      {found.length > 0 && (
        <section className="mt-10">
          <div className="ss-sec">
            <h2 className="ss-h2">{t('targets.foundOnMachine')}</h2>
            <span className="ss-cnt">{found.length}</span>
          </div>
          <div className="ss-list">
            {found.map((a) => (
              <div key={a.name} className="ss-r">
                <span className="ss-at"><AgentIcon target={a.name} size={17} /></span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="font-semibold">{a.name}</span>
                  <span className="truncate font-mono text-[12px] text-ink-3">{shortenHome(a.path)}</span>
                </span>
                <Button variant="secondary" size="sm" onClick={() => setAdding({ initial: a.name })} aria-label={t('targets.add.addNamed', { name: a.name })}>
                  <Plus size={14} />{t('targets.add.button')}
                </Button>
              </div>
            ))}
          </div>
          <p className="mt-3 text-[13px] text-ink-3">{t('targets.foundHint', { count: known.length })}</p>
        </section>
      )}

      {adding && (
        <AddTargetDialog
          available={known}
          initial={adding.initial}
          existing={targets.map((tg) => tg.name)}
          onClose={() => setAdding(null)}
          onAdded={(name) => {
            setAdding(null);
            refreshTargets(queryClient);
            toast(t('targets.targetAdded', { name }), 'success');
          }}
        />
      )}
    </div>
  );
}
