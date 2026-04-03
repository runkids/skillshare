import { useState, useEffect, useRef, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  RefreshCw, Check, ArrowUpCircle, Loader2, Search,
  Circle, CheckCircle, XCircle, MinusCircle, ShieldAlert, Zap, Trash2,
} from 'lucide-react';
import Card from '../components/Card';
import Button from '../components/Button';
import SplitButton from '../components/SplitButton';
import PageHeader from '../components/PageHeader';
import EmptyState from '../components/EmptyState';
import { Checkbox, Input } from '../components/Input';
import Badge from '../components/Badge';
import { queryKeys } from '../lib/queryKeys';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { CheckResult } from '../api/client';
import StreamProgressBar from '../components/StreamProgressBar';
import { radius } from '../design';

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

function isStaleError(message?: string): boolean {
  if (!message) return false;
  return message.includes('does not exist in repository') || message.includes('not found in repository');
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
  const { toast } = useToast();
  const [selectedRepos, setSelectedRepos] = useState<Set<string>>(new Set());
  const [selectedSkills, setSelectedSkills] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState('');
  const [phase, setPhase] = useState<UpdatePhase>('idle');
  const [itemStatuses, setItemStatuses] = useState<ItemUpdateStatus[]>([]);

  // Check state (SSE-based instead of useQuery)
  const [data, setData] = useState<CheckResult | null>(null);
  const [checking, setChecking] = useState(false);
  const [checkError, setCheckError] = useState<string | null>(null);
  const [checkProgress, setCheckProgress] = useState<{ checked: number; total: number } | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);
  const itemRefs = useRef<Record<string, HTMLDivElement | null>>({});

  // Clean up EventSource on unmount
  useEffect(() => {
    return () => { esRef.current?.close(); };
  }, []);

  // Auto-scroll to the item currently being updated
  useEffect(() => {
    if (phase !== 'updating') return;
    const inProgressItem = itemStatuses.find((s) => s.status === 'in-progress');
    if (inProgressItem) {
      const el = itemRefs.current[inProgressItem.name];
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }
  }, [itemStatuses, phase]);

  const runCheck = useCallback(() => {
    esRef.current?.close();
    setChecking(true);
    setCheckError(null);
    setCheckProgress(null);
    startTimeRef.current = Date.now();

    esRef.current = api.checkStream(
      // onDiscovering: show immediate feedback
      () => setCheckProgress({ checked: 0, total: 0 }),
      (total) => setCheckProgress({ checked: 0, total }),
      (checked) => setCheckProgress((p) => p ? { ...p, checked } : null),
      (result) => {
        setData(result);
        setChecking(false);
        setCheckProgress(null);
      },
      (err) => {
        setCheckError(err.message);
        setChecking(false);
        setCheckProgress(null);
      },
    );
  }, []);

  // Auto-run check on mount
  useEffect(() => { runCheck(); }, [runCheck]);

  if (checking) {
    return (
      <div className="space-y-5 animate-fade-in">
        <PageHeader
          icon={<ArrowUpCircle size={24} strokeWidth={2.5} />}
          title="Updates"
          subtitle="Check and apply updates for tracked repositories and installed skills."
        />
        <StreamProgressBar
          count={checkProgress?.checked ?? 0}
          total={checkProgress?.total ?? 0}
          startTime={startTimeRef.current}
          icon={RefreshCw}
          labelDiscovering="Discovering skills..."
          labelRunning="Checking for updates..."
          units="sources"
        />
      </div>
    );
  }

  if (checkError) {
    return (
      <div className="space-y-5 animate-fade-in">
        <PageHeader
          icon={<ArrowUpCircle size={24} strokeWidth={2.5} />}
          title="Updates"
          subtitle="Check and apply updates for tracked repositories and installed skills."
        />
        <Card variant="accent">
          <p className="text-danger">{checkError}</p>
          <div className="mt-3">
            <Button variant="secondary" size="sm" onClick={runCheck}>
              <RefreshCw size={16} />
              Retry
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  if (!data) return null;

  const updatableRepos = data.tracked_repos.filter((r) => r.status === 'behind');
  const updatableSkills = data.skills.filter((s) => s.status === 'update_available');
  const hasUpdates = updatableRepos.length > 0 || updatableSkills.length > 0;

  const lowerSearch = search.toLowerCase();
  const filteredRepos = lowerSearch ? updatableRepos.filter((r) => r.name.toLowerCase().includes(lowerSearch)) : updatableRepos;
  const filteredSkills = lowerSearch ? updatableSkills.filter((s) => s.name.toLowerCase().includes(lowerSearch)) : updatableSkills;

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

  const toggleAll = <T extends { name: string }>(
    items: T[],
    setter: React.Dispatch<React.SetStateAction<Set<string>>>,
  ) => {
    const nameSet = new Set(items.map((i) => i.name));
    setter((prev) =>
      [...nameSet].every((n) => prev.has(n))
        ? new Set([...prev].filter((n) => !nameSet.has(n)))
        : new Set([...prev, ...nameSet])
    );
  };

  const selectAllRepos = () => toggleAll(filteredRepos, setSelectedRepos);
  const selectAllSkills = () => toggleAll(filteredSkills, setSelectedSkills);

  const totalSelected = selectedRepos.size + selectedSkills.size;

  const handleUpdate = (opts?: { force?: boolean }) => {
    if (totalSelected === 0) return;

    // Build items list and initialize all as pending (visible immediately)
    const items: ItemUpdateStatus[] = [
      ...[...selectedRepos].map((name) => ({ name, isRepo: true, status: 'pending' as const })),
      ...[...selectedSkills].map((name) => ({ name, isRepo: false, status: 'pending' as const })),
    ];
    const names = items.map((it) => it.name);

    setItemStatuses(items);
    startTimeRef.current = Date.now();
    setPhase('updating');

    let resultIndex = 0;

    api.updateAllStream(
      // onStart: mark the first item as in-progress
      () => {
        setItemStatuses((prev) =>
          prev.map((s, idx) => idx === 0 ? { ...s, status: 'in-progress' } : s)
        );
      },
      // onResult: update current item with result, mark next as in-progress
      (item) => {
        const i = resultIndex;
        resultIndex++;
        setItemStatuses((prev) =>
          prev.map((s, idx) => {
            if (idx === i) {
              return {
                ...s,
                status: actionToStatus(item.action),
                message: item.message,
                auditRiskLabel: item.auditRiskLabel,
              };
            }
            if (idx === i + 1) {
              return { ...s, status: 'in-progress' };
            }
            return s;
          })
        );
      },
      // onDone
      () => {
        setPhase('done');
        invalidateSkillData();
      },
      // onError
      (err) => {
        toast(err.message, 'error');
        setPhase('done');
      },
      { names, force: opts?.force ?? false },
    );
  };

  const patchItem = (name: string, patch: Partial<ItemUpdateStatus>) =>
    setItemStatuses((prev) => prev.map((s) => s.name === name ? { ...s, ...patch } : s));

  const invalidateSkillData = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
  };

  const handleRetryForce = (name: string) => {
    patchItem(name, { status: 'in-progress', message: undefined });

    api.updateAllStream(
      () => {},
      (item) => {
        patchItem(name, {
          status: actionToStatus(item.action),
          message: item.message,
          auditRiskLabel: item.auditRiskLabel,
        });
      },
      () => invalidateSkillData(),
      (err) => patchItem(name, { status: 'error', message: err.message }),
      { names: [name], force: true },
    );
  };

  const handlePurge = async (name: string) => {
    patchItem(name, { status: 'in-progress', message: 'Purging...' });
    try {
      await api.batchUninstall({ names: [name], force: true });
      patchItem(name, { status: 'skipped', message: 'Purged' });
      invalidateSkillData();
    } catch (err) {
      patchItem(name, { status: 'error', message: (err as Error).message });
    }
  };

  const handleDone = () => {
    setPhase('idle');
    setItemStatuses([]);
    setSelectedRepos(new Set());
    setSelectedSkills(new Set());
    itemRefs.current = {};
    runCheck(); // Re-check after updates
  };

  const upToDateRepos = data.tracked_repos.filter((r) => r.status === 'up_to_date').length;
  const upToDateSkills = data.skills.filter((s) => s.status === 'up_to_date').length;

  // Summary counts for results
  const successCount = itemStatuses.filter((s) => s.status === 'success').length;
  const skippedCount = itemStatuses.filter((s) => s.status === 'skipped').length;
  const blockedCount = itemStatuses.filter((s) => s.status === 'blocked').length;
  const errorCount = itemStatuses.filter((s) => s.status === 'error').length;
  const completedCount = successCount + skippedCount + blockedCount + errorCount;

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<ArrowUpCircle size={24} strokeWidth={2.5} />}
        title="Updates"
        subtitle="Check and apply updates for tracked repositories and installed skills."
        actions={
          <>
            {phase === 'idle' && (
              <Button
                variant="ghost"
                size="sm"
                onClick={runCheck}
                loading={checking}
              >
                <RefreshCw size={16} />
                Check Now
              </Button>
            )}
            {phase === 'idle' && hasUpdates && totalSelected > 0 && (
              <SplitButton
                variant="primary"
                size="sm"
                onClick={() => handleUpdate()}
                dropdownAlign="right"
                items={[
                  {
                    label: 'Force Update',
                    icon: <Zap size={14} strokeWidth={2.5} />,
                    onClick: () => handleUpdate({ force: true }),
                    confirm: true,
                  },
                ]}
              >
                <ArrowUpCircle size={16} />
                Update Selected ({totalSelected})
              </SplitButton>
            )}
          </>
        }
      />

      {/* Update results panel */}
      {phase !== 'idle' && (
        <div className="space-y-4 animate-fade-in">
          {/* Sticky progress bar during update */}
          {phase === 'updating' && (
            <div className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 pt-2 pb-3">
              <StreamProgressBar
                count={completedCount}
                total={itemStatuses.length}
                startTime={startTimeRef.current}
                icon={ArrowUpCircle}
                iconClassName=""
                labelDiscovering="Starting update..."
                labelRunning="Updating..."
                units="items"
              />
            </div>
          )}

          {/* Done summary */}
          {phase === 'done' && (
            <Card tilt>
              <div className="flex items-center justify-between flex-wrap gap-3">
                <div className="flex items-center gap-2 flex-wrap">
                  {successCount > 0 && <Badge variant="success">{successCount} updated</Badge>}
                  {skippedCount > 0 && <Badge>{skippedCount} skipped</Badge>}
                  {blockedCount > 0 && <Badge variant="warning">{blockedCount} blocked</Badge>}
                  {errorCount > 0 && <Badge variant="danger">{errorCount} failed</Badge>}
                </div>
                <Button variant="ghost" size="sm" onClick={handleDone}>
                  Done
                </Button>
              </div>
            </Card>
          )}

          {/* Per-item status cards */}
          <div className="space-y-2">
            {itemStatuses.map((item, i) => (
              <div
                key={item.name}
                ref={(el) => { itemRefs.current[item.name] = el; }}
                className={`flex items-center gap-3 px-3 py-2 border transition-colors animate-fade-in ${
                  item.status === 'error'
                    ? 'border-danger/40 bg-danger-light/50'
                    : item.status === 'blocked'
                    ? 'border-warning/40 bg-warning-light/50'
                    : 'border-muted hover:bg-muted/30'
                }`}
                style={{
                  borderRadius: radius.sm,
                  animationDelay: `${i * 50}ms`,
                  animationFillMode: 'backwards',
                }}
              >
                <StatusIcon status={item.status} />
                <div className="flex-1 min-w-0">
                  <span className="text-pencil font-medium block">
                    {item.name}
                  </span>
                  {item.message && (
                    <span
                      className={`text-sm block ${
                        item.status === 'error'
                          ? 'text-danger font-medium whitespace-pre-wrap mt-1'
                          : item.status === 'blocked'
                          ? 'text-warning whitespace-pre-wrap mt-1'
                          : 'text-pencil-light truncate'
                      }`}
                    >
                      {item.message}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {item.auditRiskLabel && item.auditRiskLabel !== 'clean' && (
                    <Badge variant={item.auditRiskLabel === 'critical' || item.auditRiskLabel === 'high' ? 'danger' : 'warning'}>
                      <ShieldAlert size={12} className="mr-1" />
                      {item.auditRiskLabel}
                    </Badge>
                  )}
                  {item.status === 'error' && phase === 'done' && (
                    isStaleError(item.message) ? (
                      <Button variant="danger" size="sm" onClick={() => handlePurge(item.name)}>
                        <Trash2 size={14} />
                        Purge
                      </Button>
                    ) : (
                      <Button variant="danger" size="sm" onClick={() => handleRetryForce(item.name)}>
                        <RefreshCw size={14} />
                        Force Retry
                      </Button>
                    )
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
            <EmptyState
              icon={Check}
              title="Everything is up to date"
              description="All tracked repositories and skills are at their latest versions."
              action={
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={runCheck}
                  loading={checking}
                >
                  <RefreshCw size={16} />
                  Check Again
                </Button>
              }
            />
          ) : (
            <>
              {/* Sticky search bar */}
              <div className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 pt-2 pb-3 -mb-0">
                <div className="relative">
                  <Search
                    size={18}
                    strokeWidth={2.5}
                    className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
                  />
                  <Input
                    type="text"
                    placeholder="Filter updates..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="!pl-11"
                  />
                </div>
              </div>

              {filteredRepos.length > 0 && (
                <Card>
                  <div className="flex items-center justify-between mb-3">
                    <h2 className="text-lg font-bold text-pencil">
                      Tracked Repositories ({filteredRepos.length})
                    </h2>
                    <Button variant="ghost" size="sm" onClick={selectAllRepos}>
                      {filteredRepos.every((r) => selectedRepos.has(r.name)) ? 'Deselect All' : 'Select All'}
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {filteredRepos.map((repo) => (
                      <div
                        key={repo.name}
                        className="flex items-center gap-3 px-3 py-2 border border-muted hover:bg-muted/30 transition-colors cursor-pointer"
                        style={{ borderRadius: radius.sm }}
                        onClick={() => toggleRepo(repo.name)}
                      >
                        <span onClick={(e) => e.stopPropagation()}>
                          <Checkbox label="" checked={selectedRepos.has(repo.name)} onChange={() => toggleRepo(repo.name)} />
                        </span>
                        <div className="flex-1 min-w-0">
                          <span className="text-pencil font-medium block">
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

              {filteredRepos.length > 0 && filteredSkills.length > 0 && (
                <div className="border-t border-dashed border-pencil-light/30" />
              )}

              {filteredSkills.length > 0 && (
                <Card>
                  <div className="flex items-center justify-between mb-3">
                    <h2 className="text-lg font-bold text-pencil">
                      Skills with Updates ({filteredSkills.length})
                    </h2>
                    <Button variant="ghost" size="sm" onClick={selectAllSkills}>
                      {filteredSkills.every((s) => selectedSkills.has(s.name)) ? 'Deselect All' : 'Select All'}
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {filteredSkills.map((skill) => (
                      <div
                        key={skill.name}
                        className="flex items-center gap-3 px-3 py-2 border border-muted hover:bg-muted/30 transition-colors cursor-pointer"
                        style={{ borderRadius: radius.sm }}
                        onClick={() => toggleSkill(skill.name)}
                      >
                        <span onClick={(e) => e.stopPropagation()}>
                          <Checkbox label="" checked={selectedSkills.has(skill.name)} onChange={() => toggleSkill(skill.name)} />
                        </span>
                        <div className="flex-1 min-w-0">
                          <span className="text-pencil font-medium block">
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

              {filteredRepos.length === 0 && filteredSkills.length === 0 && search && (
                <EmptyState
                  icon={Search}
                  title="No matches"
                  description={`No updates matching "${search}".`}
                />
              )}
            </>
          )}

          {hasUpdates && (upToDateRepos + upToDateSkills > 0) && (
            <p className="text-sm text-pencil-light text-center">
              {upToDateRepos} repo(s) and {upToDateSkills} skill(s) already up to date.
            </p>
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
