import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Stethoscope,
  RefreshCw,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Info,
  ChevronDown,
  ChevronRight,
  ArrowUpCircle,
  ArrowRight,
  PackagePlus,
  PartyPopper,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { DoctorCheck } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Card from '../components/Card';
import Button from '../components/Button';
import Badge from '../components/Badge';
import SegmentedControl from '../components/SegmentedControl';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { palette } from '../design';
import { useT } from '../i18n';

type StatusFilter = 'all' | 'error' | 'warning' | 'pass';

const checkLabelFallbacks: Record<string, string> = {
  source: 'Source Directory',
  symlink_support: 'Symlink Support',
  git_status: 'Git Status',
  skills_validity: 'Skill Files',
  skill_integrity: 'Skill Integrity',
  skill_targets_field: 'Target References',
  targets: 'Targets',
  sync_drift: 'Sync Status',
  broken_symlinks: 'Broken Symlinks',
  duplicate_skills: 'Duplicate Skills',
  extras: 'Extras',
  backup: 'Backups',
  trash: 'Trash',
  agents_source: 'Agents Source',
  theme: 'Theme',
  cli_version: 'CLI Version',
  skill_version: 'Skill Version',
  skillignore: 'Skillignore',
};

function statusIcon(status: DoctorCheck['status'], size = 16) {
  switch (status) {
    case 'pass':
      return <CheckCircle2 size={size} strokeWidth={2.5} style={{ color: palette.success }} />;
    case 'warning':
      return <AlertTriangle size={size} strokeWidth={2.5} style={{ color: palette.warning }} />;
    case 'error':
      return <XCircle size={size} strokeWidth={2.5} style={{ color: palette.danger }} />;
    case 'info':
      return <Info size={size} strokeWidth={2.5} style={{ color: palette.info }} />;
  }
}

