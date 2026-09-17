import { useMemo, useState, type CSSProperties, type MouseEvent as ReactMouseEvent } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Bot,
  ChevronDown,
  ChevronRight,
  ChevronsDownUp,
  ChevronsUpDown,
  CircleCheck,
  CircleX,
  Download,
  Ellipsis,
  ExternalLink,
  Folder,
  FolderOpen,
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
import type { BatchUninstallItemResult, Skill } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { globToRegex } from '../lib/glob';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { formatTrackedRepoName, resourceHref } from '../lib/resourceNames';
import { useSyncMatrix } from '../hooks/useSyncMatrix';
import { useRepoUpdate } from '../hooks/useRepoUpdate';
import { useT } from '../i18n';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import { Checkbox } from '../components/Checkbox';
import ConfirmDialog from '../components/ConfirmDialog';
import DialogShell from '../components/DialogShell';
import AnalyzePanel from '../components/analyze/AnalyzePanel';
import EmptyState from '../components/EmptyState';
import InstallDialog from '../components/InstallDialog';
import PageHeader from '../components/PageHeader';
import SegmentedControl from '../components/SegmentedControl';
import { Select } from '../components/Select';
import { PageSkeleton } from '../components/Skeleton';
import { resolveSource, type SourceType } from '../components/SourceBadge';
import TargetMenu, { SkillContextMenu } from '../components/TargetMenu';
import { useToast } from '../components/Toast';
import TrashPage from './TrashPage';
import UpdatePage, { countUpdates, updateUnits, useCheckStatuses } from './UpdatePage';

type Kind = Skill['kind'];
type SourceFilter = 'all' | SourceType;
type StatusFilter = 'all' | 'enabled' | 'disabled';
type SortType = 'name-asc' | 'name-desc' | 'newest' | 'oldest';
type ViewType = 'list' | 'cards' | 'tree';
type GroupBy = 'source' | 'none';
type Tone = 'ok' | 'off';
type Point = { x: number; y: number };
type MenuState =
  | { mode: 'item'; skill: Skill; point: Point }
  | { mode: 'folder'; path: string; summary: TargetSummary; point: Point }
  | { mode: 'repo'; repo: string; point: Point }
  | { mode: 'bulk'; point: Point };
type SkillsData = { resources: Skill[] };

const EMPTY: Skill[] = [];
// ponytail: "Show more" paging instead of virtualization; add a virtual list if 1000+ rows get slow.
const STEP = 100;
const SOURCE_ORDER: SourceType[] = ['tracked', 'github', 'remote', 'local'];
// Source names stay in English, like the CLI.
const SOURCE_LABEL: Record<SourceFilter, string> = { all: 'All', tracked: 'Tracked', github: 'GitHub', remote: 'Remote', local: 'Local' };
const SOURCE_ICON = { tracked: GitBranch, github: Github, remote: Globe, local: Folder };
// Status labels stay in English too.
const STATUS_LABEL: Record<StatusFilter, string> = { all: 'All', enabled: 'Enabled', disabled: 'Disabled' };
const VIEW_KEY = 'skillshare:skills-view';
const COLLAPSED_KEY = 'skillshare:folder-collapsed';

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

/** Normalize skill targets: ["*"] or empty/null → [] (meaning All). */
function normalizeTargets(targets?: string[] | null): string[] {
  if (!targets || targets.length === 0 || targets.includes('*')) return [];
  return targets;
}

// Group key for sorting: tracked repo name or first dir segment.
function sortGroup(s: Skill): string {
  const slash = s.relPath.indexOf('/');
  return slash > 0 ? s.relPath.slice(0, slash) : '';
}

function sortSkills(skills: Skill[], sortType: SortType): Skill[] {
  const byName = (a: Skill, b: Skill) => sortGroup(a).localeCompare(sortGroup(b)) || a.name.localeCompare(b.name);
  const byDate = (dir: 1 | -1) => (a: Skill, b: Skill) => {
    if (!a.installedAt && !b.installedAt) return byName(a, b);
    if (!a.installedAt) return 1;
    if (!b.installedAt) return -1;
    return dir * (new Date(a.installedAt).getTime() - new Date(b.installedAt).getTime());
  };
  const sorted = [...skills];
  switch (sortType) {
    case 'name-asc': return sorted.sort(byName);
    case 'name-desc': return sorted.sort((a, b) => sortGroup(a).localeCompare(sortGroup(b)) || b.name.localeCompare(a.name));
    case 'newest': return sorted.sort(byDate(-1));
    case 'oldest': return sorted.sort(byDate(1));
  }
}

