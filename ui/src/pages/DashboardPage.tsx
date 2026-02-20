import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Puzzle,
  Target,
  FolderSync,
  Info,
  ArrowRight,
  RefreshCw,
  Star,
  X,
  Download,
  GitBranch,
  AlertTriangle,
  Check,
  Package,
  Zap,
  ShieldCheck,
} from 'lucide-react';
import Card from '../components/Card';
import Badge from '../components/Badge';
import Skeleton from '../components/Skeleton';
import { PageSkeleton } from '../components/Skeleton';
import StatusBadge from '../components/StatusBadge';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { Target as TargetType, CheckResult, AuditAllResponse } from '../api/client';
import { useApi } from '../hooks/useApi';
import { useAppContext } from '../context/AppContext';
import { wobbly, shadows } from '../design';
import { shortenHome } from '../lib/paths';

const STAR_CTA_DISMISSED_KEY = 'skillshare.dashboard.starCta.dismissed';

export default function DashboardPage() {
  const { data, loading, error } = useApi(() => api.getOverview());
  const [updatingAll, setUpdatingAll] = useState(false);
  const [showUpdateConfirm, setShowUpdateConfirm] = useState(false);
  const [showStarCta, setShowStarCta] = useState(() => {
    if (typeof window === 'undefined') return true;
    return window.localStorage.getItem(STAR_CTA_DISMISSED_KEY) !== '1';
  });
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();

  if (loading) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Oops! Something went wrong.
        </p>
        <p className="text-pencil-light text-sm mt-1">{error}</p>
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
      if (res.results.length === 0) {
        toast('No tracked repos or updatable skills found.', 'info');
      } else {
        toast(`Update complete: ${updated} updated, ${upToDate} up-to-date.`, updated > 0 ? 'success' : 'info');
      }
      errors.forEach((r) => toast(`${r.name}: ${r.message}`, 'error'));
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

  const stats = [
    {
      label: 'Skills',
      value: data.skillCount,
      subtitle: `${data.topLevelCount} top-level`,
      icon: Puzzle,
      color: 'text-blue',
      bg: 'bg-info-light',
      to: '/skills',
    },
    {
      label: 'Targets',
      value: data.targetCount,
      subtitle: 'configured',
      icon: Target,
      color: 'text-success',
      bg: 'bg-success-light',
      to: '/targets',
    },
    {
      label: 'Sync Mode',
      value: data.mode,
      subtitle: 'current mode',
      icon: FolderSync,
      color: 'text-warning',
      bg: 'bg-warning-light',
      to: '/sync',
    },
    {
      label: 'Version',
      value: data.version,
      subtitle: 'skillshare',
      icon: Info,
      color: 'text-pencil-light',
      bg: 'bg-muted',
      to: '/config',
    },
  ];

  return (
    <div className="animate-sketch-in">
      {/* Page header */}
      <div className="mb-8">
        <h2
          className="text-3xl md:text-4xl font-bold text-pencil mb-2"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Dashboard
        </h2>
        <p className="text-pencil-light text-base">
          Your skill management overview at a glance
        </p>
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 mb-8">
        {stats.map(({ label, value, subtitle, icon: Icon, color, bg, to }, i) => (
          <Link key={label} to={to}>
            <Card
              hover
              className={i % 2 === 0 ? 'rotate-[-0.5deg]' : 'rotate-[0.5deg]'}
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
                    style={{ fontFamily: 'var(--font-hand)' }}
                  >
                    {label}
                  </p>
                  <p
                    className="text-2xl font-bold text-pencil leading-tight"
                    style={{ fontFamily: 'var(--font-heading)' }}
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
      <Card decoration="tape" className="mb-8">
        <h3
          className="text-lg font-bold text-pencil mb-2"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Source Directory
        </h3>
        <p
          className="text-base text-pencil-light break-all"
          style={{ fontFamily: "'Courier New', monospace" }}
        >
          {shortenHome(data.source)}
        </p>
        <p className="text-sm text-muted-dark mt-2">
          This is where your skills live. All targets sync from here.
        </p>
      </Card>

      {/* Support CTA */}
      {showStarCta && (
        <Card variant="postit" className="mb-8">
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
                  style={{ fontFamily: 'var(--font-heading)' }}
                >
                  Enjoying skillshare?
                </h3>
                <p className="text-sm text-pencil-light mt-1">
                  If skillshare saved you time, please give us a star on GitHub:
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
            <button
              type="button"
              onClick={dismissStarCta}
              className="shrink-0 p-1 text-pencil-light hover:text-pencil border border-transparent hover:border-muted transition-colors"
              style={{ borderRadius: wobbly.sm }}
              aria-label="Dismiss star reminder"
              title="Dismiss"
            >
              <X size={16} strokeWidth={2.5} />
            </button>
          </div>
        </Card>
      )}

      {/* Tracked Repositories (hidden in project mode) */}
      {!isProjectMode && data.trackedRepos && data.trackedRepos.length > 0 && (
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
      <div className="mb-4">
        <h3
          className="text-xl font-bold text-pencil mb-4"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Quick Actions
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <Link to="/sync" className="h-full">
            <div
              className="flex items-center gap-3 px-5 py-4 h-full bg-postit border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer group"
              style={{
                borderRadius: wobbly.md,
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
                className="text-pencil group-hover:animate-spin-slow"
              />
              <div className="flex-1">
                <p className="font-medium text-pencil" style={{ fontFamily: 'var(--font-hand)' }}>
                  Sync Now
                </p>
                <p className="text-sm text-pencil-light">Push skills to all targets</p>
              </div>
              <ArrowRight size={16} className="text-pencil-light" />
            </div>
          </Link>

          <Link to="/audit" className="h-full">
            <div
              className="flex items-center gap-3 px-5 py-4 h-full bg-info-light border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer"
              style={{
                borderRadius: wobbly.md,
                boxShadow: shadows.md,
              }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.hover;
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.md;
              }}
            >
              <ShieldCheck size={22} strokeWidth={2.5} className="text-blue" />
              <div className="flex-1">
                <p className="font-medium text-pencil" style={{ fontFamily: 'var(--font-hand)' }}>
                  Security Audit
                </p>
                <p className="text-sm text-pencil-light">Scan skills for threats</p>
              </div>
              <ArrowRight size={16} className="text-pencil-light" />
            </div>
          </Link>

          <Link to="/skills" className="h-full">
            <div
              className="flex items-center gap-3 px-5 py-4 h-full bg-success-light border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer"
              style={{
                borderRadius: wobbly.md,
                boxShadow: shadows.md,
              }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.hover;
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLDivElement).style.boxShadow = shadows.md;
              }}
            >
              <Puzzle size={22} strokeWidth={2.5} className="text-success" />
              <div className="flex-1">
                <p className="font-medium text-pencil" style={{ fontFamily: 'var(--font-hand)' }}>
                  Browse Skills
                </p>
                <p className="text-sm text-pencil-light">View and manage your skills</p>
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
              className="flex items-center gap-3 px-5 py-4 h-full bg-warning-light border-2 border-pencil transition-all duration-100 hover:translate-x-[2px] hover:translate-y-[2px] cursor-pointer"
              style={{
                borderRadius: wobbly.md,
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
                className={`text-warning ${updatingAll ? 'animate-bounce' : ''}`}
              />
              <div className="flex-1">
                <p className="font-medium text-pencil" style={{ fontFamily: 'var(--font-hand)' }}>
                  {updatingAll ? 'Updating...' : 'Update All'}
                </p>
                <p className="text-sm text-pencil-light">Pull latest for all tracked repos</p>
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
            title="Update All"
            message="This will pull the latest changes for all tracked repositories. Continue?"
            confirmText="Update"
            cancelText="Cancel"
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

/* ── Tracked Repositories Section ─────────────────────── */

function TrackedReposSection({ repos }: { repos: { name: string; skillCount: number; dirty: boolean }[] }) {
  return (
    <Card className="mb-8">
      <div className="flex items-center gap-2 mb-4">
        <GitBranch size={20} strokeWidth={2.5} className="text-blue" />
        <h3
          className="text-lg font-bold text-pencil"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Tracked Repositories
        </h3>
      </div>
      <div className="space-y-3">
        {repos.map((repo) => {
          const displayName = repo.name.replace(/^_/, '');
          return (
            <div
              key={repo.name}
              className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
              style={{ borderRadius: wobbly.sm }}
            >
              <div className="flex items-center gap-2 min-w-0">
                <GitBranch size={16} className="text-pencil-light shrink-0" />
                <span
                  className="font-medium text-pencil truncate"
                  style={{ fontFamily: 'var(--font-hand)' }}
                >
                  {displayName}
                </span>
                <Badge variant="info">{repo.skillCount} skills</Badge>
              </div>
              <div className="flex items-center gap-1 shrink-0 ml-2">
                {repo.dirty ? (
                  <span className="flex items-center gap-1 text-warning text-sm">
                    <AlertTriangle size={14} strokeWidth={2.5} />
                    <span style={{ fontFamily: 'var(--font-hand)' }}>modified</span>
                  </span>
                ) : (
                  <span className="flex items-center gap-1 text-success text-sm">
                    <Check size={14} strokeWidth={2.5} />
                    <span style={{ fontFamily: 'var(--font-hand)' }}>clean</span>
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
}

/* ── Skill Updates Section ────────────────────────────── */

function SkillUpdatesSection() {
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
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Skill Updates
          </h3>
          {checked && updatableCount > 0 && (
            <Badge variant="warning">{updatableCount} available</Badge>
          )}
          {checked && updatableCount === 0 && (
            <Badge variant="success">All up to date</Badge>
          )}
        </div>
        <button
          onClick={handleCheck}
          disabled={checking}
          className="text-sm text-blue hover:underline disabled:opacity-50"
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          {checking ? 'Checking...' : checked ? 'Re-check' : 'Run Check'}
        </button>
      </div>

      {!checked && !checking && (
        <p className="text-pencil-light text-sm">
          Click "Run Check" to see if any tracked repos or skills have updates available.
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
              style={{ borderRadius: wobbly.sm }}
            >
              <div className="flex items-center gap-2">
                <GitBranch size={14} className="text-pencil-light" />
                <span className="text-pencil text-sm" style={{ fontFamily: 'var(--font-hand)' }}>
                  {repo.name.replace(/^_/, '')}
                </span>
              </div>
              {repo.status === 'up_to_date' && <Badge variant="success">Up to date</Badge>}
              {repo.status === 'behind' && <Badge variant="warning">{repo.behind} behind</Badge>}
              {repo.status === 'dirty' && <Badge variant="default">Modified</Badge>}
              {repo.status === 'error' && <Badge variant="danger">Error</Badge>}
            </div>
          ))}
          {checkData.skills.map((skill) => (
            <div
              key={skill.name}
              className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
              style={{ borderRadius: wobbly.sm }}
            >
              <div className="flex items-center gap-2">
                <Puzzle size={14} className="text-pencil-light" />
                <span className="text-pencil text-sm" style={{ fontFamily: 'var(--font-hand)' }}>
                  {skill.name}
                </span>
                {skill.source && (
                  <span className="text-xs text-muted-dark truncate max-w-[200px]">{skill.source}</span>
                )}
              </div>
              {skill.status === 'up_to_date' && <Badge variant="success">Up to date</Badge>}
              {skill.status === 'update_available' && <Badge variant="warning">Update available</Badge>}
              {skill.status === 'local' && <Badge variant="default">Local</Badge>}
              {skill.status === 'error' && <Badge variant="danger">Error</Badge>}
            </div>
          ))}
          {checkData.tracked_repos.length === 0 && checkData.skills.length === 0 && (
            <p className="text-pencil-light text-sm">No tracked repos or updatable skills found.</p>
          )}
        </div>
      )}
    </Card>
  );
}

/* ── Security Audit Section ───────────────────────────── */

function SecurityAuditSection() {
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

  return (
    <Card className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <ShieldCheck size={20} strokeWidth={2.5} className="text-blue" />
          <h3
            className="text-lg font-bold text-pencil"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Security Audit
          </h3>
          {scanned && auditData && auditData.summary.failed > 0 && (
            <Badge variant="danger">{auditData.summary.failed} critical</Badge>
          )}
          {scanned && auditData && auditData.summary.failed === 0 && auditData.summary.warning > 0 && (
            <Badge variant="warning">{auditData.summary.warning} warning(s)</Badge>
          )}
          {scanned && auditData && auditData.summary.failed === 0 && auditData.summary.warning === 0 && (
            <Badge variant="success">All clear</Badge>
          )}
        </div>
        <Link to="/audit" className="text-sm text-blue hover:underline" style={{ fontFamily: 'var(--font-hand)' }}>
          {scanned ? 'View details' : 'Run scan'}
        </Link>
      </div>

      {!scanned && !scanning && (
        <div className="flex items-center justify-between">
          <p className="text-pencil-light text-sm">
            Scan your installed skills for malicious patterns and security threats.
          </p>
          <button
            onClick={handleScan}
            className="text-sm text-blue hover:underline disabled:opacity-50 shrink-0 ml-4"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            Quick Scan
          </button>
        </div>
      )}

      {scanning && (
        <div className="space-y-3">
          <Skeleton className="w-full h-8" />
          <Skeleton className="w-3/4 h-8" />
        </div>
      )}

      {scanned && auditData && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div
            className="py-2 px-3 bg-paper-warm border border-muted text-center"
            style={{ borderRadius: wobbly.sm }}
          >
            <p className="text-lg font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
              {auditData.summary.total}
            </p>
            <p className="text-xs text-pencil-light">Scanned</p>
          </div>
          <div
            className="py-2 px-3 bg-paper-warm border border-muted text-center"
            style={{ borderRadius: wobbly.sm }}
          >
            <p className="text-lg font-bold text-success" style={{ fontFamily: 'var(--font-heading)' }}>
              {auditData.summary.passed}
            </p>
            <p className="text-xs text-pencil-light">Passed</p>
          </div>
          <div
            className="py-2 px-3 bg-paper-warm border border-muted text-center"
            style={{ borderRadius: wobbly.sm }}
          >
            <p className="text-lg font-bold text-warning" style={{ fontFamily: 'var(--font-heading)' }}>
              {auditData.summary.warning}
            </p>
            <p className="text-xs text-pencil-light">Warnings</p>
          </div>
          <div
            className={`py-2 px-3 bg-paper-warm border text-center ${auditData.summary.failed > 0 ? 'border-danger' : 'border-muted'}`}
            style={{ borderRadius: wobbly.sm }}
          >
            <p
              className={`text-lg font-bold ${auditData.summary.failed > 0 ? 'text-danger' : 'text-pencil'}`}
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              {auditData.summary.failed}
            </p>
            <p className="text-xs text-pencil-light">Critical</p>
          </div>
        </div>
      )}
    </Card>
  );
}

/* ── Targets Health Section ───────────────────────────── */

function TargetsHealthSection() {
  const { data, loading } = useApi(() => api.listTargets());

  const sourceSkillCount = data?.sourceSkillCount ?? 0;
  const driftTargets = (data?.targets ?? []).filter(
    (t) => {
      const expected = t.expectedSkillCount || sourceSkillCount;
      return (t.mode === 'merge' && t.status === 'merged' || t.mode === 'copy' && t.status === 'copied') && t.linkedCount < expected;
    }
  );
  const maxDrift = driftTargets.reduce(
    (max, t) => Math.max(max, (t.expectedSkillCount || sourceSkillCount) - t.linkedCount),
    0
  );

  return (
    <Card className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Target size={20} strokeWidth={2.5} className="text-success" />
          <h3
            className="text-lg font-bold text-pencil"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Targets Health
          </h3>
          {maxDrift > 0 && (
            <Badge variant="warning">{maxDrift} not synced</Badge>
          )}
        </div>
        <Link to="/targets" className="text-sm text-blue hover:underline" style={{ fontFamily: 'var(--font-hand)' }}>
          View all
        </Link>
      </div>
      {loading ? (
        <div className="space-y-3">
          <Skeleton className="w-full h-10" />
          <Skeleton className="w-full h-10" />
          <Skeleton className="w-3/4 h-10" />
        </div>
      ) : data?.targets && data.targets.length > 0 ? (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {data.targets.map((t: TargetType) => {
              const expected = t.expectedSkillCount || sourceSkillCount;
              const hasDrift = (t.mode === 'merge' && t.status === 'merged' || t.mode === 'copy' && t.status === 'copied') && t.linkedCount < expected;
              return (
                <Link key={t.name} to="/targets">
                  <div
                    className={`flex items-center justify-between py-2 px-3 bg-paper-warm border ${hasDrift ? 'border-warning' : 'border-muted'} hover:border-pencil-light transition-colors`}
                    style={{ borderRadius: wobbly.sm }}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <Target size={14} className="text-pencil-light shrink-0" />
                      <span
                        className="font-medium text-pencil truncate"
                        style={{ fontFamily: 'var(--font-hand)' }}
                      >
                        {t.name}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 shrink-0 ml-2">
                      <StatusBadge status={t.status} />
                      {hasDrift ? (
                        <Badge variant="warning">{t.linkedCount}/{expected} synced</Badge>
                      ) : t.linkedCount > 0 ? (
                        <span className="text-xs text-muted-dark">{t.linkedCount} {t.mode === 'copy' ? 'managed' : 'linked'}</span>
                      ) : null}
                    </div>
                  </div>
                </Link>
              );
            })}
          </div>
          {maxDrift > 0 && (
            <div className="mt-3 flex items-center gap-2 text-warning text-sm" style={{ fontFamily: 'var(--font-hand)' }}>
              <AlertTriangle size={14} strokeWidth={2.5} />
              <span>{maxDrift} skill(s) not synced — <Link to="/sync" className="underline hover:text-pencil">go to Sync page</Link></span>
            </div>
          )}
        </>
      ) : (
        <p className="text-pencil-light text-sm">No targets configured.</p>
      )}
    </Card>
  );
}

/* ── Version Status Section ───────────────────────────── */

function VersionStatusSection() {
  const { data, loading } = useApi(() => api.getVersionCheck());

  return (
    <Card className="mb-8">
      <div className="flex items-center gap-2 mb-4">
        <Package size={20} strokeWidth={2.5} className="text-pencil-light" />
        <h3
          className="text-lg font-bold text-pencil"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Version Status
        </h3>
      </div>
      {loading ? (
        <div className="space-y-3">
          <Skeleton className="w-full h-8" />
          <Skeleton className="w-3/4 h-8" />
        </div>
      ) : data ? (
        <div className="space-y-3">
          {/* CLI Version */}
          <div
            className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
            style={{ borderRadius: wobbly.sm }}
          >
            <div className="flex items-center gap-2">
              <Zap size={14} className="text-pencil-light" />
              <span className="text-pencil text-sm" style={{ fontFamily: 'var(--font-hand)' }}>
                CLI
              </span>
              <span
                className="font-medium text-pencil"
                style={{ fontFamily: "'Courier New', monospace", fontSize: '0.85rem' }}
              >
                {data.cliVersion}
              </span>
            </div>
            {data.cliUpdateAvailable ? (
              <Badge variant="warning">Update: {data.cliLatest}</Badge>
            ) : (
              <Badge variant="success">Up to date</Badge>
            )}
          </div>

          {/* Skill Version */}
          <div
            className="flex items-center justify-between py-2 px-3 bg-paper-warm border border-muted"
            style={{ borderRadius: wobbly.sm }}
          >
            <div className="flex items-center gap-2">
              <Puzzle size={14} className="text-pencil-light" />
              <span className="text-pencil text-sm" style={{ fontFamily: 'var(--font-hand)' }}>
                Skill
              </span>
              <span
                className="font-medium text-pencil"
                style={{ fontFamily: "'Courier New', monospace", fontSize: '0.85rem' }}
              >
                {data.skillVersion || 'N/A'}
              </span>
            </div>
            {data.skillVersion ? (
              data.skillUpdateAvailable ? (
                <Badge variant="warning">Update: {data.skillLatest}</Badge>
              ) : data.skillLatest ? (
                <Badge variant="success">Up to date</Badge>
              ) : (
                <Badge variant="default">Check failed</Badge>
              )
            ) : (
              <Badge variant="default">Not installed</Badge>
            )}
          </div>
        </div>
      ) : (
        <p className="text-pencil-light text-sm">Could not check versions.</p>
      )}
    </Card>
  );
}
