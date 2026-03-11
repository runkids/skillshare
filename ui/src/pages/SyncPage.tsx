import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  RefreshCw,
  Eye,
  Zap,
  ChevronDown,
  ChevronRight,
  Check,
  Minus,
  CheckCircle,
  AlertCircle,
  Folder,
  ArrowRight,
  Target,
  FileText,
  Info,
} from 'lucide-react';
import { Virtuoso } from 'react-virtuoso';
import Card from '../components/Card';
import Badge from '../components/Badge';
import HandButton from '../components/HandButton';
import { useToast } from '../components/Toast';
import { api, type SyncResult, type DiffTarget } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import StreamProgressBar from '../components/StreamProgressBar';
import { radius, shadows } from '../design';

export default function SyncPage() {
  const queryClient = useQueryClient();
  const [dryRun, setDryRun] = useState(false);
  const [force, setForce] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [results, setResults] = useState<SyncResult[] | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const { toast } = useToast();
  const toastRef = useRef(toast);
  useEffect(() => { toastRef.current = toast; });

  // Diff state (SSE-based)
  const [diffData, setDiffData] = useState<DiffTarget[] | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffProgress, setDiffProgress] = useState<{ checked: number; total: number } | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);

  useEffect(() => {
    return () => { esRef.current?.close(); };
  }, []);

  const runDiff = useCallback(() => {
    esRef.current?.close();
    setDiffLoading(true);
    setDiffProgress(null);
    startTimeRef.current = Date.now();

    esRef.current = api.diffStream(
      () => setDiffProgress({ checked: 0, total: 0 }),
      (total) => setDiffProgress({ checked: 0, total }),
      (_diff, checked) => setDiffProgress((p) => p ? { ...p, checked } : null),
      (data) => {
        setDiffData(data.diffs);
        setDiffLoading(false);
        setDiffProgress(null);
      },
      (err) => {
        toastRef.current(err.message, 'error');
        setDiffLoading(false);
        setDiffProgress(null);
      },
    );
  }, []);

  useEffect(() => { runDiff(); }, [runDiff]);

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
      runDiff(); // Re-check diff after sync
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSyncing(false);
    }
  };

  // Calculate diff summary
  const diffs = diffData ?? [];
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
    <div className="animate-fade-in">
      {/* Page header */}
      <div className="mb-8">
        <h2
          className="text-3xl md:text-4xl font-bold text-pencil mb-2"
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
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <Folder size={18} strokeWidth={2.5} className="text-warning" />
          <span className="text-base font-medium">
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
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <RefreshCw
            size={18}
            strokeWidth={2.5}
            className={`text-blue ${syncing ? 'animate-spin' : ''}`}
          />
          <span className="text-base font-medium">
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
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <Target size={18} strokeWidth={2.5} className="text-success" />
          <span className="text-base font-medium">
            Targets ({diffs.length})
          </span>
        </div>
      </div>

      {/* Sync control area */}
      <Card className="mb-6 text-center">
        <div className="flex flex-col items-center gap-4">
          {/* Status indicator */}
          {diffLoading ? (
            <p className="text-pencil-light text-base">Checking status...</p>
          ) : syncActions > 0 ? (
            <div className="flex flex-wrap items-center justify-center gap-3">
              <span className="text-base text-pencil">
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
                <span className="text-base font-medium">
                  All targets are in sync!
                </span>
              </div>
              <Badge variant="default">{pendingLocal} local only</Badge>
            </div>
          ) : (
            <div className="flex items-center gap-2 text-success">
              <CheckCircle size={18} strokeWidth={2.5} />
              <span className="text-base font-medium">
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
          >
            {showAdvanced ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
            Advanced options
          </button>

          {/* Advanced options */}
          {showAdvanced && (
            <div className="flex items-center gap-6 animate-fade-in">
              <label className="flex items-center gap-2 text-base cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={dryRun}
                  onChange={(e) => setDryRun(e.target.checked)}
                  className="w-4 h-4 accent-blue"
                />
                <Eye size={16} strokeWidth={2.5} className="text-blue" />
                <span>Dry Run</span>
              </label>

              <label className="flex items-center gap-2 text-base cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={force}
                  onChange={(e) => setForce(e.target.checked)}
                  className="w-4 h-4 accent-accent"
                />
                <Zap size={16} strokeWidth={2.5} className="text-accent" />
                <span>Force</span>
              </label>
            </div>
          )}
        </div>
      </Card>

      {/* Sync results */}
      {results && results.length > 0 && (
        <div className="space-y-3 mt-6 mb-8">
          <h2
            className="text-lg font-bold text-pencil"
          >
            {dryRun ? 'Preview Results' : 'Results'}
          </h2>
          <SyncResults results={results} />
        </div>
      )}

      {/* Diff preview */}
      <div>
        <h3
          className="text-xl font-bold text-pencil mb-4"
        >
          Current Diff
        </h3>
        {diffLoading && diffProgress && (
          <StreamProgressBar
            count={diffProgress.checked}
            total={diffProgress.total}
            startTime={startTimeRef.current}
            icon={RefreshCw}
            labelDiscovering="Discovering skills..."
            labelRunning="Computing diff..."
            units="targets"
          />
        )}
        {!diffLoading && diffData && <DiffView diffs={diffData} />}
      </div>
    </div>
  );
}

/** Sync results visualization — compact per-target cards with badge counts */
function SyncResults({ results }: { results: SyncResult[] }) {
  if (results.length === 0) {
    return (
      <Card variant="outlined">
        <p className="text-pencil-light text-center py-4">No results to show.</p>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {results.map((r, i) => {
        const linked = r.linked?.length ?? 0;
        const updated = r.updated?.length ?? 0;
        const skipped = r.skipped?.length ?? 0;
        const pruned = r.pruned?.length ?? 0;
        const hasChanges = linked > 0 || updated > 0;

        return (
          <Card
            key={r.target}
            style={{ animation: `fadeInUp 0.3s ease-out ${i * 100}ms both` }}
          >
            <div className="flex items-center gap-3">
              {hasChanges ? (
                <Check size={18} className="text-success shrink-0" />
              ) : (
                <Minus size={18} className="text-pencil-light shrink-0" />
              )}
              <span className="text-pencil font-medium flex-1">
                {r.target}
              </span>
              <div className="flex gap-2 flex-wrap">
                {linked > 0 && <Badge variant="success">{linked} linked</Badge>}
                {updated > 0 && <Badge variant="info">{updated} updated</Badge>}
                {skipped > 0 && <Badge variant="default">{skipped} skipped</Badge>}
                {pruned > 0 && <Badge variant="warning">{pruned} pruned</Badge>}
              </div>
            </div>
          </Card>
        );
      })}
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

/** Max items before switching from flat list to virtualized scroll */
const VIRTUALIZE_THRESHOLD = 100;
/** Height of the virtualized container */
const VIRTUOSO_HEIGHT = 400;

function DiffTargetCard({ diff }: { diff: DiffTarget }) {
  const items = diff.items ?? [];
  const [expanded, setExpanded] = useState(items.length <= VIRTUALIZE_THRESHOLD);
  const localOnly = useMemo(() => items.filter((i) => i.action === 'local'), [items]);
  const syncItems = useMemo(() => items.filter((i) => i.action !== 'local'), [items]);
  const inSync = items.length === 0;
  const onlyLocal = syncItems.length === 0 && localOnly.length > 0;

  const hasSyncable = syncItems.some((i) => ['link', 'update', 'skip'].includes(i.action));
  const hasLocal = localOnly.length > 0;
  const useVirtualized = items.length > VIRTUALIZE_THRESHOLD;

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
        <div className="mt-3 pl-8 animate-fade-in">
          {useVirtualized ? (
            <Virtuoso
              style={{ height: VIRTUOSO_HEIGHT }}
              totalCount={items.length}
              overscan={200}
              itemContent={(i) => <DiffItemRow item={items[i]} />}
            />
          ) : (
            <div className="space-y-1.5">
              {items.map((item, i) => (
                <DiffItemRow key={i} item={item} />
              ))}
            </div>
          )}

          {/* Action hints */}
          {(hasSyncable || hasLocal) && (
            <div className="mt-3 pt-2 border-t border-dashed border-muted-dark space-y-1">
              {hasSyncable && (
                <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                  <Info size={12} className="shrink-0" />
                  <span>
                    Run sync (or sync --force) to fix pending items
                  </span>
                </div>
              )}
              {hasLocal && (
                <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                  <FileText size={12} className="shrink-0" />
                  <span>
                    Use collect to import local-only skills to source
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {expanded && inSync && (
        <p className="mt-2 pl-8 text-base text-pencil-light">
          Everything looks good! No changes needed.
        </p>
      )}
    </Card>
  );
}

function DiffItemRow({ item }: { item: { action: string; skill: string; reason?: string } }) {
  return (
    <div className="flex items-center gap-2 text-base py-0.5">
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
  );
}