function repoOf(s: Skill): string | undefined {
  return s.isInRepo ? s.relPath.split('/')[0] : undefined;
}

/** Parent folder shown under the name. Inside a repo group the repo prefix is already in the header. */
function parentPath(s: Skill, inGroup = false): string {
  const i = s.relPath.lastIndexOf('/');
  if (i <= 0) return '';
  const dir = s.relPath.slice(0, i);
  const repo = inGroup ? repoOf(s) : undefined;
  if (repo) return dir === repo ? '' : dir.slice(repo.length + 1);
  return formatTrackedRepoName(dir);
}

function sourceName(s: Skill): string {
  const repo = repoOf(s);
  if (repo) return formatTrackedRepoName(repo);
  if (s.source) return parseRemoteURL(s.source)?.ownerRepo ?? s.source;
  return SOURCE_LABEL.local;
}

/* -- Source groups -------------------------------- */

interface Group { key: string; source: SourceType; repo?: string; items: Skill[] }

function groupBySource(items: Skill[]): Group[] {
  const groups = new Map<string, Group>();
  for (const s of items) {
    const source = resolveSource(s.type, s.isInRepo);
    const repo = repoOf(s);
    const key = repo ?? source;
    if (!groups.has(key)) groups.set(key, { key, source, repo, items: [] });
    groups.get(key)!.items.push(s);
  }
  return [...groups.values()].sort((a, b) => SOURCE_ORDER.indexOf(a.source) - SOURCE_ORDER.indexOf(b.source) || a.key.localeCompare(b.key));
}

/** Cut groups down to the first `limit` items, keeping headers only for groups that still show something. */
function limitGroups(groups: Group[], limit: number): Group[] {
  const out: Group[] = [];
  let left = limit;
  for (const g of groups) {
    if (left <= 0) break;
    out.push({ ...g, items: g.items.slice(0, left) });
    left -= g.items.length;
  }
  return out;
}

/* -- Folder tree ---------------------------------- */

interface TargetSummary {
  display: string;      // "claude" | "claude, cursor" | "4 targets"
  targets: string[];    // sorted union
  isUniform: boolean;   // every direct skill has the same target set
}

interface FolderNode {
  name: string;
  path: string;
  children: Map<string, FolderNode>;
  skills: Skill[];
  count: number;
  summary: TargetSummary;
}

type TreeRow =
  | { type: 'folder'; node: FolderNode; depth: number; collapsed: boolean }
  | { type: 'item'; skill: Skill; depth: number };

const ALL_TARGETS: TargetSummary = { display: '', targets: [], isUniform: true };

function summarize(skills: Skill[]): TargetSummary {
  const sets = skills.map((s) => [...normalizeTargets(s.targets)].sort());
  if (sets.length === 0) return ALL_TARGETS;
  const first = sets[0];
  const isUniform = sets.every((x) => x.length === first.length && x.every((v, i) => v === first[i]));
  const union = [...new Set(sets.flat())].sort();
  const shown = isUniform ? first : union;
  return { display: shown.length > 3 ? `${shown.length} targets` : shown.join(', '), targets: shown, isUniform };
}

function buildTree(skills: Skill[]): FolderNode {
  const root: FolderNode = { name: '', path: '', children: new Map(), skills: [], count: 0, summary: ALL_TARGETS };
  for (const skill of skills) {
    const slash = skill.relPath.lastIndexOf('/');
    let node = root;
    if (slash > 0) {
      for (const seg of skill.relPath.slice(0, slash).split('/')) {
        if (!node.children.has(seg)) {
          const path = node.path ? `${node.path}/${seg}` : seg;
          node.children.set(seg, { name: seg, path, children: new Map(), skills: [], count: 0, summary: ALL_TARGETS });
        }
        node = node.children.get(seg)!;
      }
    }
    node.skills.push(skill);
  }
  // Counts include subfolders; the target summary covers direct skills only, because
  // a folder's batch target change only touches the skills directly inside it.
  const finish = (node: FolderNode): number => {
    node.summary = summarize(node.skills);
    node.count = node.skills.length;
    for (const child of node.children.values()) node.count += finish(child);
    return node.count;
  };
  finish(root);
  return root;
}

