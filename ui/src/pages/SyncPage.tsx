import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  RefreshCw,
  Eye,
  Zap,
  ChevronDown,
  ChevronRight,
  Link as LinkIcon,
  ArrowUpCircle,
  SkipForward,
  Scissors,
  CheckCircle,
  AlertCircle,
  Folder,
  ArrowRight,
  Target,
  FileText,
  Info,
} from 'lucide-react';
import Card from '../components/Card';
import Badge from '../components/Badge';
import HandButton from '../components/HandButton';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type SyncResult, type DiffTarget } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { wobbly, shadows } from '../design';

export default function SyncPage() {
  const queryClient = useQueryClient();
  const [dryRun, setDryRun] = useState(false);
  const [force, setForce] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [results, setResults] = useState<SyncResult[] | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const { toast } = useToast();

  const diffQuery = useQuery({
    queryKey: queryKeys.diff(),
    queryFn: () => api.diff(),
    staleTime: staleTimes.diff,
  });

  const handleSync = async () => {
    setSyncing(true);
    try {
      const res = await api.sync({ dryRun, force });
      setResults(res.results);
      if (dryRun) {
        toast('Dry run complete -- no changes were made.', 'info');
      } else {
        const totalLinked = res.results.reduce((sum, r) => sum + (r.linked?.length ?? 0), 0);
        const totalUpdated = res.results.reduce((sum, r) => sum + (r.updated?.length ?? 0), 0);
        toast(
          `Sync complete! ${totalLinked} linked, ${totalUpdated} updated across ${res.results.length} target(s).`,
          'success',
        );
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSyncing(false);
    }
  };

  // Calculate diff summary
  const diffs = diffQuery.data?.diffs ?? [];
  const totalActions = diffs.reduce((sum, d) => sum + (d.items?.length ?? 0), 0);
  const pendingLinks = diffs.reduce(
    (sum, d) => sum + (d.items?.filter((i) => i.action === 'link').length ?? 0),
    0,
  );
  const pendingUpdates = diffs.reduce(
    (sum, d) => sum + (d.items?.filter((i) => i.action === 'update').length ?? 0),
    0,
  );
  const pendingPrunes = diffs.reduce(
    (sum, d) => sum + (d.items?.filter((i) => i.action === 'prune').length ?? 0),
    0,
  );
  const pendingSkips = diffs.reduce(
    (sum, d) => sum + (d.items?.filter((i) => i.action === 'skip').length ?? 0),
    0,
  );
  const pendingLocal = diffs.reduce(
    (sum, d) => sum + (d.items?.filter((i) => i.action === 'local').length ?? 0),
    0,
  );
  const syncActions = totalActions - pendingLocal;

  return (
    <div className="animate-sketch-in">
      {/* Page header */}
      <div className="mb-8">
        <h2
          className="text-3xl md:text-4xl font-bold text-pencil mb-2"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Sync
        </h2>
        <p className="text-pencil-light text-base">
          Push your skills from source to all configured targets
        </p>
      </div>

      {/* Visual Pipeline */}
      <div className="hidden md:flex items-center justify-center gap-4 mb-8">
        <div
          className="flex items-center gap-2 px-4 py-2 bg-postit border-2 border-pencil"
          style={{ borderRadius: wobbly.sm, boxShadow: shadows.sm }}
        >
          <Folder size={18} strokeWidth={2.5} className="text-warning" />
          <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
            Source
          </span>
        </div>

        <div className="flex items-center gap-1">
          <svg width="60" height="20" viewBox="0 0 60 20" className="text-pencil-light">
            <path
              d="M0 10 Q15 4 30 10 Q45 16 60 10"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeDasharray="4 4"
            />
          </svg>
        </div>

        <div
          className="flex items-center gap-2 px-4 py-2 bg-info-light border-2 border-pencil"
          style={{ borderRadius: wobbly.sm, boxShadow: shadows.sm }}
        >
          <RefreshCw
            size={18}
            strokeWidth={2.5}
            className={`text-blue ${syncing ? 'animate-spin' : ''}`}
          />
          <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
            Sync Engine
          </span>
        </div>

        <div className="flex items-center gap-1">
          <svg width="60" height="20" viewBox="0 0 60 20" className="text-pencil-light">
            <path
              d="M0 10 Q15 4 30 10 Q45 16 60 10"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeDasharray="4 4"
            />
          </svg>
        </div>

        <div
          className="flex items-center gap-2 px-4 py-2 bg-success-light border-2 border-pencil"
          style={{ borderRadius: wobbly.sm, boxShadow: shadows.sm }}
        >
          <Target size={18} strokeWidth={2.5} className="text-success" />
          <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
            Targets ({diffs.length})
          </span>
        </div>
      </div>

      {/* Sync control area */}
      <Card variant="postit" className="mb-6 text-center">
        <div className="flex flex-col items-center gap-4">
          {/* Status indicator */}
          {diffQuery.isPending ? (
            <p className="text-pencil-light text-base">Checking status...</p>
          ) : syncActions > 0 ? (
            <div className="flex flex-wrap items-center justify-center gap-3">
              <span className="text-base text-pencil" style={{ fontFamily: 'var(--font-hand)' }}>
                Pending changes:
              </span>
              {pendingLinks > 0 && <Badge variant="success">{pendingLinks} to link</Badge>}
              {pendingUpdates > 0 && <Badge variant="info">{pendingUpdates} to update</Badge>}
              {pendingSkips > 0 && <Badge variant="warning">{pendingSkips} skipped</Badge>}
              {pendingPrunes > 0 && <Badge variant="danger">{pendingPrunes} to prune</Badge>}
              {pendingLocal > 0 && <Badge variant="default">{pendingLocal} local only</Badge>}
            </div>
          ) : pendingLocal > 0 ? (
            <div className="flex flex-wrap items-center justify-center gap-3">
              <div className="flex items-center gap-2 text-success">
                <CheckCircle size={18} strokeWidth={2.5} />
                <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
                  All targets are in sync!
                </span>
              </div>
              <Badge variant="default">{pendingLocal} local only</Badge>
            </div>
          ) : (
            <div className="flex items-center gap-2 text-success">
              <CheckCircle size={18} strokeWidth={2.5} />
              <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
                All targets are in sync!
              </span>
            </div>
          )}

          {/* Big sync button */}
          <HandButton
            onClick={handleSync}
            disabled={syncing}
            variant="primary"
            size="lg"
            className="min-w-[200px]"
          >
            <RefreshCw
              size={22}
              strokeWidth={2.5}
              className={syncing ? 'animate-spin' : ''}
            />
            {syncing ? 'Syncing...' : dryRun ? 'Preview Sync' : 'Sync Now'}
          </HandButton>

          {/* Advanced options toggle */}
          <button
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="flex items-center gap-1 text-base text-pencil-light hover:text-pencil transition-colors cursor-pointer"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            {showAdvanced ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
            Advanced options
          </button>

          {/* Advanced options */}
          {showAdvanced && (
            <div className="flex items-center gap-6 animate-sketch-in">
              <label className="flex items-center gap-2 text-base cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={dryRun}
                  onChange={(e) => setDryRun(e.target.checked)}
                  className="w-4 h-4 accent-blue"
                />
                <Eye size={16} strokeWidth={2.5} className="text-blue" />
                <span style={{ fontFamily: 'var(--font-hand)' }}>Dry Run</span>
              </label>

              <label className="flex items-center gap-2 text-base cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={force}
                  onChange={(e) => setForce(e.target.checked)}
                  className="w-4 h-4 accent-accent"
                />
                <Zap size={16} strokeWidth={2.5} className="text-accent" />
                <span style={{ fontFamily: 'var(--font-hand)' }}>Force</span>
              </label>
            </div>
          )}
        </div>
      </Card>

      {/* Sync results */}
      {results && (
        <div className="mb-8 animate-sketch-in">
          <h3
            className="text-xl font-bold text-pencil mb-4"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            {dryRun ? 'Preview Results' : 'Sync Results'}
          </h3>
          <SyncResults results={results} />
        </div>
      )}

      {/* Diff preview */}
      <div>
        <h3
          className="text-xl font-bold text-pencil mb-4"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Current Diff
        </h3>
        {diffQuery.isPending && <PageSkeleton />}
        {diffQuery.error && (
          <Card variant="accent">
            <div className="flex items-center gap-2 text-danger">
              <AlertCircle size={18} strokeWidth={2.5} />
              <span>{diffQuery.error.message}</span>
            </div>
          </Card>
        )}
        {diffQuery.data && <DiffView diffs={diffQuery.data.diffs} />}
      </div>
    </div>
  );
}

/** Sync results visualization */
function SyncResults({ results }: { results: SyncResult[] }) {
  if (results.length === 0) {
    return (
      <Card variant="outlined">
        <p className="text-pencil-light text-center py-4">No results to show.</p>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {results.map((r) => {
        const linked = r.linked?.length ?? 0;
        const updated = r.updated?.length ?? 0;
        const skipped = r.skipped?.length ?? 0;
        const pruned = r.pruned?.length ?? 0;
        const total = linked + updated + skipped + pruned;

        return (
          <Card key={r.target}>
            <div className="flex items-center gap-3 mb-3">
              <Target size={18} strokeWidth={2.5} className="text-success" />
              <h4
                className="font-bold text-pencil"
                style={{ fontFamily: 'var(--font-heading)' }}
              >
                {r.target}
              </h4>
              <Badge variant={total > 0 ? 'info' : 'success'}>
                {total > 0 ? `${total} changes` : 'no changes'}
              </Badge>
            </div>

            {total > 0 && (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                <ResultStat label="Linked" count={linked} icon={LinkIcon} variant="success" />
                <ResultStat label="Updated" count={updated} icon={ArrowUpCircle} variant="info" />
                <ResultStat label="Skipped" count={skipped} icon={SkipForward} variant="warning" />
                <ResultStat label="Pruned" count={pruned} icon={Scissors} variant="danger" />
              </div>
            )}

            {/* Expandable details */}
            {total > 0 && <ResultDetails result={r} />}
          </Card>
        );
      })}
    </div>
  );
}

function ResultStat({
  label,
  count,
  icon: Icon,
  variant,
}: {
  label: string;
  count: number;
  icon: React.ComponentType<{ size?: number; strokeWidth?: number; className?: string }>;
  variant: 'success' | 'info' | 'warning' | 'danger';
}) {
  const bgMap = {
    success: 'bg-success-light',
    info: 'bg-info-light',
    warning: 'bg-warning-light',
    danger: 'bg-danger-light',
  };
  const colorMap = {
    success: 'text-success',
    info: 'text-blue',
    warning: 'text-warning',
    danger: 'text-danger',
  };

  return (
    <div
      className={`flex items-center gap-2 px-3 py-2 border border-dashed ${count > 0 ? bgMap[variant] : 'bg-muted/30'}`}
      style={{ borderRadius: wobbly.sm }}
    >
      <Icon
        size={16}
        strokeWidth={2.5}
        className={count > 0 ? colorMap[variant] : 'text-muted-dark'}
      />
      <div>
        <p
          className={`text-lg font-bold leading-none ${count > 0 ? colorMap[variant] : 'text-muted-dark'}`}
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          {count}
        </p>
        <p className="text-sm text-pencil-light">{label}</p>
      </div>
    </div>
  );
}

function ResultDetails({ result }: { result: SyncResult }) {
  const [open, setOpen] = useState(false);

  const allItems = [
    ...(result.linked ?? []).map((s) => ({ skill: s, action: 'linked' })),
    ...(result.updated ?? []).map((s) => ({ skill: s, action: 'updated' })),
    ...(result.skipped ?? []).map((s) => ({ skill: s, action: 'skipped' })),
    ...(result.pruned ?? []).map((s) => ({ skill: s, action: 'pruned' })),
  ];

  if (allItems.length === 0) return null;

  return (
    <div className="mt-3">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 text-sm text-pencil-light hover:text-pencil cursor-pointer transition-colors"
        style={{ fontFamily: 'var(--font-hand)' }}
      >
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        {open ? 'Hide details' : 'Show details'}
      </button>
      {open && (
        <div className="mt-2 pl-4 border-l-2 border-dashed border-muted-dark space-y-1 animate-sketch-in">
          {allItems.map((item, i) => (
            <div key={i} className="flex items-center gap-2 text-base">
              <ActionBadge action={item.action} />
              <span
                className="text-pencil-light truncate"
                style={{ fontFamily: "'Courier New', monospace", fontSize: '0.875rem' }}
              >
                {item.skill}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ActionBadge({ action }: { action: string }) {
  const map: Record<string, { variant: 'success' | 'info' | 'warning' | 'danger' | 'default'; label: string }> = {
    link: { variant: 'success', label: 'link' },
    linked: { variant: 'success', label: 'linked' },
    update: { variant: 'info', label: 'update' },
    updated: { variant: 'info', label: 'updated' },
    skip: { variant: 'warning', label: 'skip' },
    skipped: { variant: 'warning', label: 'skipped' },
    prune: { variant: 'danger', label: 'prune' },
    pruned: { variant: 'danger', label: 'pruned' },
    local: { variant: 'default', label: 'local' },
  };
  const entry = map[action] ?? { variant: 'default' as const, label: action };
  return <Badge variant={entry.variant}>{entry.label}</Badge>;
}

/** Diff preview with expandable targets */
function DiffView({ diffs: rawDiffs }: { diffs: DiffTarget[] }) {
  const diffs = rawDiffs ?? [];

  if (diffs.length === 0) {
    return (
      <Card variant="outlined">
        <div className="flex items-center justify-center gap-2 py-4 text-pencil-light">
          <AlertCircle size={18} strokeWidth={2} />
          <span>No targets configured.</span>
        </div>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {diffs.map((d) => (
        <DiffTargetCard key={d.target} diff={d} />
      ))}
    </div>
  );
}

function DiffTargetCard({ diff }: { diff: DiffTarget }) {
  const [expanded, setExpanded] = useState(true);
  const items = diff.items ?? [];
  const localOnly = items.filter((i) => i.action === 'local');
  const syncItems = items.filter((i) => i.action !== 'local');
  const inSync = items.length === 0;
  const onlyLocal = syncItems.length === 0 && localOnly.length > 0;

  const hasSyncable = syncItems.some((i) => ['link', 'update', 'skip'].includes(i.action));
  const hasLocal = localOnly.length > 0;

  return (
    <Card>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 cursor-pointer"
      >
        {expanded ? (
          <ChevronDown size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
        ) : (
          <ChevronRight size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
        )}
        <Target size={16} strokeWidth={2.5} className="text-success shrink-0" />
        <h4
          className="font-bold text-pencil text-left flex-1"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          {diff.target}
        </h4>
        {inSync ? (
          <Badge variant="success">in sync</Badge>
        ) : onlyLocal ? (
          <Badge variant="default">{localOnly.length} local only</Badge>
        ) : (
          <div className="flex items-center gap-2">
            <Badge variant="info">{syncItems.length} pending</Badge>
            {localOnly.length > 0 && <Badge variant="default">{localOnly.length} local</Badge>}
          </div>
        )}
      </button>

      {expanded && items.length > 0 && (
        <div className="mt-3 pl-8 space-y-1.5 animate-sketch-in">
          {items.map((item, i) => (
            <div key={i} className="flex items-center gap-2 text-base">
              <ActionBadge action={item.action} />
              <ArrowRight size={12} className="text-muted-dark shrink-0" />
              <span
                className="text-pencil-light truncate"
                style={{ fontFamily: "'Courier New', monospace", fontSize: '0.875rem' }}
              >
                {item.skill}
              </span>
              {item.reason && (
                <span className="text-pencil-light/60 text-xs shrink-0">({item.reason})</span>
              )}
            </div>
          ))}

          {/* Action hints */}
          {(hasSyncable || hasLocal) && (
            <div className="mt-3 pt-2 border-t border-dashed border-muted-dark space-y-1">
              {hasSyncable && (
                <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                  <Info size={12} className="shrink-0" />
                  <span style={{ fontFamily: 'var(--font-hand)' }}>
                    Run sync (or sync --force) to fix pending items
                  </span>
                </div>
              )}
              {hasLocal && (
                <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                  <FileText size={12} className="shrink-0" />
                  <span style={{ fontFamily: 'var(--font-hand)' }}>
                    Use collect to import local-only skills to source
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {expanded && inSync && (
        <p className="mt-2 pl-8 text-base text-pencil-light" style={{ fontFamily: 'var(--font-hand)' }}>
          Everything looks good! No changes needed.
        </p>
      )}
    </Card>
  );
}
