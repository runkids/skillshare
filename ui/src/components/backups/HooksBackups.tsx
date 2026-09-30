import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Webhook } from 'lucide-react';
import type { HookBackup } from '../../api/hooks';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import EmptyState from '../EmptyState';
import { PageSkeleton } from '../Skeleton';
import { useToast } from '../Toast';
import HooksRestoreDialog from '../hooks/HooksRestoreDialog';
import { backupTime, hookLabel } from '../hooks/hooksView';
import { formatDateTime, formatRelativeTime, useI18n } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import { useHooksQuery } from '../../hooks/useSharedQueries';

/** Hook backups, grouped by the Agent file they were taken of. */
export default function HooksBackups() {
  const { t, locale } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const { data, isPending, error } = useHooksQuery();
  const [restoring, setRestoring] = useState<{ group: HookBackup[]; id: string } | null>(null);

  // Newest first, so each group lists newest first and groups follow their newest backup.
  const when = (b: HookBackup) => (b.time ? new Date(b.time) : backupTime(b.id));
  const groups = new Map<string, HookBackup[]>();
  for (const b of [...(data?.backups ?? [])].sort((x, y) => when(y).getTime() - when(x).getTime())) {
    const key = `${b.target}\n${b.path}`;
    groups.set(key, [...(groups.get(key) ?? []), b]);
  }

  return (
    <>
      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : groups.size === 0 ? (
        <EmptyState icon={Webhook} title={t('backup.hooks.empty.title')} description={t('backup.hooks.empty.description')} />
      ) : (
        [...groups.values()].map((group) => (
          <div key={`${group[0].target}\n${group[0].path}`} className="ss-list">
            <div className="ss-gh !min-h-12">
              <span className="ss-at"><AgentIcon target={group[0].target} size={16} /></span>
              <span className="font-semibold">{hookLabel(group[0].target)}</span>
              <span className="min-w-0 flex-1 truncate font-mono text-ink-3" title={group[0].path}>{shortenHome(group[0].path)}</span>
              <span className="shrink-0 text-ink-3">{t(group.length === 1 ? 'backup.hooks.count.one' : 'backup.hooks.count.other', { count: group.length })}</span>
            </div>
            {group.map((b) => {
              const taken = when(b);
              return (
                <div key={b.id} className="ss-r">
                  <span className="flex min-w-0 flex-1 flex-col gap-px">
                    <span className="text-[13px] font-semibold">{formatRelativeTime(taken, locale)}</span>
                    <span className="font-mono text-xs text-ink-3">{formatDateTime(taken, locale, { dateStyle: 'medium', timeStyle: 'short' })}</span>
                  </span>
                  <Button variant="secondary" size="sm" onClick={() => setRestoring({ group, id: b.id })}>{t('backup.files.previewRestore')}</Button>
                </div>
              );
            })}
          </div>
        ))
      )}
      {restoring && (
        <HooksRestoreDialog
          backups={restoring.group}
          initialId={restoring.id}
          onClose={() => setRestoring(null)}
          onRestored={() => {
            setRestoring(null);
            void queryClient.invalidateQueries({ queryKey: queryKeys.hooks });
            toast(t('hooks.toast.restored'), 'success');
          }}
        />
      )}
    </>
  );
}
