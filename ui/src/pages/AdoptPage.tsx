import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import {
  PackagePlus,
  Folder,
  Zap,
  Eye,
  Link2Off,
  CheckCircle,
  AlertCircle,
  AlertTriangle,
  RefreshCw,
  SkipForward,
  XCircle,
  Trash2,
  Lock,
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
import { api, type AdoptCandidate, type AdoptApplyResult } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { radius } from '../design';
import { useT } from '../i18n';

type Phase = 'loading' | 'loaded' | 'applying' | 'done';

export default function AdoptPage() {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [phase, setPhase] = useState<Phase>('loading');
  const [candidates, setCandidates] = useState<AdoptCandidate[]>([]);
  const [lockPresent, setLockPresent] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [force, setForce] = useState(false);
  const [dryRun, setDryRun] = useState(false);
  const [result, setResult] = useState<AdoptApplyResult | null>(null);
  const [confirming, setConfirming] = useState(false);

  const loadPreview = async () => {
    setPhase('loading');
    setResult(null);
    try {
      const res = await api.getAdoptPreview();
      const list = res.candidates ?? [];
      setCandidates(list);
      setLockPresent(res.lockPresent);
      // Auto-select all non-conflicting candidates.
      setSelected(new Set(list.filter((c) => !c.conflict).map((c) => c.name)));
      setPhase('loaded');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setPhase('loaded');
      setCandidates([]);
    }
  };

  // Initial load.
  useEffect(() => {
    loadPreview();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleApply = async () => {
    setPhase('applying');
    try {
      const res = await api.postAdoptApply({
        names: Array.from(selected),
        force,
        dryRun,
      });
      setResult(res);
      const adoptedCount = res.adopted?.length ?? 0;
      if (res.dryRun) {
        toast(t('adopt.toast.dryRun', { adopted: adoptedCount }), 'info');
      } else {
        toast(
          t('adopt.toast.complete', {
            adopted: adoptedCount,
            trashed: res.trashed,
            pruned: res.prunedLinks,
          }),
          adoptedCount > 0 ? 'success' : 'info',
        );
      }
      setPhase('done');
      if (!res.dryRun) {
        queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
        queryClient.invalidateQueries({ queryKey: queryKeys.overview });
        queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
        queryClient.invalidateQueries({ queryKey: queryKeys.trash });
        queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setPhase('loaded');
    }
  };

  const toggle = (name: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  const selectableNames = candidates
    .filter((c) => force || !c.conflict)
    .map((c) => c.name);

  const toggleAll = (selectAll: boolean) => {
    setSelected(selectAll ? new Set(selectableNames) : new Set());
  };

  // Conflicting candidates need Force to be adoptable.
  const isSelectable = (c: AdoptCandidate) => force || !c.conflict;

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<PackagePlus size={24} strokeWidth={2.5} />}
        title={t('adopt.title')}
        subtitle={t('adopt.subtitle')}
      />

      {/* Controls */}
      <Card className="text-center">
        <div className="flex flex-col items-center gap-4">
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Button
              onClick={loadPreview}
              loading={phase === 'loading'}
              disabled={phase === 'applying'}
              variant="secondary"
              size="sm"
            >
              {phase !== 'loading' && <RefreshCw size={18} strokeWidth={2.5} />}
              {phase === 'loading' ? t('adopt.button.scanning') : t('adopt.button.rescan')}
            </Button>
          </div>

          {(phase === 'loaded' || phase === 'done') && candidates.length > 0 && (
            <div className="flex flex-wrap items-center justify-center gap-5">
              <div className="flex items-center gap-2">
                <Checkbox label={t('adopt.control.dryRun')} checked={dryRun} onChange={setDryRun} />
                <Eye size={16} strokeWidth={2.5} className="text-blue" />
              </div>
              <div className="flex items-center gap-2">
                <Checkbox label={t('adopt.control.force')} checked={force} onChange={setForce} />
                <Zap size={16} strokeWidth={2.5} className="text-accent" />
              </div>
            </div>
          )}
        </div>
      </Card>

      {/* Loading */}
      {phase === 'loading' && <PageSkeleton />}

      {/* Empty */}
      {phase !== 'loading' && candidates.length === 0 && (
        <EmptyState
          icon={CheckCircle}
          title={t('adopt.empty.title')}
          description={t('adopt.empty.description')}
        />
      )}

      {/* Candidate list */}
      {phase !== 'loading' && candidates.length > 0 && (
        <div>
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-xl font-bold text-pencil">
              {t('adopt.foundCount', { count: candidates.length })}
            </h3>
            {phase !== 'done' && (
              <div className="flex gap-2">
                <Button onClick={() => toggleAll(true)} variant="ghost" size="sm" disabled={phase === 'applying'}>
                  {t('adopt.selectAll')}
                </Button>
                <Button onClick={() => toggleAll(false)} variant="ghost" size="sm" disabled={phase === 'applying'}>
                  {t('adopt.selectNone')}
                </Button>
              </div>
            )}
          </div>

          <div className="space-y-2">
            {candidates.map((c) => {
              const selectable = isSelectable(c);
              const isSelected = selected.has(c.name);
              return (
                <Card
                  key={c.name}
                  className={`!p-3 ${!selectable ? 'opacity-60' : ''}`}
                >
                  <div
                    className={`flex items-center gap-3 ${
                      selectable && phase !== 'applying' && phase !== 'done' ? 'cursor-pointer' : ''
                    }`}
                    onClick={() => {
                      if (selectable && phase !== 'applying' && phase !== 'done') toggle(c.name);
                    }}
                  >
                    <span onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        label=""
                        checked={isSelected}
                        onChange={() => toggle(c.name)}
                        size="sm"
                        disabled={!selectable || phase === 'applying' || phase === 'done'}
                      />
                    </span>
                    <Folder size={16} strokeWidth={2.5} className="text-warning shrink-0" />
                    <span className="font-mono font-medium text-pencil text-sm">{c.name}</span>

                    <div className="flex items-center gap-2 ml-auto flex-wrap justify-end">
                      {c.sourceTool && (
                        <Badge variant="info">{c.sourceTool}</Badge>
                      )}
                      {c.conflict && (
                        <Badge variant="danger">
                          <AlertTriangle size={11} strokeWidth={2.5} className="inline" />
                          {t('adopt.badge.conflict')}
                        </Badge>
                      )}
                      {c.externalLinks.length > 0 && (
                        <Badge variant="warning">
                          <Link2Off size={11} strokeWidth={2.5} className="inline" />
                          {t('adopt.badge.externalLinks', { count: c.externalLinks.length })}
                        </Badge>
                      )}
                    </div>
                  </div>

                  {c.conflict && (
                    <p className="mt-2 pl-8 text-sm text-danger">
                      {t('adopt.conflict.hint')}
                    </p>
                  )}
                  {c.externalLinks.length > 0 && (
                    <div className="mt-2 pl-8 space-y-0.5">
                      {c.externalLinks.map((link) => (
                        <p key={link} className="font-mono text-xs text-pencil-light truncate">
                          {link}
                        </p>
                      ))}
                    </div>
                  )}
                </Card>
              );
            })}
          </div>

          {/* Apply button */}
          {phase !== 'done' && (
            <div className="mt-6 text-center">
              <Button
                onClick={() => setConfirming(true)}
                loading={phase === 'applying'}
                disabled={selected.size === 0}
                variant="primary"
                size="lg"
                className="min-w-[200px]"
              >
                {phase !== 'applying' && <PackagePlus size={22} strokeWidth={2.5} />}
                {phase === 'applying'
                  ? t('adopt.button.applying')
                  : dryRun
                    ? t('adopt.button.previewCount', { count: selected.size })
                    : t('adopt.button.adoptCount', { count: selected.size })}
              </Button>
            </div>
          )}
        </div>
      )}

      {/* Lockfile warning (preview-time hint) */}
      {phase === 'loaded' && lockPresent && candidates.length > 0 && (
        <Card variant="accent">
          <div className="flex items-start gap-3">
            <Lock size={18} strokeWidth={2.5} className="text-warning shrink-0 mt-0.5" />
            <div>
              <h4 className="font-bold text-pencil mb-1">{t('adopt.lock.title')}</h4>
              <p className="text-sm text-pencil-light">{t('adopt.lock.previewHint')}</p>
            </div>
          </div>
        </Card>
      )}

      {/* Results */}
      {phase === 'done' && result && <AdoptResults result={result} />}

      {/* Post-adopt suggestion */}
      {phase === 'done' && result && !result.dryRun && (result.adopted?.length ?? 0) > 0 && (
        <Card variant="accent" className="text-center animate-fade-in">
          <div className="flex flex-col items-center gap-3">
            <p className="text-base text-pencil">{t('adopt.postAdopt.message')}</p>
            <Link to="/sync">
              <Button variant="primary" size="sm">
                <RefreshCw size={16} strokeWidth={2.5} />
                {t('adopt.postAdopt.goToSync')}
              </Button>
            </Link>
          </div>
        </Card>
      )}

      {/* Confirm dialog */}
      <ConfirmDialog
        open={confirming}
        title={dryRun ? t('adopt.confirm.dryRunTitle') : t('adopt.confirm.title')}
        message={
          <div className="text-left">
            <p className="mb-2">
              {dryRun
                ? t('adopt.confirm.dryRunMessage', { count: selected.size })
                : t('adopt.confirm.message', {
                    count: selected.size,
                    forceSuffix: force ? t('adopt.confirm.forceOverwrite') : '',
                  })}
            </p>
            <ul className="list-none space-y-1 max-h-40 overflow-y-auto">
              {Array.from(selected).map((name) => (
                <li key={name} className="flex items-center gap-2 text-sm">
                  <Folder size={12} strokeWidth={2.5} className="text-warning shrink-0" />
                  <span className="font-mono">{name}</span>
                </li>
              ))}
            </ul>
          </div>
        }
        confirmText={dryRun ? t('adopt.button.previewCount', { count: selected.size }) : t('adopt.button.adoptCount', { count: selected.size })}
        onConfirm={() => {
          setConfirming(false);
          handleApply();
        }}
        onCancel={() => setConfirming(false)}
      />
    </div>
  );
}

/** Adopt result summary */
function AdoptResults({ result }: { result: AdoptApplyResult }) {
  const t = useT();
  const adopted = result.adopted ?? [];
  const skipped = result.skipped ?? [];
  const failed = result.failed ?? {};
  const failedEntries = Object.entries(failed);
  const lockWarnings = result.lockWarnings ?? [];

  return (
    <div className="animate-fade-in">
      <h3 className="text-xl font-bold text-pencil mb-4">
        {result.dryRun ? t('adopt.results.dryRunTitle') : t('adopt.results.title')}
      </h3>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
        <ResultStat label={t('adopt.results.adopted')} count={adopted.length} icon={CheckCircle} variant="success" />
        <ResultStat label={t('adopt.results.trashed')} count={result.trashed} icon={Trash2} variant="warning" />
        <ResultStat label={t('adopt.results.pruned')} count={result.prunedLinks} icon={Link2Off} variant="info" />
        <ResultStat label={t('adopt.results.failed')} count={failedEntries.length} icon={XCircle} variant="danger" />
      </div>

      {adopted.length > 0 && (
        <Card className="mb-3">
          <h4 className="font-bold text-success mb-2">
            <CheckCircle size={16} strokeWidth={2.5} className="inline mr-1" />
            {t('adopt.results.adopted')}
          </h4>
          <div className="space-y-1">
            {adopted.map((name) => (
              <p key={name} className="font-mono text-pencil-light text-sm">{name}</p>
            ))}
          </div>
        </Card>
      )}

      {skipped.length > 0 && (
        <Card className="mb-3">
          <h4 className="font-bold text-warning mb-2">
            <SkipForward size={16} strokeWidth={2.5} className="inline mr-1" />
            {t('adopt.results.skipped')}
          </h4>
          <div className="space-y-1">
            {skipped.map((name) => (
              <p key={name} className="font-mono text-pencil-light text-sm">{name}</p>
            ))}
          </div>
        </Card>
      )}

      {failedEntries.length > 0 && (
        <Card variant="accent" className="mb-3">
          <h4 className="font-bold text-danger mb-2">
            <AlertCircle size={16} strokeWidth={2.5} className="inline mr-1" />
            {t('adopt.results.failed')}
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

      {/* Lingering lockfile warnings — rendered prominently */}
      {lockWarnings.length > 0 && (
        <Card variant="accent" className="border-2 border-warning/40">
          <div className="flex items-start gap-3">
            <Lock size={18} strokeWidth={2.5} className="text-warning shrink-0 mt-0.5" />
            <div className="flex-1">
              <h4 className="font-bold text-warning mb-1">{t('adopt.lock.warningTitle')}</h4>
              <p className="text-sm text-pencil-light mb-2">{t('adopt.lock.warningHint')}</p>
              <div className="space-y-1">
                {lockWarnings.map((w) => (
                  <div key={w.name} className="flex items-center gap-2 text-sm">
                    <Folder size={12} strokeWidth={2.5} className="text-warning shrink-0" />
                    <span className="font-mono text-pencil">{w.name}</span>
                    {w.source_tool && <span className="text-pencil-light">← {w.source_tool}</span>}
                  </div>
                ))}
              </div>
            </div>
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
  variant: 'success' | 'warning' | 'danger' | 'info';
}) {
  const bgMap = {
    success: 'bg-success-light',
    warning: 'bg-warning-light',
    danger: 'bg-danger-light',
    info: 'bg-info-light',
  };
  const colorMap = {
    success: 'text-success',
    warning: 'text-warning',
    danger: 'text-danger',
    info: 'text-blue',
  };

  return (
    <div
      className={`flex items-center gap-2 px-3 py-2 border border-dashed border-muted ${count > 0 ? bgMap[variant] : 'bg-muted/30'}`}
      style={{ borderRadius: radius.sm }}
    >
      <Icon size={16} strokeWidth={2.5} className={count > 0 ? colorMap[variant] : 'text-muted-dark'} />
      <div>
        <p className={`text-lg font-bold leading-none ${count > 0 ? colorMap[variant] : 'text-muted-dark'}`}>
          {count}
        </p>
        <p className="text-sm text-pencil-light">{label}</p>
      </div>
    </div>
  );
}
