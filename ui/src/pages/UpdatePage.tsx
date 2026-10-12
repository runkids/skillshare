import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Bot, Check, CircleAlert, CircleArrowUp, CircleCheck, Download, FolderX, GitBranch, Link, Loader2, Puzzle, RefreshCw, ShieldAlert, Trash2, X } from 'lucide-react';
import { api } from '../api/client';
import type { CheckResult, LinkedRepo, Skill, UpdateResultItem } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { folderOf, formatTrackedRepoName } from '../lib/resourceNames';
import { repoOf } from '../lib/resourceGrouping';
import { formatRelativeTime, plural, useI18n, useT } from '../i18n';
import Button from '../components/Button';
import { Checkbox } from '../components/Checkbox';
import EmptyState from '../components/EmptyState';
import SyncPreviewModal from '../components/SyncPreviewModal';
import { useToast } from '../components/Toast';
import { useDiffQuery, useSkillsQuery } from '../hooks/useSharedQueries';
import { countChanges, resourceGroups } from '../components/sync/syncView';
import { invalidate } from '../lib/queryEvents';
import { useInstallFromConfig } from '../hooks/useInstallFromConfig';

/* -- Types ---------------------------------------- */

type Kind = Skill['kind'];

type CheckStatus = 'unchecked' | 'checking' | 'behind' | 'dirty' | 'up-to-date' | 'update-available' | 'stale' | 'error';

interface CheckItemStatus {
  status: CheckStatus;
  message?: string;
  behind?: number;
  checkedAt?: string;
}

type CheckStatuses = Map<string, CheckItemStatus>;

const CHECK_STATUS_VALUES: CheckStatus[] = ['unchecked', 'checking', 'behind', 'dirty', 'up-to-date', 'update-available', 'stale', 'error'];
const UPDATE_CHECK_CACHE_KEY = 'skillshare.updateCheckCache.global';
// 2: statuses keyed by relative path; names collide across folders.
const UPDATE_CHECK_CACHE_VERSION = 2;

interface StoredCheckCache {
  version: number;
  items: Record<string, CheckItemStatus>;
}

/** One thing the update API can act on: a whole tracked repo, or a single installed item. */
export interface UpdateUnit {
  /** Name sent to the update API: the repo directory, or the item's relative path. */
  name: string;
  label: string;
  isRepo: boolean;
  items: Skill[];
  source?: string;
}

type RunStatus = 'pending' | 'in-progress' | 'success' | 'error' | 'blocked' | 'skipped';

interface RunState {
  status: RunStatus;
  message?: string;
  auditRiskLabel?: string;
}

/* -- Shared check state --------------------------- */

const NO_STATUSES: CheckStatuses = new Map();

/** Check results survive navigation (query cache) and reloads (localStorage). */
export function useCheckStatuses() {
  const queryClient = useQueryClient();
  const { data } = useQuery({
    queryKey: queryKeys.updateCheck,
    queryFn: readStoredCheckStatuses,
    initialData: readStoredCheckStatuses,
    staleTime: Infinity,
  });
  const setStatuses = useCallback((fn: (prev: CheckStatuses) => CheckStatuses) => {
    const next = fn(queryClient.getQueryData<CheckStatuses>(queryKeys.updateCheck) ?? new Map());
    queryClient.setQueryData(queryKeys.updateCheck, next);
    if (![...next.values()].some((s) => s.status === 'checking')) writeStoredCheckStatuses(next);
  }, [queryClient]);
  return [data ?? NO_STATUSES, setStatuses] as const;
}

export function updateUnits(resources: Skill[], kind: Kind, linkedRepos: LinkedRepo[] = []): UpdateUnit[] {
  const repos = new Map<string, UpdateUnit>();
  const units: UpdateUnit[] = [];
  for (const s of resources) {
    if (s.kind !== kind || !(s.isInRepo || s.source)) continue;
    if (kind === 'skill' && linkedRepos.some((repo) => s.relPath === repo.name || s.relPath.startsWith(`${repo.name}/`))) continue;
    if (!s.isInRepo) {
      units.push({ name: s.relPath, label: s.name, isRepo: false, items: [s], source: s.source });
      continue;
    }
    const dir = folderOf(s);
    let unit = repos.get(dir);
    if (!unit) {
      unit = { name: dir, label: formatTrackedRepoName(dir), isRepo: true, items: [], source: s.repoUrl ?? s.source };
      repos.set(dir, unit);
      units.push(unit);
    }
    unit.items.push(s);
  }
  return units.sort((a, b) => a.label.localeCompare(b.label));
}

/** Repo status is copied to every item in the repo, so the first item speaks for the unit. */
/** A skill's key in the check statuses: its relative path, since names repeat across folders. */
export const checkKey = (item: Pick<Skill, 'relPath'>) => item.relPath;

