import { useState, useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  ArrowDownToLine,
  Target,
  Folder,
  Zap,
  ChevronDown,
  ChevronRight,
  CheckCircle,
  AlertCircle,
  RefreshCw,
  SkipForward,
  XCircle,
} from 'lucide-react';
import Card from '../components/Card';
import Badge from '../components/Badge';
import HandButton from '../components/HandButton';
import EmptyState from '../components/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type CollectScanTarget, type CollectResult } from '../api/client';
import { wobbly, shadows } from '../design';

type Phase = 'idle' | 'scanning' | 'scanned' | 'collecting' | 'done';

export default function CollectPage() {
  const [searchParams] = useSearchParams();
  const presetTarget = searchParams.get('target') ?? undefined;

  const [phase, setPhase] = useState<Phase>('idle');
  const [force, setForce] = useState(false);
  const [scanTargets, setScanTargets] = useState<CollectScanTarget[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [result, setResult] = useState<CollectResult | null>(null);
  const [confirming, setConfirming] = useState(false);
  const { toast } = useToast();

  // Auto-scan when target query param is present
  useEffect(() => {
    if (presetTarget) {
      handleScan(presetTarget);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [presetTarget]);

  const handleScan = async (targetFilter?: string) => {
    setPhase('scanning');
    setResult(null);
    try {
      const res = await api.collectScan(targetFilter);
      setScanTargets(res.targets);
      setTotalCount(res.totalCount);
      // Auto-select all
      const allKeys = new Set<string>();
      for (const t of res.targets) {
        for (const sk of t.skills) {
          allKeys.add(`${t.targetName}/${sk.name}`);
        }
      }
      setSelected(allKeys);
      setPhase('scanned');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setPhase('idle');
    }
  };

  const handleCollect = async () => {
    setPhase('collecting');
    try {
      const skills = Array.from(selected).map((key) => {
        const [targetName, ...rest] = key.split('/');
        return { name: rest.join('/'), targetName };
      });
      const res = await api.collect({ skills, force });
      setResult(res);
      const pulledCount = res.pulled?.length ?? 0;
      const skippedCount = res.skipped?.length ?? 0;
      const failedCount = Object.keys(res.failed ?? {}).length;
      toast(
        `Collect complete! ${pulledCount} pulled, ${skippedCount} skipped, ${failedCount} failed.`,
        pulledCount > 0 ? 'success' : 'info',
      );
      setPhase('done');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setPhase('scanned');
    }
  };

  const toggleSkill = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const toggleAll = (selectAll: boolean) => {
    if (selectAll) {
      const allKeys = new Set<string>();
      for (const t of scanTargets) {
        for (const sk of t.skills) {
          allKeys.add(`${t.targetName}/${sk.name}`);
        }
      }
      setSelected(allKeys);
    } else {
      setSelected(new Set());
    }
  };

  return (
    <div className="animate-sketch-in">
      {/* Page header */}
      <div className="mb-8">
        <h2
          className="text-3xl md:text-4xl font-bold text-pencil mb-2"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Collect
        </h2>
        <p className="text-pencil-light text-base">
          Pull local skills from targets back to source
        </p>
      </div>

      {/* Visual Pipeline (reverse direction) */}
      <div className="hidden md:flex items-center justify-center gap-4 mb-8">
        <div
          className="flex items-center gap-2 px-4 py-2 bg-success-light border-2 border-pencil"
          style={{ borderRadius: wobbly.sm, boxShadow: shadows.sm }}
        >
          <Target size={18} strokeWidth={2.5} className="text-success" />
          <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
            Targets
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
          <ArrowDownToLine
            size={18}
            strokeWidth={2.5}
            className={`text-blue ${phase === 'collecting' ? 'animate-bounce' : ''}`}
          />
          <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
            Collect Engine
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
          className="flex items-center gap-2 px-4 py-2 bg-postit border-2 border-pencil"
          style={{ borderRadius: wobbly.sm, boxShadow: shadows.sm }}
        >
          <Folder size={18} strokeWidth={2.5} className="text-warning" />
          <span className="text-base font-medium" style={{ fontFamily: 'var(--font-hand)' }}>
            Source
          </span>
        </div>
      </div>

      {/* Scan control area */}
      <Card variant="postit" className="mb-6 text-center">
        <div className="flex flex-col items-center gap-4">
          <HandButton
            onClick={() => handleScan(presetTarget)}
            disabled={phase === 'scanning' || phase === 'collecting'}
            variant="primary"
            size="lg"
            className="min-w-[200px]"
          >
            {phase === 'scanning' ? (
              <>
                <RefreshCw size={22} strokeWidth={2.5} className="animate-spin" />
                Scanning...
              </>
            ) : (
              <>
                <ArrowDownToLine size={22} strokeWidth={2.5} />
                {phase === 'idle' ? 'Scan for Local Skills' : 'Re-scan'}
              </>
            )}
          </HandButton>

          {presetTarget && (
            <p className="text-sm text-pencil-light" style={{ fontFamily: 'var(--font-hand)' }}>
              Filtering: <Badge variant="info">{presetTarget}</Badge>
            </p>
          )}

          {/* Force toggle */}
          {(phase === 'scanned' || phase === 'done') && (
            <label className="flex items-center gap-2 text-base cursor-pointer select-none">
              <input
                type="checkbox"
                checked={force}
                onChange={(e) => setForce(e.target.checked)}
                className="w-4 h-4 accent-accent"
              />
              <Zap size={16} strokeWidth={2.5} className="text-accent" />
              <span style={{ fontFamily: 'var(--font-hand)' }}>
                Force (overwrite existing in source)
              </span>
            </label>
          )}
        </div>
      </Card>

      {/* Loading state */}
      {phase === 'scanning' && <PageSkeleton />}

      {/* Scan results */}
      {(phase === 'scanned' || phase === 'collecting' || phase === 'done') && (
        <>
          {totalCount === 0 ? (
            <EmptyState
              icon={CheckCircle}
              title="No local skills found"
              description="All skills in your targets are synced from source. Nothing to collect."
            />
          ) : (
            <div className="mb-6">
              {/* Select all / none controls */}
              <div className="flex items-center justify-between mb-4">
                <h3
                  className="text-xl font-bold text-pencil"
                  style={{ fontFamily: 'var(--font-heading)' }}
                >
                  Found {totalCount} local skill{totalCount !== 1 ? 's' : ''}
                </h3>
                <div className="flex gap-2">
                  <HandButton
                    onClick={() => toggleAll(true)}
                    variant="ghost"
                    size="sm"
                    disabled={phase === 'collecting'}
                  >
                    Select All
                  </HandButton>
                  <HandButton
                    onClick={() => toggleAll(false)}
                    variant="ghost"
                    size="sm"
                    disabled={phase === 'collecting'}
                  >
                    Select None
                  </HandButton>
                </div>
              </div>

              {/* Per-target expandable cards */}
              <div className="space-y-4">
                {scanTargets.map((t) => (
                  <ScanTargetCard
                    key={t.targetName}
                    target={t}
                    selected={selected}
                    onToggle={toggleSkill}
                    disabled={phase === 'collecting'}
                  />
                ))}
              </div>

              {/* Collect button */}
              {phase !== 'done' && (
                <div className="mt-6 text-center">
                  <HandButton
                    onClick={() => setConfirming(true)}
                    disabled={selected.size === 0 || phase === 'collecting'}
                    variant="primary"
                    size="lg"
                    className="min-w-[200px]"
                  >
                    {phase === 'collecting' ? (
                      <>
                        <RefreshCw size={22} strokeWidth={2.5} className="animate-spin" />
                        Collecting...
                      </>
                    ) : (
                      <>
                        <ArrowDownToLine size={22} strokeWidth={2.5} />
                        Collect {selected.size} Skill{selected.size !== 1 ? 's' : ''}
                      </>
                    )}
                  </HandButton>
                </div>
              )}
            </div>
          )}
        </>
      )}

      {/* Collect results */}
      {phase === 'done' && result && <CollectResults result={result} />}

      {/* Post-collect suggestion */}
      {phase === 'done' && result && (result.pulled?.length ?? 0) > 0 && (
        <Card variant="accent" className="mt-6 text-center animate-sketch-in">
          <div className="flex flex-col items-center gap-3">
            <p
              className="text-base text-pencil"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              Skills collected to source! Run Sync to distribute them to all targets.
            </p>
            <Link to="/sync">
              <HandButton variant="primary" size="sm">
                <RefreshCw size={16} strokeWidth={2.5} />
                Go to Sync
              </HandButton>
            </Link>
          </div>
        </Card>
      )}

      {/* Confirm collect dialog */}
      <ConfirmDialog
        open={confirming}
        title="Confirm Collect"
        message={
          <div className="text-left">
            <p className="mb-2">
              Copy {selected.size} skill{selected.size !== 1 ? 's' : ''} to source{force ? ' (force overwrite)' : ''}?
            </p>
            <ul className="list-none space-y-1 max-h-40 overflow-y-auto">
              {Array.from(selected).map((key) => {
                const [targetName, ...rest] = key.split('/');
                return (
                  <li key={key} className="flex items-center gap-2 text-sm">
                    <Folder size={12} strokeWidth={2.5} className="text-warning shrink-0" />
                    <span style={{ fontFamily: "'Courier New', monospace" }}>{rest.join('/')}</span>
                    <span className="text-pencil-light">← {targetName}</span>
                  </li>
                );
              })}
            </ul>
          </div>
        }
        confirmText={`Collect ${selected.size} Skill${selected.size !== 1 ? 's' : ''}`}
        onConfirm={() => {
          setConfirming(false);
          handleCollect();
        }}
        onCancel={() => setConfirming(false)}
      />
    </div>
  );
}

/** Per-target scan result card with expandable skill list */
function ScanTargetCard({
  target,
  selected,
  onToggle,
  disabled,
}: {
  target: CollectScanTarget;
  selected: Set<string>;
  onToggle: (key: string) => void;
  disabled: boolean;
}) {
  const [expanded, setExpanded] = useState(true);
  const skills = target.skills ?? [];
  const selectedCount = skills.filter((sk) => selected.has(`${target.targetName}/${sk.name}`)).length;

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
          {target.targetName}
        </h4>
        <Badge variant={selectedCount > 0 ? 'info' : 'default'}>
          {selectedCount}/{skills.length} selected
        </Badge>
      </button>

      {expanded && skills.length > 0 && (
        <div className="mt-3 pl-8 space-y-2 animate-sketch-in">
          {skills.map((sk) => {
            const key = `${target.targetName}/${sk.name}`;
            const isSelected = selected.has(key);
            return (
              <label
                key={key}
                className={`flex items-center gap-3 px-3 py-2 cursor-pointer border-2 border-dashed transition-colors ${
                  isSelected
                    ? 'border-blue bg-info-light/50'
                    : 'border-transparent hover:border-muted-dark'
                } ${disabled ? 'opacity-50 pointer-events-none' : ''}`}
                style={{ borderRadius: wobbly.sm }}
              >
                <input
                  type="checkbox"
                  checked={isSelected}
                  onChange={() => onToggle(key)}
                  className="w-4 h-4 accent-blue shrink-0"
                  disabled={disabled}
                />
                <Folder size={14} strokeWidth={2.5} className="text-warning shrink-0" />
                <span
                  className="font-medium text-pencil"
                  style={{ fontFamily: "'Courier New', monospace", fontSize: '0.875rem' }}
                >
                  {sk.name}
                </span>
                <span className="text-sm text-pencil-light ml-auto">
                  {formatSize(sk.size)}
                </span>
              </label>
            );
          })}
        </div>
      )}
    </Card>
  );
}

/** Collect result summary */
function CollectResults({ result }: { result: CollectResult }) {
  const pulled = result.pulled ?? [];
  const skipped = result.skipped ?? [];
  const failed = result.failed ?? {};
  const failedEntries = Object.entries(failed);
  const total = pulled.length + skipped.length + failedEntries.length;

  if (total === 0) return null;

  return (
    <div className="animate-sketch-in">
      <h3
        className="text-xl font-bold text-pencil mb-4"
        style={{ fontFamily: 'var(--font-heading)' }}
      >
        Collect Results
      </h3>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-4">
        <ResultStat label="Pulled" count={pulled.length} icon={CheckCircle} variant="success" />
        <ResultStat label="Skipped" count={skipped.length} icon={SkipForward} variant="warning" />
        <ResultStat label="Failed" count={failedEntries.length} icon={XCircle} variant="danger" />
      </div>

      {/* Detail lists */}
      {pulled.length > 0 && (
        <DetailList title="Pulled" items={pulled} variant="success" />
      )}
      {skipped.length > 0 && (
        <DetailList title="Skipped (already in source)" items={skipped} variant="warning" />
      )}
      {failedEntries.length > 0 && (
        <Card variant="accent" className="mt-3">
          <h4
            className="font-bold text-danger mb-2"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            <AlertCircle size={16} strokeWidth={2.5} className="inline mr-1" />
            Failed
          </h4>
          <div className="space-y-1">
            {failedEntries.map(([name, err]) => (
              <div key={name} className="flex gap-2 text-sm">
                <span className="font-mono text-pencil">{name}</span>
                <span className="text-danger">{err}</span>
              </div>
            ))}
          </div>
        </Card>
      )}
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
  variant: 'success' | 'warning' | 'danger';
}) {
  const bgMap = { success: 'bg-success-light', warning: 'bg-warning-light', danger: 'bg-danger-light' };
  const colorMap = { success: 'text-success', warning: 'text-warning', danger: 'text-danger' };

  return (
    <div
      className={`flex items-center gap-2 px-3 py-2 border border-dashed ${count > 0 ? bgMap[variant] : 'bg-muted/30'}`}
      style={{ borderRadius: wobbly.sm }}
    >
      <Icon size={16} strokeWidth={2.5} className={count > 0 ? colorMap[variant] : 'text-muted-dark'} />
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

function DetailList({
  title,
  items,
  variant,
}: {
  title: string;
  items: string[];
  variant: 'success' | 'warning';
}) {
  const [open, setOpen] = useState(false);
  const colorMap = { success: 'text-success', warning: 'text-warning' };

  return (
    <div className="mt-3">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 text-sm text-pencil-light hover:text-pencil cursor-pointer transition-colors"
        style={{ fontFamily: 'var(--font-hand)' }}
      >
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <span className={colorMap[variant]}>{title}</span> ({items.length})
      </button>
      {open && (
        <div className="mt-2 pl-4 border-l-2 border-dashed border-muted-dark space-y-1 animate-sketch-in">
          {items.map((item) => (
            <p
              key={item}
              className="text-pencil-light text-sm"
              style={{ fontFamily: "'Courier New', monospace" }}
            >
              {item}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
