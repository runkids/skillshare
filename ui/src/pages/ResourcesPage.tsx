import { useMemo, useState, type MouseEvent as ReactMouseEvent } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Bot,
  ChevronsDownUp,
  ChevronsUpDown,
  CircleCheck,
  Download,
  Ellipsis,
  ExternalLink,
  Folder,
  FolderTree,
  GitBranch,
  Github,
  Globe,
  Info,
  LayoutGrid,
  List,
  Plus,
  Power,
  Puzzle,
  RefreshCw,
  Search,
  Target,
  Trash2,
  TriangleAlert,
  X,
} from 'lucide-react';
import { api } from '../api/client';
import type { Skill } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { globToRegex } from '../lib/glob';
import { folderOf, formatTrackedRepoName, resourceHref } from '../lib/resourceNames';
import {
  byTargetOrProject, countLabel, groupByFolder, groupBySource, limitGroups, parentPath, projectOf, repoOf, sortSkills, sourceName, splitTargets, syncedByTarget,
  SOURCE_LABEL, SOURCE_ORDER,
  type FolderGroup, type Group, type SortType, type SourceFilter,
} from '../lib/resourceGrouping';
import { useSyncMatrix } from '../hooks/useSyncMatrix';
import { useRepoUpdate } from '../hooks/useRepoUpdate';
import { useT } from '../i18n';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import { Checkbox } from '../components/Checkbox';
import ConfirmDialog from '../components/ConfirmDialog';
import AnalyzePanel from '../components/analyze/AnalyzePanel';
import EmptyState from '../components/EmptyState';
import InstallDialog from '../components/InstallDialog';
import SyncPreviewModal from '../components/SyncPreviewModal';
import { countChanges, resourceGroups } from '../components/sync/syncView';
import PageHeader from '../components/PageHeader';
import { resolveSource } from '../components/SourceBadge';
import SegmentedControl from '../components/SegmentedControl';
import { Select } from '../components/Select';
import { PageSkeleton } from '../components/Skeleton';
import TargetMenu, { SkillContextMenu } from '../components/TargetMenu';
import Tooltip from '../components/Tooltip';
import SkillTree from '../components/resources/SkillTree';
import type { SelectMode } from '../components/resources/SkillTree';
import TreeDetailPane from '../components/resources/TreeDetailPane';
import type { PaneSubject } from '../components/resources/TreeDetailPane';
import { UninstallDialog } from '../components/resources/UninstallDialog';
import TreeSplit from '../components/resources/TreeSplit';
import ArrangeMenu from '../components/resources/ArrangeMenu';
import { buildTree, findFolder, flattenTree, folderPaths, isRepoRoot, rangeIds, selectedSkills, skillsUnder, summarize } from '../components/resources/tree';
import type { TargetSummary, TreeRow } from '../components/resources/tree';
import { useToast } from '../components/Toast';
import TrashPage from './TrashPage';
import UpdatePage, { countUpdates, updateUnits, useCheckStatuses } from './UpdatePage';
import { useDiffQuery, useSkillsQuery, useSyncedTargetsQuery } from '../hooks/useSharedQueries';

type Kind = Skill['kind'];
type StatusFilter = 'all' | 'enabled' | 'disabled';
type ViewType = 'list' | 'cards' | 'tree';
type GroupBy = 'source' | 'folder' | 'none';
type Tone = 'ok' | 'off';
type Point = { x: number; y: number };
type MenuState =
  | { mode: 'item'; skill: Skill; point: Point }
  | { mode: 'folder'; path: string; summary: TargetSummary; point: Point }
  | { mode: 'repo'; repo: string; point: Point }
  | { mode: 'skill'; skill: Skill; point: Point }
  | { mode: 'bulk'; names: string[]; point: Point };
type SkillsData = { resources: Skill[] };

const EMPTY: Skill[] = [];
// ponytail: "Show more" paging instead of virtualization; add a virtual list if 1000+ rows get slow.
const STEP = 100;
const SOURCE_ICON = { tracked: GitBranch, github: Github, remote: Globe, local: Folder };
const VIEW_KEY = 'skillshare:skills-view';
const COLLAPSED_KEY = 'skillshare:folder-collapsed';
/** Enable/disable toasts replace each other, so quick on/off clicks never leave an outdated one on screen. */
const TOGGLE_TOAST = 'resource-toggle';

function loadView(): ViewType {
  try {
    const v = localStorage.getItem(VIEW_KEY);
    // 'grid' and 'grouped' are the names the previous dashboard stored.
    if (v === 'cards' || v === 'grid') return 'cards';
    if (v === 'tree' || v === 'grouped') return 'tree';
  } catch { /* storage unavailable */ }
  return 'list';
}

function loadCollapsed(): Set<string> {
  try {
    const raw = localStorage.getItem(COLLAPSED_KEY);
    if (raw) return new Set(JSON.parse(raw));
  } catch { /* corrupt or unavailable */ }
  return new Set();
}

function saveCollapsed(collapsed: Set<string>) {
  try { localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...collapsed])); } catch { /* storage unavailable */ }
}

/**
 * Global tools as icons, then how many projects the item reaches. A project's tools share icons
 * with the global ones, so drawing them too repeats the same icon once per project.
 * `reachable` lists the projects the item could sync to; without any, this renders as before projects existed.
 */