function CheckRow({ check }: { check: DoctorCheck }) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  const hasDetails = check.details && check.details.length > 0;
  const hasSuggestions = check.suggestions && check.suggestions.length > 0;
  const expandable = hasDetails || hasSuggestions;
  const fallback = checkLabelFallbacks[check.name] ?? check.name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
  const label = t(`doctor.check.${check.name}`, {}, fallback);

  return (
    <div className="border-b border-muted last:border-b-0">
      <button
        onClick={() => expandable && setExpanded((v) => !v)}
        className={`w-full flex items-center gap-3 px-4 py-3 text-left transition-colors ${expandable ? 'cursor-pointer hover:bg-muted/20' : 'cursor-default'}`}
      >
        {statusIcon(check.status)}
        <div className="flex-1 min-w-0">
          <span className="font-medium text-pencil text-sm">{label}</span>
          <p className="text-pencil-light text-sm mt-0.5 truncate">{check.message}</p>
        </div>
        {expandable && (
          <span className="text-pencil-light shrink-0">
            {expanded
              ? <ChevronDown size={16} strokeWidth={2.5} />
              : <ChevronRight size={16} strokeWidth={2.5} />}
          </span>
        )}
      </button>
      {expanded && expandable && (
        <div className="px-4 pb-3 pl-11 space-y-3">
          {hasDetails && <CheckDetails details={check.details!} name={check.name} />}
          {hasSuggestions && (
            <div>
              <p className="text-xs font-medium text-pencil-light mb-1.5">{t('doctor.suggestions')}</p>
              <ul className="space-y-1">
                {check.suggestions!.map((s, i) => (
                  <li key={i} className="text-sm text-pencil-light flex items-start gap-2">
                    <span className="text-muted-dark mt-0.5 shrink-0">&rarr;</span>
                    <span>{s}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function CheckDetails({ details, name }: { details: string[]; name: string }) {
  const t = useT();
  // Skillignore check uses --- to separate patterns from ignored skills
  const sepIdx = details.indexOf('---');
  if (name === 'skillignore' && sepIdx !== -1) {
    const patterns = details.slice(0, sepIdx);
    const ignored = details.slice(sepIdx + 1);
    return (
      <div className="space-y-3">
        {patterns.length > 0 && (
          <div>
            <p className="text-xs font-medium text-pencil-light mb-1.5">{t('doctor.skillignore.patterns')}</p>
            <div className="flex flex-wrap gap-1.5">
              {patterns.map((p, i) => (
                <span key={i} className="font-mono text-xs px-2 py-0.5 rounded bg-muted/60 text-pencil-light border border-muted">
                  {p}
                </span>
              ))}
            </div>
          </div>
        )}
        {ignored.length > 0 && (
          <div>
            <p className="text-xs font-medium text-pencil-light mb-1.5">{t('doctor.skillignore.ignoredSkills')}</p>
            <div className="flex flex-wrap gap-1.5">
              {ignored.map((s, i) => (
                <span key={i} className="font-mono text-xs px-2 py-0.5 rounded bg-warning-light/50 text-pencil-light border border-warning/30">
                  {s}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    );
  }

  // Default: bullet list for all other checks
  return (
    <ul className="space-y-1">
      {details.map((detail, i) => (
        <li key={i} className="text-sm text-pencil-light flex items-start gap-2">
          <span className="text-muted-dark mt-0.5 shrink-0">&bull;</span>
          <span>{detail}</span>
        </li>
      ))}
    </ul>
  );
}

export default function DoctorPage() {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data, isPending, error, isFetching, refetch } = useQuery({
    queryKey: queryKeys.doctor,
    queryFn: () => api.doctor(),
    staleTime: staleTimes.doctor,
  });
  const [filter, setFilter] = useState<StatusFilter>('all');
  const [upgrading, setUpgrading] = useState(false);
  const [upgradeMessage, setUpgradeMessage] = useState<string | null>(null);

  const filteredChecks = useMemo(() => {
    if (!data) return [];
    if (filter === 'all') return data.checks;
    if (filter === 'pass') return data.checks.filter((c) => c.status === 'pass' || c.status === 'info');
    return data.checks.filter((c) => c.status === filter);
  }, [data, filter]);

  const allPassed = data && data.summary.errors === 0 && data.summary.warnings === 0;

  const waitForRestartThenReload = async () => {
    await new Promise((resolve) => setTimeout(resolve, 800));
    for (let i = 0; i < 40; i++) {
      try {
        await api.health();
        window.location.reload();
        return;
      } catch {
        await new Promise((resolve) => setTimeout(resolve, 500));
      }
    }
    setUpgradeMessage(t('updateDialog.restartManual'));
    setUpgrading(false);
  };

  const handleUpgradeNow = async () => {
    setUpgrading(true);
    setUpgradeMessage(t('updateDialog.updating'));
    try {
      const result = await api.upgradeApp();
      if (result.devMode) {
        setUpgradeMessage(t('updateDialog.restartDev'));
        await new Promise((resolve) => setTimeout(resolve, 900));
        await Promise.all([
          refetch(),
          queryClient.invalidateQueries({ queryKey: queryKeys.versionCheck }),
        ]);
        setUpgrading(false);
        return;
      }
      setUpgradeMessage(t('updateDialog.restarting'));
      await api.restartApp({ clearCache: true });
      void waitForRestartThenReload();
    } catch (err) {
      setUpgradeMessage((err as Error).message);
      setUpgrading(false);
    }
  };

  if (isPending) return <PageSkeleton />;

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader
          title={t('doctor.title')}
          icon={<Stethoscope size={28} strokeWidth={2.5} />}
        />
        <Card>
          <div className="text-danger text-sm">
            {t('doctor.error.failedToLoad', { error: error instanceof Error ? error.message : t('common.unknownError') })}
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('doctor.title')}
        icon={<Stethoscope size={28} strokeWidth={2.5} />}
        subtitle={t('doctor.subtitle')}
        actions={
          <Button
            variant="secondary"
            size="sm"
            onClick={() => refetch()}
            loading={isFetching}
          >
            <RefreshCw size={14} strokeWidth={2.5} />
            {t('doctor.recheck')}
          </Button>
        }
      />

      {/* Summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: `${palette.success}18` }}>
              <CheckCircle2 size={20} strokeWidth={2.5} style={{ color: palette.success }} />
            </div>
            <div>
              <div className="text-2xl font-bold text-pencil">{data!.summary.pass}</div>
              <div className="text-sm text-pencil-light">{t('doctor.summary.passed')}</div>
            </div>
          </div>
        </Card>
        <Card>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: `${palette.warning}18` }}>
              <AlertTriangle size={20} strokeWidth={2.5} style={{ color: palette.warning }} />
            </div>
            <div>
              <div className="text-2xl font-bold text-pencil">{data!.summary.warnings}</div>
              <div className="text-sm text-pencil-light">{t('doctor.summary.warnings')}</div>
            </div>
          </div>
        </Card>
        <Card>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: `${palette.danger}18` }}>
              <XCircle size={20} strokeWidth={2.5} style={{ color: palette.danger }} />
            </div>
            <div>
              <div className="text-2xl font-bold text-pencil">{data!.summary.errors}</div>
              <div className="text-sm text-pencil-light">{t('doctor.summary.errors')}</div>
            </div>
          </div>
        </Card>
      </div>

      {/* All passed banner */}
      {allPassed && (
        <Card className="!bg-success-light border-success/30">
          <div className="flex items-center gap-3">
            <PartyPopper size={22} strokeWidth={2.5} style={{ color: palette.success }} />
            <div>
              <div className="font-semibold text-pencil">{t('doctor.allPassed.title')}</div>
              <div className="text-sm text-pencil-light">
                {t('doctor.allPassed.message', { count: data!.summary.total })}
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Repair entry points */}
      <Card>
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div
            className="shrink-0 w-10 h-10 rounded-full flex items-center justify-center"
            style={{ backgroundColor: `${palette.info}18` }}
          >
            <PackagePlus size={20} strokeWidth={2.5} style={{ color: palette.info }} />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm font-medium text-pencil">{t('doctor.adopt.title')}</span>
              <Badge variant="info" size="sm" dot>{t('doctor.adopt.badge')}</Badge>
            </div>
            <p className="text-sm text-pencil-light mt-1">{t('doctor.adopt.description')}</p>
          </div>
          <div className="shrink-0 sm:self-center">
            <Button variant="secondary" size="sm" onClick={() => navigate('/adopt')}>
              <PackagePlus size={14} strokeWidth={2.5} />
              {t('doctor.adopt.action')}
              <ArrowRight size={14} strokeWidth={2.5} />
            </Button>
          </div>
        </div>
      </Card>

      {/* Filter toggles */}
      <SegmentedControl<StatusFilter>
        value={filter}
        onChange={setFilter}
        options={[
          { value: 'all', label: t('doctor.filter.all'), count: data!.summary.total },
          { value: 'error', label: t('doctor.filter.error'), count: data!.summary.errors },
          { value: 'warning', label: t('doctor.filter.warning'), count: data!.summary.warnings },
          { value: 'pass', label: t('doctor.filter.pass'), count: data!.summary.pass },
        ]}
      />

      {/* Checks list */}
      <Card padding="none">
        {filteredChecks.length === 0 ? (
          <div className="py-8 text-center text-pencil-light text-sm">
            {t('doctor.filter.noMatch')}
          </div>
        ) : (
          filteredChecks.map((check, i) => (
            <CheckRow key={`${check.name}-${i}`} check={check} />
          ))
        )}
      </Card>

      {/* Version info */}
      {data!.version && (() => {
        const updateAvailable = data!.version.update_available;
        const messageIsError = upgradeMessage && (upgradeMessage.includes('failed') || upgradeMessage.includes('失敗'));
        const messageTone = upgrading ? 'progress' : messageIsError ? 'error' : 'success';
        const messageColor = palette[updateAvailable ? 'info' : 'success'];
        return (
          <Card>
            <div className="flex items-start gap-4">
              {/* Status icon — semantic anchor, mirrors Summary card style */}
              <div
                className="shrink-0 w-10 h-10 rounded-full flex items-center justify-center"
                style={{ backgroundColor: `${messageColor}18` }}
              >
                {updateAvailable ? (
                  <ArrowUpCircle size={20} strokeWidth={2.5} style={{ color: messageColor }} />
                ) : (
                  <CheckCircle2 size={20} strokeWidth={2.5} style={{ color: messageColor }} />
                )}
              </div>

              {/* Content */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium text-pencil">{t('doctor.version.title')}</span>
                  {updateAvailable ? (
                    <Badge variant="info" size="sm" dot>{t('doctor.version.updateAvailable')}</Badge>
                  ) : (
                    <Badge variant="success" size="sm" dot>{t('dashboard.version.upToDate')}</Badge>
                  )}
                </div>

                <div className="flex items-center gap-1.5 text-sm mt-1 flex-wrap">
                  <span className="font-mono text-pencil-light">{data!.version.current}</span>
                  {data!.version.latest && data!.version.latest !== data!.version.current && (
                    <>
                      <ChevronRight size={14} className="text-pencil-light shrink-0" />
                      <span className="font-mono font-semibold text-pencil">{data!.version.latest}</span>
                    </>
                  )}
                </div>

                {upgradeMessage && (
                  <p
                    className={`mt-2 inline-flex items-center gap-1.5 text-sm ${
                      messageTone === 'progress' ? 'text-pencil-light'
                        : messageTone === 'error' ? 'text-danger'
                        : 'text-success'
                    }`}
                  >
                    {messageTone === 'success' && <CheckCircle2 size={14} strokeWidth={2.5} />}
                    {messageTone === 'error' && <XCircle size={14} strokeWidth={2.5} />}
                    {upgradeMessage}
                  </p>
                )}
              </div>

              {/* CTA — only show when an update is actually available */}
              {updateAvailable && (
                <div className="shrink-0 self-center">
                  <Button variant="primary" size="sm" onClick={handleUpgradeNow} loading={upgrading}>
                    <ArrowUpCircle size={14} strokeWidth={2.5} />
                    {t('updateDialog.updateNow')}
                  </Button>
                </div>
              )}
            </div>
          </Card>
        );
      })()}
    </div>
  );
}
