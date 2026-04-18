import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useT } from '../i18n';
import {
  Puzzle,
  Target,
  ArrowRight,
  RefreshCw,
  Star,
  X,
  Download,
  GitBranch,
  AlertTriangle,
  Trash2,
  Package,
  Zap,
  ShieldCheck,
  ShieldAlert,
  FolderPlus,
  LayoutDashboard,
  Bot,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Card from '../components/Card';
import Badge from '../components/Badge';
import Button from '../components/Button';
import IconButton from '../components/IconButton';
import Skeleton from '../components/Skeleton';
import { PageSkeleton } from '../components/Skeleton';
import StatusBadge from '../components/StatusBadge';
import PageHeader from '../components/PageHeader';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { Target as TargetType, CheckResult, AuditAllResponse, Extra } from '../api/client';
import { radius, shadows } from '../design';
import { clearAuditCache } from '../lib/auditCache';
import { formatSkillDisplayName } from '../lib/resourceNames';

const STAR_CTA_DISMISSED_KEY = 'skillshare.dashboard.starCta.dismissed';

export default function DashboardPage() {
  const t = useT();
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => api.getOverview(),
    staleTime: staleTimes.overview,
  });
  const { data: extrasData } = useQuery({
    queryKey: queryKeys.extras,
    queryFn: () => api.listExtras(),
    staleTime: staleTimes.extras,
  });
  const queryClient = useQueryClient();
  const [updatingAll, setUpdatingAll] = useState(false);
  const [showUpdateConfirm, setShowUpdateConfirm] = useState(false);
  const [showStarCta, setShowStarCta] = useState(() => {
    if (typeof window === 'undefined') return true;
    return window.localStorage.getItem(STAR_CTA_DISMISSED_KEY) !== '1';
  });
  const { toast } = useToast();

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          {t("dashboard.error.title")}
        </p>
        <p className="text-pencil-light text-sm mt-1">{error.message}</p>
      </Card>
    );
  }
  if (!data) return null;

  const handleUpdateAll = async () => {
    setUpdatingAll(true);
    try {
      const res = await api.update({ all: true });
      const updated = res.results.filter((r) => r.action === 'updated').length;
      const upToDate = res.results.filter((r) => r.action === 'up-to-date').length;
      const errors = res.results.filter((r) => r.action === 'error');
      const blocked = res.results.filter((r) => r.action === 'blocked');
      if (res.results.length === 0) {
        toast(t("dashboard.toast.noTrackedRepos"), 'info');
      } else {
        const parts = [`${updated} updated`, `${upToDate} up-to-date`];
        if (blocked.length > 0) parts.push(`${blocked.length} blocked`);
        toast(t("dashboard.toast.updateComplete", { summary: parts.join(', ') }), blocked.length > 0 ? 'warning' : updated > 0 ? 'success' : 'info');
      }
      const allUpdateErrors = [
        ...blocked.map((r) => `${formatSkillDisplayName(r.name)}: ${r.message}`),
        ...errors.map((r) => `${formatSkillDisplayName(r.name)}: ${r.message}`),
      ];
      if (allUpdateErrors.length > 0) {
        const key = allUpdateErrors.length !== 1 ? "dashboard.toast.issueCount" : "dashboard.toast.issueCountSingular";
        toast(t(key, { count: allUpdateErrors.length, details: allUpdateErrors.join('; ') }), 'error');
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setUpdatingAll(false);
    }
  };

  const dismissStarCta = () => {
    setShowStarCta(false);
    if (typeof window === 'undefined') return;
    window.localStorage.setItem(STAR_CTA_DISMISSED_KEY, '1');
  };

  const totalExtraFiles = extrasData?.extras?.reduce((sum: number, e: Extra) => sum + e.file_count, 0) ?? 0;
  const totalExtraTargets = extrasData?.extras?.reduce((sum: number, e: Extra) => sum + e.targets.length, 0) ?? 0;

  const stats = [
    {
      label: t("dashboard.stats.skills"),
      value: data.skillCount,
      subtitle: t("dashboard.stats.topLevel", { count: data.topLevelCount }),
      icon: Puzzle,
      color: 'text-blue',
      bg: 'bg-info-light',
      to: '/resources?tab=skills',
    },
    {
      label: t("dashboard.stats.agents"),
      value: data.agentCount,
      subtitle: t("dashboard.stats.installed"),
      icon: Bot,
      color: 'text-accent',
      bg: 'bg-accent/10',
      to: '/resources?tab=agents',
    },
    {
      label: t("dashboard.stats.targets"),
      value: data.targetCount,
      subtitle: t("dashboard.stats.configured"),
      icon: Target,
      color: 'text-success',
      bg: 'bg-success-light',
      to: '/targets',
    },
    {
      label: t("dashboard.stats.extras"),
      value: extrasData?.extras?.length ?? 0,
      subtitle: t("dashboard.stats.extrasSubtitle", { files: totalExtraFiles, targets: totalExtraTargets }),
      icon: FolderPlus,
      color: 'text-lime-600',
      bg: 'bg-lime-100',
      to: '/extras',
    },
  ];

  return (
    <div className="animate-fade-in">
      <PageHeader icon={<LayoutDashboard size={24} strokeWidth={2.5} />} title={t("dashboard.title")} subtitle={t("dashboard.subtitle")} />

      {/* Stats grid */}
      <div data-tour="stats-grid" className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 mb-8">
        {stats.map(({ label, value, subtitle, icon: Icon, color, bg, to }) => (
          <Link key={label} to={to}>
            <Card
              hover
              className="h-full"
            >
              <div className="flex items-start gap-3">
                <div
                  className={`w-11 h-11 ${bg} border-2 border-pencil flex items-center justify-center shrink-0`}
                  style={{ borderRadius: '50%' }}
                >
                  <Icon size={20} strokeWidth={2.5} className={color} />
                </div>
                <div className="min-w-0">
                  <p
                    className="text-sm text-pencil-light uppercase tracking-wider"
                  >
                    {label}
                  </p>
                  <p
                    className="text-2xl font-bold text-pencil leading-tight"
                  >
                    {value}
                  </p>
                  <p className="text-sm text-muted-dark">{subtitle}</p>
                </div>
              </div>
            </Card>
          </Link>
        ))}
      </div>

      {/* Source path card */}
      <Card className="mb-8">
        <h3 className="text-lg font-bold text-pencil mb-3">
          {t("dashboard.source.title")}
        </h3>
        <div className="space-y-2.5">
          <SourceRow label={t("dashboard.stats.skills")} path={data.source} />
          {data.agentsSource && <SourceRow label={t("dashboard.stats.agents")} path={data.agentsSource} />}
          {data.extrasSource && <SourceRow label={t("dashboard.stats.extras")} path={data.extrasSource} />}
        </div>
        <p className="text-sm text-muted-dark mt-3">
          {t("dashboard.source.allTargetsSyncFrom")}
        </p>
      </Card>

      {/* Support CTA */}
      {showStarCta && (
        <Card className="mb-8">
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-start gap-3">
              <div
                className="w-10 h-10 bg-warning-light border-2 border-pencil flex items-center justify-center shrink-0"
                style={{ borderRadius: '50%' }}
              >
                <Star size={18} strokeWidth={2.5} className="text-warning" />
              </div>
              <div>
                <h3
                  className="text-lg font-bold text-pencil"
                >
                  {t("dashboard.starCta.enjoying")}
                </h3>
                <p className="text-sm text-pencil-light mt-1">
                  {t("dashboard.starCta.message")}
                  {' '}
                  <a
                    href="https://github.com/runkids/skillshare"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-blue hover:underline"
                  >
                    github.com/runkids/skillshare ⭐
                  </a>
                </p>
              </div>
            </div>
            <IconButton
              icon={<X size={16} strokeWidth={2.5} />}
              label={t("dashboard.starCta.dismiss")}
              size="sm"
              variant="ghost"
              onClick={dismissStarCta}
            />
          </div>
        </Card>
      )}

      {/* Tracked Repositories */}
      {data.trackedRepos && data.trackedRepos.length > 0 && (
        <TrackedReposSection repos={data.trackedRepos} />
      )}

      {/* Skill Updates Check */}
      <SkillUpdatesSection />

      {/* Security Audit */}
      <SecurityAuditSection />

      {/* Targets Health */}
      <TargetsHealthSection />

      {/* Version Status */}
      <VersionStatusSection />

      {/* Quick actions */}
      <div data-tour="quick-actions" className="mb-4">
        <h3
          className="text-xl font-bold text-pencil mb-4"
        >
          {t("dashboard.quickActions.title")}
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <Link to="/sync" className="h-full">
            <div
              className="flex items-center gap-3 px-5 py-4 h-full bg-paper border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer group"
              style={{
                borderRadius: radius.md,
                boxShadow: shadows.md,
              }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.hover;
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.md;
              }}
            >
              <RefreshCw
                size={22}
                strokeWidth={2.5}
                className="text-pencil group-hover:animate-spin"
              />
              <div className="flex-1">
                <p className="font-medium text-pencil">
                  {t("dashboard.quickActions.syncNow")}
                </p>
                <p className="text-sm text-pencil-light">{t("dashboard.quickActions.syncNow.description")}</p>
              </div>
              <ArrowRight size={16} className="text-pencil-light" />
            </div>
          </Link>

          <Link to="/audit" className="h-full">
            <div
              className="flex items-center gap-3 px-5 py-4 h-full bg-info-light border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer group"
              style={{
                borderRadius: radius.md,
                boxShadow: shadows.md,
              }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.hover;
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.md;
              }}
            >
              <ShieldCheck size={22} strokeWidth={2.5} className="text-blue group-hover:animate-pulse" />
              <div className="flex-1">
                <p className="font-medium text-pencil">
                  {t("dashboard.quickActions.securityAudit")}
                </p>
                <p className="text-sm text-pencil-light">{t("dashboard.quickActions.securityAudit.description")}</p>
              </div>
              <ArrowRight size={16} className="text-pencil-light" />
            </div>
          </Link>

          <Link to="/resources" className="h-full">
            <div
              className="flex items-center gap-3 px-5 py-4 h-full bg-success-light border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer group"
              style={{
                borderRadius: radius.md,
                boxShadow: shadows.md,
              }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.hover;
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.md;
              }}
            >
              <Puzzle size={22} strokeWidth={2.5} className="text-success group-hover:animate-bounce" />
              <div className="flex-1">
                <p className="font-medium text-pencil">
                  {t("dashboard.quickActions.browseSkills")}
                </p>
                <p className="text-sm text-pencil-light">{t("dashboard.quickActions.browseSkills.description")}</p>
              </div>
              <ArrowRight size={16} className="text-pencil-light" />
            </div>
          </Link>

          <button
            onClick={() => setShowUpdateConfirm(true)}
            disabled={updatingAll}
            className="text-left w-full h-full"
          >
            <div
              className="flex items-center gap-3 px-5 py-4 h-full bg-warning-light border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer group"
              style={{
                borderRadius: radius.md,
                boxShadow: shadows.md,
                opacity: updatingAll ? 0.6 : 1,
              }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.hover;
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.md;
              }}
            >
              <Download
                size={22}
                strokeWidth={2.5}
                className={`text-warning ${updatingAll ? 'animate-bounce' : 'group-hover:animate-bounce'}`}
              />
              <div className="flex-1">
                <p className="font-medium text-pencil">
                  {updatingAll ? t("dashboard.quickActions.updating") : t("dashboard.quickActions.updateAll")}
                </p>
                <p className="text-sm text-pencil-light">{t("dashboard.quickActions.updateAll.description")}</p>
              </div>
              {!updatingAll && <ArrowRight size={16} className="text-pencil-light" />}
            </div>
          </button>

          <ConfirmDialog
            open={showUpdateConfirm}
            onConfirm={() => {
              setShowUpdateConfirm(false);
              handleUpdateAll();
            }}
            onCancel={() => setShowUpdateConfirm(false)}
            title={t("dashboard.quickActions.updateAll.confirm.title")}
            message={t("dashboard.quickActions.updateAll.confirm.message")}
            confirmText={t("dashboard.trackedRepos.update")}
            cancelText={t("common.cancel")}
          />
        </div>
      </div>

      {/* Decorative hand-drawn divider */}
      <div className="mt-8 flex justify-center">
        <svg width="120" height="20" viewBox="0 0 120 20" className="text-muted-dark">
          <path
            d="M5 10 Q20 2 35 10 Q50 18 65 10 Q80 2 95 10 Q110 18 115 10"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
          />
        </svg>
      </div>
    </div>
  );
}