function TargetStack({ names, reachable = [], max = 4 }: { names: string[]; reachable?: string[]; max?: number }) {
  const t = useT();
  if (names.length === 0) return <span className="text-ink-3">—</span>;
  const { global, projects } = splitTargets(names);
  const icons = global.length > 0 && (
    <>
      <span className="ss-stack">
        {global.slice(0, max).map((n) => (
          <span key={n} className="ss-at"><AgentIcon target={n} size={14} /></span>
        ))}
      </span>
      {global.length > max && <span className="ml-1.5 text-xs text-ink-3">+{global.length - max}</span>}
    </>
  );
  if (reachable.length === 0) return <span className="inline-flex items-center" title={names.join(', ')}>{icons}</span>;
  const synced = new Map(projects);
  return (
    <Tooltip
      content={
        <span className="flex min-w-[180px] flex-col gap-1">
          {global.length > 0 && (
            <>
              <span className="font-semibold">{t('resources.targets.global')}</span>
              <span className="opacity-70">{global.join(', ')}</span>
            </>
          )}
          <span className={`font-semibold ${global.length > 0 ? 'mt-1.5' : ''}`}>{t('resources.targets.projects')}</span>
          {reachable.map((p) => (
            <span key={p} className="flex justify-between gap-4">
              <span>{p}</span>
              <span className="opacity-70">{synced.get(p)?.join(', ') ?? t('resources.targets.notIncluded')}</span>
            </span>
          ))}
        </span>
      }
    >
      <span className="inline-flex items-center">
        {icons}
        {projects.length > 0 && (
          <span className={`ss-pj ${global.length > 0 ? 'ml-1.5' : ''}`} aria-label={t('resources.targets.projectCount', { count: projects.length })}>
            <Folder size={12} aria-hidden="true" />{projects.length}
          </span>
        )}
      </span>
    </Tooltip>
  );
}

function menuPoint(e: ReactMouseEvent): Point {
  if (e.type === 'contextmenu') return { x: e.clientX, y: e.clientY };
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  return { x: r.left, y: r.bottom + 4 };
}

/* -- Page ----------------------------------------- */

