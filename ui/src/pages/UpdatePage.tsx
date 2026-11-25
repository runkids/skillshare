import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Bot,
  Check,
  CircleAlert,
  CircleArrowUp,
  CircleCheck,
  Download,
  Ellipsis,
  ExternalLink,
  FolderX,
  GitBranch,
  Link,
  Loader2,
  Puzzle,
  RefreshCw,
  Search,
  SearchX,
  ShieldAlert,
  Trash2,
  X,
} from 'lucide-react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { formatVersion } from '../lib/format';
import { api } from '../api/client';
import type { CheckResult, LinkedRepo, Skill, UpdateResultItem } from '../api/client';
import { queryKeys } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { folderOf, formatTrackedRepoName } from '../lib/resourceNames';
import { countLabel, repoOf, SOURCE_LABEL } from '../lib/resourceGrouping';
import {
  foldsAway,
  hasUpstreamPreview,
  matchesFilters,
  NO_FILTERS,
  repeatedLabels,
  sourcePage,
  unitFolder,
  unitSource,
  statusGroup,
  UPDATE_SOURCES,
  UPDATE_STATUSES,
  type UpdateFilters,
  type UpdateStatus,
} from '../lib/updateFilters';
import { formatNumber, formatRelativeTime, plural, useI18n, useT } from '../i18n';
import Button from '../components/Button';
import DrawnCheck from '../components/DrawnCheck';
import ProgressBar from '../components/ProgressBar';
import { Checkbox } from '../components/Checkbox';
import EmptyState from '../components/EmptyState';
import { Select } from '../components/Select';
import SyncPreviewModal from '../components/SyncPreviewModal';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import UpdatePreviewDialog from '../components/UpdatePreviewDialog';
import { useToast } from '../components/Toast';
import { useDiffQuery, useSkillsQuery } from '../hooks/useSharedQueries';
import { countChanges, resourceGroups } from '../components/sync/syncView';
import { invalidate } from '../lib/queryEvents';
import { updateCheckTitle } from '../lib/updateCheckSummary';
import { useInstallFromConfig } from '../hooks/useInstallFromConfig';

/* -- Types ---------------------------------------- */

type Kind = Skill['kind'];

type CheckStatus =
  | 'unchecked'
  | 'checking'
  | 'behind'
  | 'dirty'
  | 'up-to-date'
  | 'update-available'
  | 'stale'
  | 'error';

interface CheckItemStatus {
  status: CheckStatus;
  message?: string;
  behind?: number;
  checkedAt?: string;
}

type CheckStatuses = Map<string, CheckItemStatus>;

const CHECK_STATUS_VALUES: CheckStatus[] = [
  'unchecked',
  'checking',
  'behind',
  'dirty',
  'up-to-date',
  'update-available',
  'stale',
  'error',
];
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

type RunStatus =
  | 'pending'
  | 'in-progress'
  | 'success'
  | 'up-to-date'
  | 'error'
  | 'blocked'
  | 'skipped';

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
  const setStatuses = useCallback(
    (fn: (prev: CheckStatuses) => CheckStatuses) => {
      const next = fn(queryClient.getQueryData<CheckStatuses>(queryKeys.updateCheck) ?? new Map());
      queryClient.setQueryData(queryKeys.updateCheck, next);
      if (![...next.values()].some((s) => s.status === 'checking')) writeStoredCheckStatuses(next);
    },
    [queryClient],
  );
  return [data ?? NO_STATUSES, setStatuses] as const;
}

