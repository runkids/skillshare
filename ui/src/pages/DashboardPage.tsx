import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Bot,
  ChevronRight,
  CircleArrowUp,
  CircleCheck,
  Ellipsis,
  FolderPlus,
  Github,
  Info,
  Plug,
  Puzzle,
  RefreshCw,
  ShieldAlert,
  Star,
  Trash2,
  TriangleAlert,
  X,
} from 'lucide-react';
import { api } from '../api/client';
import type { AuditAllResponse, CheckResult, LogEntry, Overview, Target } from '../api/client';
import { mcpApi } from '../api/mcp';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { formatLogDetail } from '../lib/logFormat';
import { formatDateTime, formatRelativeTime, useI18n, useT } from '../i18n';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { useAppContext } from '../context/AppContext';
import { useRepoUpdate } from '../hooks/useRepoUpdate';

const STAR_CTA_DISMISSED_KEY = 'skillshare.dashboard.starCta.dismissed';

type Kind = 'ok' | 'warn' | 'bad' | 'off';
interface Health {
  kind: Kind;
  label: string;
  detail: string;
  pending: number;
}

function useTargetHealth() {
  const t = useT();
  return (tgt: Target, sourceSkillCount: number): Health => {
    if (tgt.status === 'not exist') return { kind: 'bad', label: t('dashboard.targets.problem'), detail: t('dashboard.targets.folderMissing'), pending: 0 };
    if (tgt.status === 'conflict' || tgt.status === 'broken') return { kind: 'bad', label: t('dashboard.targets.problem'), detail: tgt.status, pending: 0 };
    if (tgt.status === 'has files') return { kind: 'warn', label: tgt.status, detail: '', pending: 0 };
    if (tgt.status === 'unknown') return { kind: 'off', label: tgt.status, detail: '', pending: 0 };
    const counted = (tgt.mode === 'merge' && tgt.status === 'merged') || (tgt.mode === 'copy' && tgt.status === 'copied');
    const pending = counted ? Math.max(0, (tgt.expectedSkillCount || sourceSkillCount) - tgt.linkedCount) : 0;
    const linked = tgt.linkedCount > 0
      ? t(tgt.mode === 'copy' ? 'dashboard.targets.managed' : 'dashboard.targets.linked', { count: tgt.linkedCount })
      : '';
    if (pending > 0) {
      const label = t('dashboard.pending', { count: pending });
      return { kind: 'warn', label, detail: [linked, label].filter(Boolean).join(' · '), pending };
    }
    return { kind: 'ok', label: t('dashboard.targets.inSync'), detail: linked, pending: 0 };
  };
}

