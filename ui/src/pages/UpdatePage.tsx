import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  RefreshCw, Check, ArrowUpCircle, Loader2,
  Circle, CheckCircle, XCircle, MinusCircle, ShieldAlert,
} from 'lucide-react';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import { HandCheckbox } from '../components/HandInput';
import Badge from '../components/Badge';
import { PageSkeleton } from '../components/Skeleton';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { api } from '../api/client';
import type { UpdateResultItem } from '../api/client';
import { wobbly } from '../design';

type UpdatePhase = 'idle' | 'updating' | 'done';

interface ItemUpdateStatus {
  name: string;
  isRepo: boolean;
  status: 'pending' | 'in-progress' | 'success' | 'error' | 'blocked' | 'skipped';
  message?: string;
  auditRiskLabel?: string;
}

function formatRelativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  if (isNaN(then)) return dateStr;
  const diff = Math.floor((now - then) / 1000);
  if (diff < 60) return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  if (diff < 2592000) return `${Math.floor(diff / 86400)}d ago`;
  if (diff < 31536000) return `${Math.floor(diff / 2592000)}mo ago`;
  return `${Math.floor(diff / 31536000)}y ago`;
}

function actionToStatus(action: string): ItemUpdateStatus['status'] {
  switch (action) {
    case 'updated': return 'success';
    case 'error': return 'error';
    case 'blocked': return 'blocked';
    case 'skipped':
    case 'up-to-date': return 'skipped';
    default: return 'success';
  }
}