function unitCheck(statuses: CheckStatuses, unit: UpdateUnit): CheckItemStatus {
  return statuses.get(checkKey(unit.items[0])) ?? { status: 'unchecked' };
}

export function hasUpdate(status: CheckItemStatus) {
  return status.status === 'behind' || status.status === 'update-available';
}

export function countUpdates(statuses: CheckStatuses, units: UpdateUnit[]) {
  return units.filter((u) => hasUpdate(unitCheck(statuses, u))).length;
}

/* -- Tab ------------------------------------------ */

export default function UpdatePage({ kind }: { kind: Kind }) {
  const t = useT();
  const { locale } = useI18n();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const { data: skillsData } = useSkillsQuery();
  // Entries recorded in config but missing on disk (issue #212)
  const missing = useInstallFromConfig();
  const showMissing = kind === 'skill' && missing.entries.length > 0;

  const resources = useMemo(() => skillsData?.resources ?? [], [skillsData]);
  // A full check covers both kinds, so results are applied to every updatable item.
  const updatable = useMemo(() => resources.filter((s) => s.isInRepo || s.source), [resources]);
  const linkedRepos = useMemo(() => kind === 'skill' ? skillsData?.linked_repos ?? [] : [], [skillsData, kind]);
  const units = useMemo(() => updateUnits(resources, kind, linkedRepos), [resources, kind, linkedRepos]);

  const [statuses, setStatuses] = useCheckStatuses();
  const [checking, setChecking] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [force, setForce] = useState(false);
  const [run, setRun] = useState<Map<string, RunState>>(new Map());
  const [running, setRunning] = useState(false);
  const [finished, setFinished] = useState(false);
  const [pruning, setPruning] = useState<Map<string, 'working' | 'out'>>(new Map());
  const [pruned, setPruned] = useState(0);
  const [syncOpen, setSyncOpen] = useState(false);
  const [retried, setRetried] = useState<Set<string>>(new Set());
  const [openDetails, setOpenDetails] = useState<Set<string>>(new Set());
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => () => esRef.current?.close(), []);

  const lastChecked = useMemo(() => {
    const times = units.map((u) => unitCheck(statuses, u).checkedAt).filter((v): v is string => !!v);
    return times.length ? times.reduce((a, b) => (a > b ? a : b)) : undefined;
  }, [units, statuses]);

  /* -- Check -- */

  const applyCheckResult = useCallback((result: CheckResult) => {
    queryClient.setQueryData<Awaited<ReturnType<typeof api.listSkills>>>(queryKeys.skills.all, (prev) =>
      prev ? { ...prev, linked_repos: result.linked_repos ?? prev.linked_repos } : prev);
    setStatuses((prev) => {
      const next = new Map(prev);
      const pending = new Set(updatable.map(checkKey));
      const checkedAt = new Date().toISOString();
      for (const item of updatable) {
        if (item.kind === 'skill' && result.linked_repos?.some((repo) => item.relPath === repo.name || item.relPath.startsWith(`${repo.name}/`))) {
          next.delete(checkKey(item));
          pending.delete(checkKey(item));
        }
      }

      // The backend returns one entry per repo directory (e.g. `_awesome-claude-agents`);
      // copy it to every item in that repo.
      for (const repo of result.tracked_repos) {
        const repoStatus: CheckItemStatus = repo.status === 'behind'
          ? { status: 'behind', message: repo.message, behind: repo.behind, checkedAt }
          : repo.status === 'dirty'
          ? { status: 'dirty', message: repo.message, checkedAt }
          : repo.status === 'error'
          ? { status: 'error', message: repo.message, checkedAt }
          : { status: 'up-to-date', message: repo.message, checkedAt };
        for (const item of updatable) {
          if (repoOf(item) !== repo.name) continue;
          next.set(checkKey(item), repoStatus);
          pending.delete(checkKey(item));
        }
      }

      // Nested installs are reported by metadata relative path while the list shows the
      // basename; match every stable identifier so nested items don't stay checking.
      for (const skill of result.skills) {
        const item = updatable.find((i) => !i.isInRepo && matchesCheckSkill(i, skill.name));
        if (!item) continue;
        next.set(checkKey(item), {
          status: skill.status === 'update_available' ? 'update-available' : skill.status === 'stale' ? 'stale' : skill.status === 'error' ? 'error' : 'up-to-date',
          message: skill.message,
          checkedAt,
        });
        pending.delete(checkKey(item));
      }

      for (const name of pending) {
        if (next.get(name)?.status === 'checking') next.set(name, { status: 'error', checkedAt });
      }
      return next;
    });
  }, [updatable, setStatuses, queryClient]);

  const runCheck = useCallback(() => {
    esRef.current?.close();
    setChecking(true);
    setPruned(0);
    setRun(new Map());
    setFinished(false);
    setStatuses((prev) => {
      const next = new Map(prev);
      for (const item of updatable) next.set(checkKey(item), { status: 'checking' });
      return next;
    });
    esRef.current = api.checkStream(
      () => {},
      () => {},
      () => {},
      (result) => {
        applyCheckResult(result);
        setChecking(false);
      },
      (err) => {
        toast(err.message, 'error');
        setStatuses((prev) => {
          const next = new Map(prev);
          const checkedAt = new Date().toISOString();
          for (const item of updatable) next.set(checkKey(item), { status: 'error', checkedAt });
          return next;
        });
        setChecking(false);
      },
    );
  }, [updatable, applyCheckResult, setStatuses, toast]);

  /* -- Update -- */

  const invalidateSkillData = useCallback(() => {
    clearAuditCache(queryClient);
    void invalidate(queryClient, 'skillsChanged');
  }, [queryClient]);

  const applyUpdateResults = useCallback((results: UpdateResultItem[]) => {
    if (results.length === 0) return;
    setStatuses((prev) => {
      const next = new Map(prev);
      const status: CheckItemStatus = { status: 'up-to-date', checkedAt: new Date().toISOString() };
      for (const result of results) {
        if (!isSuccessfulUpdateAction(result.action)) continue;
        for (const item of updatable) {
          const hit = result.isRepo
            ? repoOf(item) === result.name
            : !item.isInRepo && matchesCheckSkill(item, result.name);
          if (hit) next.set(checkKey(item), { ...status, message: result.message });
        }
      }
      return next;
    });
  }, [updatable, setStatuses]);

  const patchRun = useCallback((name: string, patch: RunState) => {
    setRun((prev) => new Map(prev).set(name, patch));
  }, []);

  const startUpdate = useCallback((names: string[], forceUpdate: boolean) => {
    if (names.length === 0) return;
    esRef.current?.close();
    setRun(new Map(names.map((n) => [n, { status: 'pending' }])));
    setRetried(new Set());
    setOpenDetails(new Set());
    setRunning(true);
    setFinished(false);

    let index = 0;
    esRef.current = api.updateAllStream(
      () => patchRun(names[0], { status: 'in-progress' }),
      (item) => {
        const name = names[index++];
        if (name) patchRun(name, { status: actionToStatus(item.action), message: item.message, auditRiskLabel: item.auditRiskLabel });
        if (names[index]) patchRun(names[index], { status: 'in-progress' });
      },
      (data) => {
        applyUpdateResults(data.results);
        invalidateSkillData();
        setSelected(new Set());
        setRunning(false);
        setFinished(true);
      },
      (err) => {
        toast(err.message, 'error');
        setRun((prev) => new Map([...prev].map(([n, s]) => [n, s.status === 'pending' || s.status === 'in-progress' ? { status: 'error', message: err.message } : s])));
        setRunning(false);
        setFinished(true);
      },
      { names, force: forceUpdate },
    );
  }, [patchRun, applyUpdateResults, invalidateSkillData, toast]);

  const retryForce = useCallback((name: string) => {
    setRetried((prev) => new Set(prev).add(name));
    patchRun(name, { status: 'in-progress' });
    esRef.current = api.updateAllStream(
      () => {},
      (item) => patchRun(name, { status: actionToStatus(item.action), message: item.message, auditRiskLabel: item.auditRiskLabel }),
      (data) => {
        applyUpdateResults(data.results);
        invalidateSkillData();
      },
      (err) => patchRun(name, { status: 'error', message: err.message }),
      { names: [name], force: true },
    );
  }, [patchRun, applyUpdateResults, invalidateSkillData]);

  const purge = useCallback(async (name: string) => {
    setRetried((prev) => new Set(prev).add(name));
    patchRun(name, { status: 'in-progress', message: t('update.updating.purging') });
    try {
      await api.batchUninstall({ names: [name], force: true });
      patchRun(name, { status: 'skipped', message: t('update.updating.purged') });
      invalidateSkillData();
    } catch (err) {
      patchRun(name, { status: 'error', message: (err as Error).message });
    }
  }, [patchRun, invalidateSkillData, t]);

  // A finished install-missing banner collapses on its own unless something failed.
  useEffect(() => {
    if (missing.phase !== 'done' || missing.failed.size) return;
    const id = setTimeout(missing.dismiss, 3000);
    return () => clearTimeout(id);
  }, [missing.phase, missing.failed.size, missing.dismiss]);

  /* -- Prune (deleted upstream) -- */

  const setPrune = useCallback((name: string, state?: 'working' | 'out') => setPruning((prev) => {
    const next = new Map(prev);
    if (state) next.set(name, state);
    else next.delete(name);
    return next;
  }), []);

  // Like `update --prune`: the skills go to trash in one request. Rows slide out, then their gap closes.
  const prune = useCallback(async (list: UpdateUnit[]) => {
    for (const u of list) setPrune(u.name, 'working');
    let gone = list;
    try {
      const { results } = await api.batchUninstall({ names: list.map((u) => u.name), force: true });
      const failed = results.filter((r) => !r.success);
      if (failed.length) toast(failed[0].error ?? failed[0].name, 'error');
      gone = list.filter((u) => !failed.some((r) => r.name === u.name));
      for (const u of list) if (!gone.includes(u)) setPrune(u.name);
    } catch (err) {
      for (const u of list) setPrune(u.name);
      toast((err as Error).message, 'error');
      return;
    }
    for (const u of gone) setPrune(u.name, 'out');
    // ponytail: matches the .ss-collapse.out transition in components.css; change both together.
    setTimeout(() => {
      setStatuses((prev) => {
        const next = new Map(prev);
        for (const item of gone.flatMap((u) => u.items)) next.delete(checkKey(item));
        return next;
      });
      for (const u of gone) setPrune(u.name);
      setPruned((n) => n + gone.length);
      void invalidate(queryClient, 'skillsUninstalled');
    }, 520);
  }, [setPrune, setStatuses, toast, queryClient]);

  /* -- Render -- */

  const busy = running || checking;
  const shown = units.filter((u) => selected.has(u.name));
  const allSelected = units.length > 0 && shown.length === units.length;
  const toggle = (name: string) => setSelected((prev) => {
    const next = new Set(prev);
    if (!next.delete(name)) next.add(name);
    return next;
  });

  const tally = { success: 0, skipped: 0, blocked: 0, error: 0 } as Record<RunStatus, number>;
  for (const s of run.values()) tally[s.status] = (tally[s.status] ?? 0) + 1;
  const tallyText = [
    tally.success && t('update.summary.updated', { count: tally.success }),
    tally.skipped && t('update.summary.skipped', { count: tally.skipped }),
    tally.blocked && t('update.summary.blocked', { count: tally.blocked }),
    tally.error && t('update.summary.failed', { count: tally.error }),
  ].filter(Boolean).join(' · ');

  // Once a run ends, failures leave the list for their own section so their messages have room;
  // one retried from there stays put to show how the retry went.
  const showDone = finished && !running && run.size > 0;
  const checkSync = showDone && tally.success > 0;
  // Always recheck after an update; the cached preview may describe the previous source.
  const diff = useDiffQuery({ enabled: checkSync, staleTime: 0 });
  const syncChecking = checkSync && (diff.isPending || diff.isFetching);
  const syncFirst = checkSync && diff.isSuccess && !diff.isFetching
    && countChanges(resourceGroups(diff.data.diffs, [], new Set([kind]), false).groups) > 0;
  const isTrouble = (name: string) => {
    const s = run.get(name)?.status;
    return s === 'blocked' || s === 'error' || retried.has(name);
  };
  const failedUnits = showDone ? units.filter((u) => isTrouble(u.name)) : [];
  const staleUnits = units.filter((u) => unitCheck(statuses, u).status === 'stale' && !failedUnits.includes(u));
  const listUnits = units.filter((u) => !failedUnits.includes(u) && !staleUnits.includes(u));

  const runDone = [...run.values()].filter((s) => s.status !== 'pending' && s.status !== 'in-progress').length;
  const current = running ? units.find((u) => run.get(u.name)?.status === 'in-progress') : undefined;
  const available = countUpdates(statuses, units);

  const checkCell = (unit: UpdateUnit) => {
    const c = unitCheck(statuses, unit);
    switch (c.status) {
      case 'checking':
        return <span className="inline-flex items-center gap-2 text-[13px] text-ink-3"><Loader2 size={13} className="animate-spin" />{t('update.check.checking')}</span>;
      case 'behind':
        return <span className="ss-st warn">{c.behind ? t('update.check.behind', { count: c.behind }) : t('update.check.behindFallback')}</span>;
      case 'update-available':
        return <span className="ss-st warn">{t('update.check.updateAvailable')}</span>;
      case 'stale':
        return <span className="ss-st warn">{t('update.check.stale')}</span>;
      case 'dirty':
        return <span className="ss-st warn" title={c.message}>{t('update.check.dirty')}</span>;
      case 'up-to-date':
        return <span className="ss-st ok">{t('update.check.upToDate')}</span>;
      case 'error':
        return <span className="ss-st bad" title={c.message}>{t('update.check.error')}</span>;
      default:
        return <span className="ss-st off">{t('update.check.unchecked')}</span>;
    }
  };

  // A row shows one status: how its update went once it has been run, otherwise what the check found.
  const statusCell = (unit: UpdateUnit) => {
    const r = run.get(unit.name);
    if (!r) return checkCell(unit);
    switch (r.status) {
      case 'pending':
        return <span className="ss-st off">{t('update.status.pending')}</span>;
      case 'in-progress':
        return <span className="inline-flex items-center gap-2 text-[13px] font-semibold text-ink"><Loader2 size={13} className="animate-spin" />{r.message ?? t('update.status.updating')}</span>;
      case 'success':
        return <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-ok"><Check size={15} strokeWidth={2.6} className="ss-pop" />{t('update.status.updated')}</span>;
      case 'skipped':
        return <span className="ss-st off">{t('update.status.skipped')}</span>;
      case 'blocked':
        return <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-bad"><X size={15} strokeWidth={2.6} className="ss-pop" />{t('update.status.blocked')}</span>;
      case 'error':
        return <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-bad"><X size={15} strokeWidth={2.6} className="ss-pop" />{t('update.status.failed')}</span>;
    }
  };

  const riskTag = (unit: UpdateUnit) => {
    const label = run.get(unit.name)?.auditRiskLabel;
    if (!label || label === 'clean') return null;
    return <span className={`ss-tag ${label === 'critical' || label === 'high' ? 'bad' : ''}`}>{label}</span>;
  };

  const unitIcon = (unit: UpdateUnit) => (unit.isRepo
    ? <span className="w-[26px] grid place-items-center text-ink-2"><GitBranch size={15} /></span>
    : <span className={`ss-cat sm ${kind}`}>{kind === 'agent' ? <Bot size={14} /> : <Puzzle size={14} />}</span>);

  const unitName = (unit: UpdateUnit) => (
    <span className="flex items-baseline gap-3 min-w-0 flex-1">
      <span className="nm m truncate shrink-0 max-w-[60%]">{unit.label}</span>
      {unit.isRepo && <span className="ss-tag self-center">tracked</span>}
      <span className="font-mono text-xs text-ink-3 truncate">{sourceLabel(unit.source)}</span>
    </span>
  );

  const failureActions = (unit: UpdateUnit) => {
    const r = run.get(unit.name);
    if (r?.status === 'blocked' || (r?.status === 'error' && isForceRetryable(r.message))) {
      return <Button variant="secondary" size="sm" onClick={() => retryForce(unit.name)}>{t('update.updating.forceRetry')}</Button>;
    }
    if (r?.status === 'error' && isStaleError(r.message)) {
      return <Button variant="secondary" size="sm" onClick={() => purge(unit.name)}><Trash2 size={14} />{t('update.updating.purge')}</Button>;
    }
    return statusCell(unit);
  };

  let hero: { icon: ReactNode; tone: string; title: ReactNode; sub: ReactNode };
  if (checking) {
    hero = { icon: <Loader2 size={20} className="animate-spin" />, tone: '', title: t('update.tab.checking'), sub: t('update.tab.audited') };
  } else if (running) {
    hero = {
      icon: <Loader2 size={20} className="animate-spin" />,
      tone: '',
      title: <>{t('update.hero.running', { current: Math.min(runDone + 1, run.size), total: run.size })}{current && <span className="ml-3 font-mono text-[14px] font-medium text-ink-2">{current.label}</span>}</>,
      sub: tallyText || t('update.tab.audited'),
    };
  } else if (showDone) {
    const troubled = tally.blocked + tally.error > 0;
    hero = {
      icon: troubled ? <CircleAlert size={20} /> : <Check size={20} strokeWidth={2.6} className="ss-pop" />,
      tone: troubled ? 'warn' : 'ok',
      title: <>{t('update.done.subtitle')} {tallyText}</>,
      sub: !checkSync || diff.isError ? t('update.tab.audited')
        : syncChecking ? t('sync.checking')
        : syncFirst ? t('update.done.syncHint') : t('syncPreview.allUpToDate'),
    };
  } else {
    const summary = lastChecked ? t('update.check.checkedAt', { time: formatRelativeTime(lastChecked, locale) }) : '';
    hero = {
      icon: <CircleArrowUp size={20} />,
      tone: '',
      title: !lastChecked ? t('update.tab.notChecked') : available > 0 ? t('update.hero.available', { count: available }) : t('update.hero.allCurrent'),
      sub: `${summary} ${t('update.tab.audited')}`.trim(),
    };
  }

  return (
    <>
      {units.length > 0 && (
        <section className="ss-box flex flex-col gap-4 -mt-2" aria-live="polite">
          <div className="flex flex-wrap items-center gap-4">
            <span className={`ss-update-icon ${hero.tone}`}>{hero.icon}</span>
            <div className="flex flex-col gap-0.5 min-w-[240px] flex-1">
              <div key={String(running) + String(checking) + String(showDone)} className="text-[16px] font-semibold animate-fade-in">{hero.title}</div>
              <div className="text-[13px] text-ink-2">{hero.sub}</div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Checkbox size="sm" className="mr-2 text-[13px]" label={t('update.tab.force')} checked={force} onChange={setForce} disabled={busy} />
              <Button variant="secondary" loading={checking} disabled={running} onClick={runCheck}>
                <RefreshCw size={15} />
                {t(lastChecked ? 'update.tab.checkAgain' : 'update.tab.checkNow')}
              </Button>
              {shown.length > 0 && (
                <Button variant="secondary" disabled={busy} onClick={() => startUpdate(shown.map((u) => u.name), force)}>
                  {t('update.header.updateSelected', { count: shown.length })}
                </Button>
              )}
              {syncFirst ? (
                <Button variant="primary" onClick={() => setSyncOpen(true)}><RefreshCw size={15} />{t('syncPreview.syncNowButton')}</Button>
              ) : (
                <Button variant="primary" disabled={busy || syncChecking} onClick={() => startUpdate(units.map((u) => u.name), force)}>
                  <CircleArrowUp size={15} />
                  {t('update.tab.updateAll')}
                </Button>
              )}
            </div>
          </div>
          {checkSync && diff.isError && (
            <div className="ss-note bad">
              <CircleAlert size={16} />
              <span className="flex-1">{diff.error.message}</span>
              <Button variant="secondary" size="sm" onClick={() => void diff.refetch()}>{t('syncPreview.retryButton')}</Button>
            </div>
          )}
          {busy && (
            <div className={`ss-bar ${checking ? 'indet' : ''}`} role="progressbar" aria-valuemin={0} aria-valuemax={run.size} aria-valuenow={checking ? undefined : runDone}>
              <span style={checking ? undefined : { width: `${run.size ? (runDone / run.size) * 100 : 0}%` }} />
            </div>
          )}
        </section>
      )}

      {showMissing && (() => {
        const { done, summary, contents } = missing;
        const failed = missing.failed.size;
        return (
          <div className={`ss-collapse ${missing.phase === 'closing' ? 'shut' : ''}`}>
            <div className={`ss-note live items-center ${done && !failed ? 'ok' : 'warn'}`} aria-live="polite">
              {missing.phase === 'running' ? <Loader2 size={16} className="animate-spin" />
                : done ? (failed ? <CircleAlert size={16} className="ss-pop" /> : <Check size={16} strokeWidth={2.6} className="ss-pop" />)
                : <FolderX size={16} />}
              <div key={missing.phase} className="flex-1 min-w-0 animate-fade-in">
                {missing.phase === 'running' ? (
                  <><b>{t('install.fromConfig.installing', { file: missing.file })}</b> {t('install.audited')}</>
                ) : done ? (
                  <><b>{summary}.</b> {t(failed ? 'install.fromConfig.failedHint' : 'install.fromConfig.syncHint', { file: missing.file })}</>
                ) : (
                  <>
                    <b>{t(plural('update.missingEntries.title', missing.entries.length), { count: missing.entries.length, file: missing.file })}</b>{' '}
                    ({contents}): <span className="font-mono">{missing.entries.map((e) => e.name).join(', ')}</span>. {t('update.missingEntries.description')}
                    {missing.error && <span className="block text-bad">{missing.error}</span>}
                  </>
                )}
              </div>
              {(missing.phase === 'idle' || (done && failed > 0)) && (
                <Button variant="secondary" size="sm" onClick={missing.run}>
                  <Download size={14} />{t(done ? 'install.preview.retry' : 'update.missingEntries.install')}
                </Button>
              )}
              {missing.phase === 'running' && <span className="ss-rowbar" aria-hidden />}
            </div>
          </div>
        );
      })()}

      {(staleUnits.length > 0 || pruned > 0) && (
        <div className={`ss-collapse ${staleUnits.length === 0 ? 'shut' : ''}`}>
          <section className="flex flex-col gap-2" aria-labelledby="update-stale-title">
            <div className="flex items-center gap-3">
              <h2 id="update-stale-title" className="m-0 flex-1 text-[13px] font-semibold text-warn">
                {t('update.stale.title', { count: staleUnits.filter((u) => pruning.get(u.name) !== 'out').length })}
              </h2>
              <Button
                variant="secondary"
                size="sm"
                disabled={busy || pruning.size > 0}
                onClick={() => void prune(staleUnits)}
              >
                <Trash2 size={14} />{t('update.stale.pruneAll')}
              </Button>
            </div>
            <div className="ss-list">
              {staleUnits.map((unit, i) => {
                const p = pruning.get(unit.name);
                return (
                  // Rows of one Prune all leave one after another.
                  <div key={unit.name} className={`ss-collapse ${p === 'out' ? 'out' : ''}`} style={{ transitionDelay: p === 'out' ? `${i * 60}ms` : undefined }}>
                    <div className={`ss-r ${p ? 'updating' : ''}`}>
                      {unitIcon(unit)}
                      {unitName(unit)}
                      {p ? (
                        <span className="inline-flex items-center gap-2 text-[13px] text-ink-2 animate-fade-in"><Loader2 size={13} className="animate-spin" />{t('update.updating.purging')}</span>
                      ) : (
                        <>
                          <span className="ss-st warn">{t('update.check.stale')}</span>
                          <Button variant="secondary" size="sm" disabled={busy} onClick={() => void prune([unit])}>{t('update.updating.purge')}</Button>
                        </>
                      )}
                      {p && <span className="ss-rowbar" aria-hidden />}
                    </div>
                  </div>
                );
              })}
            </div>
            <p className="m-0 text-[13px] text-ink-3">{t('update.stale.note')}</p>
          </section>
        </div>
      )}
      {pruned > 0 && staleUnits.length === 0 && (
        <p className="m-0 -mt-3 flex items-center gap-2 text-[13px] text-ink-2 animate-fade-in">
          <Check size={15} strokeWidth={2.6} className="ss-pop text-ok" />{t(plural('update.stale.pruned', pruned), { count: pruned })}
        </p>
      )}

      {failedUnits.length > 0 && (
        <section className="flex flex-col gap-2 animate-fade-in" aria-labelledby="update-failed-title">
          <h2 id="update-failed-title" className="m-0 text-[13px] font-semibold text-bad">{t('update.failed.title', { count: failedUnits.length })}</h2>
          <div className="ss-list">
            {failedUnits.map((unit) => {
              const r = run.get(unit.name);
              const message = stripCliHint(r?.message);
              const open = openDetails.has(unit.name);
              const trouble = r?.status === 'blocked' || r?.status === 'error';
              return (
                <div key={unit.name} className="ss-r flex-col !items-stretch gap-2 py-3">
                  <div className="flex items-center gap-3">
                    {r?.status === 'blocked' ? <span className="w-[26px] grid place-items-center text-bad"><ShieldAlert size={16} /></span> : unitIcon(unit)}
                    <span className="flex flex-col min-w-0 flex-1 gap-0.5">
                      {unitName(unit)}
                      {trouble && message && <span className="text-[13px] text-bad truncate">{failureSummary(message)}</span>}
                    </span>
                    {riskTag(unit)}
                    {trouble && message && (
                      <Button variant="ghost" size="sm" aria-expanded={open} onClick={() => setOpenDetails((prev) => {
                        const next = new Set(prev);
                        if (!next.delete(unit.name)) next.add(unit.name);
                        return next;
                      })}>
                        {t(open ? 'update.failed.hideDetails' : 'update.failed.showDetails')}
                      </Button>
                    )}
                    <span className="flex items-center justify-end gap-2 min-w-[96px]">{trouble ? failureActions(unit) : statusCell(unit)}</span>
                  </div>
                  {trouble && open && message && (
                    <pre className="m-0 ml-[38px] p-3 rounded-lg bg-sunken font-mono text-xs leading-relaxed text-ink-2 whitespace-pre-wrap break-words animate-fade-in">{message}</pre>
                  )}
                </div>
              );
            })}
          </div>
          <p className="m-0 text-[13px] text-ink-3">{t('update.tab.footer')}</p>
        </section>
      )}

      {linkedRepos.length > 0 && (
        <div className="ss-list">
          {linkedRepos.map((repo) => (
            <div key={repo.name} className="ss-r text-ink-3">
              <span className="w-[26px] grid place-items-center"><Link size={15} /></span>
              <div className="flex flex-col min-w-0 gap-1">
                <span className="nm m">{repo.name}</span>
                <span className="font-mono text-xs break-all">{repo.target}</span>
                <span className="text-[13px]">{t('update.linkedRepos.note')}</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {units.length === 0 ? (
        !showMissing && linkedRepos.length === 0 && (
          <EmptyState
            icon={CircleCheck}
            title={t(kind === 'agent' ? 'update.empty.agentsTitle' : 'update.empty.title')}
            description={t(kind === 'agent' ? 'update.empty.agentsDescription' : 'update.empty.description')}
          />
        )
      ) : listUnits.length > 0 && (
        <div className="ss-list">
          <div className="ss-lh">
            <Checkbox
              hideLabel
              label={t('resources.select.selectAll')}
              checked={allSelected}
              indeterminate={shown.length > 0 && !allSelected}
              disabled={busy}
              onChange={() => setSelected(allSelected ? new Set() : new Set(units.map((u) => u.name)))}
            />
            <span className="w-[26px]" />
            <span className="flex-1">{t('resources.col.name')}</span>
          </div>
          {listUnits.map((unit) => {
            const r = run.get(unit.name);
            const c = unitCheck(statuses, unit).status;
            const canUpdate = !r && c !== 'up-to-date' && c !== 'checking';
            const rowState = r?.status === 'in-progress' ? 'updating' : r?.status === 'pending' || (running && !r) ? 'dim' : r?.status === 'success' ? 'flash' : '';
            return (
              <div key={unit.name} className={`ss-r group ${selected.has(unit.name) ? 'sel' : ''} ${rowState}`}>
                <Checkbox hideLabel label={unit.label} checked={selected.has(unit.name)} disabled={busy} onChange={() => toggle(unit.name)} />
                {unitIcon(unit)}
                {unitName(unit)}
                {riskTag(unit)}
                {canUpdate && (
                  <Button
                    variant="secondary"
                    size="sm"
                    className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100 transition-opacity"
                    disabled={busy}
                    onClick={() => startUpdate([unit.name], force)}
                  >
                    {t('update.row.update')}
                  </Button>
                )}
                <span className="w-[140px] flex justify-end">{statusCell(unit)}</span>
                {rowState === 'updating' && <span className="ss-rowbar" aria-hidden />}
              </div>
            );
          })}
        </div>
      )}
      <SyncPreviewModal open={syncOpen} onClose={() => setSyncOpen(false)} kind={kind} />
    </>
  );
}

/* -- Helpers -------------------------------------- */

function sourceLabel(source?: string): string {
  if (!source) return '';
  return parseRemoteURL(source)?.ownerRepo ?? source;
}

function readStoredCheckStatuses(): CheckStatuses {
  if (typeof window === 'undefined') return new Map();
  try {
    const raw = window.localStorage.getItem(UPDATE_CHECK_CACHE_KEY);
    if (!raw) return new Map();
    const parsed = JSON.parse(raw) as Partial<StoredCheckCache>;
    if (parsed.version !== UPDATE_CHECK_CACHE_VERSION || !parsed.items) return new Map();
    return new Map(Object.entries(parsed.items).filter(([, status]) => isStoredCheckStatus(status)));
  } catch {
    return new Map();
  }
}

function writeStoredCheckStatuses(statuses: CheckStatuses) {
  if (typeof window === 'undefined') return;
  const items: Record<string, CheckItemStatus> = {};
  for (const [name, status] of statuses) {
    if (status.status === 'checking' || status.status === 'unchecked') continue;
    items[name] = status;
  }
  try {
    if (Object.keys(items).length === 0) {
      window.localStorage.removeItem(UPDATE_CHECK_CACHE_KEY);
      return;
    }
    window.localStorage.setItem(UPDATE_CHECK_CACHE_KEY, JSON.stringify({ version: UPDATE_CHECK_CACHE_VERSION, items }));
  } catch {
    // Best-effort UI cache only.
  }
}

function isStoredCheckStatus(value: unknown): value is CheckItemStatus {
  if (!value || typeof value !== 'object') return false;
  const status = (value as Partial<CheckItemStatus>).status;
  return typeof status === 'string'
    && CHECK_STATUS_VALUES.includes(status as CheckStatus)
    && status !== 'checking'
    && status !== 'unchecked'
    // Prune acts on stale, so it must come from a check this session, not an older or other project's cache.
    && status !== 'stale';
}

function isStaleError(message?: string): boolean {
  if (!message) return false;
  return message.includes('does not exist in repository') || message.includes('not found in repository');
}

// Force Retry only helps for failures force actually changes: an audit block,
// or a pull the backend rejected but would retry with --force. Everything else
// (permission denied, missing metadata, scan failures that stay fail-closed)
// would just fail identically, so no button is offered.
export function isForceRetryable(message?: string): boolean {
  if (!message) return false;
  if (message.includes('post-update audit failed')) return false;
  return message.includes('blocked by security audit') || message.includes('(try force update)');
}

// Backend errors carry CLI-only hints that are meaningless in the web UI, where
// the Force Retry button plays that role.
export function stripCliHint(message?: string): string | undefined {
  return message
    ?.replace(/\n*Use --force to override or --skip-audit to bypass scanning: /, '\n\n')
    .replace(/\n*Use --skip-audit to bypass: /, '\n\n')
    .replace(/ \(use --skip-audit to bypass\)/, '')
    .replace(/ \(try force update\)/, '');
}

// The first finding (or first line) stands in for the whole message; the rest opens on demand.
function failureSummary(message: string): string {
  const lines = message.split('\n').map((l) => l.trim()).filter(Boolean);
  return lines.find((l) => /^(CRITICAL|HIGH|MEDIUM|LOW|INFO):/.test(l)) ?? lines[0] ?? '';
}

function matchesCheckSkill(item: Skill, resultName: string): boolean {
  return item.name === resultName || item.flatName === resultName || item.relPath === resultName;
}

function isSuccessfulUpdateAction(action: string): boolean {
  return action === 'updated' || action === 'up-to-date';
}

function actionToStatus(action: string): RunStatus {
  switch (action) {
    case 'updated': return 'success';
    case 'error': return 'error';
    case 'blocked': return 'blocked';
    case 'skipped':
    case 'up-to-date': return 'skipped';
    default: return 'success';
  }
}