export default function DashboardPage() {
  const t = useT();
  const { locale } = useI18n();
  const { isProjectMode } = useAppContext();
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => api.getOverview(),
    staleTime: staleTimes.overview,
  });
  const { data: targetsData } = useQuery({
    queryKey: queryKeys.targets.all,
    queryFn: () => api.listTargets(),
    staleTime: staleTimes.targets,
  });
  const { data: extrasData } = useQuery({
    queryKey: queryKeys.extras,
    queryFn: () => api.listExtras(),
    staleTime: staleTimes.extras,
  });
  const { data: mcpData } = useQuery({ queryKey: queryKeys.mcp, queryFn: mcpApi.list });
  const { data: lastSync } = useQuery({
    queryKey: queryKeys.log('ops', 1, { cmd: 'sync' }),
    queryFn: () => api.listLog('ops', 1, { cmd: 'sync' }),
    staleTime: staleTimes.log,
  });
  const health = useTargetHealth();

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <div className="ss-empty">
        <TriangleAlert size={24} className="text-bad" />
        <h3 className="font-semibold text-ink">{t('dashboard.error.title')}</h3>
        <p className="text-[13px]">{error.message}</p>
      </div>
    );
  }
  if (!data) return null;

  const targets = targetsData?.targets ?? [];
  const healths = targets.map((tgt) => health(tgt, targetsData?.sourceSkillCount ?? 0));
  const synced = healths.filter((h) => h.kind === 'ok').length;
  const pending = healths.reduce((max, h) => Math.max(max, h.pending), 0);
  const lastSyncTs = lastSync?.entries[0]?.ts;
  const counts = [
    { kind: 'skill', icon: Puzzle, value: data.skillCount, label: t('dashboard.stats.skills'), to: '/skills' },
    { kind: 'agent', icon: Bot, value: data.agentCount, label: t('dashboard.stats.agents'), to: '/agents' },
    { kind: 'extra', icon: FolderPlus, value: extrasData?.extras?.length ?? 0, label: t('dashboard.stats.extras'), to: '/extras' },
    { kind: 'mcp', icon: Plug, value: mcpData ? Object.keys(mcpData.source.servers ?? {}).length : 0, label: t('dashboard.stats.mcp'), to: '/mcp' },
  ];

  const subtitle = [
    isProjectMode ? t('dashboard.projectSummary') : '',
    t('dashboard.summary', { synced, total: targets.length }),
    lastSyncTs ? t('dashboard.lastSync', { time: formatRelativeTime(lastSyncTs, locale) }) : '',
  ].filter(Boolean).join(' ');

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader
        className="!mb-0"
        title={t('dashboard.title')}
        subtitle={subtitle}
        actions={
          <span data-tour="quick-actions" className="flex items-center gap-3">
            {pending > 0 && <span className="ss-st warn">{t('dashboard.pending', { count: pending })}</span>}
            <Link to="/sync" className="ss-btn pri">
              <RefreshCw size={15} />
              {t('layout.nav.sync')}
            </Link>
          </span>
        }
      />

      {isProjectMode && (
        <div className="ss-note inf">
          <Info size={16} />
          <div className="flex-1">{t('dashboard.projectNote')}</div>
        </div>
      )}

      <div data-tour="stats-grid" className="flex flex-col gap-7">
        <div className="ss-counts ss-only-clean">
          {counts.map(({ kind, icon: Icon, value, label, to }) => (
            <Link key={kind} to={to}>
              <span className={`ss-cat ${kind}`}><Icon size={17} /></span>
              <span className="flex flex-col gap-[3px]">
                <b>{value}</b>
                <span className="lbl">{label}</span>
              </span>
            </Link>
          ))}
        </div>

        <div>
          <div className="ss-sec ss-only-clean">
            <h2>{t('dashboard.stats.targets')}</h2>
            <span className="ss-cnt">{targets.length}</span>
            <Link to="/targets" className="more">{t('dashboard.targets.manage')}</Link>
          </div>
          <div className="ss-list ss-only-clean">
            {targets.length === 0 && <div className="ss-r text-[13px] text-ink-2">{t('dashboard.targets.noTargets')}</div>}
            {targets.map((tgt, i) => (
              <Link key={tgt.name} to="/targets" className="ss-r link">
                <span className="ss-at"><AgentIcon target={tgt.name} size={17} /></span>
                <span className="flex flex-col min-w-0 flex-1 gap-px">
                  <span className="font-semibold">{tgt.name}</span>
                  <span className="font-mono text-xs text-ink-3 truncate">{tgt.path}</span>
                </span>
                <span className="w-[70px]"><span className="ss-tag">{tgt.mode}</span></span>
                <span className="w-[190px] text-[13px] text-ink-2 truncate">{healths[i].detail}</span>
                <span className="w-[96px]"><span className={`ss-st ${healths[i].kind}`}>{healths[i].label}</span></span>
                <ChevronRight size={15} className="text-ink-3" />
              </Link>
            ))}
          </div>
          <TargetBoard data={data} targets={targets} healths={healths} counts={counts} />
        </div>
      </div>

      <div className="grid grid-cols-[minmax(0,1.45fr)_minmax(0,1fr)] gap-10">
        <NeedsAttention targets={targets} healths={healths} />
        <RecentLog />
      </div>

      <div className="grid grid-cols-[minmax(0,1.45fr)_minmax(0,1fr)] gap-10">
        {(data.trackedRepos?.length ?? 0) > 0 && <TrackedRepos repos={data.trackedRepos} />}
        <Versions data={data} />
      </div>

      <StarReminder />
    </div>
  );
}

/* ── Playful: source note strung to every target ── */