export default function UpdatePage() {
  const queryClient = useQueryClient();
  const [selectedRepos, setSelectedRepos] = useState<Set<string>>(new Set());
  const [selectedSkills, setSelectedSkills] = useState<Set<string>>(new Set());
  const [phase, setPhase] = useState<UpdatePhase>('idle');
  const [itemStatuses, setItemStatuses] = useState<ItemUpdateStatus[]>([]);

  const checkQuery = useQuery({
    queryKey: queryKeys.check,
    queryFn: () => api.check(),
    staleTime: staleTimes.check,
  });

  const { data, isPending, error } = checkQuery;

  if (isPending) return <PageSkeleton />;
  if (error) return <Card variant="accent"><p className="text-danger p-2" style={{ fontFamily: 'var(--font-hand)' }}>{(error as Error).message}</p></Card>;
  if (!data) return null;

  const updatableRepos = data.tracked_repos.filter((r) => r.status === 'behind');
  const updatableSkills = data.skills.filter((s) => s.status === 'update_available');
  const hasUpdates = updatableRepos.length > 0 || updatableSkills.length > 0;

  const toggleRepo = (name: string) => {
    setSelectedRepos((prev) => {
      const next = new Set(prev);
      next.has(name) ? next.delete(name) : next.add(name);
      return next;
    });
  };

  const toggleSkill = (name: string) => {
    setSelectedSkills((prev) => {
      const next = new Set(prev);
      next.has(name) ? next.delete(name) : next.add(name);
      return next;
    });
  };

  const selectAllRepos = () => {
    setSelectedRepos((prev) =>
      prev.size === updatableRepos.length ? new Set() : new Set(updatableRepos.map((r) => r.name))
    );
  };

  const selectAllSkills = () => {
    setSelectedSkills((prev) =>
      prev.size === updatableSkills.length ? new Set() : new Set(updatableSkills.map((s) => s.name))
    );
  };

  const totalSelected = selectedRepos.size + selectedSkills.size;

  const handleUpdate = async () => {
    if (totalSelected === 0) return;

    // Build items list with repo/skill distinction
    const items: { name: string; isRepo: boolean }[] = [
      ...[...selectedRepos].map((name) => ({ name, isRepo: true })),
      ...[...selectedSkills].map((name) => ({ name, isRepo: false })),
    ];

    // Initialize statuses
    const initial: ItemUpdateStatus[] = items.map((it) => ({
      name: it.name,
      isRepo: it.isRepo,
      status: 'pending',
    }));
    setItemStatuses(initial);
    setPhase('updating');

    for (let i = 0; i < items.length; i++) {
      // Mark current as in-progress
      setItemStatuses((prev) =>
        prev.map((s, idx) => idx === i ? { ...s, status: 'in-progress' } : s)
      );

      try {
        const resp = await api.update({ name: items[i].name });
        const result: UpdateResultItem | undefined = resp.results?.[0];
        const status = result ? actionToStatus(result.action) : 'success';
        setItemStatuses((prev) =>
          prev.map((s, idx) =>
            idx === i
              ? {
                  ...s,
                  status,
                  message: result?.message,
                  auditRiskLabel: result?.auditRiskLabel,
                }
              : s
          )
        );
      } catch (e) {
        setItemStatuses((prev) =>
          prev.map((s, idx) =>
            idx === i ? { ...s, status: 'error', message: (e as Error).message } : s
          )
        );
      }
    }

    setPhase('done');
    queryClient.invalidateQueries({ queryKey: queryKeys.check });
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
  };

  const handleDone = () => {
    setPhase('idle');
    setItemStatuses([]);
    setSelectedRepos(new Set());
    setSelectedSkills(new Set());
  };

  const upToDateRepos = data.tracked_repos.filter((r) => r.status === 'up_to_date').length;
  const upToDateSkills = data.skills.filter((s) => s.status === 'up_to_date').length;

  // Summary counts for results
  const successCount = itemStatuses.filter((s) => s.status === 'success').length;
  const skippedCount = itemStatuses.filter((s) => s.status === 'skipped').length;
  const blockedCount = itemStatuses.filter((s) => s.status === 'blocked').length;
  const errorCount = itemStatuses.filter((s) => s.status === 'error').length;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
            Updates
          </h1>
          <p className="text-pencil-light text-sm mt-1" style={{ fontFamily: 'var(--font-hand)' }}>
            Check and apply updates for tracked repositories and installed skills.
          </p>
        </div>
        <div className="flex gap-2">
          {phase === 'idle' && (
            <HandButton
              variant="ghost"
              size="sm"
              onClick={() => queryClient.invalidateQueries({ queryKey: queryKeys.check })}
              disabled={checkQuery.isFetching}
            >
              <RefreshCw size={16} className={checkQuery.isFetching ? 'animate-spin' : ''} />
              Check Now
            </HandButton>
          )}
          {phase === 'idle' && totalSelected > 0 && (
            <HandButton onClick={handleUpdate}>
              <ArrowUpCircle size={16} />
              Update Selected ({totalSelected})
            </HandButton>
          )}
        </div>
      </div>

      {/* Update results panel */}
      {phase !== 'idle' && (
        <div className="space-y-4 animate-sketch-in">
          {/* Summary bar */}
          <Card variant="postit" decoration="tape" className="rotate-[-0.3deg]">
            <div className="flex items-center justify-between flex-wrap gap-3">
              <div className="flex items-center gap-2 flex-wrap">
                {phase === 'updating' && (
                  <span className="flex items-center gap-1.5 text-pencil font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
                    <Loader2 size={16} className="animate-spin text-blue" />
                    Updating...
                  </span>
                )}
                {phase === 'done' && (
                  <>
                    {successCount > 0 && <Badge variant="success">{successCount} updated</Badge>}
                    {skippedCount > 0 && <Badge>{skippedCount} skipped</Badge>}
                    {blockedCount > 0 && <Badge variant="warning">{blockedCount} blocked</Badge>}
                    {errorCount > 0 && <Badge variant="danger">{errorCount} failed</Badge>}
                  </>
                )}
              </div>
              {phase === 'done' && (
                <HandButton variant="ghost" size="sm" onClick={handleDone}>
                  Done
                </HandButton>
              )}
            </div>
          </Card>

          {/* Per-item status cards */}
          <div className="space-y-2">
            {itemStatuses.map((item, i) => (
              <div
                key={item.name}
                className="flex items-center gap-3 px-3 py-2 border border-pencil/10 animate-sketch-in"
                style={{
                  borderRadius: wobbly.sm,
                  animationDelay: `${i * 50}ms`,
                  animationFillMode: 'backwards',
                }}
              >
                <StatusIcon status={item.status} />
                <div className="flex-1 min-w-0">
                  <span className="text-pencil font-medium block" style={{ fontFamily: 'var(--font-hand)' }}>
                    {item.name}
                  </span>
                  {item.message && (
                    <span className="text-pencil-light text-sm block truncate">{item.message}</span>
                  )}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {item.auditRiskLabel && item.auditRiskLabel !== 'clean' && (
                    <Badge variant={item.auditRiskLabel === 'critical' || item.auditRiskLabel === 'high' ? 'danger' : 'warning'}>
                      <ShieldAlert size={12} className="mr-1" />
                      {item.auditRiskLabel}
                    </Badge>
                  )}
                  <StatusBadge status={item.status} />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Idle state content */}
      {phase === 'idle' && (
        <>
          {!hasUpdates ? (
            <Card variant="postit" decoration="tape" className="rotate-[-0.5deg]">
              <div className="flex flex-col items-center py-6 text-center">
                <div className="w-14 h-14 bg-success-light border-2 border-success rounded-full flex items-center justify-center mb-4">
                  <Check size={28} strokeWidth={2.5} className="text-success" />
                </div>
                <h3
                  className="text-xl text-pencil mb-1"
                  style={{ fontFamily: 'var(--font-heading)' }}
                >
                  Everything is up to date
                </h3>
                <p className="text-pencil-light text-base max-w-xs" style={{ fontFamily: 'var(--font-hand)' }}>
                  All tracked repositories and skills are at their latest versions.
                </p>
                <div className="mt-4">
                  <HandButton
                    variant="secondary"
                    size="sm"
                    onClick={() => queryClient.invalidateQueries({ queryKey: queryKeys.check })}
                    disabled={checkQuery.isFetching}
                  >
                    <RefreshCw size={16} className={checkQuery.isFetching ? 'animate-spin' : ''} />
                    Check Again
                  </HandButton>
                </div>
              </div>
            </Card>
          ) : (
            <>
              {updatableRepos.length > 0 && (
                <Card>
                  <div className="flex items-center justify-between mb-3">
                    <h2 className="text-lg font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                      Tracked Repositories ({updatableRepos.length})
                    </h2>
                    <HandButton variant="ghost" size="sm" onClick={selectAllRepos}>
                      {selectedRepos.size === updatableRepos.length ? 'Deselect All' : 'Select All'}
                    </HandButton>
                  </div>
                  <div className="space-y-2">
                    {updatableRepos.map((repo) => (
                      <div
                        key={repo.name}
                        className="flex items-center gap-3 px-3 py-2 border border-pencil/10"
                        style={{ borderRadius: wobbly.sm }}
                      >
                        <HandCheckbox label="" checked={selectedRepos.has(repo.name)} onChange={() => toggleRepo(repo.name)} />
                        <div className="flex-1 min-w-0">
                          <span className="text-pencil font-medium block" style={{ fontFamily: 'var(--font-hand)' }}>
                            {repo.name}
                          </span>
                          {repo.message && (
                            <span className="text-pencil-light text-sm block truncate">{repo.message}</span>
                          )}
                        </div>
                        <Badge variant="warning">{repo.behind} commit(s) behind</Badge>
                      </div>
                    ))}
                  </div>
                </Card>
              )}

              {updatableSkills.length > 0 && (
                <Card>
                  <div className="flex items-center justify-between mb-3">
                    <h2 className="text-lg font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                      Skills with Updates ({updatableSkills.length})
                    </h2>
                    <HandButton variant="ghost" size="sm" onClick={selectAllSkills}>
                      {selectedSkills.size === updatableSkills.length ? 'Deselect All' : 'Select All'}
                    </HandButton>
                  </div>
                  <div className="space-y-2">
                    {updatableSkills.map((skill) => (
                      <div
                        key={skill.name}
                        className="flex items-center gap-3 px-3 py-2 border border-pencil/10"
                        style={{ borderRadius: wobbly.sm }}
                      >
                        <HandCheckbox label="" checked={selectedSkills.has(skill.name)} onChange={() => toggleSkill(skill.name)} />
                        <div className="flex-1 min-w-0">
                          <span className="text-pencil font-medium block" style={{ fontFamily: 'var(--font-hand)' }}>
                            {skill.name}
                          </span>
                          <span className="text-pencil-light text-sm truncate block">
                            {[skill.source, skill.installed_at && formatRelativeTime(skill.installed_at)]
                              .filter(Boolean)
                              .join(' · ')}
                          </span>
                        </div>
                        <Badge variant="info">Update available</Badge>
                      </div>
                    ))}
                  </div>
                </Card>
              )}
            </>
          )}

          {(upToDateRepos + upToDateSkills > 0) && (
            <Card variant="outlined">
              <p className="text-pencil-light text-sm" style={{ fontFamily: 'var(--font-hand)' }}>
                {upToDateRepos} repo(s) and {upToDateSkills} skill(s) already up to date.
              </p>
            </Card>
          )}
        </>
      )}
    </div>
  );
}

function StatusIcon({ status }: { status: ItemUpdateStatus['status'] }) {
  switch (status) {
    case 'pending':
      return <Circle size={16} className="text-muted-dark shrink-0" />;
    case 'in-progress':
      return <Loader2 size={16} className="text-blue animate-spin shrink-0" />;
    case 'success':
      return <CheckCircle size={16} className="text-success shrink-0" />;
    case 'error':
      return <XCircle size={16} className="text-danger shrink-0" />;
    case 'blocked':
      return <ShieldAlert size={16} className="text-warning shrink-0" />;
    case 'skipped':
      return <MinusCircle size={16} className="text-muted-dark shrink-0" />;
  }
}

function StatusBadge({ status }: { status: ItemUpdateStatus['status'] }) {
  switch (status) {
    case 'pending':
      return <Badge>Pending</Badge>;
    case 'in-progress':
      return <Badge variant="info">Updating</Badge>;
    case 'success':
      return <Badge variant="success">Updated</Badge>;
    case 'error':
      return <Badge variant="danger">Failed</Badge>;
    case 'blocked':
      return <Badge variant="warning">Blocked</Badge>;
    case 'skipped':
      return <Badge>Skipped</Badge>;
  }
}