function flattenTree(root: FolderNode, collapsed: ReadonlySet<string>, expandAll: boolean): TreeRow[] {
  const rows: TreeRow[] = [];
  const walk = (node: FolderNode, depth: number) => {
    for (const child of [...node.children.values()].sort((a, b) => a.name.localeCompare(b.name))) {
      const isCollapsed = !expandAll && collapsed.has(child.path);
      rows.push({ type: 'folder', node: child, depth, collapsed: isCollapsed });
      if (!isCollapsed) walk(child, depth + 1);
    }
    for (const skill of node.skills) rows.push({ type: 'item', skill, depth });
  };
  walk(root, 0);
  return rows;
}

function folderPaths(node: FolderNode): string[] {
  return [...node.children.values()].flatMap((c) => [c.path, ...folderPaths(c)]);
}

/* -- Small pieces --------------------------------- */

function TargetStack({ names, max = 4 }: { names: string[]; max?: number }) {
  if (names.length === 0) return <span className="text-ink-3">—</span>;
  return (
    <span className="inline-flex items-center" title={names.join(', ')}>
      <span className="ss-stack">
        {names.slice(0, max).map((n) => (
          <span key={n} className="ss-at"><AgentIcon target={n} size={14} /></span>
        ))}
      </span>
      {names.length > max && <span className="ml-1.5 text-xs text-ink-3">+{names.length - max}</span>}
    </span>
  );
}