const STRING_STYLE: Record<Kind, { stroke: string; width: number; dash?: string }> = {
  ok: { stroke: '#2D5DA1', width: 2.2 },
  warn: { stroke: '#B26A00', width: 2.2, dash: '7 6' },
  bad: { stroke: '#C8372D', width: 2.2, dash: '2 7' },
  off: { stroke: '#B9AF9A', width: 1.8, dash: '3 6' },
};

function TargetBoard({ data, targets, healths, counts }: {
  data: Overview;
  targets: Target[];
  healths: Health[];
  counts: { kind: string; icon: typeof Puzzle; value: number; label: string; to: string }[];
}) {
  const t = useT();
  // ponytail: fixed 1080px canvas like the design; desktop widths only.
  const height = Math.max(340, 24 + targets.length * 66 + 20);
  const sx = 340;
  const sy = height / 2;
  const dashed = healths.some((h) => h.kind !== 'ok');
  return (
    <div className="ss-board ss-only-playful" style={{ height }}>
      <svg className="strings" width="1080" height={height} viewBox={`0 0 1080 ${height}`} aria-hidden="true">
        {targets.map((tgt, i) => {
          const ty = 24 + i * 66 + 27;
          const s = STRING_STYLE[healths[i].kind];
          return (
            <path key={tgt.name} d={`M${sx} ${sy} C ${sx + 190} ${sy}, 516 ${ty}, 716 ${ty}`} fill="none" strokeLinecap="round" stroke={s.stroke} strokeWidth={s.width} strokeDasharray={s.dash} />
          );
        })}
        {dashed && (
          <>
            <path d="M468 86 q 26 10 34 40" fill="none" stroke="#5A5A5A" strokeWidth="1.6" strokeLinecap="round" />
            <path d="M494 118 l 8 8 l 3 -11" fill="none" stroke="#5A5A5A" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
          </>
        )}
      </svg>
      <div className="ss-pinnote src flex-col !items-stretch justify-center gap-2.5 !px-5 !py-4" style={{ left: 40, top: sy - 108, width: 300, height: 216 }}>
        <span className="flex flex-col gap-0.5">
          <span className="ss-hand !text-[22px] !font-bold !text-ink">{t('dashboard.board.source')}</span>
          <span className="font-mono text-[11.5px] text-ink-3 truncate">{data.source}</span>
        </span>
        <span className="flex flex-col gap-[7px]">
          {counts.map(({ kind, icon: Icon, value, label, to }) => (
            <Link key={kind} to={to} className="flex items-center gap-[9px]">
              <span className={`ss-cat sm ${kind}`}><Icon size={14} /></span>
              <b className="w-[26px]">{value}</b>
              <span className="text-[13px] text-ink-2">{label}</span>
            </Link>
          ))}
        </span>
      </div>
      <span className="ss-pin blue" style={{ left: sx - 7, top: sy - 7 }} />
      {targets.map((tgt, i) => {
        const y = 24 + i * 66;
        const h = healths[i];
        return (
          <span key={tgt.name}>
            <Link to="/targets" className={`ss-pinnote ${h.kind === 'off' ? 'off' : ''}`} style={{ left: 716, top: y, width: 324, height: 54 }}>
              <AgentIcon target={tgt.name} size={20} />
              <span className="flex flex-col min-w-0 flex-1 gap-px">
                <span className="font-semibold">{tgt.name}</span>
                <span className="font-mono text-[11.5px] text-ink-3 truncate">{tgt.path}</span>
              </span>
              <span className={`ss-st ${h.kind}`}>{h.label}</span>
            </Link>
            <span className={`ss-pin ${h.kind === 'off' ? 'off' : ''}`} style={{ left: 709, top: y + 20 }} />
          </span>
        );
      })}
      {dashed && <span className="ss-hand absolute -rotate-3" style={{ left: 372, top: 44 }}>{t('dashboard.board.dashed')}</span>}
      <span className="ss-hand absolute -rotate-[1.5deg]" style={{ left: 60, top: sy + 132 }}>{t('dashboard.board.caption')}</span>
    </div>
  );
}

/* ── Needs attention ── */

