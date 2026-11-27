import { Fragment, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Plug } from 'lucide-react';
import type { MCPBackup } from '../../api/mcp';
import AgentTile from '../AgentTile';
import Button from '../Button';
import EmptyState from '../EmptyState';
import { PageSkeleton } from '../Skeleton';
import { useToast } from '../Toast';
import MCPRestoreDialog from '../mcp/MCPRestoreDialog';
import { backupTime, targetLabel } from '../mcp/mcpView';
import { formatDateTime, formatRelativeTime, useI18n, plural } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { mcpChanges } from './backupView';
import { useMcpQuery } from '../../hooks/useSharedQueries';
import { invalidate } from '../../lib/queryEvents';

/** MCP config backups, grouped by the Agent file they were taken of. */
export default function MCPBackups() {
  const { t, locale } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const { data, isPending, error } = useMcpQuery();
  const [restoring, setRestoring] = useState<{ group: MCPBackup[]; id: string } | null>(null);

  // Newest first, so each group lists newest first and groups follow their newest backup.
  const when = (b: MCPBackup) => (b.time ? new Date(b.time) : backupTime(b.id));
  const groups = new Map<string, MCPBackup[]>();
  for (const b of [...(data?.backups ?? [])].sort(
    (x, y) => when(y).getTime() - when(x).getTime(),
  )) {
    const key = `${b.target}\n${b.path}`;
    groups.set(key, [...(groups.get(key) ?? []), b]);
  }

  return (
    <>
      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad">
          <span className="flex-1">{error.message}</span>
        </div>
      ) : groups.size === 0 ? (
        <EmptyState
          icon={Plug}
          title={t('backup.mcp.empty.title')}
          description={t('backup.mcp.empty.description')}
        />
      ) : (
        [...groups.values()].map((group) => (
          <div key={`${group[0].target}\n${group[0].path}`} className="ss-list">
            <div className="ss-gh !min-h-12">
              <AgentTile target={group[0].target} iconSize={16} />
              <span className="font-semibold">{targetLabel(group[0].target)}</span>
              <span className="min-w-0 flex-1 truncate font-mono text-ink-3" title={group[0].path}>
                {shortenHome(group[0].path)}
              </span>
              <span className="shrink-0 text-ink-3">
                {t(plural('backup.mcp.count', group.length), { count: group.length })}
              </span>
            </div>
            {group.map((b) => {
              const taken = when(b);
              const changes = mcpChanges(b.servers);
              return (
                <div key={b.id} className="ss-r">
                  <span className="flex w-[200px] shrink-0 flex-col gap-px">
                    <span className="text-[13px] font-semibold">
                      {formatRelativeTime(taken, locale)}
                    </span>
                    <span className="font-mono text-xs text-ink-3">
                      {formatDateTime(taken, locale, { dateStyle: 'medium', timeStyle: 'short' })}
                    </span>
                  </span>
                  <span className="min-w-0 flex-1 text-[13px] text-ink-2">
                    {changes.length === 0
                      ? t('backup.mcp.noChanges')
                      : changes.map((c, i) => (
                          <Fragment key={c.change}>
                            {i > 0 && ' · '}
                            {t(`backup.mcp.${c.change}`, {
                              names: c.names.join(t('backup.mcp.listSep')),
                            })}
                          </Fragment>
                        ))}
                  </span>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setRestoring({ group, id: b.id })}
                  >
                    {t('backup.files.previewRestore')}
                  </Button>
                </div>
              );
            })}
          </div>
        ))
      )}
      {restoring && (
        <MCPRestoreDialog
          backups={restoring.group}
          initialId={restoring.id}
          onClose={() => setRestoring(null)}
          onRestored={() => {
            setRestoring(null);
            void invalidate(queryClient, 'mcpRestored');
            toast(t('mcp.toast.restored'), 'success');
          }}
        />
      )}
    </>
  );
}
