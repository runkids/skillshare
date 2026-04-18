import { useState, useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
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
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import Button from '../components/Button';
import { Checkbox } from '../components/Input';
import EmptyState from '../components/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type CollectScanTarget, type CollectResult } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { radius, shadows } from '../design';
import { formatSize } from '../lib/format';
import KindBadge from '../components/KindBadge';
import SegmentedControl from '../components/SegmentedControl';
import { useT } from '../i18n';

type Phase = 'idle' | 'scanning' | 'scanned' | 'collecting' | 'done';
type CollectScope = 'skill' | 'agent' | 'both';

function parseCollectScope(value: string | null): CollectScope | null {
  if (value === 'skill' || value === 'agent' || value === 'both') return value;
  return null;
}

export default function CollectPage() {
  const t = useT();
  const queryClient = useQueryClient();
  const [searchParams] = useSearchParams();
  const presetTarget = searchParams.get('target') ?? undefined;
  const presetScope = parseCollectScope(searchParams.get('scope'));

  const [phase, setPhase] = useState<Phase>('idle');
  const [force, setForce] = useState(false);
  const [scanTargets, setScanTargets] = useState<CollectScanTarget[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [result, setResult] = useState<CollectResult | null>(null);
  const [confirming, setConfirming] = useState(false);
  const { toast } = useToast();
  const [scope, setScope] = useState<CollectScope>(presetScope ?? 'skill');

  const scopeLabels: Record<CollectScope, { noun: string; nounPlural: string; scanBtn: string; entity: string }> = {
    skill: {
      noun: t('collect.noun.skill'),
      nounPlural: t('collect.nounPlural.skill'),
      scanBtn: t('collect.scan.button.skill'),
      entity: t('collect.entity.skill'),
    },
    agent: {
      noun: t('collect.noun.agent'),
      nounPlural: t('collect.nounPlural.agent'),
      scanBtn: t('collect.scan.button.agent'),
      entity: t('collect.entity.agent'),
    },
    both: {
      noun: t('collect.noun.both'),
      nounPlural: t('collect.nounPlural.both'),
      scanBtn: t('collect.scan.button.both'),
      entity: t('collect.entity.both'),
    },
  };
  const labels = scopeLabels[scope];

  // Reset state when scope changes
  useEffect(() => {
    setPhase('idle');
    setScanTargets([]);
    setTotalCount(0);
    setSelected(new Set());
    setResult(null);
  }, [scope]);

  useEffect(() => {
    if (presetScope && presetScope !== scope) {
      setScope(presetScope);
    }
  }, [presetScope, scope]);

  // Auto-scan when target query param is present
  useEffect(() => {
    if (presetTarget) {
      handleScan(presetTarget, presetScope ?? scope);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [presetTarget, presetScope]);

  const handleScan = async (targetFilter?: string, scopeOverride?: CollectScope) => {
    const activeScope = scopeOverride ?? scope;
    setPhase('scanning');
    setResult(null);
    try {
      const res = await api.collectScan(targetFilter, activeScope === 'both' ? undefined : activeScope);
      setScanTargets(res.targets);
      setTotalCount(res.totalCount);
      // Auto-select all
      const allKeys = new Set<string>();
      for (const t of res.targets) {
        for (const sk of t.skills) {
          allKeys.add(`${t.targetName}/${sk.kind ?? 'skill'}/${sk.name}`);
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
        const [targetName, kind, ...rest] = key.split('/');
        return { name: rest.join('/'), targetName, kind };
      });
      const res = await api.collect({ skills, force });
      setResult(res);
      const pulledCount = res.pulled?.length ?? 0;
      const skippedCount = res.skipped?.length ?? 0;
      const failedCount = Object.keys(res.failed ?? {}).length;
      toast(
        t('collect.toast.complete', { pulled: pulledCount, skipped: skippedCount, failed: failedCount }),
        pulledCount > 0 ? 'success' : 'info',
      );
      setPhase('done');
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
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
          allKeys.add(`${t.targetName}/${sk.kind ?? 'skill'}/${sk.name}`);
        }
      }
      setSelected(allKeys);
    } else {
      setSelected(new Set());
    }
  };

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader icon={<ArrowDownToLine size={24} strokeWidth={2.5} />} title={t('collect.title')} subtitle={t('collect.subtitle')} />

      {/* Visual Pipeline (reverse direction) */}
      <div className="hidden md:flex items-center justify-center gap-4">
        <div
          className="flex items-center gap-2 px-4 py-2 bg-success-light border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <Target size={18} strokeWidth={2.5} className="text-success" />
          <span className="text-base font-medium">
            {t('collect.pipeline.targets')}
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
          <ArrowDownToLine
            size={18}
            strokeWidth={2.5}
            className={`text-blue ${phase === 'collecting' ? 'animate-bounce' : ''}`}
          />
          <span className="text-base font-medium">
            {t('collect.pipeline.collectEngine')}
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
          className="flex items-center gap-2 px-4 py-2 bg-paper border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <Folder size={18} strokeWidth={2.5} className="text-warning" />
          <span className="text-base font-medium">
            {t('collect.pipeline.source')}
          </span>
        </div>
      </div>

      {/* Scan control area */}
      <Card className="text-center">
        <div data-tour="collect-scan" className="flex flex-col items-center gap-4">
          <div className="flex flex-wrap items-center justify-center gap-3">
            <SegmentedControl
              value={scope}
              onChange={setScope}
              options={[
                { value: 'skill' as const, label: t('collect.entity.skill') },
                { value: 'agent' as const, label: t('collect.entity.agent') },
                { value: 'both' as const, label: t('collect.entity.both') },
              ]}
              size="sm"
              connected
            />
            <Button
              onClick={() => handleScan(presetTarget)}
              loading={phase === 'scanning'}
              disabled={phase === 'collecting'}
              variant="primary"
              size="sm"
            >
              {phase !== 'scanning' && <ArrowDownToLine size={18} strokeWidth={2.5} />}
              {phase === 'scanning' ? t('collect.button.scanning') : phase === 'idle' ? labels.scanBtn : t('collect.button.rescan')}
            </Button>
          </div>

          {presetTarget && (
            <p className="text-sm text-pencil-light">
              {t('collect.control.filteringBy')} <Badge variant="info">{presetTarget}</Badge>
            </p>
          )}

          {/* Force toggle */}
          {(phase === 'scanned' || phase === 'done') && (
            <div className="flex items-center gap-2">
              <Checkbox
                label={t('collect.control.force')}
                checked={force}
                onChange={setForce}
              />
              <Zap size={16} strokeWidth={2.5} className="text-accent" />
            </div>
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
              title={t('collect.empty.title', { nounPlural: labels.nounPlural })}
              description={t('collect.empty.description', { nounPlural: labels.nounPlural })}
            />
          ) : (
            <div>
              {/* Select all / none controls */}
              <div className="flex items-center justify-between mb-4">
                <h3
                  className="text-xl font-bold text-pencil"
                >
                  {t('collect.scan.foundCount', { count: totalCount, noun: totalCount !== 1 ? labels.nounPlural : labels.noun })}
                </h3>
                <div className="flex gap-2">
                  <Button
                    onClick={() => toggleAll(true)}
                    variant="ghost"
                    size="sm"
                    disabled={phase === 'collecting'}
                  >
                    {t('collect.scan.selectAll')}
                  </Button>
                  <Button
                    onClick={() => toggleAll(false)}
                    variant="ghost"
                    size="sm"
                    disabled={phase === 'collecting'}
                  >
                    {t('collect.scan.selectNone')}
                  </Button>
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
                  <Button
                    onClick={() => setConfirming(true)}
                    loading={phase === 'collecting'}
                    disabled={selected.size === 0}
                    variant="primary"
                    size="lg"
                    className="min-w-[200px]"
                  >
                    {phase !== 'collecting' && <ArrowDownToLine size={22} strokeWidth={2.5} />}
                    {phase === 'collecting'
                      ? t('collect.button.collecting')
                      : t('collect.button.collectCount', { count: selected.size, entity: labels.entity })}
                  </Button>
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
        <Card variant="accent" className="text-center animate-fade-in">
          <div className="flex flex-col items-center gap-3">
            <p
              className="text-base text-pencil"
            >
              {t('collect.postCollect.message', { entity: labels.entity })}
            </p>
            <Link to="/sync">
              <Button variant="primary" size="sm">
                <RefreshCw size={16} strokeWidth={2.5} />
                {t('collect.postCollect.goToSync')}
              </Button>
            </Link>
          </div>
        </Card>
      )}

      {/* Confirm collect dialog */}
      <ConfirmDialog
        open={confirming}
        title={t('collect.confirm.title')}
        message={
          <div className="text-left">
            <p className="mb-2">
              {t('collect.confirm.message', {
                count: selected.size,
                noun: selected.size !== 1 ? labels.nounPlural : labels.noun,
                forceSuffix: force ? t('collect.confirm.forceOverwrite') : '',
              })}
            </p>
            <ul className="list-none space-y-1 max-h-40 overflow-y-auto">
              {Array.from(selected).map((key) => {
                const [targetName, , ...rest] = key.split('/');
                return (
                  <li key={key} className="flex items-center gap-2 text-sm">
                    <Folder size={12} strokeWidth={2.5} className="text-warning shrink-0" />
                    <span className="font-mono">{rest.join('/')}</span>
                    <span className="text-pencil-light">← {targetName}</span>
                  </li>
                );
              })}
            </ul>
          </div>
        }
        confirmText={t('collect.button.collectCount', { count: selected.size, entity: labels.entity })}
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
  const t = useT();
  const [expanded, setExpanded] = useState(true);
  const skills = target.skills ?? [];
  const selectedCount = skills.filter((sk) => selected.has(`${target.targetName}/${sk.kind ?? 'skill'}/${sk.name}`)).length;

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
          {target.targetName}
        </h4>
        <Badge variant={selectedCount > 0 ? 'info' : 'default'}>
          {t('collect.scan.selectedCount', { selected: selectedCount, total: skills.length })}
        </Badge>
      </button>

      {expanded && skills.length > 0 && (
        <div className="mt-3 pl-8 space-y-2 animate-fade-in">
          {skills.map((sk) => {
            const key = `${target.targetName}/${sk.kind ?? 'skill'}/${sk.name}`;
            const isSelected = selected.has(key);
            return (
              <div
                key={key}
                className={`flex items-center gap-3 px-3 py-2 cursor-pointer border border-dashed transition-colors ${
                  isSelected
                    ? 'border-blue bg-info-light/50'
                    : 'border-transparent hover:border-muted-dark'
                } ${disabled ? 'opacity-50 pointer-events-none' : ''}`}
                style={{ borderRadius: radius.sm }}
                onClick={() => onToggle(key)}
              >
                <span onClick={(e) => e.stopPropagation()}>
                  <Checkbox label="" checked={isSelected} onChange={() => onToggle(key)} size="sm" disabled={disabled} />
                </span>
                <Folder size={14} strokeWidth={2.5} className="text-warning shrink-0" />
                {sk.kind && <KindBadge kind={sk.kind} />}
                <span className="font-mono font-medium text-pencil text-sm">
                  {sk.name}
                </span>
                <span className="text-sm text-pencil-light ml-auto">
                  {formatSize(sk.size)}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}

/** Collect result summary */
function CollectResults({ result }: { result: CollectResult }) {
  const t = useT();
  const pulled = result.pulled ?? [];
  const skipped = result.skipped ?? [];
  const failed = result.failed ?? {};
  const failedEntries = Object.entries(failed);
  const total = pulled.length + skipped.length + failedEntries.length;

  if (total === 0) return null;

  return (
    <div className="animate-fade-in">
      <h3
        className="text-xl font-bold text-pencil mb-4"
      >
        {t('collect.results.title')}
      </h3>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-4">
        <ResultStat label={t('collect.results.pulled')} count={pulled.length} icon={CheckCircle} variant="success" />
        <ResultStat label={t('collect.results.skipped')} count={skipped.length} icon={SkipForward} variant="warning" />
        <ResultStat label={t('collect.results.failed')} count={failedEntries.length} icon={XCircle} variant="danger" />
      </div>

      {/* Detail lists */}
      {pulled.length > 0 && (
        <DetailList title={t('collect.results.pulled')} items={pulled} variant="success" />
      )}
      {skipped.length > 0 && (
        <DetailList title={t('collect.results.skippedInSource')} items={skipped} variant="warning" />
      )}
      {failedEntries.length > 0 && (
        <Card variant="accent" className="mt-3">
          <h4
            className="font-bold text-danger mb-2"
          >
            <AlertCircle size={16} strokeWidth={2.5} className="inline mr-1" />
            {t('collect.results.failed')}
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
      className={`flex items-center gap-2 px-3 py-2 border border-dashed border-muted ${count > 0 ? bgMap[variant] : 'bg-muted/30'}`}
      style={{ borderRadius: radius.sm }}
    >
      <Icon size={16} strokeWidth={2.5} className={count > 0 ? colorMap[variant] : 'text-muted-dark'} />
      <div>
        <p
          className={`text-lg font-bold leading-none ${count > 0 ? colorMap[variant] : 'text-muted-dark'}`}
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
      >
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <span className={colorMap[variant]}>{title}</span> ({items.length})
      </button>
      {open && (
        <div className="mt-2 pl-6 space-y-1 animate-fade-in">
          {items.map((item) => (
            <p
              key={item}
              className="font-mono text-pencil-light text-sm"
            >
              {item}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}