export function updateUnits(
  resources: Skill[],
  kind: Kind,
  linkedRepos: LinkedRepo[] = [],
): UpdateUnit[] {
  const repos = new Map<string, UpdateUnit>();
  const units: UpdateUnit[] = [];
  for (const s of resources) {
    if (s.kind !== kind || !(s.isInRepo || s.source)) continue;
    if (
      kind === 'skill' &&
      linkedRepos.some((repo) => s.relPath === repo.name || s.relPath.startsWith(`${repo.name}/`))
    )
      continue;
    if (!s.isInRepo) {
      units.push({ name: s.relPath, label: s.name, isRepo: false, items: [s], source: s.source });
      continue;
    }
    const dir = folderOf(s);
    let unit = repos.get(dir);
    if (!unit) {
      unit = {
        name: dir,
        label: formatTrackedRepoName(dir),
        isRepo: true,
        items: [],
        source: s.repoUrl ?? s.source,
      };
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
  const linkedRepos = useMemo(
    () => (kind === 'skill' ? (skillsData?.linked_repos ?? []) : []),
    [skillsData, kind],
  );
  const units = useMemo(
    () => updateUnits(resources, kind, linkedRepos),
    [resources, kind, linkedRepos],
  );

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
  const [filters, setFilters] = useState<UpdateFilters>(NO_FILTERS);
  const [showCurrent, setShowCurrent] = useState(false);
  const [menu, setMenu] = useState<{ unit: UpdateUnit; point: { x: number; y: number } } | null>(
    null,
  );
  const [preview, setPreview] = useState<{
    unit: UpdateUnit;
    mode: 'upstream' | 'local';
  } | null>(null);
  const esRef = useRef<EventSource | null>(null);
  // The toolbar is marked stuck once it pins to the top. A callback ref, because the list (and the
  // sentinel above it) mounts only after the resources load.
  const [stuck, setStuck] = useState(false);
  const sentinel = useCallback((el: HTMLDivElement | null) => {
    if (!el || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver(([e]) => setStuck(!e.isIntersecting));
    io.observe(el);
    return () => io.disconnect();
  }, []);

  useEffect(() => () => esRef.current?.close(), []);

  const lastChecked = useMemo(() => {
    const times = units
      .map((u) => unitCheck(statuses, u).checkedAt)
      .filter((v): v is string => !!v);
    return times.length ? times.reduce((a, b) => (a > b ? a : b)) : undefined;
  }, [units, statuses]);

  /* -- Check -- */

  const applyCheckResult = useCallback(
    (result: CheckResult) => {
      queryClient.setQueryData<Awaited<ReturnType<typeof api.listSkills>>>(
        queryKeys.skills.all,
        (prev) =>
          prev ? { ...prev, linked_repos: result.linked_repos ?? prev.linked_repos } : prev,
      );
      setStatuses((prev) => {
        const next = new Map(prev);
        const pending = new Set(updatable.map(checkKey));
        const checkedAt = new Date().toISOString();
        for (const item of updatable) {
          if (
            item.kind === 'skill' &&
            result.linked_repos?.some(
              (repo) => item.relPath === repo.name || item.relPath.startsWith(`${repo.name}/`),
            )
          ) {
            next.delete(checkKey(item));
            pending.delete(checkKey(item));
          }
        }

        // The backend returns one entry per skills repo directory (e.g. `_awesome-claude-agents`);
        // copy it to every skill in that repo. An agents repo of the same name is a separate
        // checkout, reported under `agents`.
        for (const repo of result.tracked_repos) {
          const repoStatus: CheckItemStatus =
            repo.status === 'behind'
              ? { status: 'behind', message: repo.message, behind: repo.behind, checkedAt }
              : repo.status === 'dirty'
                ? { status: 'dirty', message: repo.message, checkedAt }
                : repo.status === 'error'
                  ? { status: 'error', message: repo.message, checkedAt }
                  : { status: 'up-to-date', message: repo.message, checkedAt };
          for (const item of updatable) {
            if (item.kind !== 'skill' || repoOf(item) !== repo.name) continue;
            next.set(checkKey(item), repoStatus);
            pending.delete(checkKey(item));
          }
        }

        // Nested installs are reported by metadata relative path while the list shows the
        // basename; match every stable identifier so nested items don't stay checking.
        for (const skill of result.skills) {
          const item = updatable.find(
            (i) => i.kind === 'skill' && !i.isInRepo && matchesCheckSkill(i, skill.name),
          );
          if (!item) continue;
          next.set(checkKey(item), {
            status:
              skill.status === 'update_available'
                ? 'update-available'
                : skill.status === 'stale'
                  ? 'stale'
                  : skill.status === 'modified'
                    ? 'dirty'
                    : skill.status === 'error'
                      ? 'error'
                      : 'up-to-date',
            // The modified reason is English API copy; the translated label says it.
            message: skill.status === 'modified' ? undefined : skill.message,
            checkedAt,
          });
          pending.delete(checkKey(item));
        }

        for (const agent of result.agents ?? []) {
          const item = updatable.find((i) => agentKey(i) === agent.name);
          if (!item) continue;
          const edited = agent.status === 'drifted' || agent.status === 'dirty';
          next.set(checkKey(item), {
            status:
              agent.status === 'update_available'
                ? 'update-available'
                : edited
                  ? 'dirty'
                  : agent.status === 'error'
                    ? 'error'
                    : 'up-to-date',
            // As for a modified skill, the translated label says it.
            message: edited ? undefined : agent.message,
            checkedAt,
          });
          pending.delete(checkKey(item));
        }

        for (const name of pending) {
          if (next.get(name)?.status === 'checking') next.set(name, { status: 'error', checkedAt });
        }
        return next;
      });
    },
    [updatable, setStatuses, queryClient],
  );

  // Checks the given units, leaving every other result as it is; all of them when omitted.
  const runCheck = useCallback(
    (only?: UpdateUnit[]) => {
      const items = only ? only.flatMap((u) => u.items) : updatable;
      esRef.current?.close();
      setChecking(true);
      setPruned(0);
      setRun(new Map());
      setFinished(false);
      setStatuses((prev) => {
        const next = new Map(prev);
        for (const item of items) next.set(checkKey(item), { status: 'checking' });
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
            for (const item of items) next.set(checkKey(item), { status: 'error', checkedAt });
            return next;
          });
          setChecking(false);
        },
        only?.map((u) => u.name),
      );
    },
    [updatable, applyCheckResult, setStatuses, toast],
  );

  /* -- Update -- */

  const invalidateSkillData = useCallback(() => {
    clearAuditCache(queryClient);
    void invalidate(queryClient, 'skillsChanged');
  }, [queryClient]);

  const applyUpdateResults = useCallback(
    (results: UpdateResultItem[]) => {
      if (results.length === 0) return;
      setStatuses((prev) => {
        const next = new Map(prev);
        const status: CheckItemStatus = {
          status: 'up-to-date',
          checkedAt: new Date().toISOString(),
        };
        for (const result of results) {
          if (!isSuccessfulUpdateAction(result.action)) continue;
          for (const item of updatable) {
            if (item.kind !== (result.kind ?? 'skill')) continue;
            const hit = result.isRepo
              ? repoOf(item) === result.name
              : !item.isInRepo &&
                (matchesCheckSkill(item, result.name) || agentKey(item) === result.name);
            if (hit) next.set(checkKey(item), { ...status, message: result.message });
          }
        }
        return next;
      });
    },
    [updatable, setStatuses],
  );

  const patchRun = useCallback((name: string, patch: RunState) => {
    setRun((prev) => new Map(prev).set(name, patch));
  }, []);

  const startUpdate = useCallback(
    (names: string[], forceUpdate: boolean, merge = false) => {
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
          if (name)
            patchRun(name, {
              status: actionToStatus(item.action),
              message: item.message,
              auditRiskLabel: item.auditRiskLabel,
            });
          if (names[index]) patchRun(names[index], { status: 'in-progress' });
        },
        (data) => {
          // A merged item still differs from its install, so its check result stands.
          if (!merge) applyUpdateResults(data.results);
          invalidateSkillData();
          setSelected(new Set());
          setRunning(false);
          setFinished(true);
        },
        (err) => {
          toast(err.message, 'error');
          setRun(
            (prev) =>
              new Map(
                [...prev].map(([n, s]) => [
                  n,
                  s.status === 'pending' || s.status === 'in-progress'
                    ? { status: 'error', message: err.message }
                    : s,
                ]),
              ),
          );
          setRunning(false);
          setFinished(true);
        },
        merge ? { names, kind, merge } : { names, kind, force: forceUpdate },
      );
    },
    [kind, patchRun, applyUpdateResults, invalidateSkillData, toast],
  );

  // Force overwrites local edits; merge keeps them and applies the upstream changes.
  const retryUpdate = useCallback(
    (name: string, mode: 'force' | 'merge') => {
      setRetried((prev) => new Set(prev).add(name));
      patchRun(name, { status: 'in-progress' });
      esRef.current = api.updateAllStream(
        () => {},
        (item) =>
          patchRun(name, {
            status: actionToStatus(item.action),
            message: item.message,
            auditRiskLabel: item.auditRiskLabel,
          }),
        (data) => {
          // A merged item still differs from its install, so its check result stands.
          if (mode === 'force') applyUpdateResults(data.results);
          invalidateSkillData();
        },
        (err) => patchRun(name, { status: 'error', message: err.message }),
        { names: [name], kind, [mode]: true },
      );
    },
    [kind, patchRun, applyUpdateResults, invalidateSkillData],
  );

  const purge = useCallback(
    async (name: string) => {
      setRetried((prev) => new Set(prev).add(name));
      patchRun(name, { status: 'in-progress', message: t('update.updating.purging') });
      try {
        await api.batchUninstall({ names: [name], kind, force: true });
        patchRun(name, { status: 'skipped', message: t('update.updating.purged') });
        invalidateSkillData();
      } catch (err) {
        patchRun(name, { status: 'error', message: (err as Error).message });
      }
    },
    [kind, patchRun, invalidateSkillData, t],
  );

  // A finished install-missing banner collapses on its own unless something failed.
  useEffect(() => {
    if (missing.phase !== 'done' || missing.failed.size) return;
    const id = setTimeout(missing.dismiss, 3000);
    return () => clearTimeout(id);
  }, [missing.phase, missing.failed.size, missing.dismiss]);

  /* -- Prune (deleted upstream) -- */

  const setPrune = useCallback(
    (name: string, state?: 'working' | 'out') =>
      setPruning((prev) => {
        const next = new Map(prev);
        if (state) next.set(name, state);
        else next.delete(name);
        return next;
      }),
    [],
  );

  // Like `update --prune`: the skills go to trash in one request. Rows slide out, then their gap closes.
  const prune = useCallback(
    async (list: UpdateUnit[]) => {
      for (const u of list) setPrune(u.name, 'working');
      let gone = list;
      try {
        const { results } = await api.batchUninstall({
          names: list.map((u) => u.name),
          force: true,
        });
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
      // ponytail: matches the .ss-collapse.out transition in components.css (each row 60ms after the
      // last, gap closing from 180ms over 300ms); change both together.
      setTimeout(
        () => {
          setStatuses((prev) => {
            const next = new Map(prev);
            for (const item of gone.flatMap((u) => u.items)) next.delete(checkKey(item));
            return next;
          });
          for (const u of gone) setPrune(u.name);
          setPruned((n) => n + gone.length);
          void invalidate(queryClient, 'skillsUninstalled');
        },
        180 + 60 * (gone.length - 1) + 300,
      );
    },
    [setPrune, setStatuses, toast, queryClient],
  );

  /* -- Render -- */

  const busy = running || checking;
  const toggle = (name: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (!next.delete(name)) next.add(name);
      return next;
    });

  const tally = { success: 0, 'up-to-date': 0, skipped: 0, blocked: 0, error: 0 } as Record<
    RunStatus,
    number
  >;
  for (const s of run.values()) tally[s.status] = (tally[s.status] ?? 0) + 1;
  const tallyText = [
    tally.success && t('update.summary.updated', { count: tally.success }),
    tally['up-to-date'] && t('update.summary.upToDate', { count: tally['up-to-date'] }),
    tally.skipped && t('update.summary.skipped', { count: tally.skipped }),
    tally.blocked && t('update.summary.blocked', { count: tally.blocked }),
    tally.error && t('update.summary.failed', { count: tally.error }),
  ]
    .filter(Boolean)
    .join(' · ');

  // Once a run ends, failures leave the list for their own section so their messages have room;
  // one retried from there stays put to show how the retry went.
  const showDone = finished && !running && run.size > 0;
  const checkSync = showDone && tally.success > 0;
  // Always recheck after an update; the cached preview may describe the previous source.
  const diff = useDiffQuery({ enabled: checkSync, staleTime: 0 });
  const syncChecking = checkSync && (diff.isPending || diff.isFetching);
  const syncFirst =
    checkSync &&
    diff.isSuccess &&
    !diff.isFetching &&
    countChanges(resourceGroups(diff.data.diffs, [], new Set([kind]), false).groups) > 0;
  const isTrouble = (name: string) => {
    const s = run.get(name)?.status;
    return s === 'blocked' || s === 'error' || retried.has(name);
  };
  const failedUnits = showDone ? units.filter((u) => isTrouble(u.name)) : [];
  const staleUnits = units.filter(
    (u) => unitCheck(statuses, u).status === 'stale' && !failedUnits.includes(u),
  );
  const listUnits = units.filter((u) => !failedUnits.includes(u) && !staleUnits.includes(u));

  // Search and filters narrow the main list only; failed and stale sections report every unit.
  const statusOf = (u: UpdateUnit) => unitCheck(statuses, u).status;
  const query = filters.query.trim();
  const filtering =
    query !== '' || filters.source !== 'all' || filters.status !== 'all' || filters.folder !== null;
  const matched = listUnits.filter((u) => matchesFilters(u, filters, statusOf(u)));
  const foldable = matched
    .filter((u) => foldsAway(filters, statusOf(u), run.has(u.name)))
    .map((u) => u.name);
  const folded = new Set(showCurrent ? [] : foldable);
  const visible = matched.filter((u) => !folded.has(u.name));
  const repeated = repeatedLabels(units);
  // Selection acts on rows in view: a selected row hidden by a filter is left out.
  const shown = visible.filter((u) => selected.has(u.name));
  const allSelected = visible.length > 0 && shown.length === visible.length;
  const setFilter = (patch: Partial<UpdateFilters>) =>
    setFilters((prev) => ({ ...prev, ...patch }));

  const chipCounts = (skip: 'source' | 'status' | 'folder') =>
    listUnits.filter((u) => matchesFilters(u, filters, statusOf(u), skip));
  const chipOf = (key: string) => ({
    clearValue: 'all',
    clearLabel: t('resources.toolbar.clearFilter', { name: t(key) }),
  });
  const countOption = (value: string, label: string, count: number, current: string) => ({
    value,
    label,
    trailing: String(count),
    disabled: count === 0 && value !== current && value !== 'all',
  });
  const folders = new Map<string, number>();
  for (const u of listUnits) folders.set(unitFolder(u), (folders.get(unitFolder(u)) ?? 0) + 1);
  const statusLabel: Record<UpdateStatus, string> = {
    updates: t('update.check.updateAvailable'),
    modified: t('update.check.dirty'),
    failed: t('update.check.error'),
    current: t('update.check.upToDate'),
  };

  const runDone = [...run.values()].filter(
    (s) => s.status !== 'pending' && s.status !== 'in-progress',
  ).length;
  const current = running
    ? units.find((u) => run.get(u.name)?.status === 'in-progress')
    : undefined;
  const available = countUpdates(statuses, units);

  const checkCell = (unit: UpdateUnit) => {
    const c = unitCheck(statuses, unit);
    switch (c.status) {
      case 'checking':
        return (
          <span className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink-3">
            <Loader2 size={13} className="animate-spin" />
            {t('update.check.checking')}
          </span>
        );
      case 'behind':
        return (
          <span className="ss-st warn">
            {c.behind
              ? t(plural('update.check.behind', c.behind), {
                  count: formatNumber(c.behind, locale),
                })
              : t('update.check.behindFallback')}
          </span>
        );
      case 'update-available':
        return <span className="ss-st warn">{t('update.check.updateAvailable')}</span>;
      case 'stale':
        return <span className="ss-st warn">{t('update.check.stale')}</span>;
      case 'dirty':
        return (
          <span className="ss-st warn" title={c.message}>
            {t('update.check.dirty')}
          </span>
        );
      case 'up-to-date':
        return <span className="ss-st ok">{t('update.check.upToDate')}</span>;
      case 'error':
        return (
          <span className="ss-st bad" title={c.message}>
            {t('update.check.error')}
          </span>
        );
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
        return (
          <span className="ss-st on ss-shimmer">{r.message ?? t('update.status.updating')}</span>
        );
      case 'success':
        return (
          <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-ok">
            <Check size={15} strokeWidth={2.6} className="ss-pop" />
            {t('update.status.updated')}
          </span>
        );
      case 'up-to-date':
        return <span className="ss-st ok">{t('update.check.upToDate')}</span>;
      case 'skipped':
        // A skill skipped for its local changes can be overwritten from its row.
        if (isForceRetryable(r.message)) {
          return (
            <span className="inline-flex flex-wrap justify-end gap-2">
              {isMergeable(r.message) && (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => retryUpdate(unit.name, 'merge')}
                >
                  {t('update.updating.merge')}
                </Button>
              )}
              <Button variant="secondary" size="sm" onClick={() => retryUpdate(unit.name, 'force')}>
                {t('update.updating.forceRetry')}
              </Button>
            </span>
          );
        }
        return <span className="ss-st off">{t('update.status.skipped')}</span>;
      case 'blocked':
        return (
          <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-bad">
            <X size={15} strokeWidth={2.6} className="ss-pop" />
            {t('update.status.blocked')}
          </span>
        );
      case 'error':
        return (
          <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-bad">
            <X size={15} strokeWidth={2.6} className="ss-pop" />
            {t('update.status.failed')}
          </span>
        );
    }
  };

  const riskTag = (unit: UpdateUnit) => {
    const label = run.get(unit.name)?.auditRiskLabel;
    if (!label || label === 'clean') return null;
    return (
      <span className={`ss-tag ${label === 'critical' || label === 'high' ? 'bad' : ''}`}>
        {label}
      </span>
    );
  };

  const unitIcon = (unit: UpdateUnit) =>
    unit.isRepo ? (
      <span className="w-[26px] grid place-items-center text-ink-2">
        <GitBranch size={15} />
      </span>
    ) : (
      <span className={`ss-cat sm ${kind}`}>
        {kind === 'agent' ? <Bot size={14} /> : <Puzzle size={14} />}
      </span>
    );

  const unitName = (unit: UpdateUnit) => (
    <span className="flex flex-wrap md:flex-nowrap items-baseline gap-x-3 gap-y-1 min-w-0 flex-1">
      <span
        className="nm m truncate max-w-full md:shrink-0 md:max-w-[60%]"
        title={unit.isRepo ? unit.label : unit.name}
      >
        {/* Names repeated across folders read the same, so those rows lead with their folder. */}
        {!unit.isRepo && repeated.has(unit.label) && unitFolder(unit) !== '' && (
          <span className="font-normal text-ink-3">{unitFolder(unit)}/</span>
        )}
        {unit.label}
      </span>
      {unit.isRepo && <span className="ss-tag self-center">tracked</span>}
      {!unit.isRepo && unit.items[0]?.metadataVersion && (
        <span
          className="ss-tag self-center max-w-[30%] font-mono"
          title={unit.items[0].metadataVersion}
        >
          <span className="truncate">{formatVersion(unit.items[0].metadataVersion)}</span>
        </span>
      )}
      {unit.items[0]?.branch && (
        <span className="ss-tag self-center max-w-[40%]" title={unit.items[0].branch}>
          <GitBranch size={11} className="shrink-0" />
          <span className="truncate">{unit.items[0].branch}</span>
        </span>
      )}
      <span className="basis-full md:basis-auto font-mono text-xs text-ink-3 truncate">
        {sourceLabel(unit.source)}
      </span>
    </span>
  );

  // A failed check says why under the name, where it can be read without hovering.
  const listName = (unit: UpdateUnit) => {
    const c = unitCheck(statuses, unit);
    if (c.status !== 'error' || !c.message || run.has(unit.name)) return unitName(unit);
    return (
      <span className="flex flex-col min-w-0 flex-1 gap-0.5">
        {unitName(unit)}
        <span className="text-[12.5px] text-bad truncate" title={c.message}>
          {failureSummary(c.message)}
        </span>
      </span>
    );
  };

  const openMenu = (e: ReactMouseEvent, unit: UpdateUnit) => {
    const r = e.currentTarget.getBoundingClientRect();
    setMenu({ unit, point: { x: r.right, y: r.bottom + 4 } });
  };
  // Which row actions apply. A check limited by names leaves agents out, so only skills can be
  // checked one by one.
  const rowActions = (unit: UpdateUnit) => {
    const c = unitCheck(statuses, unit).status;
    return {
      check: kind === 'skill' && !busy,
      update: !busy && !run.has(unit.name) && c !== 'up-to-date' && c !== 'checking',
      page: sourcePage(unit),
    };
  };
  const menuItems = (unit: UpdateUnit): ContextMenuItem[] => {
    const { check, update, page } = rowActions(unit);
    return [
      ...(check
        ? [
            {
              key: 'check',
              label: t('update.tab.checkNow'),
              icon: <RefreshCw size={14} />,
              onSelect: () => runCheck([unit]),
            },
          ]
        : []),
      ...(update
        ? [
            {
              key: 'update',
              label: t('update.row.update'),
              icon: <CircleArrowUp size={14} />,
              onSelect: () => startUpdate([unit.name], force),
            },
          ]
        : []),
      ...(page
        ? [
            {
              key: 'source',
              label: t('update.row.openOn', { host: page.host }),
              icon: <ExternalLink size={14} />,
              onSelect: () => window.open(page.url, '_blank', 'noopener,noreferrer'),
            },
          ]
        : []),
    ];
  };

  const failureActions = (unit: UpdateUnit) => {
    const r = run.get(unit.name);
    if (r?.status === 'blocked' || (r?.status === 'error' && isForceRetryable(r.message))) {
      return (
        <Button variant="secondary" size="sm" onClick={() => retryUpdate(unit.name, 'force')}>
          {t('update.updating.forceRetry')}
        </Button>
      );
    }
    if (r?.status === 'error' && isStaleError(r.message)) {
      return (
        <Button variant="secondary" size="sm" onClick={() => purge(unit.name)}>
          <Trash2 size={14} />
          {t('update.updating.purge')}
        </Button>
      );
    }
    return statusCell(unit);
  };

  let hero: { icon: ReactNode; tone: string; title: ReactNode; sub: ReactNode };
  if (checking) {
    hero = {
      icon: <CircleArrowUp size={20} />,
      tone: '',
      title: t('update.tab.checking'),
      sub: t('update.tab.audited'),
    };
  } else if (running) {
    hero = {
      icon: <CircleArrowUp size={20} />,
      tone: '',
      title: (
        <>
          {t('update.hero.running', { current: Math.min(runDone + 1, run.size), total: run.size })}
          {current && (
            <span className="ml-3 font-mono text-[14px] font-medium text-ink-2">
              {current.label}
            </span>
          )}
        </>
      ),
      sub: tallyText || t('update.tab.audited'),
    };
  } else if (showDone) {
    const troubled = tally.blocked + tally.error > 0;
    hero = {
      icon: troubled ? <CircleAlert size={20} /> : <DrawnCheck />,
      tone: troubled ? 'warn' : 'ok',
      title: (
        <>
          {t('update.done.subtitle')} {tallyText}
        </>
      ),
      sub:
        !checkSync || diff.isError
          ? t('update.tab.audited')
          : syncChecking
            ? t('sync.checking')
            : syncFirst
              ? t('update.done.syncHint')
              : troubled
                ? t('update.done.failedHint')
                : t('syncPreview.allUpToDate'),
    };
  } else {
    const summary = lastChecked
      ? t('update.check.checkedAt', { time: formatRelativeTime(lastChecked, locale) })
      : '';
    const failedChecks = units.filter((u) => unitCheck(statuses, u).status === 'error').length;
    // Failures headline only when nothing can be updated; otherwise the updates lead.
    const failedLead = failedChecks > 0 && available === 0;
    hero = {
      icon: failedLead ? <CircleAlert size={20} /> : <CircleArrowUp size={20} />,
      tone: failedLead ? 'warn' : '',
      title: t(updateCheckTitle(failedChecks > 0, !!lastChecked, available), {
        count: available,
      }),
      sub: (
        <>
          {failedChecks > 0 && !failedLead && (
            <>
              <button
                type="button"
                className="inline-flex min-h-6 items-center font-medium text-bad hover:underline"
                onClick={() => setFilters({ ...NO_FILTERS, status: 'failed' })}
              >
                {t(plural('update.hero.failedChecks', failedChecks), {
                  count: formatNumber(failedChecks, locale),
                })}
              </button>
              {' · '}
            </>
          )}
          {`${summary} ${t('update.tab.audited')}`.trim()}
        </>
      ),
    };
  }

  return (
    <>
      {units.length > 0 && (
        <section className="ss-box flex flex-col gap-4 -mt-2" aria-live="polite">
          <div className="flex flex-wrap items-center gap-4">
            <span className={`ss-update-icon ${hero.tone}`}>{hero.icon}</span>
            <div className="flex flex-col gap-0.5 min-w-0 sm:min-w-[240px] flex-1 break-words">
              <div
                key={String(running) + String(checking) + String(showDone)}
                className="text-[16px] font-semibold animate-fade-in"
              >
                {hero.title}
              </div>
              <div className="text-[13px] text-ink-2">{hero.sub}</div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Checkbox
                size="sm"
                className="mr-2 text-[13px]"
                label={t('update.tab.force')}
                checked={force}
                onChange={setForce}
                disabled={busy}
              />
              <Button
                variant="secondary"
                loading={checking}
                disabled={running}
                onClick={() => runCheck()}
              >
                <RefreshCw size={15} />
                {t(lastChecked ? 'update.tab.checkAgain' : 'update.tab.checkNow')}
              </Button>
              {/* A check limited by names leaves agents out, so only skills can be checked by selection. */}
              {kind === 'skill' && shown.length > 0 && (
                <Button variant="secondary" disabled={busy} onClick={() => runCheck(shown)}>
                  {t('update.header.checkSelected', { count: shown.length })}
                </Button>
              )}
              {shown.length > 0 && (
                <Button
                  variant="secondary"
                  disabled={busy}
                  onClick={() =>
                    startUpdate(
                      shown.map((u) => u.name),
                      force,
                    )
                  }
                >
                  {t('update.header.updateSelected', { count: shown.length })}
                </Button>
              )}
              {syncFirst ? (
                <Button variant="primary" onClick={() => setSyncOpen(true)}>
                  <RefreshCw size={15} />
                  {t('syncPreview.syncNowButton')}
                </Button>
              ) : (
                <Button
                  variant="primary"
                  disabled={busy || syncChecking}
                  onClick={() =>
                    startUpdate(
                      units.map((u) => u.name),
                      force,
                    )
                  }
                >
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
              <Button variant="secondary" size="sm" onClick={() => void diff.refetch()}>
                {t('syncPreview.retryButton')}
              </Button>
            </div>
          )}
          <ProgressBar
            active={busy}
            value={checking ? undefined : runDone}
            max={run.size}
            label={checking ? t('update.tab.checking') : t('update.status.updating')}
            className="[--bar-gap:16px]"
          />
        </section>
      )}

      {showMissing &&
        (() => {
          const { done, summary, contents } = missing;
          const failed = missing.failed.size;
          return (
            <div className={`ss-collapse ${missing.phase === 'closing' ? 'shut' : ''}`}>
              <div
                className={`ss-note live items-center ${done && !failed ? 'ok' : 'warn'}`}
                aria-live="polite"
              >
                {missing.phase === 'running' ? (
                  <Download size={16} />
                ) : done ? (
                  failed ? (
                    <CircleAlert size={16} className="ss-pop" />
                  ) : (
                    <Check size={16} strokeWidth={2.6} className="ss-pop" />
                  )
                ) : (
                  <FolderX size={16} />
                )}
                <div key={missing.phase} className="flex-1 min-w-0 animate-fade-in">
                  {missing.phase === 'running' ? (
                    <>
                      <b className="ss-shimmer">
                        {t('install.fromConfig.installing', { file: missing.file })}
                      </b>{' '}
                      {t('install.audited')}
                    </>
                  ) : done ? (
                    <>
                      <b>{summary}.</b>{' '}
                      {t(failed ? 'install.fromConfig.failedHint' : 'install.fromConfig.syncHint', {
                        file: missing.file,
                      })}
                    </>
                  ) : (
                    <>
                      <b>
                        {t(plural('update.missingEntries.title', missing.entries.length), {
                          count: missing.entries.length,
                          file: missing.file,
                        })}
                      </b>{' '}
                      ({contents}):{' '}
                      <span className="font-mono">
                        {missing.entries.map((e) => e.name).join(', ')}
                      </span>
                      . {t('update.missingEntries.description')}
                      {missing.error && <span className="block text-bad">{missing.error}</span>}
                    </>
                  )}
                </div>
                {(missing.phase === 'idle' || (done && failed > 0)) && (
                  <Button variant="secondary" size="sm" onClick={missing.run}>
                    <Download size={14} />
                    {t(done ? 'install.preview.retry' : 'update.missingEntries.install')}
                  </Button>
                )}
              </div>
            </div>
          );
        })()}

      {(staleUnits.length > 0 || pruned > 0) && (
        <div className={`ss-collapse ${staleUnits.length === 0 ? 'shut' : ''}`}>
          <section className="flex flex-col gap-2" aria-labelledby="update-stale-title">
            <div className="flex items-center gap-3">
              <h2
                id="update-stale-title"
                className="m-0 flex-1 text-[13px] font-semibold text-warn"
              >
                {t('update.stale.title', {
                  count: staleUnits.filter((u) => pruning.get(u.name) !== 'out').length,
                })}
              </h2>
              <Button
                variant="secondary"
                size="sm"
                disabled={busy || pruning.size > 0}
                onClick={() => void prune(staleUnits)}
              >
                <Trash2 size={14} />
                {t('update.stale.pruneAll')}
              </Button>
            </div>
            <div className="ss-list">
              {staleUnits.map((unit) => {
                const p = pruning.get(unit.name);
                const order = staleUnits.filter((u) => pruning.get(u.name) === 'out').indexOf(unit);
                return (
                  // Rows of one Prune all leave one after another.
                  <div
                    key={unit.name}
                    className={`ss-collapse ${p === 'out' ? 'out' : ''}`}
                    style={
                      p === 'out' ? ({ '--d': `${order * 60}ms` } as CSSProperties) : undefined
                    }
                  >
                    <div className={`ss-r flex-wrap md:flex-nowrap ${p ? 'updating' : ''}`}>
                      {unitIcon(unit)}
                      {unitName(unit)}
                      <div className="w-full flex flex-wrap items-center gap-2 pl-[38px] md:contents">
                        {p ? (
                          <span className="ss-st on ss-shimmer">
                            {t('update.updating.purging')}
                          </span>
                        ) : (
                          <>
                            <span className="ss-st warn">{t('update.check.stale')}</span>
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={busy}
                              onClick={() => void prune([unit])}
                            >
                              {t('update.updating.purge')}
                            </Button>
                          </>
                        )}
                      </div>
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
          <Check size={15} strokeWidth={2.6} className="ss-pop text-ok" />
          {t(plural('update.stale.pruned', pruned), { count: pruned })}
        </p>
      )}

      {failedUnits.length > 0 && (
        <section
          className="flex flex-col gap-2 animate-fade-in"
          aria-labelledby="update-failed-title"
        >
          <h2 id="update-failed-title" className="m-0 text-[13px] font-semibold text-bad">
            {t('update.failed.title', { count: failedUnits.length })}
          </h2>
          <div className="ss-list">
            {failedUnits.map((unit) => {
              const r = run.get(unit.name);
              const message = stripCliHint(r?.message);
              const open = openDetails.has(unit.name);
              const trouble = r?.status === 'blocked' || r?.status === 'error';
              return (
                <div key={unit.name} className="ss-r flex-col !items-stretch gap-2 py-3">
                  <div className="flex flex-wrap md:flex-nowrap items-center gap-3">
                    {r?.status === 'blocked' ? (
                      <span className="w-[26px] grid place-items-center text-bad">
                        <ShieldAlert size={16} />
                      </span>
                    ) : (
                      unitIcon(unit)
                    )}
                    <span className="flex flex-col min-w-0 flex-1 gap-0.5">
                      {unitName(unit)}
                      {trouble && message && (
                        <span className="text-[13px] text-bad truncate">
                          {failureSummary(message)}
                        </span>
                      )}
                    </span>
                    <div className="w-full flex flex-wrap items-center gap-2 pl-[38px] md:contents">
                      {riskTag(unit)}
                      {trouble && message && (
                        <Button
                          variant="ghost"
                          size="sm"
                          aria-expanded={open}
                          onClick={() =>
                            setOpenDetails((prev) => {
                              const next = new Set(prev);
                              if (!next.delete(unit.name)) next.add(unit.name);
                              return next;
                            })
                          }
                        >
                          {t(open ? 'update.failed.hideDetails' : 'update.failed.showDetails')}
                        </Button>
                      )}
                      <span className="flex flex-wrap items-center justify-end gap-2 md:min-w-[96px]">
                        {trouble ? failureActions(unit) : statusCell(unit)}
                      </span>
                    </div>
                  </div>
                  {trouble && open && message && (
                    <pre className="m-0 ml-[38px] p-3 rounded-lg bg-sunken font-mono text-xs leading-relaxed text-ink-2 whitespace-pre-wrap break-words animate-fade-in">
                      {message}
                    </pre>
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
              <span className="w-[26px] grid place-items-center">
                <Link size={15} />
              </span>
              <div className="flex flex-col min-w-0 gap-1">
                <span className="nm m">{repo.name}</span>
                <span className="font-mono text-xs break-all">{repo.target}</span>
                <span className="text-[13px]">{t('update.linkedRepos.note')}</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {units.length === 0
        ? !showMissing &&
          linkedRepos.length === 0 && (
            <EmptyState
              icon={CircleCheck}
              title={t(kind === 'agent' ? 'update.empty.agentsTitle' : 'update.empty.title')}
              description={t(
                kind === 'agent' ? 'update.empty.agentsDescription' : 'update.empty.description',
              )}
            />
          )
        : listUnits.length > 0 && (
            <div className="flex flex-col gap-3">
              <div ref={sentinel} className="-mb-[13px] h-px" />
              <div
                className={`ss-stickbar flex flex-wrap items-center gap-2 ${stuck ? 'stuck' : ''}`}
              >
                <label className="ss-inp w-[200px] shrink-0">
                  <Search size={15} className="shrink-0 text-ink-3" />
                  <input
                    type="text"
                    value={filters.query}
                    onChange={(e) => setFilter({ query: e.target.value })}
                    placeholder={t(
                      kind === 'agent' ? 'resources.search.agents' : 'resources.search.skills',
                    )}
                    aria-label={t(
                      kind === 'agent' ? 'resources.search.agents' : 'resources.search.skills',
                    )}
                  />
                  {filters.query ? (
                    <button
                      type="button"
                      className="ss-ib !w-6 !h-6 -mr-1.5 shrink-0"
                      aria-label={t('resources.search.clear')}
                      onClick={(e) => {
                        setFilter({ query: '' });
                        e.currentTarget.closest('label')?.querySelector('input')?.focus();
                      }}
                    >
                      <X size={14} />
                    </button>
                  ) : (
                    <span className="k">/</span>
                  )}
                </label>
                {filtering && (
                  <span className="shrink-0 text-[13px] text-ink-3">
                    {matched.length} / {countLabel(t, kind, listUnits.length)}
                  </span>
                )}
                <Select
                  className="shrink-0 ml-2"
                  prefix={t('resources.toolbar.source')}
                  chip={chipOf('resources.toolbar.source')}
                  value={filters.source}
                  onChange={(v) => setFilter({ source: v as UpdateFilters['source'] })}
                  options={(() => {
                    const base = chipCounts('source');
                    return (['all', ...UPDATE_SOURCES] as const).map((v) =>
                      countOption(
                        v,
                        SOURCE_LABEL[v],
                        v === 'all' ? base.length : base.filter((u) => unitSource(u) === v).length,
                        filters.source,
                      ),
                    );
                  })()}
                />
                <Select
                  className="shrink-0"
                  prefix={t('resources.toolbar.status')}
                  chip={chipOf('resources.toolbar.status')}
                  value={filters.status}
                  onChange={(v) => setFilter({ status: v as UpdateFilters['status'] })}
                  options={(() => {
                    const base = chipCounts('status');
                    return [
                      countOption('all', t('resources.status.all'), base.length, filters.status),
                      ...UPDATE_STATUSES.map((v) =>
                        countOption(
                          v,
                          statusLabel[v],
                          base.filter((u) => statusGroup(statusOf(u)) === v).length,
                          filters.status,
                        ),
                      ),
                    ];
                  })()}
                />
                {folders.size > 1 && (
                  <Select
                    className="shrink-0"
                    prefix={t('resources.toolbar.folder')}
                    chip={chipOf('resources.toolbar.folder')}
                    // Folder values carry a '/' prefix: '' (the root) and a folder named "all" stay distinct from All.
                    value={filters.folder === null ? 'all' : `/${filters.folder}`}
                    onChange={(v) => setFilter({ folder: v === 'all' ? null : v.slice(1) })}
                    options={(() => {
                      const base = chipCounts('folder');
                      const current = filters.folder === null ? 'all' : `/${filters.folder}`;
                      return [
                        countOption('all', t('resources.status.all'), base.length, current),
                        ...[...folders.keys()]
                          .sort((a, b) =>
                            formatTrackedRepoName(a).localeCompare(formatTrackedRepoName(b)),
                          )
                          .map((key) =>
                            countOption(
                              `/${key}`,
                              key === '' ? t('resources.folder.root') : formatTrackedRepoName(key),
                              base.filter((u) => unitFolder(u) === key).length,
                              current,
                            ),
                          ),
                      ];
                    })()}
                  />
                )}
              </div>
              {matched.length === 0 ? (
                <EmptyState
                  icon={SearchX}
                  title={t('resources.noMatches.title')}
                  description={t('resources.noMatches.description')}
                  action={
                    <Button variant="secondary" size="sm" onClick={() => setFilters(NO_FILTERS)}>
                      {t('resources.clearFilters')}
                    </Button>
                  }
                />
              ) : (
                <div className="ss-list">
                  {visible.length > 0 && (
                    <div className="ss-lh">
                      <Checkbox
                        hideLabel
                        label={t('resources.select.selectAll')}
                        checked={allSelected}
                        indeterminate={shown.length > 0 && !allSelected}
                        disabled={busy}
                        onChange={() =>
                          setSelected((prev) => {
                            const next = new Set(prev);
                            for (const u of visible) {
                              if (allSelected) next.delete(u.name);
                              else next.add(u.name);
                            }
                            return next;
                          })
                        }
                      />
                      <span className="w-[26px]" />
                      <span className="flex-1">{t('resources.col.name')}</span>
                    </div>
                  )}
                  {visible.map((unit) => {
                    const r = run.get(unit.name);
                    const c = unitCheck(statuses, unit).status;
                    const hasPreview =
                      !r &&
                      kind === 'skill' &&
                      hasUpstreamPreview(unit) &&
                      hasUpdate(unitCheck(statuses, unit));
                    const hasChanges = !r && kind === 'skill' && !unit.isRepo && c === 'dirty';
                    const rowState =
                      r?.status === 'in-progress'
                        ? 'updating'
                        : r?.status === 'pending' || (running && !r)
                          ? 'dim'
                          : r?.status === 'success'
                            ? 'flash'
                            : '';
                    return (
                      <div
                        key={unit.name}
                        className={`ss-r flex-wrap md:flex-nowrap ${selected.has(unit.name) ? 'sel' : ''} ${rowState}`}
                      >
                        <Checkbox
                          hideLabel
                          label={unit.label}
                          checked={selected.has(unit.name)}
                          disabled={busy}
                          onChange={() => toggle(unit.name)}
                        />
                        {unitIcon(unit)}
                        {listName(unit)}
                        <div className="w-full flex flex-wrap items-center gap-2 pl-[66px] md:contents">
                          {riskTag(unit)}
                          {hasPreview && (
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={busy}
                              onClick={() => setPreview({ unit, mode: 'upstream' })}
                            >
                              {t('update.row.preview')}
                            </Button>
                          )}
                          {hasChanges && (
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={busy}
                              onClick={() => setPreview({ unit, mode: 'local' })}
                            >
                              {t('update.check.viewChanges')}
                            </Button>
                          )}
                          {/* With nothing to offer (an agent mid-run, say) the button would open nothing. */}
                          {Object.values(rowActions(unit)).some(Boolean) ? (
                            <button
                              type="button"
                              className="ss-ib shrink-0"
                              aria-label={t('update.row.more', { name: unit.label })}
                              aria-haspopup="menu"
                              onClick={(e) => openMenu(e, unit)}
                            >
                              <Ellipsis size={16} />
                            </button>
                          ) : (
                            <span className="ss-ib invisible shrink-0" aria-hidden="true" />
                          )}
                          {/* Keyed by status, so a result that arrives fades in instead of swapping in place. */}
                          <span
                            key={`${run.get(unit.name)?.status ?? ''}:${statusOf(unit)}`}
                            className="animate-fade-in max-w-full md:max-w-none md:min-w-[140px] md:shrink-0 flex justify-end ml-auto md:ml-0"
                          >
                            {statusCell(unit)}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                  {foldable.length > 0 ? (
                    <button
                      type="button"
                      className="ss-r w-full !min-h-[44px] justify-center text-[13px] font-medium text-ink-2 hover:text-ink"
                      aria-expanded={showCurrent}
                      onClick={() => setShowCurrent((v) => !v)}
                    >
                      {showCurrent
                        ? t('update.list.hideCurrent')
                        : t(plural('update.list.showCurrent', foldable.length), {
                            count: formatNumber(foldable.length, locale),
                          })}
                    </button>
                  ) : null}
                </div>
              )}
            </div>
          )}
      <SkillContextMenu
        open={!!menu}
        anchorPoint={menu?.point}
        align="end"
        items={menu ? menuItems(menu.unit) : []}
        onClose={() => setMenu(null)}
      />
      <UpdatePreviewDialog
        unit={preview?.unit ?? null}
        mode={preview?.mode ?? 'upstream'}
        onClose={() => setPreview(null)}
        onUpdate={({ force: forced, merge }) => {
          const unit = preview!.unit;
          setPreview(null);
          startUpdate([unit.name], forced ?? force, merge);
        }}
      />
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
    return new Map(
      Object.entries(parsed.items).filter(([, status]) => isStoredCheckStatus(status)),
    );
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
    window.localStorage.setItem(
      UPDATE_CHECK_CACHE_KEY,
      JSON.stringify({ version: UPDATE_CHECK_CACHE_VERSION, items }),
    );
  } catch {
    // Best-effort UI cache only.
  }
}

function isStoredCheckStatus(value: unknown): value is CheckItemStatus {
  if (!value || typeof value !== 'object') return false;
  const status = (value as Partial<CheckItemStatus>).status;
  return (
    typeof status === 'string' &&
    CHECK_STATUS_VALUES.includes(status as CheckStatus) &&
    status !== 'checking' &&
    status !== 'unchecked' &&
    // Prune acts on stale, so it must come from a check this session, not an older or other project's cache.
    status !== 'stale'
  );
}

function isStaleError(message?: string): boolean {
  if (!message) return false;
  return (
    message.includes('does not exist in repository') || message.includes('not found in repository')
  );
}

// Force Retry only helps for failures force actually changes: an audit block,
// a pull the backend rejected but would retry with --force, or a skill skipped
// for its local changes. Everything else
// (permission denied, missing metadata, scan failures that stay fail-closed)
// would just fail identically, so no button is offered.
export function isForceRetryable(message?: string): boolean {
  if (!message) return false;
  if (message.includes('post-update audit failed')) return false;
  return message.includes('blocked by security audit') || message.includes('(try force update)');
}

// Merge is offered only for the local-changes skip. A merge that conflicted or an
// audit block ends in a different message, where merging again would not help.
export function isMergeable(message?: string): boolean {
  return !!message?.includes('has local changes since install');
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
  const lines = message
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);
  return lines.find((l) => /^(CRITICAL|HIGH|MEDIUM|LOW|INFO):/.test(l)) ?? lines[0] ?? '';
}

function matchesCheckSkill(item: Skill, resultName: string): boolean {
  return item.name === resultName || item.flatName === resultName || item.relPath === resultName;
}

/** An agent's update result names it by its path without `.md`. */
function agentKey(item: Skill): string | undefined {
  return item.kind === 'agent' ? item.relPath.replace(/\.md$/, '') : undefined;
}

function isSuccessfulUpdateAction(action: string): boolean {
  return action === 'updated' || action === 'up-to-date';
}

function actionToStatus(action: string): RunStatus {
  switch (action) {
    case 'updated':
      return 'success';
    case 'error':
      return 'error';
    case 'blocked':
      return 'blocked';
    case 'up-to-date':
      return 'up-to-date';
    case 'skipped':
      return 'skipped';
    default:
      return 'success';
  }
}
