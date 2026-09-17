import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Bot, CircleAlert, CircleArrowUp, CircleCheck, FolderX, GitBranch, Loader2, Puzzle, RefreshCw, Trash2 } from 'lucide-react';
import { api } from '../api/client';
import type { CheckResult, Skill, UpdateResultItem } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { formatTrackedRepoName } from '../lib/resourceNames';
import { formatRelativeTime, useI18n, useT } from '../i18n';
import Button from '../components/Button';
import { Checkbox } from '../components/Checkbox';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';

/* -- Types ---------------------------------------- */

type Kind = Skill['kind'];

type CheckStatus = 'unchecked' | 'checking' | 'behind' | 'dirty' | 'up-to-date' | 'update-available' | 'error';

interface CheckItemStatus {
  status: CheckStatus;
  message?: string;
  behind?: number;
  checkedAt?: string;
}

type CheckStatuses = Map<string, CheckItemStatus>;

const CHECK_STATUS_VALUES: CheckStatus[] = ['unchecked', 'checking', 'behind', 'dirty', 'up-to-date', 'update-available', 'error'];
const UPDATE_CHECK_CACHE_KEY = 'skillshare.updateCheckCache.global';
const UPDATE_CHECK_CACHE_VERSION = 1;

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

export function updateUnits(resources: Skill[], kind: Kind): UpdateUnit[] {
  const repos = new Map<string, UpdateUnit>();
  const units: UpdateUnit[] = [];
  for (const s of resources) {
    if (s.kind !== kind || !(s.isInRepo || s.source)) continue;
    if (!s.isInRepo) {
      units.push({ name: s.relPath, label: s.name, isRepo: false, items: [s], source: s.source });
      continue;
    }
    const dir = s.relPath.split('/')[0];
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
function unitCheck(statuses: CheckStatuses, unit: UpdateUnit): CheckItemStatus {
  return statuses.get(unit.items[0].name) ?? { status: 'unchecked' };
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
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const { data: skillsData } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  // Tracked repos declared in metadata but missing on disk (issue #212)
  const { data: missingReposData } = useQuery({
    queryKey: queryKeys.missingTrackedRepos,
    queryFn: () => api.missingTrackedRepos(),
    staleTime: staleTimes.missingTrackedRepos,
  });
  const missingRepos = missingReposData?.repos ?? [];

  const resources = useMemo(() => skillsData?.resources ?? [], [skillsData]);
  // A full check covers both kinds, so results are applied to every updatable item.
  const updatable = useMemo(() => resources.filter((s) => s.isInRepo || s.source), [resources]);
  const units = useMemo(() => updateUnits(resources, kind), [resources, kind]);

  const [statuses, setStatuses] = useCheckStatuses();
  const [checking, setChecking] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [force, setForce] = useState(false);
  const [run, setRun] = useState<Map<string, RunState>>(new Map());
  const [running, setRunning] = useState(false);
  const [finished, setFinished] = useState(false);
  const [rehydrating, setRehydrating] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => () => esRef.current?.close(), []);

  const lastChecked = useMemo(() => {
    const times = units.map((u) => unitCheck(statuses, u).checkedAt).filter((v): v is string => !!v);
    return times.length ? times.reduce((a, b) => (a > b ? a : b)) : undefined;
  }, [units, statuses]);

  /* -- Check -- */

  const applyCheckResult = useCallback((result: CheckResult) => {
    setStatuses((prev) => {
      const next = new Map(prev);
      const pending = new Set(updatable.map((item) => item.name));
      const checkedAt = new Date().toISOString();

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
          if (!item.isInRepo || item.relPath.split('/')[0] !== repo.name) continue;
          next.set(item.name, repoStatus);
          pending.delete(item.name);
        }
      }

      // Nested installs are reported by metadata relative path while the list shows the
      // basename; match every stable identifier so nested items don't stay checking.
      for (const skill of result.skills) {
        const item = updatable.find((i) => !i.isInRepo && matchesCheckSkill(i, skill.name));
        if (!item) continue;
        next.set(item.name, {
          status: skill.status === 'update_available' ? 'update-available' : skill.status === 'error' ? 'error' : 'up-to-date',
          checkedAt,
        });
        pending.delete(item.name);
      }

      for (const name of pending) {
        if (next.get(name)?.status === 'checking') next.set(name, { status: 'error', checkedAt });
      }
      return next;
    });
  }, [updatable, setStatuses]);

  const runCheck = useCallback(() => {
    esRef.current?.close();
    setChecking(true);
    setStatuses((prev) => {
      const next = new Map(prev);
      for (const item of updatable) next.set(item.name, { status: 'checking' });
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
          for (const item of updatable) next.set(item.name, { status: 'error', checkedAt });
          return next;
        });
        setChecking(false);
      },
    );
  }, [updatable, applyCheckResult, setStatuses, toast]);

  /* -- Update -- */

  const invalidateSkillData = useCallback(() => {
    clearAuditCache(queryClient);
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
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
            ? item.isInRepo && item.relPath.split('/')[0] === result.name
            : !item.isInRepo && matchesCheckSkill(item, result.name);
          if (hit) next.set(item.name, { ...status, message: result.message });
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
    patchRun(name, { status: 'in-progress', message: t('update.updating.purging') });
    try {
      await api.batchUninstall({ names: [name], force: true });
      patchRun(name, { status: 'skipped', message: t('update.updating.purged') });
      invalidateSkillData();
    } catch (err) {
      patchRun(name, { status: 'error', message: (err as Error).message });
    }
  }, [patchRun, invalidateSkillData, t]);

  const rehydrate = useCallback(async () => {
    setRehydrating(true);
    try {
      const { results } = await api.rehydrateTrackedRepos();
      const failed = results.filter((r) => r.action !== 'rehydrated');
      if (failed.length > 0) toast(t('update.missingRepos.rehydratePartial', { count: failed.length }), 'error');
      else toast(t('update.missingRepos.rehydrateSuccess', { count: results.length }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.missingTrackedRepos });
      invalidateSkillData();
    } catch (err) {
      toast((err as Error).message, 'error');
    } finally {
      setRehydrating(false);
    }
  }, [t, toast, queryClient, invalidateSkillData]);

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
  const troubled = tally.blocked + tally.error > 0;

  const checkCell = (unit: UpdateUnit) => {
    const c = unitCheck(statuses, unit);
    switch (c.status) {
      case 'checking':
        return <span className="inline-flex items-center gap-2 text-[13px] text-ink-2"><Loader2 size={13} className="animate-spin" />{t('update.check.checking')}</span>;
      case 'behind':
        return <span className="ss-st warn">{c.behind ? t('update.check.behind', { count: c.behind }) : t('update.check.behindFallback')}</span>;
      case 'update-available':
        return <span className="ss-st warn">{t('update.check.updateAvailable')}</span>;
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

  // After a run, result cells (risk tag + status + retry) and row messages need room; before it the action cell only holds
  // the Update button, so Source takes the space
  const [sourceFlex, actionWidth] = run.size > 0 ? ['flex-1', 'w-[230px]'] : ['flex-[2]', 'w-[90px]'];
  const actionCell = (unit: UpdateUnit) => {
    const r = run.get(unit.name);
    if (!r) {
      const c = unitCheck(statuses, unit).status;
      if (c === 'up-to-date' || c === 'checking') return null;
      return (
        <Button variant="secondary" size="sm" disabled={busy} onClick={() => startUpdate([unit.name], force)}>
          {t('update.row.update')}
        </Button>
      );
    }
    const risk = r.auditRiskLabel && r.auditRiskLabel !== 'clean' ? <span className="ss-tag">{r.auditRiskLabel}</span> : null;
    switch (r.status) {
      case 'pending':
        return <span className="ss-st off">{t('update.status.pending')}</span>;
      case 'in-progress':
        return <span className="inline-flex items-center gap-2 text-[13px] text-ink-2"><Loader2 size={13} className="animate-spin" />{t('update.status.updating')}</span>;
      case 'success':
        return <>{risk}<span className="ss-st ok">{t('update.status.updated')}</span></>;
      case 'skipped':
        return <span className="ss-st off">{t('update.status.skipped')}</span>;
      case 'blocked':
        return (
          <>
            {risk}
            <span className="ss-st bad">{t('update.status.blocked')}</span>
            <Button variant="ghost" size="sm" disabled={running} onClick={() => retryForce(unit.name)}>{t('update.updating.forceRetry')}</Button>
          </>
        );
      case 'error':
        return (
          <>
            <span className="ss-st bad">{t('update.status.failed')}</span>
            {isStaleError(r.message) ? (
              <Button variant="ghost" size="sm" disabled={running} onClick={() => purge(unit.name)}>
                <Trash2 size={14} />
                {t('update.updating.purge')}
              </Button>
            ) : isForceRetryable(r.message) ? (
              <Button variant="ghost" size="sm" disabled={running} onClick={() => retryForce(unit.name)}>{t('update.updating.forceRetry')}</Button>
            ) : null}
          </>
        );
    }
  };

  const subLine = (unit: UpdateUnit) => {
    const r = run.get(unit.name);
    if (r?.message && (r.status === 'error' || r.status === 'blocked')) {
      return <span className="text-[13px] text-bad whitespace-pre-wrap break-words">{stripCliHint(r.message)}</span>;
    }
    if (r?.message) return <span className="text-[13px] text-ink-3 truncate">{stripCliHint(r.message)}</span>;
    if (!unit.isRepo) {
      const dir = unit.name.includes('/') ? unit.name.slice(0, unit.name.lastIndexOf('/')) : '';
      return dir ? <span className="font-mono text-xs text-ink-3 truncate">{dir}</span> : null;
    }
    const meta = [t(`resources.count.${kind}${unit.items.length === 1 ? '' : 's'}`, { count: unit.items.length }), unit.items[0].branch];
    return <span className="text-xs text-ink-3 truncate">{meta.filter(Boolean).join(' · ')}</span>;
  };

  const summary = checking
    ? t('update.tab.checking')
    : lastChecked
    ? t('update.check.checkedAt', { time: formatRelativeTime(lastChecked, locale) })
    : t('update.tab.notChecked');

  return (
    <>
      {units.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 -mt-2">
          <span className="flex-1 min-w-[260px] text-[13px] text-ink-2">{summary} {t('update.tab.audited')}</span>
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
          <Button variant="primary" disabled={busy} onClick={() => startUpdate(units.map((u) => u.name), force)}>
            <CircleArrowUp size={15} />
            {t('update.tab.updateAll')}
          </Button>
        </div>
      )}

      {finished && run.size > 0 && (
        <div className={`ss-note ${troubled ? 'warn' : 'inf'}`}>
          {troubled ? <CircleAlert size={16} /> : <CircleCheck size={16} />}
          <div className="flex-1">
            <b>{t('update.done.subtitle')}</b>{' '}
            {[
              tally.success && t('update.summary.updated', { count: tally.success }),
              tally.skipped && t('update.summary.skipped', { count: tally.skipped }),
              tally.blocked && t('update.summary.blocked', { count: tally.blocked }),
              tally.error && t('update.summary.failed', { count: tally.error }),
            ].filter(Boolean).join(' · ')}
            {tally.success > 0 && <div>{t('update.done.syncHint')}</div>}
          </div>
          {tally.success > 0 && (
            <Button variant="secondary" size="sm" onClick={() => navigate('/sync')}>{t('batchUninstall.results.goToSync')}</Button>
          )}
        </div>
      )}

      {missingRepos.length > 0 && (
        <div className="ss-note warn">
          <FolderX size={16} />
          <div className="flex-1">
            <b>{t('update.missingRepos.title', { count: missingRepos.length })}</b>{' '}
            <span className="font-mono">{missingRepos.map((r) => r.name).join(', ')}</span>. {t('update.missingRepos.description')}
          </div>
          <Button variant="secondary" size="sm" loading={rehydrating} onClick={rehydrate}>{t('update.missingRepos.rehydrate')}</Button>
        </div>
      )}

      {units.length === 0 ? (
        missingRepos.length === 0 && (
          <EmptyState
            icon={CircleCheck}
            title={t(kind === 'agent' ? 'update.empty.agentsTitle' : 'update.empty.title')}
            description={t(kind === 'agent' ? 'update.empty.agentsDescription' : 'update.empty.description')}
          />
        )
      ) : (
        <div className="-mt-3 flex flex-col gap-3">
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
              <span className="w-[150px]">{t('resources.col.status')}</span>
              <span className={`min-w-0 ${sourceFlex}`}>{t('resources.col.source')}</span>
              <span className={actionWidth} />
            </div>
            {units.map((unit) => (
              <div key={unit.name} className={`ss-r ${selected.has(unit.name) ? 'sel' : ''}`}>
                <Checkbox hideLabel label={unit.label} checked={selected.has(unit.name)} disabled={busy} onChange={() => toggle(unit.name)} />
                {unit.isRepo
                  ? <span className="w-[26px] grid place-items-center text-ink-2"><GitBranch size={15} /></span>
                  : <span className={`ss-cat sm ${kind}`}>{kind === 'agent' ? <Bot size={14} /> : <Puzzle size={14} />}</span>}
                <span className="flex flex-col min-w-0 flex-1 gap-px py-2">
                  <span className="flex items-center gap-2 min-w-0">
                    <span className="nm m truncate">{unit.label}</span>
                    {unit.isRepo && <span className="ss-tag">tracked</span>}
                  </span>
                  {subLine(unit)}
                </span>
                <span className="w-[150px]">{checkCell(unit)}</span>
                <span className={`min-w-0 ${sourceFlex} font-mono text-xs text-ink-3 truncate`}>{sourceLabel(unit.source)}</span>
                <span className={`${actionWidth} flex items-center justify-end gap-2`}>{actionCell(unit)}</span>
              </div>
            ))}
          </div>
          {troubled && <p className="text-[13px] text-ink-3">{t('update.tab.footer')}</p>}
        </div>
      )}
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
    && status !== 'unchecked';
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