/** "1 skill" / "3 agents". The i18n layer has no plural rules, so pick the key by count. */
function countLabel(t: ReturnType<typeof useT>, kind: Kind, count: number): string {
  return t(`resources.count.${kind}${count === 1 ? '' : 's'}`, { count });
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

  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const { data: trashData } = useQuery({
    queryKey: queryKeys.trash,
    queryFn: () => api.listTrash(),
    staleTime: staleTimes.trash,
  });
  const { matrix, getSkillTargets } = useSyncMatrix();
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
      else p.delete('install');
      return p;
    }, { replace: true });

  const [search, setSearch] = useState('');
  const [source, setSource] = useState<SourceFilter>('all');
  const [status, setStatus] = useState<StatusFilter>('all');
  const [sort, setSort] = useState<SortType>('name-asc');
  const [group, setGroup] = useState<GroupBy>(isAgent ? 'none' : 'source');
  const [view, setView] = useState<ViewType>(loadView);
  const [limit, setLimit] = useState(STEP);
  const [collapsed, setCollapsed] = useState<Set<string>>(loadCollapsed);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [uninstalling, setUninstalling] = useState<Skill[] | null>(null);
  const [confirmDisable, setConfirmDisable] = useState<string[] | null>(null);

  const all = data?.resources ?? EMPTY;
  const items = useMemo(() => all.filter((s) => s.kind === kind), [all, kind]);
  const updateCount = useMemo(() => countUpdates(checks, updateUnits(all, kind)), [checks, all, kind]);
  const query = search.trim();
  const isGlob = /[*?]/.test(query);
  const filtering = query !== '' || source !== 'all' || status !== 'all';

  const filtered = useMemo(() => {
    const re = query ? globToRegex(query) : null;
    const glob = /[*?]/.test(query);
    return sortSkills(items.filter((s) =>
      (!re || re.test(s.name) || re.test(s.relPath) || re.test(s.flatName) || (!glob && re.test(s.source ?? ''))) &&
      (source === 'all' || resolveSource(s.type, s.isInRepo) === source) &&
      (status === 'all' || (status === 'disabled') === !!s.disabled),
    ), sort);
  }, [items, query, source, status, sort]);

  const groups = useMemo(() => groupBySource(filtered), [filtered]);
  const tree = useMemo(() => buildTree(filtered), [filtered]);
  const treeRows = useMemo(() => flattenTree(tree, collapsed, filtering), [tree, collapsed, filtering]);

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

  const rowInfo = (s: Skill): { synced: string[]; tone: Tone; label: string } => {
    const entries = getSkillTargets(s.flatName);
    const synced = entries.filter((e) => e.status === 'synced').map((e) => e.target).sort();
    const applicable = entries.some((e) => e.status !== 'na');
    if (s.disabled) return { synced, tone: 'off', label: STATUS_LABEL.disabled };
    if (entries.length > 0 && !applicable) return { synced, tone: 'off', label: t('resources.tree.noAgentTargets.label') };
    if (applicable && synced.length === 0) return { synced, tone: 'off', label: t('resources.tree.filteredOut.label') };
    return { synced, tone: 'ok', label: STATUS_LABEL.enabled };
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
      toast(t(disable ? 'resources.toast.disabled' : 'resources.toast.enabled', { kind: kindLabel, name: s.name }), 'success');
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
      if (failed > 0 && updated > 0) toast(t('resources.batchToggle.toast.partial', { updated, failed }), 'warning');
      else if (failed > 0) toast(t('resources.batchToggle.toast.failed', { count: failed }), 'error');
      else if (updated === 0 && unchanged > 0) toast(t('resources.batchToggle.toast.noChange'), 'info');
      else toast(t(enable ? 'resources.batchToggle.toast.enabled' : 'resources.batchToggle.toast.disabled', { count: updated }), 'success');
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
    onSuccess: (res, { folder, target }) => {
      if (res.updated === 0 && res.skipped > 0) toast(t('resources.folder.noEditableSkills', { folder }), 'error');
      else toast(t('resources.folder.skillsUpdated', { count: res.updated, folder, target: target ?? t('resources.targets.all') }), 'success');
    },
    onError: (err: Error) => toast(err.message, 'error'),
    onSettled: refreshAfterTargets,
  });

  const setSelectedTargets = async (target: string | null) => {
    const names = selectedItems.map((s) => s.flatName);
    const results = await Promise.allSettled(names.map((n) => api.setSkillTargets(n, target)));
    const failed = results.filter((r) => r.status === 'rejected').length;
    if (failed === 0) toast(t('resources.bulk.targetsSet', { count: names.length, target: target ?? t('resources.targets.all') }), 'success');
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
  const resetting = <T,>(set: (v: T) => void) => (v: T) => { set(v); setLimit(STEP); };
  const clearFilters = () => { setSearch(''); setSource('all'); setStatus('all'); setLimit(STEP); };

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

  const itemRow = (s: Skill, depth?: number) => {
    const { synced, tone, label } = rowInfo(s);
    const sub = depth === undefined ? parentPath(s, group === 'source') : '';
    return (
      <div
        key={s.flatName}
        className={`ss-r link ${depth !== undefined ? 'tr' : ''} ${selected.has(s.flatName) ? 'sel' : ''}`}
        style={depth !== undefined ? ({ '--d': depth } as CSSProperties) : undefined}
        onClick={(e) => openRow(e, s)}
        onContextMenu={(e) => openItemMenu(e, s)}
      >
        {selectBox(s)}
        <span className="flex flex-col min-w-0 flex-1 gap-px">
          <Link to={resourceHref(s)} className={`nm m truncate hover:underline ${s.disabled ? 'text-ink-3' : ''}`}>{s.name}</Link>
          {sub && <span className="font-mono text-xs text-ink-3 truncate">{sub}</span>}
        </span>
        {group === 'none' && view === 'list' && <span className="w-[150px] font-mono text-xs text-ink-3 truncate">{sourceName(s)}</span>}
        <span className="w-[140px]"><TargetStack names={synced} /></span>
        <span className="w-[120px]">{status$(tone, label)}</span>
        {actionsButton(s)}
      </div>
    );
  };

  const folderRow = (row: Extract<TreeRow, { type: 'folder' }>) => {
    const { node, depth } = row;
    const repo = depth === 0 && node.name.startsWith('_') ? node.path : null;
    const editable = !isAgent && !node.path.startsWith('_') && node.skills.length > 0;
    return (
      <div key={`f:${node.path}`} className={`ss-r fold tr ${depth > 0 ? 'sub' : ''}`} style={{ '--d': depth } as CSSProperties}>
        <button
          type="button"
          className="flex items-center gap-[9px] min-w-0 flex-1 text-left cursor-pointer"
          aria-expanded={!row.collapsed}
          onClick={() => toggleFolder(node.path)}
        >
          {row.collapsed ? <ChevronRight size={14} className="shrink-0 text-ink-3" /> : <ChevronDown size={14} className="shrink-0 text-ink-3" />}
          {repo ? <GitBranch size={15} className="shrink-0" /> : row.collapsed ? <Folder size={15} className="shrink-0" /> : <FolderOpen size={15} className="shrink-0" />}
          <b className="font-mono truncate">{repo ? formatTrackedRepoName(node.name) : node.name}</b>
          {repo && <span className="ss-tag">tracked</span>}
          <span className="text-ink-3 whitespace-nowrap">{countLabel(t, kind, node.count)}</span>
        </button>
        {repo && !isAgent && repoActions(repo)}
        {editable && (
          <>
            {!node.summary.isUniform
              ? status$('off', t('resources.tree.mixed'))
              : node.summary.targets.length > 0 && <span className="text-xs text-ink-3">{node.summary.display}</span>}
            <Button variant="ghost" size="sm" onClick={(e) => setMenu({ mode: 'folder', path: node.path, summary: node.summary, point: menuPoint(e) })}>
              <Target size={14} />
              {t('resources.setTargets')}
            </Button>
          </>
        )}
      </div>
    );
  };

  const card = (s: Skill) => {
    const { synced, tone, label } = rowInfo(s);
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
        <span className="ds text-[13px] text-ink-2">{group === 'source' ? parentPath(s, true) : parentPath(s) || sourceName(s)}</span>
        <div className="ft">
          <TargetStack names={synced} max={3} />
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
        action={<Button variant="primary" onClick={() => setInstall('search')}><Download size={15} />{t('resources.install')}</Button>}
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
    const shownGroups = group === 'source' ? limitGroups(groups, limit) : null;
    content = shownGroups ? (
      <div className="flex flex-col gap-6">
        {shownGroups.map((g) => (
          <div key={g.key}>
            {groupHead(g, true)}
            <div className="ss-tiles mt-3">{g.items.map(card)}</div>
          </div>
        ))}
      </div>
    ) : (
      <div className="ss-tiles">{filtered.slice(0, limit).map(card)}</div>
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
          <span className="flex-1">{t(view === 'tree' ? 'resources.col.folderName' : 'resources.col.name')}</span>
        )}
        {group === 'none' && view === 'list' && <span className="w-[150px]">{t('resources.col.source')}</span>}
        <span className="w-[140px]">{t('resources.col.targets')}</span>
        <span className="w-[120px]">{t('resources.col.status')}</span>
        <span className="w-[30px]" />
      </div>
    );
    let body: React.ReactNode;
    if (view === 'tree') {
      let left = limit;
      const rows: TreeRow[] = [];
      for (const r of treeRows) {
        if (r.type === 'item' && left-- <= 0) break;
        rows.push(r);
      }
      body = rows.map((r) => (r.type === 'folder' ? folderRow(r) : itemRow(r.skill, r.depth)));
    } else if (group === 'source') {
      body = limitGroups(groups, limit).map((g) => [groupHead(g, false), ...g.items.map((s) => itemRow(s))]);
    } else {
      body = filtered.slice(0, limit).map((s) => itemRow(s));
    }
    content = <div className="ss-list">{header}{body}</div>;
  }

  const count = selectedItems.length;

  return (
    <div className={`ss-wrap animate-fade-in ${tab === 'installed' && count > 0 ? 'pb-16' : ''}`}>
      <PageHeader
        className="!mb-0"
        title={t(isAgent ? 'layout.nav.agents' : 'layout.nav.skills')}
        subtitle={isAgent ? t('resources.agents.subtitle') : t('resources.skills.subtitle', { count: items.length })}
        actions={tab === 'installed' && (
          <>
            {!isAgent && (
              <Link to="/skills/new" className="ss-btn">
                <Plus size={15} />
                {t('resources.newSkill')}
              </Link>
            )}
            <Button variant="primary" data-tour="install-button" onClick={() => setInstall('search')}>
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
                <b>{t('resources.agents.supportTitle', { count: agentSupport.supported.length, total: agentSupport.total })}</b>{' '}
                {agentSupport.supported.join(', ')}. {t('resources.agents.supportRest')}
              </div>
            </div>
          )}

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
            <SegmentedControl
              className="!flex-nowrap shrink-0"
              value={source}
              onChange={resetting(setSource)}
              options={(['all', ...SOURCE_ORDER] as SourceFilter[]).map((v) => ({ value: v, label: SOURCE_LABEL[v] }))}
            />
            <span className="flex-1" />
            <Select
              className="w-[112px] shrink-0"
              prefix={t('resources.toolbar.status')}
              value={status}
              onChange={(v) => resetting(setStatus)(v as StatusFilter)}
              options={(['all', 'enabled', 'disabled'] as StatusFilter[]).map((v) => ({ value: v, label: STATUS_LABEL[v] }))}
            />
            {view === 'tree' ? (
              tree.children.size > 0 && <div className="ss-seg ic !flex-nowrap shrink-0" role="group">
                <button type="button" title={t('resources.folder.expandAll')} aria-label={t('resources.folder.expandAll')} onClick={() => updateCollapsed(new Set())}>
                  <ChevronsUpDown size={16} />
                </button>
                <button type="button" title={t('resources.folder.collapseAll')} aria-label={t('resources.folder.collapseAll')} onClick={() => updateCollapsed(new Set(folderPaths(tree)))}>
                  <ChevronsDownUp size={16} />
                </button>
              </div>
            ) : (
              <Select
                className="w-[140px] shrink-0"
                prefix={t('resources.toolbar.group')}
                value={group}
                onChange={(v) => setGroup(v as GroupBy)}
                options={[
                  { value: 'source', label: t('resources.group.source') },
                  { value: 'none', label: t('resources.group.none') },
                ]}
              />
            )}
            <Select
              className="w-[124px] shrink-0"
              prefix={t('resources.toolbar.sort')}
              value={sort}
              onChange={(v) => setSort(v as SortType)}
              options={[
                { value: 'name-asc', label: t('resources.sort.nameAsc') },
                { value: 'name-desc', label: t('resources.sort.nameDesc') },
                { value: 'newest', label: t('resources.sort.newestFirst') },
                { value: 'oldest', label: t('resources.sort.oldestFirst') },
              ]}
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
                <Button variant="secondary" size="sm" onClick={(e) => setMenu({ mode: 'bulk', point: menuPoint(e) })}>
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
          {menu?.mode === 'bulk' && (
            <TargetMenu open flat anchorPoint={menu.point} currentTargets={null} isUniform={false} onSelect={setSelectedTargets} onClose={() => setMenu(null)} />
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
            title={t('resources.batchToggle.confirmTitle', { count: confirmDisable?.length ?? 0 })}
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
                if (removed) setSelected(new Set());
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
      {installTab && <InstallDialog kind={kind} initialTab={installTab === 'url' ? 'url' : 'search'} onClose={() => setInstall(null)} />}
    </div>
  );
}

/* -- Uninstall dialog ----------------------------- */

export function UninstallDialog({ kind, selection, all, onClose }: {
  kind: Kind;
  selection: Skill[];
  all: Skill[];
  onClose: (removed: boolean) => void;
}) {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [force, setForce] = useState(false);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<BatchUninstallItemResult[] | null>(null);

  // A skill inside a tracked repo can only go with its repo. Agents are removed one by one.
  const repos = new Map<string, number>();
  const singles: Skill[] = [];
  for (const s of selection) {
    const repo = kind === 'skill' ? repoOf(s) : undefined;
    if (repo) repos.set(repo, all.filter((x) => repoOf(x) === repo).length);
    else singles.push(s);
  }
  const names = [...repos.keys(), ...singles.map((s) => s.flatName)];
  const removedCount = singles.length + [...repos.values()].reduce((a, b) => a + b, 0);

  const run = async (targets: string[], withForce: boolean) => {
    setRunning(true);
    try {
      const res = await api.batchUninstall({ names: targets, kind, force: withForce });
      clearAuditCache(queryClient);
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      queryClient.invalidateQueries({ queryKey: ['sync-matrix'] });
      if (res.summary.failed === 0) {
        toast(t('batchUninstall.toast.success', { count: res.summary.succeeded }), 'success');
        onClose(true);
        return;
      }
      setResults(res.results);
    } catch (err) {
      toast(t('batchUninstall.toast.uninstallFailed', { error: err instanceof Error ? err.message : String(err) }), 'error');
    } finally {
      setRunning(false);
    }
  };

  if (results) {
    const failed = results.filter((r) => !r.success);
    return (
      <DialogShell open onClose={() => onClose(true)} maxWidth="lg" padding="none" ariaLabel={t('batchUninstall.results.partialResult')} preventClose={running}>
        <div className="dh">
          <div className="flex flex-col gap-1">
            <h2 className="ss-h2">{t('batchUninstall.results.partialResult')}</h2>
            <p className="text-[13px] text-ink-2">{t('resources.uninstall.resultSummary', { removed: results.length - failed.length, failed: failed.length })}</p>
          </div>
        </div>
        <div className="db">
          <div className="ss-list !shadow-none">
            {results.map((r) => (
              <div key={r.name} className="ss-r !min-h-11">
                {r.success ? <CircleCheck size={16} className="shrink-0 text-ok" /> : <CircleX size={16} className="shrink-0 text-bad" />}
                <span className="nm m flex-1 truncate">{formatTrackedRepoName(r.name)}</span>
                <span className={`text-[13px] ${r.success ? 'text-ink-2' : 'text-bad'}`}>{r.success ? t('resources.uninstall.movedToTrash') : r.error}</span>
              </div>
            ))}
          </div>
          <div className="ss-note warn">
            <RefreshCw size={16} />
            <div className="flex-1">{t('resources.uninstall.syncReminder')}</div>
          </div>
        </div>
        <div className="df">
          {!force && failed.length > 0 && (
            <Button variant="ghost" loading={running} onClick={() => { setForce(true); run(failed.map((r) => r.name), true); }}>
              {t('resources.uninstall.retryForce')}
            </Button>
          )}
          <span className="flex-1" />
          <Button variant="secondary" onClick={() => onClose(true)}>{t('batchUninstall.results.continueButton')}</Button>
          <Button variant="primary" onClick={() => navigate('/sync')}>
            <RefreshCw size={15} />
            {t('batchUninstall.results.goToSync')}
          </Button>
        </div>
      </DialogShell>
    );
  }

  const title = t('resources.uninstall.title', { what: countLabel(t, kind, removedCount) });
  return (
    <DialogShell open onClose={() => onClose(false)} maxWidth="lg" padding="none" ariaLabel={title} preventClose={running}>
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
      </div>
      <div className="db">
        <div className="ss-list !shadow-none max-h-64 overflow-y-auto">
          {[...repos].map(([repo, n]) => (
            <div key={repo} className="ss-r !min-h-[42px]">
              <GitBranch size={15} className="shrink-0 text-ink-2" />
              <span className="nm m flex-1 truncate">{formatTrackedRepoName(repo)}</span>
              <span className="text-xs text-ink-3">{t('resources.uninstall.wholeRepo', { what: countLabel(t, 'skill', n) })}</span>
            </div>
          ))}
          {singles.map((s) => (
            <div key={s.flatName} className="ss-r !min-h-[42px]">
              <span className={`ss-cat sm ${kind}`}>{kind === 'agent' ? <Bot size={14} /> : <Puzzle size={14} />}</span>
              <span className="nm m flex-1 truncate">{s.name}</span>
              <span className="font-mono text-xs text-ink-3 truncate">{parentPath(s)}</span>
            </div>
          ))}
        </div>
        {repos.size > 0 && (
          <div className="ss-note warn">
            <TriangleAlert size={16} />
            <div className="flex-1">{t('resources.uninstall.repoNote')}</div>
          </div>
        )}
        {repos.size > 0 && (
          <Checkbox size="sm" label={t('batchUninstall.confirm.forceLabel')} checked={force} onChange={setForce} />
        )}
        <p className="text-[13px] text-ink-2">{t('resources.uninstall.trashNote')}</p>
      </div>
      <div className="df">
        <Button variant="ghost" onClick={() => onClose(false)} disabled={running}>{t('common.cancel')}</Button>
        <Button variant="secondary" loading={running} onClick={() => run(names, force)}>
          <Trash2 size={15} />
          {t('resources.contextMenu.uninstall')}
        </Button>
      </div>
    </DialogShell>
  );
}