function NeedsAttention({ targets, healths }: { targets: Target[]; healths: Health[] }) {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [check, setCheck] = useState<CheckResult | null>(null);
  const [checking, setChecking] = useState(false);
  const [scanning, setScanning] = useState(false);
  // Reads a scan the Audit page (or this page) already ran; never scans on its own.
  const { data: audit } = useQuery<AuditAllResponse>({
    queryKey: queryKeys.audit.all('skills'),
    queryFn: () => api.auditAll('skills'),
    enabled: false,
    staleTime: staleTimes.audit,
  });

  const runCheck = async () => {
    setChecking(true);
    try {
      const res = await api.check();
      setCheck(res);
      if (!res.tracked_repos.some((r) => r.status === 'behind') && !res.skills.some((s) => s.status === 'update_available')) {
        toast(t('dashboard.updates.allUpToDate'), 'success');
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setChecking(false);
    }
  };

  const runScan = async () => {
    setScanning(true);
    try {
      const res = await api.auditAll('skills');
      queryClient.setQueryData(queryKeys.audit.all('skills'), res);
      if (res.summary.critical + res.summary.high === 0) toast(t('dashboard.security.allClear'), 'success');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setScanning(false);
    }
  };

  const rows: { key: string; kind: 'warn' | 'bad'; icon: typeof Puzzle; title: string; sub: string; action: string; to: string }[] = [];
  if (check) {
    const names = [
      ...check.tracked_repos.filter((r) => r.status === 'behind').map((r) => r.name.replace(/^_/, '')),
      ...check.skills.filter((s) => s.status === 'update_available').map((s) => s.name),
    ];
    if (names.length > 0) {
      rows.push({ key: 'updates', kind: 'warn', icon: CircleArrowUp, title: t('dashboard.attention.updates', { count: names.length }), sub: names.join(', '), action: t('dashboard.trackedRepos.update'), to: '/skills?tab=updates' });
    }
  }
  if (audit && audit.summary.critical + audit.summary.high > 0) {
    const names = audit.results.filter((r) => r.riskLabel === 'high' || r.riskLabel === 'critical').map((r) => r.skillName);
    rows.push({ key: 'audit', kind: 'bad', icon: ShieldAlert, title: t('dashboard.attention.audit', { count: audit.summary.critical + audit.summary.high }), sub: names.join(', '), action: t('dashboard.attention.review'), to: '/audit' });
  }
  targets.forEach((tgt, i) => {
    if (healths[i].kind !== 'bad') return;
    const title = tgt.status === 'not exist' ? t('dashboard.attention.targetMissing', { name: tgt.name }) : `${tgt.name}: ${tgt.status}`;
    rows.push({ key: `target-${tgt.name}`, kind: 'bad', icon: TriangleAlert, title, sub: tgt.path, action: t('dashboard.attention.open'), to: '/targets' });
  });

  return (
    <div>
      <div className="ss-sec">
        <h2>{t('dashboard.attention.title')}</h2>
        {rows.length > 0 && <span className="ss-cnt">{rows.length}</span>}
        <span className="ml-auto flex items-center gap-4">
          <button type="button" className="ss-more cursor-pointer disabled:opacity-50" onClick={runCheck} disabled={checking}>
            {checking ? t('dashboard.updates.checking') : t('dashboard.attention.checkUpdates')}
          </button>
          <button type="button" className="ss-more cursor-pointer disabled:opacity-50" onClick={runScan} disabled={scanning}>
            {t('dashboard.security.quickScan')}
          </button>
        </span>
      </div>
      <div className="ss-list">
        {rows.length === 0 ? (
          <div className="ss-r text-[13px] text-ink-2">
            <CircleCheck size={16} className="text-ok" />
            {t('dashboard.attention.none')}
          </div>
        ) : rows.map(({ key, kind, icon: Icon, title, sub, action, to }) => (
          <div key={key} className="ss-r">
            <span className={`ss-cat sm ${kind}`}><Icon size={14} /></span>
            <span className="flex flex-col min-w-0 flex-1">
              <span className="font-semibold">{title}</span>
              <span className="font-mono text-[13px] text-ink-2 truncate">{sub}</span>
            </span>
            <Link to={to} className="ss-btn sm">{action}</Link>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ── Recent operations ── */

function RecentLog() {
  const t = useT();
  const { locale } = useI18n();
  const { data } = useQuery({
    queryKey: queryKeys.log('ops', 5),
    queryFn: () => api.listLog('ops', 5),
    staleTime: staleTimes.log,
  });
  const when = (e: LogEntry) => {
    const d = new Date(e.ts);
    const age = Date.now() - d.getTime();
    if (new Date().toDateString() === d.toDateString()) return formatDateTime(d, locale, { hour: '2-digit', minute: '2-digit', hour12: false });
    if (age < 6 * 24 * 3600 * 1000) return formatDateTime(d, locale, { weekday: 'short' });
    return formatDateTime(d, locale, { month: 'numeric', day: 'numeric' });
  };
  const entries = data?.entries ?? [];
  return (
    <div>
      <div className="ss-sec">
        <h2>{t('dashboard.recent.title')}</h2>
        <Link to="/log" className="more">{t('dashboard.recent.openLog')}</Link>
      </div>
      <div className="ss-plain">
        {entries.length === 0 && <div className="ss-r text-[13px] text-ink-2">{t('dashboard.recent.empty')}</div>}
        {entries.map((e, i) => (
          <div key={`${e.ts}-${i}`} className="ss-r">
            <span className="w-11 font-mono text-xs text-ink-3">{when(e)}</span>
            <span className={`min-w-16 shrink-0 whitespace-nowrap font-semibold ${e.status === 'error' ? 'text-bad' : ''}`}>{e.cmd}</span>
            <span className="flex-1 min-w-0 truncate text-[13px] text-ink-2">{formatLogDetail(e)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ── Tracked repositories ── */

function TrackedRepos({ repos }: { repos: Overview['trackedRepos'] }) {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { updating, update } = useRepoUpdate();
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuFor) return;
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !menuRef.current?.contains(e.target as Node)) setMenuFor(null);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', close);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', close);
    };
  }, [menuFor]);

  const refresh = async () => {
    clearAuditCache(queryClient);
    await queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    await queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    await queryClient.invalidateQueries({ queryKey: queryKeys.trash });
  };

  const uninstall = async () => {
    if (!toDelete) return;
    setDeleting(true);
    try {
      await api.deleteRepo(toDelete);
      toast(t('dashboard.toast.repoUninstalled', { name: toDelete.replace(/^_/, '') }), 'success');
      setToDelete(null);
      await refresh();
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div>
      <div className="ss-sec">
        <h2>{t('dashboard.trackedRepos.title')}</h2>
        <span className="ss-cnt">{repos.length}</span>
      </div>
      <div className="ss-list !overflow-visible">
        {repos.map((repo) => (
          <div key={repo.name} className="ss-r">
            <Github size={15} />
            <span className="flex flex-col min-w-0 flex-1 gap-px">
              <span className="font-mono text-[13px] font-semibold truncate">{repo.name.replace(/^_/, '')}</span>
              <span className="text-[13px] text-ink-2">{t('dashboard.trackedRepos.skillCount', { count: repo.skillCount })}</span>
            </span>
            <span className={`ss-st ${repo.dirty ? 'warn' : 'ok'}`}>
              {repo.dirty ? t('dashboard.trackedRepos.modified') : t('dashboard.trackedRepos.clean')}
            </span>
            <Button variant="secondary" size="sm" onClick={() => update(repo.name)} loading={updating === repo.name} disabled={updating !== null || deleting}>
              {t('dashboard.trackedRepos.update')}
            </Button>
            <div className="relative" ref={menuFor === repo.name ? menuRef : undefined}>
              <button type="button" className="ss-ib" aria-label={t('dashboard.trackedRepos.actions')} aria-expanded={menuFor === repo.name} onClick={() => setMenuFor(menuFor === repo.name ? null : repo.name)}>
                <Ellipsis size={16} />
              </button>
              {menuFor === repo.name && (
                <div className="ss-menu absolute right-0 top-full mt-1 z-20 !w-44 animate-dropdown-in" role="menu">
                  <button type="button" role="menuitem" className="dng" onClick={() => { setMenuFor(null); setToDelete(repo.name); }}>
                    <Trash2 size={15} />
                    {t('dashboard.trackedRepos.uninstall')}
                  </button>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
      <ConfirmDialog
        open={toDelete !== null}
        title={t('dashboard.trackedRepos.uninstallConfirm.title')}
        message={toDelete ? t('dashboard.trackedRepos.uninstallConfirm.message', { name: toDelete.replace(/^_/, '') }) : ''}
        confirmText={t('dashboard.trackedRepos.uninstall')}
        variant="danger"
        loading={deleting}
        onConfirm={uninstall}
        onCancel={() => !deleting && setToDelete(null)}
      />
    </div>
  );
}

/* ── Versions ── */

function Versions({ data }: { data: Overview }) {
  const t = useT();
  const { data: v, isPending } = useQuery({
    queryKey: queryKeys.versionCheck,
    queryFn: () => api.getVersionCheck(),
    staleTime: staleTimes.version,
  });
  const ver = (s?: string) => (s && /^\d/.test(s) ? `v${s}` : s);
  const sources = [data.source, data.agentsSource, data.extrasSource].filter(Boolean);
  return (
    <div>
      <div className="ss-sec">
        <h2>{t('dashboard.version.title')}</h2>
      </div>
      <div className="ss-list">
        {isPending ? (
          <div className="ss-r"><span className="ss-skel w-2/3" /></div>
        ) : !v ? (
          <div className="ss-r text-[13px] text-ink-2">{t('dashboard.version.couldNotCheck')}</div>
        ) : (
          <>
            <div className="ss-r">
              <span className="w-[120px] font-semibold">{t('dashboard.version.cli')}</span>
              <span className="flex-1 font-mono text-[13px]">{ver(v.cliVersion)}</span>
              {v.cliUpdateAvailable
                ? <span className="ss-st warn">{t('dashboard.version.update', { version: ver(v.cliLatest) })}</span>
                : <span className="ss-st ok">{t('dashboard.version.upToDate')}</span>}
            </div>
            <div className="ss-r">
              <span className="w-[120px] font-semibold">{t('dashboard.version.builtInSkill')}</span>
              <span className="flex-1 font-mono text-[13px]">{ver(v.skillVersion) || '—'}</span>
              {!v.skillVersion
                ? <span className="ss-st off">{t('dashboard.version.notInstalled')}</span>
                : v.skillUpdateAvailable
                  ? <span className="ss-st warn">{t('dashboard.version.update', { version: ver(v.skillLatest) })}</span>
                  : v.skillLatest
                    ? <span className="ss-st ok">{t('dashboard.version.upToDate')}</span>
                    : <span className="ss-st off">{t('dashboard.version.checkFailed')}</span>}
            </div>
          </>
        )}
      </div>
      <div className="mt-2.5 flex flex-col gap-0.5 min-w-0 text-[13px] text-ink-3">
        <span>{t('dashboard.source.label')}</span>
        {sources.map((s) => (
          <span key={s} className="font-mono text-xs truncate">{s}</span>
        ))}
      </div>
    </div>
  );
}

/* ── Star reminder ── */

function StarReminder() {
  const t = useT();
  const [show, setShow] = useState(() => {
    try {
      return window.localStorage.getItem(STAR_CTA_DISMISSED_KEY) !== '1';
    } catch {
      return true;
    }
  });
  if (!show) return null;
  const dismiss = () => {
    setShow(false);
    try {
      window.localStorage.setItem(STAR_CTA_DISMISSED_KEY, '1');
    } catch {
      // Storage unavailable: the reminder simply comes back next visit.
    }
  };
  return (
    <div className="flex items-center gap-2 text-[13px] text-ink-2">
      <Star size={14} />
      <span>
        {t('dashboard.starCta.enjoying')}{' '}
        <a href="https://github.com/runkids/skillshare" target="_blank" rel="noopener noreferrer" className="font-semibold text-ink hover:underline">
          {t('dashboard.starCta.link')}
        </a>
      </span>
      <button type="button" className="ss-ib !w-6 !h-6" onClick={dismiss} aria-label={t('dashboard.starCta.dismiss')}>
        <X size={14} />
      </button>
    </div>
  );
}