function SourceRow({ label, path }: { label: string; path: string }) {
  return (
    <div className="flex items-baseline gap-3">
      <span className="text-xs font-bold uppercase tracking-wider text-pencil-light w-12 shrink-0">{label}</span>
      <span className="font-mono text-sm text-pencil break-all">{path}</span>
    </div>
  );
}

/* -- Tracked Repositories Section --------------------- */

function TrackedReposSection({ repos }: { repos: { name: string; skillCount: number; dirty: boolean }[] }) {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [updatingRepos, setUpdatingRepos] = useState<Set<string>>(new Set());
  const [repoToDelete, setRepoToDelete] = useState<string | null>(null);
  const [deletingRepos, setDeletingRepos] = useState<Set<string>>(new Set());

  const invalidateRepoData = async () => {
    clearAuditCache(queryClient);
    await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    await queryClient.invalidateQueries({ queryKey: queryKeys.trash });
  };

  const handleUpdateRepo = async (repoName: string) => {
    setUpdatingRepos((prev) => new Set(prev).add(repoName));
    try {
      const res = await api.update({ name: repoName });
      const item = res.results[0];
      const displayName = repoName.replace(/^_/, '');

      if (item?.action === 'updated') {
        toast(t("dashboard.toast.repoUpdated", { name: displayName, message: item.message ?? 'done' }), 'success');
      } else if (item?.action === 'up-to-date') {
        toast(t("dashboard.toast.repoAlreadyUpToDate", { name: displayName }), 'info');
      } else if (item?.action === 'blocked') {
        toast(item.message ?? t("dashboard.toast.updateBlockedFor", { name: displayName }), 'error');
      } else if (item?.action === 'error') {
        toast(item.message ?? t("dashboard.toast.updateFailedFor", { name: displayName }), 'error');
      } else {
        toast(item?.message ?? t("dashboard.toast.repoSkipped", { name: displayName }), 'warning');
      }

      await invalidateRepoData();
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setUpdatingRepos((prev) => {
        const next = new Set(prev);
        next.delete(repoName);
        return next;
      });
    }
  };

  const handleDeleteRepo = async () => {
    if (!repoToDelete) return;

    const targetRepo = repoToDelete;
    setDeletingRepos((prev) => new Set(prev).add(targetRepo));
    try {
      const displayName = targetRepo.replace(/^_/, '');
      await api.deleteRepo(targetRepo);
      toast(t("dashboard.toast.repoUninstalled", { name: displayName }), 'success');
      setRepoToDelete(null);
      await invalidateRepoData();
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setDeletingRepos((prev) => {
        const next = new Set(prev);
        next.delete(targetRepo);
        return next;
      });
    }
  };

  return (
    <>
      <Card className="mb-8">
        <div className="flex items-center gap-2 mb-4">
          <GitBranch size={20} strokeWidth={2.5} className="text-blue" />
          <h3
            className="text-lg font-bold text-pencil"
          >
            {t("dashboard.trackedRepos.title")}
          </h3>
        </div>
        <div className="space-y-3">
          {repos.map((repo) => {
            const displayName = repo.name.replace(/^_/, '');
            const isUpdating = updatingRepos.has(repo.name);
            const isDeleting = deletingRepos.has(repo.name);
            const hasAnyDeleteInProgress = deletingRepos.size > 0;
            const isBusy = isUpdating || isDeleting;

            return (
              <div
                key={repo.name}
                className="flex flex-col gap-3 py-3 px-3 bg-paper-warm border border-muted md:flex-row md:items-center md:justify-between"
                style={{ borderRadius: radius.sm }}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <GitBranch size={16} className="text-pencil-light shrink-0" />
                  <span
                    className="font-medium text-pencil truncate"
                  >
                    {displayName}
                  </span>
                  <Badge variant="info">{t("dashboard.trackedRepos.skillCount", { count: repo.skillCount })}</Badge>
                  {repo.dirty ? (
                    <Badge variant="warning" dot>{t("dashboard.trackedRepos.modified")}</Badge>
                  ) : (
                    <Badge variant="default">{t("dashboard.trackedRepos.clean")}</Badge>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-2 shrink-0">
                  <Button
                    variant="secondary"
                    size="xs"
                    onClick={() => handleUpdateRepo(repo.name)}
                    loading={isUpdating}
                    disabled={hasAnyDeleteInProgress}
                  >
                    <RefreshCw size={12} />
                    {t("dashboard.trackedRepos.update")}
                  </Button>
                  <Button
                    variant="danger"
                    size="xs"
                    onClick={() => setRepoToDelete(repo.name)}
                    disabled={isBusy || repoToDelete !== null || updatingRepos.size > 0}
                  >
                    <Trash2 size={12} />
                    {t("dashboard.trackedRepos.uninstall")}
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      </Card>

      <ConfirmDialog
        open={repoToDelete !== null}
        title={t("dashboard.trackedRepos.uninstallConfirm.title")}
        message={
          repoToDelete
            ? t("dashboard.trackedRepos.uninstallConfirm.message", { name: repoToDelete.replace(/^_/, '') })
            : ''
        }
        confirmText={t("dashboard.trackedRepos.uninstall")}
        variant="danger"
        loading={repoToDelete !== null && deletingRepos.has(repoToDelete)}
        onConfirm={handleDeleteRepo}
        onCancel={() => {
          if (repoToDelete === null || !deletingRepos.has(repoToDelete)) setRepoToDelete(null);
        }}
      />
    </>
  );
}

/* -- Skill Updates Section ---------------------------- */

function SkillUpdatesSection() {
  const t = useT();
  const [checkData, setCheckData] = useState<CheckResult | null>(null);
  const [checking, setChecking] = useState(false);
  const [checked, setChecked] = useState(false);
  const { toast } = useToast();

  const handleCheck = async () => {
    setChecking(true);
    try {
      const result = await api.check();
      setCheckData(result);
      setChecked(true);
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setChecking(false);
    }
  };

  const updatableCount = checkData
    ? checkData.tracked_repos.filter((r) => r.status === 'behind').length +
      checkData.skills.filter((s) => s.status === 'update_available').length
    : 0;

  return (
    <Card className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Download size={20} strokeWidth={2.5} className="text-blue" />
          <h3
            className="text-lg font-bold text-pencil"
          >
            {t("dashboard.updates.title")}
          </h3>
          {checked && updatableCount > 0 && (
            <Badge variant="warning">{t("dashboard.updates.available", { count: updatableCount })}</Badge>
          )}
          {checked && updatableCount === 0 && (
            <Badge variant="success">{t("dashboard.updates.allUpToDate")}</Badge>
          )}
        </div>
        <Button variant="link" onClick={handleCheck} disabled={checking}>
          {checking ? t("dashboard.updates.checking") : checked ? t("dashboard.updates.recheck") : t("dashboard.updates.runCheck")}
        </Button>
      </div>

      {!checked && !checking && (
        <p className="text-pencil-light text-sm">
          {t("dashboard.updates.runCheckDescription")}
        </p>
      )}

      {checking && (
        <div className="space-y-3">
          <Skeleton className="w-full h-8" />
          <Skeleton className="w-3/4 h-8" />
        </div>
      )}

      {checked && checkData && (
        <div className="space-y-2">
          {checkData.tracked_repos.map((repo) => (
            <div
              key={repo.name}
              className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
              style={{ borderRadius: radius.sm }}
            >
              <div className="flex items-center gap-2">
                <GitBranch size={14} className="text-pencil-light" />
                <span className="text-pencil text-sm">
                  {repo.name.replace(/^_/, '')}
                </span>
              </div>
              {repo.status === 'up_to_date' && <Badge variant="success">{t("dashboard.updates.status.upToDate")}</Badge>}
              {repo.status === 'behind' && <Badge variant="warning">{t("dashboard.updates.status.behind", { count: repo.behind })}</Badge>}
              {repo.status === 'dirty' && <Badge variant="default">{t("dashboard.updates.status.modified")}</Badge>}
              {repo.status === 'error' && <Badge variant="danger">{t("dashboard.updates.status.error")}</Badge>}
            </div>
          ))}
          {checkData.skills.map((skill) => (
            <div
              key={skill.name}
              className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
              style={{ borderRadius: radius.sm }}
            >
              <div className="flex items-center gap-2">
                <Puzzle size={14} className="text-pencil-light" />
                <span className="text-pencil text-sm">
                  {skill.name}
                </span>
                {skill.source && (
                  <span className="text-xs text-muted-dark truncate max-w-[200px]">{skill.source}</span>
                )}
              </div>
              {skill.status === 'up_to_date' && <Badge variant="success">{t("dashboard.updates.status.upToDate")}</Badge>}
              {skill.status === 'update_available' && <Badge variant="warning">{t("dashboard.updates.status.updateAvailable")}</Badge>}
              {skill.status === 'local' && <Badge variant="default">{t("dashboard.updates.status.local")}</Badge>}
              {skill.status === 'error' && <Badge variant="danger">{t("dashboard.updates.status.error")}</Badge>}
            </div>
          ))}
          {checkData.tracked_repos.length === 0 && checkData.skills.length === 0 && (
            <p className="text-pencil-light text-sm">{t("dashboard.updates.noTrackedRepos")}</p>
          )}
        </div>
      )}
    </Card>
  );
}

/* -- Security Audit Section --------------------------- */

const riskLabelVariant: Record<string, 'success' | 'default' | 'info' | 'warning' | 'danger'> = {
  clean: 'success',
  low: 'default',
  medium: 'info',
  high: 'warning',
  critical: 'danger',
};

function SecurityAuditSection() {
  const t = useT();
  const [auditData, setAuditData] = useState<AuditAllResponse | null>(null);
  const [scanning, setScanning] = useState(false);
  const [scanned, setScanned] = useState(false);
  const { toast } = useToast();

  const handleScan = async () => {
    setScanning(true);
    try {
      const result = await api.auditAll();
      setAuditData(result);
      setScanned(true);
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setScanning(false);
    }
  };

  const hasCritical = scanned && auditData && auditData.summary.critical > 0;
  const hasFindings = scanned && auditData && (
    auditData.summary.critical + auditData.summary.high + auditData.summary.medium +
    auditData.summary.low + auditData.summary.info
  ) > 0;
  const ShieldIcon = hasCritical ? ShieldAlert : ShieldCheck;

  const severityCounts: { label: string; count: number; variant: 'danger' | 'warning' | 'info' | 'default' }[] = scanned && auditData
    ? [
        { label: 'CRITICAL', count: auditData.summary.critical, variant: 'danger' },
        { label: 'HIGH', count: auditData.summary.high, variant: 'warning' },
        { label: 'MEDIUM', count: auditData.summary.medium, variant: 'info' },
        { label: 'LOW', count: auditData.summary.low, variant: 'default' },
        { label: 'INFO', count: auditData.summary.info, variant: 'default' },
      ]
    : [];

  return (
    <Card variant={hasCritical ? 'accent' : 'default'} className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <ShieldIcon
            size={20}
            strokeWidth={2.5}
            className={hasCritical ? 'text-danger' : 'text-blue'}
          />
          <h3
            className="text-lg font-bold text-pencil"
          >
            {t("dashboard.security.title")}
          </h3>
          {scanned && auditData && (
            <Badge variant={riskLabelVariant[auditData.summary.riskLabel] ?? 'default'}>
              {auditData.summary.riskLabel}
            </Badge>
          )}
        </div>
        <Link to="/audit" className="text-sm text-blue hover:underline">
          {scanned ? t("dashboard.security.viewDetails") : t("dashboard.security.runScan")}
        </Link>
      </div>

      {!scanned && !scanning && (
        <div className="flex items-center justify-between">
          <p className="text-pencil-light text-sm">
            {t("dashboard.security.scanDescription")}
          </p>
          <Button variant="link" onClick={handleScan} className="shrink-0 ml-4">
            {t("dashboard.security.quickScan")}
          </Button>
        </div>
      )}

      {scanning && (
        <div className="space-y-3">
          <Skeleton className="w-full h-8" />
          <Skeleton className="w-3/4 h-8" />
        </div>
      )}

      {scanned && auditData && (
        <div className="space-y-4">
          {/* Summary stats row */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div
              className="py-2 px-3 bg-paper-warm border border-muted text-center"
              style={{ borderRadius: radius.sm }}
            >
              <p className="text-lg font-bold text-pencil">
                {auditData.summary.total}
              </p>
              <p className="text-xs text-pencil-light">{t("dashboard.security.scanned")}</p>
            </div>
            <div
              className="py-2 px-3 bg-paper-warm border border-muted text-center"
              style={{ borderRadius: radius.sm }}
            >
              <p className="text-lg font-bold text-success">
                {auditData.summary.passed}
              </p>
              <p className="text-xs text-pencil-light">{t("dashboard.security.passed")}</p>
            </div>
            <div
              className="py-2 px-3 bg-paper-warm border border-muted text-center"
              style={{ borderRadius: radius.sm }}
            >
              <p className="text-lg font-bold text-warning">
                {auditData.summary.warning}
              </p>
              <p className="text-xs text-pencil-light">{t("dashboard.security.warnings")}</p>
            </div>
            <div
              className={`py-2 px-3 bg-paper-warm border text-center ${auditData.summary.failed > 0 ? 'border-danger' : 'border-muted'}`}
              style={{ borderRadius: radius.sm }}
            >
              <p
                className={`text-lg font-bold ${auditData.summary.failed > 0 ? 'text-danger' : 'text-pencil'}`}
              >
                {auditData.summary.failed}
              </p>
              <p className="text-xs text-pencil-light">{t("dashboard.security.failed")}</p>
            </div>
          </div>

          {/* Severity breakdown */}
          {hasFindings ? (
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm text-pencil-light">
                {t("dashboard.security.findings")}
              </span>
              {severityCounts
                .filter((s) => s.count > 0)
                .map((s) => (
                  <Badge key={s.label} variant={s.variant}>
                    {s.count} {s.label}
                  </Badge>
                ))}
            </div>
          ) : (
            <div className="flex items-center gap-2 text-success">
              <ShieldCheck size={16} strokeWidth={2.5} />
              <span className="text-sm font-medium">
                {t("dashboard.security.allClear")}
              </span>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

/* -- Targets Health Section --------------------------- */

function TargetsHealthSection() {
  const t = useT();
  const { data, isPending } = useQuery({
    queryKey: queryKeys.targets.all,
    queryFn: () => api.listTargets(),
    staleTime: staleTimes.targets,
  });

  const sourceSkillCount = data?.sourceSkillCount ?? 0;
  const driftTargets = (data?.targets ?? []).filter(
    (tgt) => {
      const expected = tgt.expectedSkillCount || sourceSkillCount;
      return (tgt.mode === 'merge' && tgt.status === 'merged' || tgt.mode === 'copy' && tgt.status === 'copied') && tgt.linkedCount < expected;
    }
  );
  const maxDrift = driftTargets.reduce(
    (max, tgt) => Math.max(max, (tgt.expectedSkillCount || sourceSkillCount) - tgt.linkedCount),
    0
  );

  return (
    <Card className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Target size={20} strokeWidth={2.5} className="text-success" />
          <h3
            className="text-lg font-bold text-pencil"
          >
            {t("dashboard.targets.title")}
          </h3>
          {maxDrift > 0 && (
            <Badge variant="warning">{t("dashboard.targets.notSynced", { count: maxDrift })}</Badge>
          )}
        </div>
        <Link to="/targets" className="text-sm text-blue hover:underline">
          {t("dashboard.targets.viewAll")}
        </Link>
      </div>
      {isPending ? (
        <div className="space-y-3">
          <Skeleton className="w-full h-10" />
          <Skeleton className="w-full h-10" />
          <Skeleton className="w-3/4 h-10" />
        </div>
      ) : data?.targets && data.targets.length > 0 ? (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {data.targets.map((tgt: TargetType) => {
              const expected = tgt.expectedSkillCount || sourceSkillCount;
              const hasDrift = (tgt.mode === 'merge' && tgt.status === 'merged' || tgt.mode === 'copy' && tgt.status === 'copied') && tgt.linkedCount < expected;
              return (
                <Link key={tgt.name} to="/targets">
                  <div
                    className={`flex items-center justify-between py-2 px-3 bg-paper-warm border ${hasDrift ? 'border-warning' : 'border-muted'} hover:border-pencil-light transition-colors`}
                    style={{ borderRadius: radius.sm }}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <Target size={14} className="text-pencil-light shrink-0" />
                      <span
                        className="font-medium text-pencil truncate"
                      >
                        {tgt.name}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 shrink-0 ml-2">
                      <StatusBadge status={tgt.status} />
                      {hasDrift ? (
                        <Badge variant="warning">{t("dashboard.targets.synced", { linked: tgt.linkedCount, expected })}</Badge>
                      ) : tgt.linkedCount > 0 ? (
                        <span className="text-xs text-muted-dark">{tgt.mode === 'copy' ? t("dashboard.targets.managed", { count: tgt.linkedCount }) : t("dashboard.targets.linked", { count: tgt.linkedCount })}</span>
                      ) : null}
                    </div>
                  </div>
                </Link>
              );
            })}
          </div>
          {maxDrift > 0 && (
            <div className="mt-3 flex items-center gap-2 text-warning text-sm">
              <AlertTriangle size={14} strokeWidth={2.5} />
              <span>{t("dashboard.targets.driftWarning", { count: maxDrift })} <Link to="/sync" className="underline hover:text-pencil">{t("dashboard.targets.goToSync")}</Link></span>
            </div>
          )}
        </>
      ) : (
        <p className="text-pencil-light text-sm">{t("dashboard.targets.noTargets")}</p>
      )}
    </Card>
  );
}

/* -- Version Status Section --------------------------- */

function VersionStatusSection() {
  const t = useT();
  const { data, isPending } = useQuery({
    queryKey: queryKeys.versionCheck,
    queryFn: () => api.getVersionCheck(),
    staleTime: staleTimes.version,
  });

  return (
    <Card className="mb-8">
      <div className="flex items-center gap-2 mb-4">
        <Package size={20} strokeWidth={2.5} className="text-pencil-light" />
        <h3
          className="text-lg font-bold text-pencil"
        >
          {t("dashboard.version.title")}
        </h3>
      </div>
      {isPending ? (
        <div className="space-y-3">
          <Skeleton className="w-full h-8" />
          <Skeleton className="w-3/4 h-8" />
        </div>
      ) : data ? (
        <div className="space-y-3">
          {/* CLI Version */}
          <div
            className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
            style={{ borderRadius: radius.sm }}
          >
            <div className="flex items-center gap-2">
              <Zap size={14} className="text-pencil-light" />
              <span className="text-pencil text-sm">
                {t("dashboard.version.cli")}
              </span>
              <span
                className="font-mono font-medium text-pencil"
                style={{ fontSize: '0.85rem' }}
              >
                {data.cliVersion}
              </span>
            </div>
            {data.cliUpdateAvailable ? (
              <Badge variant="warning">{t("dashboard.version.update", { version: data.cliLatest })}</Badge>
            ) : (
              <Badge variant="success">{t("dashboard.version.upToDate")}</Badge>
            )}
          </div>

          {/* Skill Version */}
          <div
            className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
            style={{ borderRadius: radius.sm }}
          >
            <div className="flex items-center gap-2">
              <Puzzle size={14} className="text-pencil-light" />
              <span className="text-pencil text-sm">
                {t("dashboard.version.skill")}
              </span>
              <span
                className="font-mono font-medium text-pencil"
                style={{ fontSize: '0.85rem' }}
              >
                {data.skillVersion || 'N/A'}
              </span>
            </div>
            {data.skillVersion ? (
              data.skillUpdateAvailable ? (
                <Badge variant="warning">{t("dashboard.version.update", { version: data.skillLatest })}</Badge>
              ) : data.skillLatest ? (
                <Badge variant="success">{t("dashboard.version.upToDate")}</Badge>
              ) : (
                <Badge variant="default">{t("dashboard.version.checkFailed")}</Badge>
              )
            ) : (
              <Badge variant="default">{t("dashboard.version.notInstalled")}</Badge>
            )}
          </div>
        </div>
      ) : (
        <p className="text-pencil-light text-sm">{t("dashboard.version.couldNotCheck")}</p>
      )}
    </Card>
  );
}