export default function ResourcesPage({ kind }: { kind: Kind }) {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const isAgent = kind === 'agent';

  const { data, isPending, error } = useSkillsQuery();
  const { data: trashData } = useQuery({
    queryKey: queryKeys.trash,
    queryFn: () => api.listTrash(),
    staleTime: staleTimes.trash,
  });
  // Same queries as the sidebar Sync badge, so the dot costs no extra requests.
  const { data: diffData } = useDiffQuery();
  const { data: targetsData } = useSyncedTargetsQuery();
  const syncPending = diffData ? countChanges(resourceGroups(diffData.diffs, targetsData?.targets ?? [], new Set([kind]), false).groups) > 0 : false;
  const [syncOpen, setSyncOpen] = useState(false);
  const { matrix, getSkillTargets } = useSyncMatrix();
  // The project count needs room, and the wider column fits more icons; without projects nothing changes.
  const hasProjects = matrix.some((e) => projectOf(e.target));
  const targetsCol = hasProjects ? 'w-[360px]' : 'w-[140px]';
  const { updating, update } = useRepoUpdate();
  const [checks] = useCheckStatuses();
  const [params, setParams] = useSearchParams();
  const requestedTab = params.get('tab');
  // Analyze only measures skills, so the tab is hidden on the agents page.
  const tab = requestedTab === 'updates' ? 'updates' : requestedTab === 'trash' ? 'trash' : requestedTab === 'analyze' && !isAgent ? 'analyze' : 'installed';
  // The dialog lives in the URL so the old /install and /search routes can open it.
  const installTab = params.get('install');
  const setInstall = (value: string | null) =>
    setParams((p) => {
      if (value) p.set('install', value);
      else { p.delete('install'); p.delete('source'); }
      return p;
    }, { replace: true });

  const [search, setSearch] = useState('');
  const [source, setSource] = useState<SourceFilter>('all');
  const [status, setStatus] = useState<StatusFilter>('all');
  const [target, setTarget] = useState('all');
  // null = all folders; '' is the source root.
  const [folder, setFolder] = useState<string | null>(null);
  const [sort, setSort] = useState<SortType>('name-asc');
  const [group, setGroup] = useState<GroupBy>(isAgent ? 'none' : 'source');
  const [view, setView] = useState<ViewType>(loadView);
  const [limit, setLimit] = useState(STEP);
  const [collapsed, setCollapsed] = useState<Set<string>>(loadCollapsed);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  // The tree view selects folders and skills by row id, apart from the list's checkboxes.
  const [treeSel, setTreeSel] = useState<Set<string>>(new Set());
  const [anchor, setAnchor] = useState<string | null>(null);
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [uninstalling, setUninstalling] = useState<Skill[] | null>(null);
  const [confirmDisable, setConfirmDisable] = useState<string[] | null>(null);

  const all = data?.resources ?? EMPTY;
  const items = useMemo(() => all.filter((s) => s.kind === kind), [all, kind]);

  const targetIndex = useMemo(() => byTargetOrProject(syncedByTarget(items, matrix)), [items, matrix]);
  // A target that stopped appearing (kind switch, uninstall) would filter everything out.
  const activeTarget = targetIndex.has(target) ? target : 'all';
  // Global tools, then projects. Headings appear only once there are projects to tell apart.
  const targetOptions = useMemo(() => {
    const entries = [...targetIndex].sort(([a], [b]) => Number(a.endsWith('@')) - Number(b.endsWith('@')) || a.localeCompare(b));
    const grouped = entries.some(([name]) => name.endsWith('@'));
    return entries.map(([name, set]) => name.endsWith('@')
      ? { value: name, label: `${name.slice(0, -1)} (${set.size})`, icon: <Folder size={13} />, group: t('resources.targets.projects') }
      : { value: name, label: `${name} (${set.size})`, group: grouped ? t('resources.targets.global') : undefined });
  }, [targetIndex, t]);
  const folderIndex = useMemo(() => {
    const counts = new Map<string, number>();
    for (const s of items) counts.set(folderOf(s), (counts.get(folderOf(s)) ?? 0) + 1);
    return counts;
  }, [items]);
  const activeFolder = folder !== null && folderIndex.has(folder) ? folder : null;
  const updateCount = useMemo(() => countUpdates(checks, updateUnits(all, kind)), [checks, all, kind]);
  const query = search.trim();
  const isGlob = /[*?]/.test(query);
  const filtering = query !== '' || source !== 'all' || status !== 'all' || activeTarget !== 'all' || activeFolder !== null;

  const filtered = useMemo(() => {
    const re = query ? globToRegex(query) : null;
    const glob = /[*?]/.test(query);
    return sortSkills(items.filter((s) =>
      (!re || re.test(s.name) || re.test(s.relPath) || re.test(s.flatName) || (!glob && re.test(s.source ?? ''))) &&
      (source === 'all' || resolveSource(s.type, s.isInRepo) === source) &&
      (status === 'all' || (status === 'disabled') === !!s.disabled) &&
      (activeTarget === 'all' || (targetIndex.get(activeTarget)?.has(s.flatName) ?? false)) &&
      (activeFolder === null || folderOf(s) === activeFolder),
    ), sort);
  }, [items, query, source, status, sort, activeTarget, targetIndex, activeFolder]);

  const groups = useMemo(() => groupBySource(filtered), [filtered]);
  const folderGroups = useMemo(() => groupByFolder(filtered), [filtered]);
  const tree = useMemo(() => buildTree(filtered), [filtered]);
  const treeRows = useMemo(() => flattenTree(tree, collapsed, filtering), [tree, collapsed, filtering]);
  const shownTreeRows = useMemo(() => {
    let left = limit;
    const rows: TreeRow[] = [];
    for (const r of treeRows) {
      if (r.type === 'item' && left-- <= 0) break;
      rows.push(r);
    }
    return rows;
  }, [treeRows, limit]);
  const byName = useMemo(() => new Map(items.map((s) => [s.flatName, s])), [items]);
  const treeSkills = useMemo(() => selectedSkills(tree, treeSel, byName), [tree, treeSel, byName]);
  const paneSubject = useMemo((): PaneSubject => {
    if (treeSkills.length === 0) return { type: 'none' };
    const [only] = treeSel.size === 1 ? treeSel : [];
    const node = only?.startsWith('f:') ? findFolder(tree, only.slice(2)) : undefined;
    if (node) return { type: 'folder', node, skills: treeSkills };
    if (only) return { type: 'skill', skill: treeSkills[0] };
    return { type: 'multi', skills: treeSkills };
  }, [tree, treeSel, treeSkills]);

  const selectedItems = useMemo(() => items.filter((s) => selected.has(s.flatName)), [items, selected]);
  const allSelected = filtered.length > 0 && filtered.every((s) => selected.has(s.flatName));
  const someSelected = !allSelected && filtered.some((s) => selected.has(s.flatName));
  const trashCount = (trashData?.items ?? []).filter((i) => (i.kind ?? 'skill') === kind).length;

  // Which targets can receive agents at all. Only agents get 'na' entries in the matrix.
  const agentSupport = useMemo(() => {
    if (!isAgent) return null;
    const names = new Set(items.map((s) => s.flatName));
    const targets = new Set<string>();
    const supported = new Set<string>();
    for (const e of matrix) {
      targets.add(e.target);
      if (names.has(e.skill) && e.status !== 'na') supported.add(e.target);
    }
    return { supported: [...supported].sort(), total: targets.size };
  }, [isAgent, items, matrix]);

  const rowInfo = (s: Skill): { synced: string[]; reachable: string[]; tone: Tone; label: string } => {
    const entries = getSkillTargets(s.flatName);
    const synced = entries.filter((e) => e.status === 'synced').map((e) => e.target).sort();
    const applicable = entries.some((e) => e.status !== 'na');
    // Projects this item could sync to, whether or not it does: the tooltip names the ones it misses.
    const reachable = [...new Set(entries.filter((e) => e.status !== 'na').map((e) => projectOf(e.target)).filter(Boolean))].sort();
    if (s.disabled) return { synced, reachable, tone: 'off', label: t('resources.status.disabled') };
    if (entries.length > 0 && !applicable) return { synced, reachable, tone: 'off', label: t('resources.tree.noAgentTargets.label') };
    if (applicable && synced.length === 0) return { synced, reachable, tone: 'off', label: t('resources.tree.filteredOut.label') };
    return { synced, reachable, tone: 'ok', label: t('resources.status.enabled') };
  };
  const syncedCell = (s: Skill) => {
    const { synced, reachable } = rowInfo(s);
    return <TargetStack names={synced} reachable={reachable} max={8} />;
  };

  /* -- Mutations -- */

  const refreshAfterTargets = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    queryClient.invalidateQueries({ queryKey: ['sync-matrix'] });
  };

  /** Optimistic patch of the skills cache; returns the snapshot to roll back to. */
  const patch = (fn: (skills: Skill[]) => Skill[]) => {
    queryClient.cancelQueries({ queryKey: queryKeys.skills.all });
    const previous = queryClient.getQueryData<SkillsData>(queryKeys.skills.all);
    if (previous) queryClient.setQueryData<SkillsData>(queryKeys.skills.all, { ...previous, resources: fn(previous.resources) });
    return { previous };
  };
  const rollback = (err: Error, _: unknown, ctx?: { previous?: SkillsData }) => {
    if (ctx?.previous) queryClient.setQueryData(queryKeys.skills.all, ctx.previous);
    toast(err.message, 'error');
  };

  const toggleOne = useMutation({
    mutationFn: ({ s, disable }: { s: Skill; disable: boolean }) =>
      disable ? api.disableResource(s.flatName, s.kind) : api.enableResource(s.flatName, s.kind),
    onMutate: ({ s, disable }) => patch((list) => list.map((x) => (x.flatName === s.flatName && x.kind === s.kind ? { ...x, disabled: disable } : x))),
    onSuccess: (_, { s, disable }) => {
      const kindLabel = isAgent ? 'Agent' : 'Skill';
      toast(t(disable ? 'resources.toast.disabled' : 'resources.toast.enabled', { kind: kindLabel, name: s.name }), 'success', { key: TOGGLE_TOAST });
    },
    onError: rollback,
    onSettled: refreshAfterTargets,
  });

  const toggleMany = useMutation({
    mutationFn: ({ names, enable }: { names: string[]; enable: boolean }) => api.batchToggleResources(names, enable, kind),
    onMutate: ({ names, enable }) => {
      const set = new Set(names);
      return patch((list) => list.map((x) => (x.kind === kind && set.has(x.flatName) ? { ...x, disabled: !enable } : x)));
    },
    onSuccess: (res, { enable }) => {
      const { updated, unchanged, failed } = res.summary;
      // Name the first failure: the count alone hides why an item stayed as it was.
      const firstError = res.results.find((r) => r.error);
      const reason = firstError ? ` — ${firstError.name}: ${firstError.error}` : '';
      if (failed > 0 && updated > 0) toast(t('resources.batchToggle.toast.partial', { updated, failed }) + reason, 'warning');
      else if (failed > 0) toast(t(failed === 1 ? 'resources.batchToggle.toast.failed.one' : 'resources.batchToggle.toast.failed.other', { count: failed }) + reason, 'error');
      else if (updated === 0 && unchanged > 0) toast(t('resources.batchToggle.toast.noChange'), 'info', { key: TOGGLE_TOAST });
      else toast(t(`resources.batchToggle.toast.${enable ? 'enabled' : 'disabled'}.${updated === 1 ? 'one' : 'other'}`, { count: updated }), 'success', { key: TOGGLE_TOAST });
      setSelected(new Set());
    },
    onError: rollback,
    onSettled: () => {
      refreshAfterTargets();
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    },
  });

  const setTargets = useMutation({
    mutationFn: ({ name, target }: { name: string; target: string | null }) => api.setSkillTargets(name, target),
    onMutate: ({ name, target }) => patch((list) => list.map((x) => (x.flatName === name ? { ...x, targets: target ? [target] : undefined } : x))),
    onSuccess: (_, { name, target }) => toast(t('resources.toast.nowAvailableIn', { name, target: target ?? t('resources.targets.all') }), 'success'),
    onError: rollback,
    onSettled: refreshAfterTargets,
  });

  const setFolderTargets = useMutation({
    mutationFn: ({ folder, target }: { folder: string; target: string | null }) => api.batchSetTargets(folder, target),
    onSuccess: (res, { folder: path, target }) => {
      const folder = formatTrackedRepoName(path);
      if (res.updated === 0 && res.skipped > 0) toast(t('resources.folder.noEditableSkills', { folder }), 'error');
      else toast(t('resources.folder.skillsUpdated', { count: res.updated, folder, target: target ?? t('resources.targets.all') }), 'success');
    },
    onError: (err: Error) => toast(err.message, 'error'),
    onSettled: refreshAfterTargets,
  });

  const setManyTargets = async (names: string[], target: string | null) => {
    const results = await Promise.allSettled(names.map((n) => api.setSkillTargets(n, target)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    if (failed === 0) toast(t(names.length === 1 ? 'resources.bulk.targetsSet.one' : 'resources.bulk.targetsSet.other', { count: names.length, target: target ?? t('resources.targets.all') }), 'success');
    else toast(t('resources.batchToggle.toast.partial', { updated: names.length - failed, failed }), failed === names.length ? 'error' : 'warning');
    refreshAfterTargets();
  };

  /* -- Selection & filters -- */

  const toggle = (name: string) => setSelected((prev) => {
    const next = new Set(prev);
    if (!next.delete(name)) next.add(name);
    return next;
  });
  const selectAll = (on: boolean) => setSelected(on ? new Set(filtered.map((s) => s.flatName)) : new Set());
  const selectNode = (id: string, mode: SelectMode) => {
    if (mode === 'range') {
      setTreeSel(new Set(rangeIds(shownTreeRows, anchor, id)));
      if (!anchor) setAnchor(id);
      return;
    }
    setAnchor(id);
    setTreeSel((prev) => {
      if (mode === 'only') return new Set([id]);
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  };
  const resetting = <T,>(set: (v: T) => void) => (v: T) => { set(v); setLimit(STEP); };
  /** Toolbar filters are chips: 'all' is unset, and the chip's clear button goes back to it. */
  const chipOf = (key: string) => ({ clearValue: 'all', clearLabel: t('resources.toolbar.clearFilter', { name: t(key) }) });
  const clearFilters = () => { setSearch(''); setSource('all'); setStatus('all'); setFolder(null); setLimit(STEP); };

  const changeView = (v: ViewType) => {
    setView(v);
    setLimit(STEP);
    try { localStorage.setItem(VIEW_KEY, v); } catch { /* storage unavailable */ }
  };
  const updateCollapsed = (next: Set<string>) => { setCollapsed(next); saveCollapsed(next); };
  const toggleFolder = (path: string) => {
    const next = new Set(collapsed);
    if (!next.delete(path)) next.add(path);
    updateCollapsed(next);
  };

  const openItemMenu = (e: ReactMouseEvent, skill: Skill) => {
    e.preventDefault();
    e.stopPropagation();
    setMenu({ mode: 'item', skill, point: menuPoint(e) });
  };
  const openRow = (e: ReactMouseEvent, s: Skill) => {
    if ((e.target as HTMLElement).closest('a,button,label,input')) return;
    navigate(resourceHref(s));
  };

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <div className="ss-empty">
        <TriangleAlert size={24} className="text-bad" />
        <h3 className="font-semibold text-ink">{t('resources.error.failedToLoad')}</h3>
        <p className="text-[13px]">{error.message}</p>
      </div>
    );
  }

  /* -- Rendering helpers -- */

  const status$ = (tone: Tone, label: string) => <span className={`ss-st ${tone}`}>{label}</span>;
  const actionsButton = (s: Skill) => (
    <button type="button" className="ss-ib" aria-label={t('resources.table.actions')} onClick={(e) => openItemMenu(e, s)}>
      <Ellipsis size={16} />
    </button>
  );
  const selectBox = (s: Skill) => (
    <Checkbox hideLabel label={s.name} checked={selected.has(s.flatName)} onChange={() => toggle(s.flatName)} />
  );

  const repoActions = (repo: string) => (
    <>
      <Button variant="secondary" size="sm" loading={updating === repo} disabled={updating !== null} onClick={() => update(repo)}>
        {t('resources.repo.update')}
      </Button>
      <button
        type="button"
        className="ss-ib"
        aria-label={t('resources.repo.actions')}
        onClick={(e) => { e.stopPropagation(); setMenu({ mode: 'repo', repo, point: menuPoint(e) }); }}
      >
        <Ellipsis size={16} />
      </button>
    </>
  );

  const groupHead = (g: Group, asLabel: boolean) => {
    const Icon = SOURCE_ICON[g.source];
    const meta = [countLabel(t, kind, g.items.length), g.repo ? g.items[0].branch : ''];
    return (
      <div key={`g:${g.key}`} className={asLabel ? 'ss-gl' : 'ss-gh'}>
        <Icon size={15} className="shrink-0 text-ink-2" />
        {g.repo ? <b className="font-mono">{formatTrackedRepoName(g.repo)}</b> : <b>{SOURCE_LABEL[g.source]}</b>}
        {g.repo && <span className="ss-tag">tracked</span>}
        <span className="text-ink-3">{meta.filter(Boolean).join(' · ')}</span>
        <span className="flex-1" />
        {g.repo && !isAgent && repoActions(g.repo)}
      </div>
    );
  };

  const folderName = (key: string) => (key === '' ? t('resources.folder.root') : formatTrackedRepoName(key));

  const folderHead = (g: FolderGroup, asLabel: boolean) => (
    <div key={`f:${g.key}`} className={asLabel ? 'ss-gl' : 'ss-gh'}>
      <Folder size={15} className="shrink-0 text-ink-2" />
      <b className={g.key ? 'font-mono' : ''}>{folderName(g.key)}</b>
      {g.repo && <span className="ss-tag">tracked</span>}
      <span className="text-ink-3">{countLabel(t, kind, g.items.length)}</span>
    </div>
  );

  const itemRow = (s: Skill) => {
    const { synced, reachable, tone, label } = rowInfo(s);
    // Grouped by folder, the header already names the parent path.
    const sub = group === 'folder' ? '' : parentPath(s, group === 'source');
    return (
      <div
        key={s.flatName}
        className={`ss-r link ${selected.has(s.flatName) ? 'sel' : ''}`}
        onClick={(e) => openRow(e, s)}
        onContextMenu={(e) => openItemMenu(e, s)}
      >
        {selectBox(s)}
        <span className="flex flex-col min-w-0 flex-1 gap-px">
          <span className="flex min-w-0 items-center gap-2">
            <Link to={resourceHref(s)} className={`nm m truncate hover:underline ${s.disabled ? 'text-ink-3' : ''}`}>{s.name}</Link>
            {s.manualOnly && <span className="ss-tag shrink-0" title={t('frontmatterEditor.field.disableModelInvocation.hint')}>manual only</span>}
          </span>
          {sub && <span className="font-mono text-xs text-ink-3 truncate">{sub}</span>}
        </span>
        {group !== 'source' && view === 'list' && <span className="w-[150px] font-mono text-xs text-ink-3 truncate">{sourceName(s)}</span>}
        <span className={targetsCol}><TargetStack names={synced} reachable={reachable} max={hasProjects ? 8 : 4} /></span>
        <span className="w-[120px]">{status$(tone, label)}</span>
        {actionsButton(s)}
      </div>
    );
  };

  const card = (s: Skill) => {
    const { synced, reachable, tone, label } = rowInfo(s);
    return (
      <div
        key={s.flatName}
        className={`ss-tile cursor-pointer ${selected.has(s.flatName) ? 'sel' : ''}`}
        onClick={(e) => openRow(e, s)}
        onContextMenu={(e) => openItemMenu(e, s)}
      >
        <div className="hd flex items-center gap-2.5">
          {selectBox(s)}
          <Link to={resourceHref(s)} className={`nm flex-1 min-w-0 break-words hover:underline ${s.disabled ? 'text-ink-3' : ''}`}>{s.name}</Link>
          {actionsButton(s)}
        </div>
        <span className="ds text-[13px] text-ink-2">{group === 'source' ? parentPath(s, true) : group === 'folder' ? sourceName(s) : parentPath(s) || sourceName(s)}</span>
        <div className="ft">
          <span className="flex items-center gap-2">
            <TargetStack names={synced} reachable={reachable} max={3} />
            {s.manualOnly && <span className="ss-tag shrink-0" title={t('frontmatterEditor.field.disableModelInvocation.hint')}>manual only</span>}
          </span>
          {status$(tone, label)}
        </div>
      </div>
    );
  };

  /* -- Content -- */

  const treeItemTotal = treeRows.filter((r) => r.type === 'item').length;
  const total = view === 'tree' ? treeItemTotal : filtered.length;
  const shown = Math.min(limit, total);
  let content: React.ReactNode;

  if (items.length === 0 || filtered.length === 0) {
    content = items.length === 0 ? (
      <EmptyState
        icon={isAgent ? Bot : Puzzle}
        title={t(isAgent ? 'resources.agents.empty.title' : 'resources.skills.empty.title')}
        description={t(isAgent ? 'resources.agents.empty.description' : 'resources.skills.empty.description')}
        action={<Button variant="primary" onClick={() => setInstall('url')}><Download size={15} />{t('resources.install')}</Button>}
      />
    ) : (
      <EmptyState
        icon={Search}
        title={t('resources.noMatches.title')}
        description={t('resources.noMatches.description')}
        action={<Button variant="secondary" size="sm" onClick={clearFilters}>{t('resources.clearFilters')}</Button>}
      />
    );
  } else if (view === 'cards') {
    content = group === 'source' ? (
      <div className="flex flex-col gap-6">
        {limitGroups(groups, limit).map((g) => (
          <div key={g.key}>
            {groupHead(g, true)}
            <div className="ss-tiles mt-3">{g.items.map(card)}</div>
          </div>
        ))}
      </div>
    ) : group === 'folder' ? (
      <div className="flex flex-col gap-6">
        {limitGroups(folderGroups, limit).map((g) => (
          <div key={g.key}>
            {folderHead(g, true)}
            <div className="ss-tiles mt-3">{g.items.map(card)}</div>
          </div>
        ))}
      </div>
    ) : (
      <div className="ss-tiles">{filtered.slice(0, limit).map(card)}</div>
    );
  } else if (view === 'tree') {
    const subject = paneSubject;
    const repoRoot = subject.type === 'folder' && !isAgent && isRepoRoot(subject.node) ? subject.node.path : null;
    content = (
      <TreeSplit
        tree={
          <SkillTree
            rows={shownTreeRows}
            selected={treeSel}
            kind={kind}
            label={t(isAgent ? 'layout.nav.agents' : 'layout.nav.skills')}
            onSelect={selectNode}
            onToggleFolder={toggleFolder}
            onOpen={(s) => navigate(resourceHref(s))}
          />
        }
        pane={
          <TreeDetailPane
            kind={kind}
            subject={subject}
            busy={toggleMany.isPending || toggleOne.isPending}
            onToggleAll={(list, enable) => toggleMany.mutate({ names: list.map((s) => s.flatName), enable })}
            onToggleOne={(s) => toggleOne.mutate({ s, disable: !s.disabled })}
            onSetTargets={(e) => {
              const point = menuPoint(e);
              if (subject.type === 'folder') setMenu({ mode: 'folder', path: subject.node.path, summary: summarize(skillsUnder(subject.node)), point });
              else if (subject.type === 'skill') setMenu({ mode: 'skill', skill: subject.skill, point });
              else if (subject.type === 'multi') setMenu({ mode: 'bulk', names: subject.skills.map((s) => s.flatName), point });
            }}
            onUninstall={() => setUninstalling(treeSkills)}
            syncedTo={subject.type === 'skill' && syncedCell(subject.skill)}
            repoActions={repoRoot && (
              <>
                <Button variant="secondary" size="sm" loading={updating === repoRoot} disabled={updating !== null} onClick={() => update(repoRoot)}>
                  <RefreshCw size={14} />
                  {t('resources.repo.update')}
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setUninstalling(items.filter((s) => repoOf(s) === repoRoot))}>
                  <Trash2 size={14} />
                  {t('resources.contextMenu.uninstallRepo')}
                </Button>
              </>
            )}
          />
        }
      />
    );
  } else {
    const header = (
      <div className="ss-lh">
        <Checkbox hideLabel label={t('resources.select.selectAll')} checked={allSelected} indeterminate={someSelected} onChange={selectAll} />
        {isGlob ? (
          <span className="flex-1 flex items-center gap-3 text-[13px] text-ink-2 normal-case tracking-normal">
            <span>{t('resources.glob.match', { count: filtered.length, total: items.length })} <span className="font-mono">{query}</span></span>
            <button type="button" className="font-semibold text-ink hover:underline cursor-pointer" onClick={() => selectAll(true)}>
              {t('resources.glob.selectAll', { count: filtered.length })}
            </button>
          </span>
        ) : (
          <span className="flex-1">{t('resources.col.name')}</span>
        )}
        {group !== 'source' && view === 'list' && <span className="w-[150px]">{t('resources.col.source')}</span>}
        <span className={targetsCol}>{t('resources.col.targets')}</span>
        <span className="w-[120px]">{t('resources.col.status')}</span>
        <span className="w-[30px]" />
      </div>
    );
    let body: React.ReactNode;
    if (group === 'source') {
      body = limitGroups(groups, limit).map((g) => [groupHead(g, false), ...g.items.map((s) => itemRow(s))]);
    } else if (group === 'folder') {
      body = limitGroups(folderGroups, limit).map((g) => [folderHead(g, false), ...g.items.map((s) => itemRow(s))]);
    } else {
      body = filtered.slice(0, limit).map((s) => itemRow(s));
    }
    content = <div className="ss-list">{header}{body}</div>;
  }

  // The tree view acts on its own selection through the detail pane, not the bulk toolbar.
  const count = view === 'tree' ? 0 : selectedItems.length;

  return (
    <div className={`ss-wrap animate-fade-in ${tab === 'installed' && count > 0 ? 'pb-16' : ''}`}>
      <PageHeader
        className="!mb-0"
        title={t(isAgent ? 'layout.nav.agents' : 'layout.nav.skills')}
        subtitle={isAgent ? t('resources.agents.subtitle') : t('resources.skills.subtitle', { count: items.length })}
        actions={tab === 'installed' && (
          <>
            <Button variant="secondary" onClick={() => setSyncOpen(true)}>
              <RefreshCw size={15} />
              {t(`syncPreview.title.${kind}`)}
              {syncPending && <span className="size-1.5 rounded-full bg-warn" role="img" aria-label={t('plugins.pending')} />}
            </Button>
            {!isAgent && <Link to="/hubs" className="ss-btn">{t('hubs.title')}</Link>}
            {!isAgent && (
              <Link to="/skills/new" className="ss-btn">
                <Plus size={15} />
                {t('resources.newSkill')}
              </Link>
            )}
            <Button variant="primary" data-tour="install-button" onClick={() => setInstall('url')}>
              <Download size={15} />
              {t('resources.install')}
            </Button>
          </>
        )}
      />

      <nav className="ss-tabs" aria-label={t(isAgent ? 'layout.nav.agents' : 'layout.nav.skills')} data-tour="skills-view">
        {([
          ['installed', '', t('resources.tab.installed'), items.length],
          ['updates', '?tab=updates', t('resources.tab.updates'), updateCount],
          ['trash', '?tab=trash', t('trash.title'), trashCount],
          ...(isAgent ? [] : [['analyze', '?tab=analyze', t('resources.tab.analyze'), 0] as const]),
        ] as const).map(([key, query, label, n]) => (
          <Link key={key} to={`${isAgent ? '/agents' : '/skills'}${query}`} className={tab === key ? 'on' : ''} aria-current={tab === key ? 'page' : undefined}>
            {label}
            {(key === 'installed' || n > 0) && <span className="ss-cnt">{n}</span>}
          </Link>
        ))}
      </nav>

      {tab === 'analyze' ? (
        <AnalyzePanel />
      ) : tab === 'installed' ? (
        <>
          {agentSupport && items.length > 0 && agentSupport.supported.length > 0 && agentSupport.supported.length < agentSupport.total && (
            <div className="ss-note inf">
              <Info size={16} />
              <div className="flex-1">
                <b>{t(agentSupport.supported.length === 1 ? 'resources.agents.supportTitle.one' : 'resources.agents.supportTitle.other', { count: agentSupport.supported.length, total: agentSupport.total })}</b>{' '}
                {agentSupport.supported.join(', ')}. {t('resources.agents.supportRest')}
              </div>
            </div>
          )}

          {/* Every control sizes to its label: fixed widths truncated the longer values
              ("xcode-claude", "Disabled") and pushed the row past the container. */}
          <div className="flex flex-wrap items-center gap-2 -mt-2">
            <label className="ss-inp w-[200px] shrink-0">
              <Search size={15} className="shrink-0 text-ink-3" />
              <input
                type="text"
                value={search}
                onChange={(e) => { setSearch(e.target.value); setLimit(STEP); }}
                placeholder={t(isAgent ? 'resources.search.agents' : 'resources.search.skills')}
                aria-label={t(isAgent ? 'resources.search.agents' : 'resources.search.skills')}
              />
              {!search && <span className="k">/</span>}
            </label>
            <Select
              className="shrink-0 ml-2"
              prefix={t('resources.toolbar.source')}
              chip={chipOf('resources.toolbar.source')}
              value={source}
              onChange={(v) => resetting(setSource)(v as SourceFilter)}
              options={(['all', ...SOURCE_ORDER] as SourceFilter[]).map((v) => ({ value: v, label: SOURCE_LABEL[v] }))}
            />
            <Select
              className="shrink-0"
              prefix={t('resources.toolbar.status')}
              chip={chipOf('resources.toolbar.status')}
              value={status}
              onChange={(v) => resetting(setStatus)(v as StatusFilter)}
              options={(['all', 'enabled', 'disabled'] as StatusFilter[]).map((v) => ({ value: v, label: t(`resources.status.${v}`) }))}
            />
            {targetIndex.size > 1 && (
              <Select
                className="shrink-0"
                prefix={t('resources.toolbar.target')}
                chip={chipOf('resources.toolbar.target')}
                value={activeTarget}
                onChange={resetting(setTarget)}
                options={[
                  { value: 'all', label: 'All' },
                  ...targetOptions,
                ]}
              />
            )}
            {folderIndex.size > 1 && (
              <Select
                className="shrink-0"
                prefix={t('resources.toolbar.folder')}
                chip={chipOf('resources.toolbar.folder')}
                // Folder values carry a '/' prefix: '' (the root) and a folder named "all" stay distinct from All.
                value={activeFolder === null ? 'all' : `/${activeFolder}`}
                onChange={(v) => resetting(setFolder)(v === 'all' ? null : v.slice(1))}
                options={[
                  { value: 'all', label: 'All' },
                  ...[...folderIndex]
                    .sort((a, b) => formatTrackedRepoName(a[0]).localeCompare(formatTrackedRepoName(b[0])))
                    .map(([key, n]) => ({ value: `/${key}`, label: `${folderName(key)} (${n})` })),
                ]}
              />
            )}
            <span className="flex-1" />
            {view === 'tree' && tree.children.size > 0 && (() => {
              // One button: collapses everything once all is open, otherwise opens everything.
              const paths = folderPaths(tree);
              const allOpen = !paths.some((p) => collapsed.has(p));
              const label = t(allOpen ? 'resources.folder.collapseAll' : 'resources.folder.expandAll');
              return (
                <button
                  type="button"
                  className="ss-ib !w-[34px] !h-[34px] shrink-0"
                  title={label}
                  aria-label={label}
                  onClick={() => updateCollapsed(allOpen ? new Set(paths) : new Set())}
                >
                  {allOpen ? <ChevronsDownUp size={16} /> : <ChevronsUpDown size={16} />}
                </button>
              );
            })()}
            <ArrangeMenu
              group={view === 'tree' ? undefined : {
                label: t('resources.toolbar.group'),
                value: group,
                onChange: (v) => setGroup(v as GroupBy),
                options: [
                  { value: 'source', label: t('resources.group.source') },
                  { value: 'folder', label: t('resources.group.folder') },
                  { value: 'none', label: t('resources.group.none') },
                ],
              }}
              sort={{
                label: t('resources.toolbar.sort'),
                value: sort,
                onChange: (v) => setSort(v as SortType),
                options: [
                  { value: 'name-asc', label: t('resources.sort.nameAsc') },
                  { value: 'name-desc', label: t('resources.sort.nameDesc') },
                  { value: 'newest', label: t('resources.sort.newestFirst') },
                  { value: 'oldest', label: t('resources.sort.oldestFirst') },
                ],
              }}
            />
            <SegmentedControl
              className="ic !flex-nowrap shrink-0"
              value={view}
              onChange={changeView}
              options={[
                { value: 'list', label: <List size={15} />, title: t('resources.view.list') },
                { value: 'cards', label: <LayoutGrid size={15} />, title: t('resources.view.cards') },
                { value: 'tree', label: <FolderTree size={15} />, title: t('resources.view.tree') },
              ]}
            />
          </div>

          <div className="-mt-3">
            {content}
            {total > 0 && (
              <div className="flex items-center justify-between mt-3">
                <span className="text-[13px] text-ink-3">{t('resources.shown', { shown, total })}</span>
                {shown < total && (
                  <Button variant="ghost" size="sm" onClick={() => setLimit((l) => l + STEP)}>{t('resources.showMore')}</Button>
                )}
              </div>
            )}
          </div>

          {count > 0 && (
            <div className="ss-bulk" role="toolbar" aria-label={t('resources.select.count', { count })}>
              <b>{t('resources.select.count', { count })}</b>
              <span className="dv" />
              <Button variant="secondary" size="sm" disabled={toggleMany.isPending} onClick={() => toggleMany.mutate({ names: selectedItems.map((s) => s.flatName), enable: true })}>
                <CircleCheck size={15} />
                {t('resources.batchToggle.enable')}
              </Button>
              <Button variant="secondary" size="sm" disabled={toggleMany.isPending} onClick={() => setConfirmDisable(selectedItems.map((s) => s.flatName))}>
                <Power size={15} />
                {t('resources.batchToggle.disable')}
              </Button>
              {!isAgent && (
                <Button variant="secondary" size="sm" onClick={(e) => setMenu({ mode: 'bulk', names: selectedItems.map((s) => s.flatName), point: menuPoint(e) })}>
                  <Target size={15} />
                  {t('resources.setTargets')}
                </Button>
              )}
              <Button variant="secondary" size="sm" onClick={() => setUninstalling(selectedItems)}>
                <Trash2 size={15} />
                {t('resources.contextMenu.uninstall')}
              </Button>
              <span className="dv" />
              <button type="button" className="ss-ib" aria-label={t('resources.select.clear')} onClick={() => setSelected(new Set())}>
                <X size={16} />
              </button>
            </div>
          )}

          {menu?.mode === 'item' && (
            <TargetMenu
              open
              anchorPoint={menu.point}
              currentTargets={menu.skill.targets ?? null}
              label={t('resources.contextMenu.availableIn')}
              showTargets={!isAgent}
              onSelect={(target) => setTargets.mutate({ name: menu.skill.flatName, target })}
              onClose={() => setMenu(null)}
              extraItems={[
                { key: 'detail', label: t('resources.contextMenu.viewDetail'), icon: <ExternalLink size={14} />, onSelect: () => navigate(resourceHref(menu.skill)) },
                {
                  key: 'toggle',
                  label: t(menu.skill.disabled ? 'resources.contextMenu.enable' : 'resources.contextMenu.disable'),
                  icon: menu.skill.disabled ? <CircleCheck size={14} /> : <Power size={14} />,
                  onSelect: () => toggleOne.mutate({ s: menu.skill, disable: !menu.skill.disabled }),
                },
                {
                  key: 'uninstall',
                  label: t(menu.skill.isInRepo && !isAgent ? 'resources.contextMenu.uninstallRepo' : 'resources.contextMenu.uninstall'),
                  icon: <Trash2 size={14} />,
                  danger: true,
                  onSelect: () => setUninstalling([menu.skill]),
                },
              ]}
            />
          )}
          {menu?.mode === 'folder' && (
            <TargetMenu
              open
              flat
              anchorPoint={menu.point}
              currentTargets={menu.summary.targets}
              isUniform={menu.summary.isUniform}
              onSelect={(target) => setFolderTargets.mutate({ folder: menu.path, target })}
              onClose={() => setMenu(null)}
            />
          )}
          {menu?.mode === 'skill' && (
            <TargetMenu
              open
              flat
              anchorPoint={menu.point}
              currentTargets={menu.skill.targets ?? null}
              onSelect={(target) => setTargets.mutate({ name: menu.skill.flatName, target })}
              onClose={() => setMenu(null)}
            />
          )}
          {menu?.mode === 'bulk' && (
            <TargetMenu open flat anchorPoint={menu.point} currentTargets={null} isUniform={false} onSelect={(target) => setManyTargets(menu.names, target)} onClose={() => setMenu(null)} />
          )}
          {menu?.mode === 'repo' && (
            <SkillContextMenu
              open
              anchorPoint={menu.point}
              onClose={() => setMenu(null)}
              items={[{
                key: 'uninstall-repo',
                label: t('resources.contextMenu.uninstallRepo'),
                icon: <Trash2 size={14} />,
                danger: true,
                onSelect: () => setUninstalling(items.filter((s) => repoOf(s) === menu.repo)),
              }]}
            />
          )}

          <ConfirmDialog
            open={!!confirmDisable}
            title={t(confirmDisable?.length === 1 ? 'resources.batchToggle.confirmTitle.one' : 'resources.batchToggle.confirmTitle.other', { count: confirmDisable?.length ?? 0 })}
            confirmText={t('resources.batchToggle.confirmButton', { count: confirmDisable?.length ?? 0 })}
            variant="danger"
            loading={toggleMany.isPending}
            onConfirm={() => {
              if (confirmDisable) toggleMany.mutate({ names: confirmDisable, enable: false });
              setConfirmDisable(null);
            }}
            onCancel={() => setConfirmDisable(null)}
            message={t('resources.batchToggle.confirmMessage')}
          />

          {uninstalling && (
            <UninstallDialog
              kind={kind}
              selection={uninstalling}
              all={items}
              onClose={(removed) => {
                if (removed) { setSelected(new Set()); setTreeSel(new Set()); }
                setUninstalling(null);
              }}
            />
          )}
        </>
      ) : tab === 'updates' ? (
        <UpdatePage kind={kind} />
      ) : (
        <TrashPage kind={kind} />
      )}
      <SyncPreviewModal open={syncOpen} onClose={() => setSyncOpen(false)} kind={kind} />
      {installTab && <InstallDialog kind={kind} initialTab={installTab === 'url' ? 'url' : 'search'} initialSource={params.get('source') ?? undefined} onClose={() => setInstall(null)} />}
    </div>
  );
}
