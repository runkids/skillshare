import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import {
  PackagePlus,
  Folder,
  FolderCheck,
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
import SplitButton from '../components/SplitButton';
import Spinner from '../components/Spinner';
import { Checkbox } from '../components/Input';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api, type AdoptCandidate, type AdoptApplyResult } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { radius, shadows } from '../design';
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
  const [result, setResult] = useState<AdoptApplyResult | null>(null);
  // Pending destructive confirmation; null = closed, boolean = the force flag.
  const [confirmForce, setConfirmForce] = useState<boolean | null>(null);

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

  const handleApply = async (opts: { dryRun?: boolean; force?: boolean } = {}) => {
    const dryRun = opts.dryRun ?? false;
    const force = opts.force ?? false;
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

  const toggleAll = (selectAll: boolean) => {
    setSelected(selectAll ? new Set(candidates.map((c) => c.name)) : new Set());
  };

  const conflictCount = candidates.filter((c) => c.conflict).length;
  const hasCandidates = candidates.length > 0;
  const busy = phase === 'applying';
  const rowsLocked = busy || phase === 'done';

  // Status summary line, mirroring the Sync page's stat-parts pattern.
  const statParts = [
    { n: candidates.length, label: t('adopt.stat.adoptable'), cls: 'text-info' },
    conflictCount > 0 && { n: conflictCount, label: t('adopt.stat.conflicts'), cls: 'text-danger' },
  ].filter((x): x is { n: number; label: string; cls: string } => !!x);

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<PackagePlus size={24} strokeWidth={2.5} />}
        title={t('adopt.title')}
        subtitle={t('adopt.subtitle')}
      />

      {/* Visual pipeline: agents target → adopt engine → source */}
      <div className="hidden md:flex items-center justify-center gap-4">
        <div
          className="flex items-center gap-2 px-4 py-2 bg-paper border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <Folder size={18} strokeWidth={2.5} className="text-warning" />
          <span className="text-base font-medium">{t('adopt.pipeline.agents')}</span>
        </div>

        <WavyConnector active={busy} />

        <div
          className="flex items-center gap-2 px-4 py-2 bg-info-light border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          {busy ? (
            <Spinner size="sm" className="text-blue" />
          ) : (
            <PackagePlus size={18} strokeWidth={2.5} className="text-blue" />
          )}
          <span className="text-base font-medium">{t('adopt.pipeline.engine')}</span>
        </div>

        <WavyConnector active={busy} />

        <div
          className="flex items-center gap-2 px-4 py-2 bg-success-light border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <FolderCheck size={18} strokeWidth={2.5} className="text-success" />
          <span className="text-base font-medium">{t('adopt.pipeline.source')}</span>
        </div>
      </div>

      {/* Control area */}
      <Card className="text-center">
        <div className="flex flex-col items-center gap-4">
          {/* Status indicator */}
          {phase === 'loading' ? (
            <p className="text-pencil-light text-base">{t('adopt.button.scanning')}</p>
          ) : hasCandidates ? (
            <p className="text-sm">
              {statParts.map((p, i) => (
                <span key={i}>
                  {i > 0 && <span className="text-muted-dark mx-1.5">·</span>}
                  <strong className={p.cls}>{p.n}</strong>{' '}
                  <span className="text-pencil-light">{p.label}</span>
                </span>
              ))}
            </p>
          ) : (
            <div className="flex flex-wrap items-center justify-center gap-2 text-sm">
              <CheckCircle size={16} strokeWidth={2.5} className="text-success" />
              <span className="font-medium text-success">{t('adopt.empty.title')}</span>
              <span className="text-muted-dark">·</span>
              <span className="text-pencil-light">{t('adopt.empty.description')}</span>
            </div>
          )}

          {/* Action bar */}
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Button
              onClick={loadPreview}
              loading={phase === 'loading'}
              disabled={busy}
              variant="secondary"
              size="sm"
            >
              {phase !== 'loading' && <RefreshCw size={18} strokeWidth={2.5} />}
              {phase === 'loading' ? t('adopt.button.scanning') : t('adopt.button.rescan')}
            </Button>

            {hasCandidates && phase !== 'done' && (
              <SplitButton
                onClick={() => setConfirmForce(false)}
                loading={busy}
                disabled={selected.size === 0}
                variant="primary"
                size="sm"
                dropdownAlign="right"
                items={[
                  {
                    label: t('adopt.button.previewCount', { count: selected.size }),
                    icon: <Eye size={16} strokeWidth={2.5} />,
                    onClick: () => handleApply({ dryRun: true }),
                  },
                  {
                    label: t('adopt.button.forceAdopt', { count: selected.size }),
                    icon: <Zap size={16} strokeWidth={2.5} />,
                    onClick: () => setConfirmForce(true),
                    confirm: true,
                  },
                ]}
              >
                {!busy && <PackagePlus size={18} strokeWidth={2.5} />}
                {busy
                  ? t('adopt.button.applying')
                  : t('adopt.button.adoptCount', { count: selected.size })}
              </SplitButton>
            )}
          </div>
        </div>
      </Card>

      {/* Loading */}
      {phase === 'loading' && <PageSkeleton />}

      {/* Lockfile warning (preview-time hint) */}
      {phase === 'loaded' && lockPresent && hasCandidates && (
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

      {/* Candidate list */}
      {phase !== 'loading' && hasCandidates && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold text-pencil">
              {t('adopt.foundCount', { count: candidates.length })}
            </h2>
            {phase !== 'done' && (
              <div className="flex gap-2">
                <Button onClick={() => toggleAll(true)} variant="ghost" size="sm" disabled={busy}>
                  {t('adopt.selectAll')}
                </Button>
                <Button onClick={() => toggleAll(false)} variant="ghost" size="sm" disabled={busy}>
                  {t('adopt.selectNone')}
                </Button>
              </div>
            )}
          </div>

          <div className="space-y-2">
            {candidates.map((c) => {
              const isSelected = selected.has(c.name);
              return (
                <Card key={c.name} className="!p-3">
                  <div
                    className={`flex items-center gap-3 ${!rowsLocked ? 'cursor-pointer' : ''}`}
                    onClick={() => {
                      if (!rowsLocked) toggle(c.name);
                    }}
                  >
                    <span onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        label=""
                        checked={isSelected}
                        onChange={() => toggle(c.name)}
                        size="sm"
                        disabled={rowsLocked}
                      />
                    </span>
                    <Folder size={16} strokeWidth={2.5} className="text-warning shrink-0" />
                    <span className="font-mono font-medium text-pencil text-sm">{c.name}</span>

                    <div className="flex items-center gap-2 ml-auto flex-wrap justify-end">
                      {c.sourceTool && <Badge variant="info">{c.sourceTool}</Badge>}
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
                    <p className="mt-2 pl-8 text-sm text-danger">{t('adopt.conflict.hint')}</p>
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
        </div>
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

      {/* Confirm dialog (destructive: trashes originals) */}
      <ConfirmDialog
        open={confirmForce !== null}
        title={t('adopt.confirm.title')}
        message={
          <div className="text-left">
            <p className="mb-2">
              {t('adopt.confirm.message', {
                count: selected.size,
                forceSuffix: confirmForce ? t('adopt.confirm.forceOverwrite') : '',
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
        confirmText={t('adopt.button.adoptCount', { count: selected.size })}
        onConfirm={() => {
          const force = confirmForce ?? false;
          setConfirmForce(null);
          handleApply({ force });
        }}
        onCancel={() => setConfirmForce(null)}
      />
    </div>
  );
}

/** Hand-drawn wavy connector between pipeline stages (mirrors the Sync page). */
function WavyConnector({ active }: { active: boolean }) {
  return (
    <div className="flex items-center gap-1">
      <svg width="60" height="20" viewBox="0 0 60 20" className="text-pencil-light">
        <path
          d="M0 10 Q15 4 30 10 Q45 16 60 10"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeDasharray="4 4"
          className={active ? 'animate-flow' : ''}
        />
      </svg>
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
    <div className="space-y-3 animate-fade-in">
      <h2 className="text-lg font-bold text-pencil">
        {result.dryRun ? t('adopt.results.dryRunTitle') : t('adopt.results.title')}
      </h2>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <ResultStat label={t('adopt.results.adopted')} count={adopted.length} icon={CheckCircle} variant="success" />
        <ResultStat label={t('adopt.results.trashed')} count={result.trashed} icon={Trash2} variant="warning" />
        <ResultStat label={t('adopt.results.pruned')} count={result.prunedLinks} icon={Link2Off} variant="info" />
        <ResultStat label={t('adopt.results.failed')} count={failedEntries.length} icon={XCircle} variant="danger" />
      </div>

      {adopted.length > 0 && (
        <Card>
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
        <Card>
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
        <Card variant="accent">
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
