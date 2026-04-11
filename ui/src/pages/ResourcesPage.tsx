import { useState, useMemo, useCallback, useEffect, forwardRef, memo, type ReactElement } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import {
  Bot,
  Search,
  GitBranch,
  Folder,
  Puzzle,
  ArrowUpDown,
  Users,
  Globe,
  FolderOpen,
  LayoutGrid,
  List,
  Plus,
  ChevronRight,
  ChevronDown,
  ChevronsUpDown,
  ChevronsDownUp,
  Target,
  MoreHorizontal,
  Trash2,
  Eye,
  EyeOff,
  ExternalLink,
  MousePointerClick,
  X,
  Layers,
  FileText,
} from 'lucide-react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { VirtuosoGrid, Virtuoso } from 'react-virtuoso';
import type { GridComponents } from 'react-virtuoso';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import Badge from '../components/Badge';
import KindBadge from '../components/KindBadge';
import SourceBadge from '../components/SourceBadge';
import { Input, Select, type SelectOption } from '../components/Input';
import { PageSkeleton } from '../components/Skeleton';
import EmptyState from '../components/EmptyState';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import SegmentedControl from '../components/SegmentedControl';
import Pagination from '../components/Pagination';
import { api } from '../api/client';
import type { ManagedHook, ManagedRule, RuleItem, Skill, SyncMatrixEntry } from '../api/client';
import { radius } from '../design';
import ScrollToTop from '../components/ScrollToTop';
import Tooltip from '../components/Tooltip';
import { formatHookDiscoveryGroupTitle, getHookActionPayload, groupDiscoveredHooks, type HookDiscoveryGroup } from '../lib/hookDiscovery';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { formatSkillDisplayName, formatAgentDisplayName, formatTrackedRepoName } from '../lib/resourceNames';
import { useToast } from '../components/Toast';
import TargetMenu, { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import ConfirmDialog from '../components/ConfirmDialog';
import Spinner from '../components/Spinner';
import { useSyncMatrix } from '../hooks/useSyncMatrix';

/* -- Sticky-note pastel palette (8 colors) --------- */

const SKILL_PASTELS = [
  '#fff9c4', '#dceefb', '#fce4ec', '#e0f2e1',
  '#f3e5f5', '#fff3e0', '#e0f7fa', '#fbe9e7',
];
const SKILL_PASTELS_DARK = [
  'rgba(255,249,196,0.08)', 'rgba(220,238,251,0.08)',
  'rgba(252,228,236,0.08)', 'rgba(224,242,225,0.08)',
  'rgba(243,229,245,0.08)', 'rgba(255,243,224,0.08)',
  'rgba(224,247,250,0.08)', 'rgba(251,233,231,0.08)',
];

/* -- Shared skill action items hook --------------- */

type SkillsData = { resources: Skill[] };
const EMPTY_RESOURCES: Skill[] = [];

function resourceDetailHref(resource: Pick<Skill, 'flatName' | 'kind'>): string {
  const kindQuery = resource.kind === 'agent' ? '?kind=agent' : '';
  return `/resources/${encodeURIComponent(resource.flatName)}${kindQuery}`;
}

function resourceLabel(kind: Skill['kind'], capitalize = false): string {
  const label = kind === 'agent' ? 'agent' : 'skill';
  return capitalize ? label[0].toUpperCase() + label.slice(1) : label;
}

function summarizeAgentTargets(entries: SyncMatrixEntry[]): { label: string; title: string } {
  if (entries.length === 0) {
    return {
      label: 'No agent targets',
      title: 'No configured targets currently support agent sync.',
    };
  }

  const applicable = entries.filter((entry) => entry.status !== 'na');
  const synced = applicable
    .filter((entry) => entry.status === 'synced')
    .map((entry) => entry.target)
    .sort();

  if (synced.length === 0) {
    return {
      label: 'Filtered out',
      title: 'This agent is excluded by the current target agent filters.',
    };
  }

  // All applicable targets synced → "All"
  if (synced.length === applicable.length) {
    return {
      label: 'All',
      title: 'All targets',
    };
  }

  return {
    label: synced.length > 2 ? `${synced.length} targets` : synced.join(', '),
    title: synced.join(', '),
  };
}

/** Optimistic update helper: patch skills cache and return rollback snapshot. */
function optimisticPatch(
  queryClient: ReturnType<typeof useQueryClient>,
  patchFn: (skills: Skill[]) => Skill[],
) {
  queryClient.cancelQueries({ queryKey: queryKeys.skills.all });
  const previous = queryClient.getQueryData<SkillsData>(queryKeys.skills.all);
  if (previous) {
    queryClient.setQueryData<SkillsData>(queryKeys.skills.all, {
      ...previous,
      resources: patchFn(previous.resources),
    });
  }
  return previous;
}

function useResourceActions() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const navigate = useNavigate();

  const toggleMutation = useMutation({
    mutationFn: ({ name, kind, disable }: { name: string; kind: Skill['kind']; disable: boolean }) =>
      disable ? api.disableResource(name, kind) : api.enableResource(name, kind),
    onMutate: async ({ name, kind, disable }) => {
      const previous = optimisticPatch(queryClient, (skills) =>
        skills.map((s) => s.flatName === name && s.kind === kind ? { ...s, disabled: disable } : s),
      );
      return { previous };
    },
    onSuccess: (_, { name, kind, disable }) => {
      const display = formatAgentDisplayName(name);
      toast(`${resourceLabel(kind, true)} ${display} ${disable ? 'disabled' : 'enabled'}`, 'success');
    },
    onError: (err: Error, _, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(queryKeys.skills.all, ctx.previous);
      toast(err.message, 'error');
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.skills.all }),
  });

  const uninstallMutation = useMutation({
    mutationFn: ({ name, kind }: { name: string; kind: Skill['kind'] }) => api.deleteResource(name, kind),
    onMutate: async ({ name, kind }) => {
      const previous = optimisticPatch(queryClient, (skills) =>
        skills.filter((s) => !(s.flatName === name && s.kind === kind)),
      );
      return { previous };
    },
    onSuccess: (_, { name, kind }) => {
      clearAuditCache(queryClient);
      const display = formatAgentDisplayName(name);
      toast(`Uninstalled ${resourceLabel(kind)} ${display}`, 'success');
    },
    onError: (err: Error, _, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(queryKeys.skills.all, ctx.previous);
      toast(err.message, 'error');
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.skills.all }),
  });

  const uninstallRepoMutation = useMutation({
    mutationFn: (repoName: string) => api.deleteRepo(repoName),
    onMutate: async (repoName) => {
      const prefix = repoName + '/';
      const previous = optimisticPatch(queryClient, (skills) =>
        skills.filter((s) => !s.relPath.startsWith(prefix) && s.relPath !== repoName),
      );
      return { previous };
    },
    onSuccess: (_, repoName) => {
      clearAuditCache(queryClient);
      const display = formatTrackedRepoName(repoName);
      toast(`Uninstalled repo ${display}`, 'success');
    },
    onError: (err: Error, _, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(queryKeys.skills.all, ctx.previous);
      toast(err.message, 'error');
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.skills.all }),
  });

  /** Optimistic target update for a single skill. */
  const setTargetMutation = useMutation({
    mutationFn: ({ name, target }: { name: string; target: string | null }) =>
      api.setSkillTargets(name, target),
    onMutate: async ({ name, target }) => {
      const previous = optimisticPatch(queryClient, (skills) =>
        skills.map((s) => s.flatName === name
          ? { ...s, targets: target ? [target] : undefined }
          : s,
        ),
      );
      return { previous };
    },
    onSuccess: (_, { name, target }) => {
      const display = formatAgentDisplayName(name);
      toast(`${display} now available in ${target ?? 'All'}`, 'success');
    },
    onError: (err: Error, _, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(queryKeys.skills.all, ctx.previous);
      toast(err.message, 'error');
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.skills.all }),
  });

  /** Build extra context menu items for a single skill. */
  function buildResourceExtraItems(
    skill: Pick<Skill, 'flatName' | 'name' | 'relPath' | 'disabled' | 'isInRepo' | 'kind'>,
    onUninstall: () => void,
    onUninstallRepo: (repoName: string) => void,
  ): ContextMenuItem[] {
    const items: ContextMenuItem[] = [
      {
        key: 'detail',
        label: 'View Detail',
        icon: <ExternalLink size={13} strokeWidth={2.5} />,
        onSelect: () => navigate(resourceDetailHref(skill)),
      },
      {
        key: 'toggle',
        label: skill.disabled ? 'Enable' : 'Disable',
        icon: skill.disabled
          ? <Eye size={13} strokeWidth={2.5} />
          : <EyeOff size={13} strokeWidth={2.5} />,
        onSelect: () => toggleMutation.mutate({ name: skill.flatName, kind: skill.kind, disable: !skill.disabled }),
      },
    ];
    if (skill.kind === 'skill' && skill.isInRepo) {
      items.push({
        key: 'uninstall-repo',
        label: 'Uninstall Repo',
        icon: <Trash2 size={13} strokeWidth={2.5} />,
        onSelect: () => onUninstallRepo(skill.relPath.split('/')[0]),
      });
    } else {
      items.push({
        key: 'uninstall',
        label: 'Uninstall',
        icon: <Trash2 size={13} strokeWidth={2.5} />,
        onSelect: onUninstall,
      });
    }
    return items;
  }

  return { uninstallMutation, uninstallRepoMutation, setTargetMutation, buildResourceExtraItems };
}

/** Normalize skill targets: ["*"] or empty/null → [] (meaning All). */
function normalizeTargets(targets?: string[] | null): string[] {
  if (!targets || targets.length === 0 || targets.includes('*')) return [];
  return targets;
}

/** Deterministic hash → palette index. Same string always maps to same color. */
function hashToIndex(s: string, len: number): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = ((h << 5) - h + s.charCodeAt(i)) | 0;
  return ((h % len) + len) % len;
}

/** Extract owner/repo from a git remote URL, fallback to raw string. */
function shortSource(source: string): string {
  return parseRemoteURL(source)?.ownerRepo ?? source;
}

/* -- Folder tree types & helpers -------------------- */

interface TargetSummary {
  display: string;      // "All" | "claude" | "claude, cursor" | "3 targets"
  targets: string[];    // full union list (sorted)
  isUniform: boolean;   // all skills in subtree have identical target sets
}

const defaultTargetSummary: TargetSummary = { display: 'All', targets: [], isUniform: true };

interface FolderNode {
  name: string;
  path: string;
  children: Map<string, FolderNode>;
  skills: Skill[];
  skillCount: number;
  targetSummary: TargetSummary;
}

interface TreeNode {
  type: 'folder' | 'skill';
  name: string;
  path: string;
  depth: number;
  skill?: Skill;
  childCount: number;
  isRoot?: boolean;
  targetSummary?: TargetSummary;
}

/** Build a nested folder tree from skills' relPath values. Computes skillCount bottom-up. */
function buildTree(skills: Skill[]): FolderNode {
  const root: FolderNode = { name: '', path: '', children: new Map(), skills: [], skillCount: 0, targetSummary: defaultTargetSummary };
  for (const skill of skills) {
    const rp = skill.relPath ?? '';
    const lastSlash = rp.lastIndexOf('/');
    if (lastSlash <= 0) {
      root.skills.push(skill);
      continue;
    }
    const dirPath = rp.substring(0, lastSlash);
    const segments = dirPath.split('/');
    let node = root;
    let currentPath = '';
    for (const seg of segments) {
      currentPath = currentPath ? `${currentPath}/${seg}` : seg;
      if (!node.children.has(seg)) {
        node.children.set(seg, { name: seg, path: currentPath, children: new Map(), skills: [], skillCount: 0, targetSummary: defaultTargetSummary });
      }
      node = node.children.get(seg)!;
    }
    node.skills.push(skill);
  }
  // Compute skillCount bottom-up (O(n) total)
  function computeCounts(node: FolderNode): number {
    let count = node.skills.length;
    for (const child of node.children.values()) count += computeCounts(child);
    node.skillCount = count;
    return count;
  }
  computeCounts(root);

  // Compute targetSummary for each folder from its DIRECT skills only.
  // Child folders have their own visible badges — aggregating them into
  // the parent would be misleading (batch target on root only affects
  // root-level skills, not descendants in subfolders).
  function computeTargets(node: FolderNode) {
    // Recurse into children first so they get their own summaries
    for (const child of node.children.values()) {
      computeTargets(child);
    }

    const allSets: string[][] = [];
    for (const sk of node.skills) {
      const t = normalizeTargets(sk.targets);
      allSets.push(t.length > 0 ? [...t].sort() : []);
    }

    if (allSets.length === 0) {
      node.targetSummary = defaultTargetSummary;
      return;
    }

    const first = allSets[0];
    const isUniform = allSets.every(
      (s) => s.length === first.length && s.every((v, i) => v === first[i]),
    );

    if (isUniform) {
      if (first.length === 0) {
        node.targetSummary = { display: 'All', targets: [], isUniform: true };
      } else {
        const display = first.length > 3 ? `${first.length} targets` : first.join(', ');
        node.targetSummary = { display, targets: first, isUniform: true };
      }
    } else {
      const unionSet = new Set<string>();
      let hasAll = false;
      for (const s of allSets) {
        if (s.length === 0) hasAll = true;
        else s.forEach((v) => unionSet.add(v));
      }
      const union = [...unionSet].sort();
      let display: string;
      if (hasAll) {
        display = 'Mixed';
      } else if (union.length > 3) {
        display = `${union.length} targets`;
      } else {
        display = union.join(', ');
      }
      node.targetSummary = { display, targets: union, isUniform: false };
    }
  }
  computeTargets(root);

  return root;
}

/** Flatten the tree into a list of TreeNode for virtualized rendering. */
function flattenTree(
  root: FolderNode,
  collapsed: ReadonlySet<string>,
  isSearching: boolean,
): TreeNode[] {
  const result: TreeNode[] = [];

  function walkFolder(node: FolderNode, depth: number) {
    // Sort child folders alphabetically
    const sortedChildren = [...node.children.entries()].sort((a, b) => a[0].localeCompare(b[0]));
    for (const [, child] of sortedChildren) {
      const cc = child.skillCount;
      if (cc === 0) continue; // skip empty folders (filtered out)
      result.push({
        type: 'folder',
        name: child.name,
        path: child.path,
        depth,
        childCount: cc,
        targetSummary: child.targetSummary,
      });
      const isCollapsed = !isSearching && collapsed.has(child.path);
      if (!isCollapsed) {
        walkFolder(child, depth + 1);
      }
    }
    // Skills directly in this folder
    for (const skill of node.skills) {
      result.push({
        type: 'skill',
        name: skill.name,
        path: skill.relPath,
        depth,
        skill,
        childCount: 0,
      });
    }
  }

  // Walk top-level children
  const sortedChildren = [...root.children.entries()].sort((a, b) => a[0].localeCompare(b[0]));
  for (const [, child] of sortedChildren) {
    const cc = child.skillCount;
    if (cc === 0) continue;
    result.push({
      type: 'folder',
      name: child.name,
      path: child.path,
      depth: 0,
      childCount: cc,
      targetSummary: child.targetSummary,
    });
    const isCollapsed = !isSearching && collapsed.has(child.path);
    if (!isCollapsed) {
      walkFolder(child, 1);
    }
  }

  // Root-level skills last, under a virtual "(root)" folder
  if (root.skills.length > 0) {
    result.push({
      type: 'folder',
      name: '(root)',
      path: '',
      depth: 0,
      childCount: root.skills.length,
      isRoot: true,
      targetSummary: root.targetSummary,
    });
    const rootCollapsed = !isSearching && collapsed.has('');
    if (!rootCollapsed) {
      for (const skill of root.skills) {
        result.push({
          type: 'skill',
          name: skill.name,
          path: skill.relPath,
          depth: 1,
          skill,
          childCount: 0,
        });
      }
    }
  }

  return result;
}

/** Collect all folder paths from a tree (for Expand/Collapse All). */
function collectAllFolderPaths(root: FolderNode): string[] {
  const paths: string[] = [];
  function walk(node: FolderNode) {
    for (const child of node.children.values()) {
      paths.push(child.path);
      walk(child);
    }
  }
  walk(root);
  if (root.skills.length > 0) paths.push(''); // root virtual folder
  return paths;
}

const COLLAPSED_STORAGE_KEY = 'skillshare:folder-collapsed';

function loadCollapsed(): Set<string> {
  try {
    const raw = localStorage.getItem(COLLAPSED_STORAGE_KEY);
    if (raw) return new Set(JSON.parse(raw));
  } catch { /* ignore corrupt data */ }
  return new Set();
}

function saveCollapsed(collapsed: Set<string>) {
  localStorage.setItem(COLLAPSED_STORAGE_KEY, JSON.stringify([...collapsed]));
}

/* -- Filter, Sort & View types -------------------- */

type ResourceTab = 'skills' | 'agents' | 'rules' | 'hooks';
type ManagedResourceMode = 'managed' | 'discovered';
type SkillFilterType = 'all' | 'tracked' | 'github' | 'local';
type RuleFilterType = 'all' | 'project' | 'user' | 'collectible' | `tool:${string}`;
type HookFilterType = 'all' | 'project' | 'user' | 'collectible' | `tool:${string}`;
type FilterType = SkillFilterType | RuleFilterType | HookFilterType;
type SkillSortType = 'name-asc' | 'name-desc' | 'newest' | 'oldest';
type RuleSortType = 'name-asc' | 'name-desc' | 'tool-asc' | 'path-asc';
type HookSortType = 'name-asc' | 'name-desc' | 'tool-asc' | 'event-asc';
type SortType = SkillSortType | RuleSortType | HookSortType;
type ViewType = 'grid' | 'grouped' | 'table';
type FilterOption<T extends string = string> = {
  key: T;
  label: string;
  count: number;
  icon: React.ReactNode;
};

const skillFilterOptions: { key: SkillFilterType; label: string; icon: React.ReactNode }[] = [
  { key: 'all', label: 'All', icon: <LayoutGrid size={14} strokeWidth={2.5} /> },
  { key: 'tracked', label: 'Tracked', icon: <Users size={14} strokeWidth={2.5} /> },
  { key: 'github', label: 'GitHub', icon: <Globe size={14} strokeWidth={2.5} /> },
  { key: 'local', label: 'Local', icon: <FolderOpen size={14} strokeWidth={2.5} /> },
];

function isManagedResourceMode(value: string | null): value is ManagedResourceMode {
  return value === 'managed' || value === 'discovered';
}

function isResourceTab(value: string | null): value is ResourceTab {
  return value === 'skills' || value === 'agents' || value === 'rules' || value === 'hooks';
}

function compareText(left: string, right: string): number {
  return left.localeCompare(right, undefined, { sensitivity: 'base' });
}

function matchFilter(skill: Skill, filterType: SkillFilterType): boolean {
  switch (filterType) {
    case 'all':
      return true;
    case 'tracked':
      return skill.isInRepo;
    case 'github':
      return (skill.type === 'github' || skill.type === 'github-subdir') && !skill.isInRepo;
    case 'local':
      return !skill.type && !skill.isInRepo;
  }
}

function getRuleRef(rule: RuleItem): string {
  return rule.id ?? rule.path;
}

function sortManagedRules(rules: ManagedRule[], sortType: RuleSortType): ManagedRule[] {
  const sorted = [...rules];
  switch (sortType) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(left.name, right.name));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(right.name, left.name));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.tool, right.tool) || compareText(left.name, right.name));
    case 'path-asc':
      return sorted.sort((left, right) => compareText(left.relativePath, right.relativePath));
  }
}

function sortDiscoveredRules(rules: RuleItem[], sortType: RuleSortType): RuleItem[] {
  const sorted = [...rules];
  switch (sortType) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(left.name, right.name));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(right.name, left.name));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.sourceTool, right.sourceTool) || compareText(left.name, right.name));
    case 'path-asc':
      return sorted.sort((left, right) => compareText(left.path, right.path));
  }
}

function managedRuleMatchesSearch(rule: ManagedRule, search: string): boolean {
  if (!search) return true;
  const haystack = [rule.name, rule.tool, rule.relativePath, rule.content].join('\n').toLowerCase();
  return haystack.includes(search);
}

function discoveredRuleMatchesSearch(rule: RuleItem, search: string): boolean {
  if (!search) return true;
  const haystack = [
    rule.name,
    rule.sourceTool,
    rule.scope,
    rule.path,
    rule.content,
    rule.collectReason ?? '',
  ].join('\n').toLowerCase();
  return haystack.includes(search);
}

function managedRuleMatchesFilter(rule: ManagedRule, filterType: RuleFilterType): boolean {
  if (filterType === 'all') return true;
  if (filterType.startsWith('tool:')) return rule.tool === filterType.slice(5);
  return true;
}

function discoveredRuleMatchesFilter(rule: RuleItem, filterType: RuleFilterType): boolean {
  switch (filterType) {
    case 'all':
      return true;
    case 'collectible':
      return Boolean(rule.collectible && rule.id);
    case 'project':
      return rule.scope === 'project';
    case 'user':
      return rule.scope === 'user';
    default:
      return filterType.startsWith('tool:') ? rule.sourceTool === filterType.slice(5) : true;
  }
}

function buildManagedRuleFilterOptions(rules: ManagedRule[]): FilterOption<RuleFilterType>[] {
  const byTool = new Map<string, number>();
  for (const rule of rules) {
    byTool.set(rule.tool, (byTool.get(rule.tool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: rules.length, icon: <LayoutGrid size={14} strokeWidth={2.5} /> },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as RuleFilterType,
        label: tool,
        count,
        icon: <FileText size={14} strokeWidth={2.5} />,
      })),
  ];
}

function buildDiscoveredRuleFilterOptions(rules: RuleItem[]): FilterOption<RuleFilterType>[] {
  const byTool = new Map<string, number>();
  for (const rule of rules) {
    byTool.set(rule.sourceTool, (byTool.get(rule.sourceTool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: rules.length, icon: <LayoutGrid size={14} strokeWidth={2.5} /> },
    {
      key: 'collectible',
      label: 'Collectible',
      count: rules.filter((rule) => Boolean(rule.collectible && rule.id)).length,
      icon: <Target size={14} strokeWidth={2.5} />,
    },
    {
      key: 'project',
      label: 'Project',
      count: rules.filter((rule) => rule.scope === 'project').length,
      icon: <FolderOpen size={14} strokeWidth={2.5} />,
    },
    {
      key: 'user',
      label: 'User',
      count: rules.filter((rule) => rule.scope === 'user').length,
      icon: <Users size={14} strokeWidth={2.5} />,
    },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as RuleFilterType,
        label: tool,
        count,
        icon: <FileText size={14} strokeWidth={2.5} />,
      })),
  ];
}

function sortManagedHooks(hooks: ManagedHook[], sortType: HookSortType): ManagedHook[] {
  const sorted = [...hooks];
  switch (sortType) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(left.id, right.id));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(right.id, left.id));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.tool, right.tool) || compareText(left.id, right.id));
    case 'event-asc':
      return sorted.sort((left, right) => compareText(left.event, right.event) || compareText(left.id, right.id));
  }
}

function sortDiscoveredHookGroups(groups: HookDiscoveryGroup[], sortType: HookSortType): HookDiscoveryGroup[] {
  const sorted = [...groups];
  switch (sortType) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(formatHookDiscoveryGroupTitle(left), formatHookDiscoveryGroupTitle(right)));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(formatHookDiscoveryGroupTitle(right), formatHookDiscoveryGroupTitle(left)));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.sourceTool, right.sourceTool) || compareText(formatHookDiscoveryGroupTitle(left), formatHookDiscoveryGroupTitle(right)));
    case 'event-asc':
      return sorted.sort((left, right) => compareText(left.event, right.event) || compareText(formatHookDiscoveryGroupTitle(left), formatHookDiscoveryGroupTitle(right)));
  }
}

function managedHookMatchesSearch(hook: ManagedHook, search: string): boolean {
  if (!search) return true;
  const haystack = [
    hook.id,
    hook.tool,
    hook.event,
    hook.matcher ?? '',
    ...hook.handlers.flatMap((handler) => [
      handler.type,
      handler.command ?? '',
      handler.url ?? '',
      handler.prompt ?? '',
      handler.timeout ?? '',
      handler.statusMessage ?? '',
    ]),
  ].join('\n').toLowerCase();
  return haystack.includes(search);
}

function discoveredHookMatchesSearch(group: HookDiscoveryGroup, search: string): boolean {
  if (!search) return true;
  const haystack = [
    group.id,
    formatHookDiscoveryGroupTitle(group),
    group.sourceTool,
    group.scope,
    group.event,
    group.matcher,
    group.collectReason ?? '',
    ...group.hooks.flatMap((hook) => [
      hook.actionType,
      getHookActionPayload(hook),
      hook.path,
      hook.statusMessage ?? '',
    ]),
  ].join('\n').toLowerCase();
  return haystack.includes(search);
}

function managedHookMatchesFilter(hook: ManagedHook, filterType: HookFilterType): boolean {
  if (filterType === 'all') return true;
  if (filterType.startsWith('tool:')) return hook.tool === filterType.slice(5);
  return true;
}

function discoveredHookMatchesFilter(group: HookDiscoveryGroup, filterType: HookFilterType): boolean {
  switch (filterType) {
    case 'all':
      return true;
    case 'collectible':
      return group.collectible;
    case 'project':
      return group.scope === 'project';
    case 'user':
      return group.scope === 'user';
    default:
      return filterType.startsWith('tool:') ? group.sourceTool === filterType.slice(5) : true;
  }
}

function buildManagedHookFilterOptions(hooks: ManagedHook[]): FilterOption<HookFilterType>[] {
  const byTool = new Map<string, number>();
  for (const hook of hooks) {
    byTool.set(hook.tool, (byTool.get(hook.tool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: hooks.length, icon: <LayoutGrid size={14} strokeWidth={2.5} /> },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as HookFilterType,
        label: tool,
        count,
        icon: <Target size={14} strokeWidth={2.5} />,
      })),
  ];
}

function buildDiscoveredHookFilterOptions(groups: HookDiscoveryGroup[]): FilterOption<HookFilterType>[] {
  const byTool = new Map<string, number>();
  for (const group of groups) {
    byTool.set(group.sourceTool, (byTool.get(group.sourceTool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: groups.length, icon: <LayoutGrid size={14} strokeWidth={2.5} /> },
    {
      key: 'collectible',
      label: 'Collectible',
      count: groups.filter((group) => group.collectible).length,
      icon: <Target size={14} strokeWidth={2.5} />,
    },
    {
      key: 'project',
      label: 'Project',
      count: groups.filter((group) => group.scope === 'project').length,
      icon: <FolderOpen size={14} strokeWidth={2.5} />,
    },
    {
      key: 'user',
      label: 'User',
      count: groups.filter((group) => group.scope === 'user').length,
      icon: <Users size={14} strokeWidth={2.5} />,
    },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as HookFilterType,
        label: tool,
        count,
        icon: <Target size={14} strokeWidth={2.5} />,
      })),
  ];
}

const skillSortOptions: SelectOption[] = [
  { value: 'name-asc', label: 'Name A → Z' },
  { value: 'name-desc', label: 'Name Z → A' },
  { value: 'newest', label: 'Newest first' },
  { value: 'oldest', label: 'Oldest first' },
];

const ruleSortOptions: SelectOption[] = [
  { value: 'name-asc', label: 'Name A → Z' },
  { value: 'name-desc', label: 'Name Z → A' },
  { value: 'tool-asc', label: 'Tool A → Z' },
  { value: 'path-asc', label: 'Path A → Z' },
];

const hookSortOptions: SelectOption[] = [
  { value: 'name-asc', label: 'Name A → Z' },
  { value: 'name-desc', label: 'Name Z → A' },
  { value: 'tool-asc', label: 'Tool A → Z' },
  { value: 'event-asc', label: 'Event A → Z' },
];


// Extract group key from relPath for sorting: tracked repo name or first dir segment.
function sortGroup(s: Skill): string {
  const slash = s.relPath.indexOf('/');
  return slash > 0 ? s.relPath.slice(0, slash) : '';
}

function sortSkills(skills: Skill[], sortType: SkillSortType): Skill[] {
  const sorted = [...skills];
  switch (sortType) {
    case 'name-asc':
      return sorted.sort((a, b) => sortGroup(a).localeCompare(sortGroup(b)) || a.name.localeCompare(b.name));
    case 'name-desc':
      return sorted.sort((a, b) => sortGroup(a).localeCompare(sortGroup(b)) || b.name.localeCompare(a.name));
    case 'newest':
      return sorted.sort((a, b) => {
        if (!a.installedAt && !b.installedAt) return sortGroup(a).localeCompare(sortGroup(b)) || a.name.localeCompare(b.name);
        if (!a.installedAt) return 1;
        if (!b.installedAt) return -1;
        return new Date(b.installedAt).getTime() - new Date(a.installedAt).getTime();
      });
    case 'oldest':
      return sorted.sort((a, b) => {
        if (!a.installedAt && !b.installedAt) return sortGroup(a).localeCompare(sortGroup(b)) || a.name.localeCompare(b.name);
        if (!a.installedAt) return 1;
        if (!b.installedAt) return -1;
        return new Date(a.installedAt).getTime() - new Date(b.installedAt).getTime();
      });
  }
}

/* -- VirtuosoGrid components (OUTSIDE component function) -- */

const GridList = forwardRef<HTMLDivElement, React.ComponentPropsWithRef<'div'>>(
  ({ style, children, ...props }, ref) => (
    <div
      ref={ref}
      {...props}
      style={{ display: 'flex', flexWrap: 'wrap', gap: '1.25rem', ...style }}
    >
      {children}
    </div>
  ),
);
GridList.displayName = 'GridList';

const GridItem = ({ children, ...props }: React.ComponentPropsWithRef<'div'>) => (
  <div
    {...props}
    className="!w-full md:!w-[calc(50%-0.625rem)] xl:!w-[calc(33.333%-0.834rem)]"
    style={{ display: 'flex', flex: 'none', boxSizing: 'border-box' }}
  >
    {children}
  </div>
);

const GridPlaceholder = () => (
  <div
    className="!w-full md:!w-[calc(50%-0.625rem)] xl:!w-[calc(33.333%-0.834rem)]"
    style={{ display: 'flex', flex: 'none', boxSizing: 'border-box' }}
  >
    <div className="w-full h-32 bg-muted animate-pulse" style={{ borderRadius: radius.md }} />
  </div>
);

const gridComponents: GridComponents = {
  List: GridList as GridComponents['List'],
  Item: GridItem as GridComponents['Item'],
  ScrollSeekPlaceholder: GridPlaceholder as GridComponents['ScrollSeekPlaceholder'],
};

/* -- Skill card ----------------------------------- */

const SkillPostit = memo(function SkillPostit({
  skill,
  onContextMenu,
  highlighted = false,
}: {
  skill: Skill;
  onContextMenu?: (e: React.MouseEvent) => void;
  highlighted?: boolean;
}) {
  // Extract repo name from relPath (e.g., "_awesome-skillshare-skills/frontend-dugong" -> "awesome-skillshare-skills")
  const repoName = skill.isInRepo && skill.relPath.startsWith('_')
    ? formatTrackedRepoName(skill.relPath.split('/')[0])
    : undefined;

  // Color key: tracked skills from the same repo share a color
  const colorKey = repoName ?? skill.name;
  const colorIdx = hashToIndex(colorKey, SKILL_PASTELS.length);

  return (
    <Link
      to={resourceDetailHref(skill)}
      className={`w-full h-full${skill.disabled ? ' opacity-50' : ''}`}
      onContextMenu={onContextMenu}
    >
      <div
        className={`ss-card ss-skill-card relative p-5 pb-4 bg-surface cursor-pointer border shadow-sm rounded-[var(--radius-md)] transition-all duration-150 hover:shadow-hover hover:border-muted-dark h-full flex flex-col ${highlighted ? 'border-muted-dark shadow-hover' : 'border-muted'}`}
        style={{
          '--skill-pastel': SKILL_PASTELS[colorIdx],
          '--skill-pastel-dark': SKILL_PASTELS_DARK[colorIdx],
        } as React.CSSProperties}
      >
        {/* Skill name row */}
        <div className="flex items-center gap-2 mb-2">
          <div className="shrink-0">
            {skill.isInRepo
              ? <GitBranch size={18} strokeWidth={2.5} className="text-pencil-light" />
              : skill.kind === 'agent'
                ? <FileText size={18} strokeWidth={2.5} className="text-pencil-light" />
                : <Folder size={18} strokeWidth={2.5} className="text-pencil-light" />
            }
          </div>
          <h3 className="font-bold text-pencil text-lg truncate leading-tight flex items-center gap-1.5">
            {skill.kind && <KindBadge kind={skill.kind} />}
            {skill.name}
          </h3>
        </div>

        {/* Org banner (tracked only) */}
        {skill.isInRepo && repoName && (
          <div className="flex items-center gap-1 mb-2">
            <Users size={12} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            <span className="text-xs text-pencil-light truncate">{repoName}</span>
          </div>
        )}

        {/* Path */}
        <p
          className="font-mono text-sm text-pencil-light truncate mb-2"
        >
          {formatSkillDisplayName(skill.relPath)}
        </p>

        {/* Bottom row */}
        <div className="flex items-center justify-between gap-2 mt-auto">
          {skill.source ? (
            <span className="text-sm text-pencil-light truncate flex-1">{shortSource(skill.source)}</span>
          ) : (
            <span />
          )}
          <div className="flex items-center gap-1.5 shrink-0">
            {skill.disabled && <Badge variant="danger">Disabled</Badge>}
            <SourceBadge type={skill.type} isInRepo={skill.isInRepo} />
            {skill.branch && (
              <Badge variant="default">
                <GitBranch size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                {skill.branch}
              </Badge>
            )}
            {skill.targets && skill.targets.length > 0 && (
              <Badge variant="default">
                <Target size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                {skill.targets.length > 2 ? `${skill.targets.length} targets` : skill.targets.join(', ')}
              </Badge>
            )}
          </div>
        </div>
      </div>
    </Link>
  );
});

/* -- Right-click tip banner ----------------------- */

const TIP_DISMISSED_KEY = 'skillshare:skills-context-menu-tip';

function ContextMenuTip() {
  const [dismissed, setDismissed] = useState(() =>
    localStorage.getItem(TIP_DISMISSED_KEY) === '1',
  );

  if (dismissed) return null;

  return (
    <div className="ss-tip-banner mb-3 flex items-center gap-3 px-4 py-2.5 bg-muted/40 border-2 border-dashed border-muted-dark/40 text-sm text-pencil-light"
      style={{ borderRadius: 'var(--radius-md)' }}
    >
      <MousePointerClick size={18} strokeWidth={2} className="text-pencil-light/60 shrink-0" />
      <p className="flex-1">
        <span className="font-medium text-pencil">Right-click</span> any resource or folder for quick actions — enable/disable, uninstall, and more.
      </p>
      <button
        className="shrink-0 px-2.5 py-1 text-xs font-medium text-pencil-light hover:text-pencil hover:bg-muted/60 transition-colors cursor-pointer"
        style={{ borderRadius: 'var(--radius-sm)' }}
        onClick={() => {
          localStorage.setItem(TIP_DISMISSED_KEY, '1');
          setDismissed(true);
        }}
        aria-label="Dismiss tip"
      >
        <X size={14} strokeWidth={2.5} />
      </button>
    </div>
  );
}

/* -- Main page ------------------------------------ */

export default function SkillsPage() {
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });

  const [toolbarH, setToolbarH] = useState(0);
  const toolbarRef = useCallback((node: HTMLDivElement | null) => {
    if (!node) return;
    const ro = new ResizeObserver(() => setToolbarH(node.offsetHeight));
    ro.observe(node);
    return () => ro.disconnect();
  }, []);
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeTab, setActiveTab] = useState<ResourceTab>(() => {
    const urlTab = searchParams.get('tab');
    if (isResourceTab(urlTab)) return urlTab;
    const saved = localStorage.getItem('skillshare:resources-tab');
    return isResourceTab(saved) ? saved : 'skills';
  });

  useEffect(() => {
    const urlTab = searchParams.get('tab');
    if (isResourceTab(urlTab) && activeTab !== urlTab) {
      setActiveTab(urlTab);
    } else if (!urlTab && activeTab !== 'skills') {
      setActiveTab('skills');
    }
  }, [searchParams]); // eslint-disable-line react-hooks/exhaustive-deps

  const activeMode: ManagedResourceMode = isManagedResourceMode(searchParams.get('mode'))
    ? searchParams.get('mode')
    : 'managed';

  const changeTab = (tab: ResourceTab) => {
    setActiveTab(tab);
    localStorage.setItem('skillshare:resources-tab', tab);
    setFilterType('all');
    setSearch('');
    setSortType('name-asc');

    const params = new URLSearchParams(searchParams);
    if (tab === 'skills') params.delete('tab');
    else params.set('tab', tab);
    if (tab !== 'rules' && tab !== 'hooks') params.delete('mode');
    setSearchParams(params);
  };

  const changeManagedMode = (mode: ManagedResourceMode) => {
    const params = new URLSearchParams(searchParams);
    params.set('tab', activeTab);
    if (mode === 'managed') params.delete('mode');
    else params.set('mode', mode);
    setSearchParams(params);
    setFilterType('all');
    setSearch('');
    setSortType('name-asc');
  };

  const [search, setSearch] = useState('');
  const [filterType, setFilterType] = useState<FilterType>('all');
  const [sortType, setSortType] = useState<SortType>('name-asc');
  const [viewType, setViewType] = useState<ViewType>(() => {
    const saved = localStorage.getItem('skillshare:skills-view');
    return (saved === 'grid' || saved === 'grouped' || saved === 'table') ? saved : 'grid';
  });

  const changeViewType = (v: ViewType) => {
    setViewType(v);
    localStorage.setItem('skillshare:skills-view', v);
  };

  const [gridContextMenu, setGridContextMenu] = useState<{
    point: { x: number; y: number };
    skillFlatName: string;
    skillName: string;
    kind: Skill['kind'];
    relPath: string;
    disabled: boolean;
    isInRepo: boolean;
    currentTargets: string[] | null;
  } | null>(null);

  const {
    uninstallMutation: gridUninstallMutation,
    uninstallRepoMutation: gridUninstallRepoMutation,
    setTargetMutation: gridSingleMutation,
    buildResourceExtraItems,
  } = useResourceActions();

  const [gridConfirmUninstall, setGridConfirmUninstall] = useState<{
    flatName: string;
    name: string;
    kind: Skill['kind'];
  } | null>(null);
  const [gridConfirmUninstallRepo, setGridConfirmUninstallRepo] = useState<string | null>(null);

  const skills = data?.resources ?? EMPTY_RESOURCES;
  const normalizedSearch = search.trim().toLowerCase();

  const managedRulesQuery = useQuery({
    queryKey: queryKeys.rules.managed,
    queryFn: () => api.managedRules.list(),
    staleTime: staleTimes.rules,
    enabled: activeTab === 'rules' && activeMode === 'managed',
  });

  const discoveredRulesQuery = useQuery({
    queryKey: queryKeys.rules.discovered,
    queryFn: () => api.listRules(),
    staleTime: staleTimes.rules,
    enabled: activeTab === 'rules' && activeMode === 'discovered',
  });

  const managedHooksQuery = useQuery({
    queryKey: queryKeys.hooks.managed,
    queryFn: () => api.managedHooks.list(),
    staleTime: staleTimes.hooks,
    enabled: activeTab === 'hooks' && activeMode === 'managed',
  });

  const discoveredHooksQuery = useQuery({
    queryKey: queryKeys.hooks.discovered,
    queryFn: () => api.listHooks(),
    staleTime: staleTimes.hooks,
    enabled: activeTab === 'hooks' && activeMode === 'discovered',
  });

  const managedRules = useMemo(() => managedRulesQuery.data?.rules ?? [], [managedRulesQuery.data?.rules]);
  const discoveredRules = useMemo(() => discoveredRulesQuery.data?.rules ?? [], [discoveredRulesQuery.data?.rules]);
  const ruleWarnings = useMemo(() => discoveredRulesQuery.data?.warnings ?? [], [discoveredRulesQuery.data?.warnings]);

  const managedHooks = useMemo(() => managedHooksQuery.data?.hooks ?? [], [managedHooksQuery.data?.hooks]);
  const discoveredHooks = useMemo(() => discoveredHooksQuery.data?.hooks ?? [], [discoveredHooksQuery.data?.hooks]);
  const hookWarnings = useMemo(() => discoveredHooksQuery.data?.warnings ?? [], [discoveredHooksQuery.data?.warnings]);
  const discoveredHookGroups = useMemo(() => groupDiscoveredHooks(discoveredHooks), [discoveredHooks]);

  // Compute counts for each filter type — scoped to the active tab
  const skillFilterCounts = useMemo(() => {
    const tabSkills = activeTab === 'agents'
      ? skills.filter((s) => s.kind === 'agent')
      : skills.filter((s) => s.kind !== 'agent');
    const counts: Record<SkillFilterType, number> = {
      all: tabSkills.length,
      tracked: 0,
      github: 0,
      local: 0,
    };
    for (const s of tabSkills) {
      if (s.isInRepo) counts.tracked++;
      if ((s.type === 'github' || s.type === 'github-subdir') && !s.isInRepo) counts.github++;
      if (!s.type && !s.isInRepo) counts.local++;
    }
    return counts;
  }, [skills, activeTab]);

  const activeSkillFilter: SkillFilterType = skillFilterOptions.some((option) => option.key === filterType)
    ? filterType as SkillFilterType
    : 'all';

  const filtered = useMemo(() => {
    const result = skills.filter(
      (s) =>
        (s.name.toLowerCase().includes(normalizedSearch) ||
          s.flatName.toLowerCase().includes(normalizedSearch) ||
          (s.source ?? '').toLowerCase().includes(normalizedSearch)) &&
        matchFilter(s, activeSkillFilter),
    );
    return sortSkills(result, sortType as SkillSortType);
  }, [activeSkillFilter, normalizedSearch, skills, sortType]);

  const skillItems = useMemo(() => filtered.filter((s) => s.kind !== 'agent'), [filtered]);
  const agentItems = useMemo(() => filtered.filter((s) => s.kind === 'agent'), [filtered]);
  const tabFiltered = activeTab === 'agents' ? agentItems : skillItems;

  const managedRuleFilterOptions = useMemo(() => buildManagedRuleFilterOptions(managedRules), [managedRules]);
  const discoveredRuleFilterOptions = useMemo(() => buildDiscoveredRuleFilterOptions(discoveredRules), [discoveredRules]);
  const activeRuleFilterOptions = activeMode === 'managed' ? managedRuleFilterOptions : discoveredRuleFilterOptions;
  const activeRuleFilter: RuleFilterType = activeRuleFilterOptions.some((option) => option.key === filterType)
    ? filterType as RuleFilterType
    : 'all';
  const filteredManagedRules = useMemo(
    () =>
      sortManagedRules(
        managedRules.filter((rule) => managedRuleMatchesFilter(rule, activeRuleFilter) && managedRuleMatchesSearch(rule, normalizedSearch)),
        sortType as RuleSortType,
      ),
    [activeRuleFilter, managedRules, normalizedSearch, sortType],
  );
  const filteredDiscoveredRules = useMemo(
    () =>
      sortDiscoveredRules(
        discoveredRules.filter((rule) => discoveredRuleMatchesFilter(rule, activeRuleFilter) && discoveredRuleMatchesSearch(rule, normalizedSearch)),
        sortType as RuleSortType,
      ),
    [activeRuleFilter, discoveredRules, normalizedSearch, sortType],
  );

  const managedHookFilterOptions = useMemo(() => buildManagedHookFilterOptions(managedHooks), [managedHooks]);
  const discoveredHookFilterOptions = useMemo(() => buildDiscoveredHookFilterOptions(discoveredHookGroups), [discoveredHookGroups]);
  const activeHookFilterOptions = activeMode === 'managed' ? managedHookFilterOptions : discoveredHookFilterOptions;
  const activeHookFilter: HookFilterType = activeHookFilterOptions.some((option) => option.key === filterType)
    ? filterType as HookFilterType
    : 'all';
  const filteredManagedHooks = useMemo(
    () =>
      sortManagedHooks(
        managedHooks.filter((hook) => managedHookMatchesFilter(hook, activeHookFilter) && managedHookMatchesSearch(hook, normalizedSearch)),
        sortType as HookSortType,
      ),
    [activeHookFilter, managedHooks, normalizedSearch, sortType],
  );
  const filteredDiscoveredHookGroups = useMemo(
    () =>
      sortDiscoveredHookGroups(
        discoveredHookGroups.filter((group) => discoveredHookMatchesFilter(group, activeHookFilter) && discoveredHookMatchesSearch(group, normalizedSearch)),
        sortType as HookSortType,
      ),
    [activeHookFilter, discoveredHookGroups, normalizedSearch, sortType],
  );

  const rulesTabCount = activeMode === 'managed' ? managedRules.length : discoveredRules.length;
  const hooksTabCount = activeMode === 'managed' ? managedHooks.length : discoveredHookGroups.length;
  const isSkillsTab = activeTab === 'skills' || activeTab === 'agents';
  const isRulesTab = activeTab === 'rules';
  const isHooksTab = activeTab === 'hooks';
  const activeRules = activeMode === 'managed' ? filteredManagedRules : filteredDiscoveredRules;
  const activeRulesTotal = activeMode === 'managed' ? managedRules.length : discoveredRules.length;
  const activeHooks = activeMode === 'managed' ? filteredManagedHooks : filteredDiscoveredHookGroups;
  const activeHooksTotal = activeMode === 'managed' ? managedHooks.length : discoveredHookGroups.length;
  const activeFilterValue: FilterType = isRulesTab
    ? activeRuleFilter
    : isHooksTab
      ? activeHookFilter
      : activeSkillFilter;
  const activeSortOptions = isRulesTab
    ? ruleSortOptions
    : isHooksTab
      ? hookSortOptions
      : skillSortOptions;
  const activeSearchPlaceholder = isRulesTab
    ? 'Filter rules...'
    : isHooksTab
      ? 'Filter hooks...'
      : 'Filter skills...';
  const activeFilterOptions = isRulesTab
    ? activeRuleFilterOptions
    : isHooksTab
      ? activeHookFilterOptions
      : skillFilterOptions.map((option) => ({ ...option, count: skillFilterCounts[option.key] }));
  const hasActiveFilters = normalizedSearch.length > 0 || activeFilterValue !== 'all';
  const resultSummary = isRulesTab
    ? { shown: activeRules.length, total: activeRulesTotal, label: 'rules' }
    : isHooksTab
      ? { shown: activeHooks.length, total: activeHooksTotal, label: 'hooks' }
      : { shown: tabFiltered.length, total: skills.length, label: 'resources' };
  const headerAction = activeTab === 'agents'
    ? { to: '/resources/new?kind=agent', label: 'New Agent' }
    : activeTab === 'rules'
      ? { to: '/rules/new', label: 'New Rule' }
      : activeTab === 'hooks'
        ? { to: '/hooks/new', label: 'New Hook' }
        : { to: '/resources/new', label: 'New Skill' };
  const resourceTabs = [
    { key: 'skills' as ResourceTab, icon: <Puzzle size={16} strokeWidth={2.5} />, label: 'Skills', count: skillItems.length },
    { key: 'agents' as ResourceTab, icon: <Bot size={16} strokeWidth={2.5} />, label: 'Agents', count: agentItems.length },
    { key: 'rules' as ResourceTab, icon: <FileText size={16} strokeWidth={2.5} />, label: 'Rules', count: rulesTabCount },
    { key: 'hooks' as ResourceTab, icon: <Target size={16} strokeWidth={2.5} />, label: 'Hooks', count: hooksTabCount },
  ];
  const activeManagedQueryPending = isRulesTab
    ? activeMode === 'managed'
      ? managedRulesQuery.isPending && !managedRulesQuery.data
      : discoveredRulesQuery.isPending && !discoveredRulesQuery.data
    : isHooksTab
      ? activeMode === 'managed'
        ? managedHooksQuery.isPending && !managedHooksQuery.data
        : discoveredHooksQuery.isPending && !discoveredHooksQuery.data
      : false;
  const activeManagedQueryError = isRulesTab
    ? activeMode === 'managed'
      ? managedRulesQuery.error
      : discoveredRulesQuery.error
    : isHooksTab
      ? activeMode === 'managed'
        ? managedHooksQuery.error
        : discoveredHooksQuery.error
      : null;

  if (isPending) return <PageSkeleton />;
  if (activeManagedQueryPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          Failed to load skills
        </p>
        <p className="text-pencil-light text-base mt-1">{error.message}</p>
      </Card>
    );
  }
  if (activeManagedQueryError) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          Failed to load {isRulesTab ? 'rules' : 'hooks'}
        </p>
        <p className="text-pencil-light text-base mt-1">{activeManagedQueryError.message}</p>
      </Card>
    );
  }

  return (
    <div data-tour="skills-view" className="animate-fade-in">
      {/* Header */}
      <PageHeader
        icon={<Layers size={24} strokeWidth={2.5} />}
        title="Resources"
        subtitle=""
        className="mb-4!"
        actions={(
          <Link to={headerAction.to}>
            <Button variant="primary" size="sm">
              <Plus size={16} strokeWidth={2.5} />
              {headerAction.label}
            </Button>
          </Link>
        )}
      />

      {/* Resource type underline tabs */}
      <nav className="ss-resource-tabs flex items-center gap-6 border-b-2 border-muted mb-3 -mx-4 px-4 md:-mx-8 md:px-8" role="tablist">
        {resourceTabs.map((tab) => (
          <button
            key={tab.key}
            role="tab"
            aria-selected={activeTab === tab.key}
            onClick={() => changeTab(tab.key)}
            className={`
              ss-resource-tab
              inline-flex items-center gap-1.5 px-1 pb-2.5 text-sm font-semibold cursor-pointer
              transition-all duration-150 border-b-[3px] -mb-[2px]
              ${activeTab === tab.key
                ? 'border-pencil text-pencil'
                : 'border-transparent text-pencil-light hover:text-pencil hover:border-muted-dark'
              }
            `}
          >
            {tab.icon}
            {tab.label}
            <span className={`
              text-[11px] font-medium px-1.5 py-0.5 rounded-[var(--radius-sm)]
              ${activeTab === tab.key ? 'bg-pencil/10 text-pencil' : 'bg-muted text-pencil-light'}
            `}>
              {tab.count}
            </span>
          </button>
        ))}
      </nav>

      {/* Sticky toolbar */}
      <div ref={toolbarRef} className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 pt-2 pb-4">
        {/* Search + Sort + View row */}
        <div className="flex flex-col sm:flex-row gap-3 mb-2">
          <div className="relative flex-1">
            <Search
              size={18}
              strokeWidth={2.5}
              className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
            />
            <Input
              type="text"
              placeholder={activeSearchPlaceholder}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="!pl-11"
            />
          </div>
          <div className="flex items-center gap-2 sm:w-52">
            <ArrowUpDown size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            <Select
              value={sortType}
              onChange={(v) => setSortType(v as SortType)}
              size="sm"
              options={activeSortOptions}
            />
          </div>
          {isSkillsTab && (
            <SegmentedControl
              value={viewType}
              onChange={changeViewType}
              options={[
                { value: 'grid', label: <LayoutGrid size={16} strokeWidth={2.5} /> },
                { value: 'grouped', label: <FolderOpen size={16} strokeWidth={2.5} /> },
                { value: 'table', label: <List size={16} strokeWidth={2.5} /> },
              ]}
              size="md"
              connected
            />
          )}
        </div>

        {(isRulesTab || isHooksTab) && (
          <div className="mb-2">
            <SegmentedControl
              value={activeMode}
              onChange={(value) => changeManagedMode(value as ManagedResourceMode)}
              options={[
                {
                  value: 'managed',
                  label: 'Managed',
                  count: isRulesTab ? managedRules.length : managedHooks.length,
                },
                {
                  value: 'discovered',
                  label: 'Discovered',
                  count: isRulesTab ? discoveredRules.length : discoveredHookGroups.length,
                },
              ]}
            />
          </div>
        )}

        {/* Filter tabs */}
        <SegmentedControl
          value={activeFilterValue}
          onChange={setFilterType}
          options={activeFilterOptions.map((opt) => ({
            value: opt.key,
            label: <span className="inline-flex items-center gap-1.5">{opt.icon}{opt.label}</span>,
            count: opt.count,
          }))}
        />
      </div>

      {/* Result count — hidden in folder view (merged into folder toolbar) */}
      {hasActiveFilters && (!isSkillsTab || viewType !== 'grouped') && (
        <p className="text-pencil-light text-sm mb-3">
          Showing {resultSummary.shown} of {resultSummary.total} {resultSummary.label}
          {activeFilterValue !== 'all' && (
            <>
              {' '}
              &middot;{' '}
              <Button
                variant="link"
                onClick={() => {
                  setFilterType('all');
                  setSearch('');
                }}
              >
                Clear filters
              </Button>
            </>
          )}
        </p>
      )}

      {/* Right-click tip — shown once, dismissed permanently */}
      {isSkillsTab && <ContextMenuTip />}

      {isSkillsTab ? (
        tabFiltered.length > 0 ? (
          viewType === 'grid' ? (
            <VirtuosoGrid
              useWindowScroll
              totalCount={tabFiltered.length}
              overscan={200}
              components={gridComponents}
              scrollSeekConfiguration={{
                enter: (velocity) => Math.abs(velocity) > 800,
                exit: (velocity) => Math.abs(velocity) < 200,
              }}
              itemContent={(index) => {
                const skill = tabFiltered[index];
                return (
                  <SkillPostit
                    skill={skill}
                    highlighted={gridContextMenu?.skillFlatName === skill.flatName}
                    onContextMenu={(e) => {
                      e.preventDefault();
                      setGridContextMenu({
                        point: { x: e.clientX, y: e.clientY },
                        skillFlatName: skill.flatName,
                        skillName: skill.name,
                        kind: skill.kind,
                        relPath: skill.relPath,
                        disabled: !!skill.disabled,
                        isInRepo: !!skill.isInRepo,
                        currentTargets: skill.targets ?? null,
                      });
                    }}
                  />
                );
              }}
            />
          ) : viewType === 'grouped' ? (
            <FolderTreeView
              skills={tabFiltered}
              resourceKind={activeTab === 'agents' ? 'agent' : 'skill'}
              totalCount={skills.length}
              isSearching={hasActiveFilters}
              stickyTop={toolbarH}
              onClearFilters={hasActiveFilters ? () => { setFilterType('all'); setSearch(''); } : undefined}
            />
          ) : (
            <SkillsTable skills={tabFiltered} resourceKind={activeTab === 'agents' ? 'agent' : 'skill'} />
          )
        ) : (
          <EmptyState
            icon={Puzzle}
            title={hasActiveFilters ? 'No matches' : 'No skills yet'}
            description={
              hasActiveFilters
                ? 'Try a different search term or filter.'
                : 'Install skills from GitHub or add them to your source directory.'
            }
          />
        )
      ) : isRulesTab ? (
        <div className="space-y-4">
          {activeMode === 'discovered' && ruleWarnings.length > 0 && (
            <Card variant="accent">
              <div className="space-y-1 text-sm text-pencil-light">
                {ruleWarnings.map((warning) => (
                  <p key={warning}>{warning}</p>
                ))}
              </div>
            </Card>
          )}

          {activeRules.length > 0 ? (
            <div className="grid gap-4">
              {activeMode === 'managed'
                ? filteredManagedRules.map((rule) => (
                  <Card key={rule.id} hover>
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0 space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                            {rule.name}
                          </h3>
                          <Badge variant="accent">{rule.tool}</Badge>
                        </div>
                        <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                          {rule.relativePath}
                        </p>
                        <p className="line-clamp-3 whitespace-pre-wrap text-sm text-pencil-light">
                          {rule.content}
                        </p>
                      </div>
                      <Link to={`/rules/manage/${encodeURIComponent(rule.id)}`}>
                        <Button variant="secondary" size="sm">
                          <Eye size={16} strokeWidth={2.5} />
                          Edit Rule
                        </Button>
                      </Link>
                    </div>
                  </Card>
                ))
                : filteredDiscoveredRules.map((rule) => (
                  <Card key={getRuleRef(rule)}>
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0 space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                            {rule.name}
                          </h3>
                          <Badge variant={rule.scope === 'project' ? 'info' : 'default'}>{rule.scope}</Badge>
                          <Badge variant="accent">{rule.sourceTool}</Badge>
                          {rule.collectible && <Badge variant="success">Collectible</Badge>}
                        </div>
                        <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                          {rule.path}
                        </p>
                        {rule.collectReason && <p className="text-sm text-pencil-light">{rule.collectReason}</p>}
                        <p className="line-clamp-3 whitespace-pre-wrap text-sm text-pencil-light">
                          {rule.content}
                        </p>
                      </div>
                      <Link
                        to={`/rules/discovered/${encodeURIComponent(getRuleRef(rule))}`}
                        state={{ rule }}
                      >
                        <Button variant="secondary" size="sm" aria-label={`View rule ${rule.name}`}>
                          <Eye size={16} strokeWidth={2.5} />
                          View Rule
                        </Button>
                      </Link>
                    </div>
                  </Card>
                ))}
            </div>
          ) : (
            <EmptyState
              icon={FileText}
              title={hasActiveFilters ? 'No rules match these filters' : activeMode === 'managed' ? 'No rules found' : 'No discovered rules found'}
              description={hasActiveFilters ? 'Try a different search term or filter.' : activeMode === 'managed' ? 'Create a rule to start publishing compiled files.' : 'No rule files were discovered in the current user or project scope.'}
            />
          )}
        </div>
      ) : (
        <div className="space-y-4">
          {activeMode === 'discovered' && hookWarnings.length > 0 && (
            <Card variant="accent">
              <div className="space-y-1 text-sm text-pencil-light">
                {hookWarnings.map((warning) => (
                  <p key={warning}>{warning}</p>
                ))}
              </div>
            </Card>
          )}

          {activeHooks.length > 0 ? (
            <div className="grid gap-4">
              {activeMode === 'managed'
                ? filteredManagedHooks.map((hook) => (
                  <Card key={hook.id} hover>
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0 space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                            {hook.id}
                          </h3>
                          <Badge variant="accent">{hook.tool}</Badge>
                          <Badge variant="warning">{hook.event}</Badge>
                        </div>
                        <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                          {hook.matcher || 'All'}
                        </p>
                        <p className="text-sm text-pencil-light">
                          {hook.handlers.length} handler{hook.handlers.length !== 1 ? 's' : ''}
                        </p>
                      </div>
                      <Link to={`/hooks/manage/${encodeURIComponent(hook.id)}`}>
                        <Button variant="secondary" size="sm">
                          <Eye size={16} strokeWidth={2.5} />
                          Edit Hook
                        </Button>
                      </Link>
                    </div>
                  </Card>
                ))
                : filteredDiscoveredHookGroups.map((group) => (
                  <Card key={group.id}>
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0 space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                            {formatHookDiscoveryGroupTitle(group)}
                          </h3>
                          <Badge variant={group.scope === 'project' ? 'info' : 'default'}>{group.scope}</Badge>
                          <Badge variant="accent">{group.sourceTool}</Badge>
                          <Badge variant="warning">{group.event}</Badge>
                          {group.collectible && <Badge variant="success">Collectible</Badge>}
                        </div>
                        <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                          {group.matcher || 'All'}
                        </p>
                        <p className="text-sm text-pencil-light">
                          {group.hooks[0]?.path ?? ''}
                        </p>
                        {group.collectReason && <p className="text-sm text-pencil-light">{group.collectReason}</p>}
                      </div>
                      <Link
                        to={`/hooks/discovered/${encodeURIComponent(group.id)}`}
                        state={{ group }}
                      >
                        <Button variant="secondary" size="sm">
                          <Eye size={16} strokeWidth={2.5} />
                          View Hook
                        </Button>
                      </Link>
                    </div>
                  </Card>
                ))}
            </div>
          ) : (
            <EmptyState
              icon={Target}
              title={hasActiveFilters ? 'No hooks match these filters' : activeMode === 'managed' ? 'No hooks found' : 'No discovered hooks found'}
              description={hasActiveFilters ? 'Try a different search term or filter.' : activeMode === 'managed' ? 'Create a hook to start publishing compiled files.' : 'No hook diagnostics were discovered in the current user or project scope.'}
            />
          )}
        </div>
      )}

      <ScrollToTop />

      {gridContextMenu && (
        <TargetMenu
          open={true}
          anchorPoint={gridContextMenu.point}
          currentTargets={gridContextMenu.currentTargets}
          isUniform={true}
          extraItems={buildResourceExtraItems(
            {
              flatName: gridContextMenu.skillFlatName,
              name: gridContextMenu.skillName,
              relPath: gridContextMenu.relPath,
              disabled: gridContextMenu.disabled,
              isInRepo: gridContextMenu.isInRepo,
              kind: gridContextMenu.kind,
            },
            () => setGridConfirmUninstall({ flatName: gridContextMenu.skillFlatName, name: gridContextMenu.skillName, kind: gridContextMenu.kind }),
            (repoName) => { setGridConfirmUninstallRepo(repoName); setGridContextMenu(null); },
          )}
          onSelect={(target) => {
            gridSingleMutation.mutate({ name: gridContextMenu.skillFlatName, target });
            setGridContextMenu(null);
          }}
          onClose={() => setGridContextMenu(null)}
        />
      )}
      <ConfirmDialog
        open={!!gridConfirmUninstall}
        title={`Uninstall ${resourceLabel(gridConfirmUninstall?.kind ?? 'skill')}`}
        message={<>Are you sure you want to uninstall <strong>{gridConfirmUninstall?.name}</strong>?</>}
        confirmText="Uninstall"
        variant="danger"
        loading={gridUninstallMutation.isPending}
        onConfirm={() => {
          if (gridConfirmUninstall) gridUninstallMutation.mutate({ name: gridConfirmUninstall.flatName, kind: gridConfirmUninstall.kind });
          setGridConfirmUninstall(null);
        }}
        onCancel={() => setGridConfirmUninstall(null)}
      />
      <ConfirmDialog
        open={!!gridConfirmUninstallRepo}
        title="Uninstall tracked repo"
        message={<>Are you sure you want to uninstall all skills in <strong>{gridConfirmUninstallRepo}</strong>?</>}
        confirmText="Uninstall Repo"
        variant="danger"
        loading={gridUninstallRepoMutation.isPending}
        onConfirm={() => {
          if (gridConfirmUninstallRepo) gridUninstallRepoMutation.mutate(gridConfirmUninstallRepo);
          setGridConfirmUninstallRepo(null);
        }}
        onCancel={() => setGridConfirmUninstallRepo(null)}
      />
    </div>
  );
}

/* -- Folder tree view (virtualized flat list) -------- */

const INDENT_PX = 24;


function FolderTreeView({ skills, resourceKind, totalCount, isSearching, stickyTop = 0, onClearFilters }: {
  skills: Skill[];
  resourceKind: Skill['kind'];
  totalCount: number;
  isSearching: boolean;
  stickyTop?: number;
  onClearFilters?: () => void;
}) {
  const [collapsed, setCollapsed] = useState<Set<string>>(loadCollapsed);
  const [stickyFolder, setStickyFolder] = useState<{ node: TreeNode; index: number } | null>(null);
  const [contextMenu, setContextMenu] = useState<{
    point: { x: number; y: number };
    mode: 'folder' | 'skill';
    folderPath?: string;
    skillFlatName?: string;
    skillName?: string;
    kind?: Skill['kind'];
    relPath?: string;
    disabled?: boolean;
    isInRepo?: boolean;
    currentTargets: string[] | null;
    isUniform: boolean;
  } | null>(null);

  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { getSkillTargets: treeGetSkillTargets } = useSyncMatrix();
  const {
    uninstallMutation,
    uninstallRepoMutation,
    setTargetMutation: singleMutation,
    buildResourceExtraItems: buildExtraItems,
  } = useResourceActions();
  const [confirmUninstallRepo, setConfirmUninstallRepo] = useState<string | null>(null);

  const batchMutation = useMutation({
    mutationFn: ({ folder, target }: { folder: string; target: string | null }) =>
      api.batchSetTargets(folder, target),
    onMutate: async ({ folder, target }) => {
      setContextMenu(null);
      const newTargets = target ? [target] : undefined;
      const previous = optimisticPatch(queryClient, (skills) =>
        skills.map((s) => {
          // Match the same logic as the server's matchesFolder
          if (s.disabled || s.isInRepo) return s;
          const dir = s.relPath.substring(0, s.relPath.lastIndexOf('/')) || '.';
          const matches = folder === '*'
            ? true
            : folder === ''
              ? dir === '.'
              : dir === folder || dir.startsWith(folder + '/');
          return matches ? { ...s, targets: newTargets } : s;
        }),
      );
      return { previous };
    },
    onSuccess: (data, { folder, target }) => {
      const label = target ?? 'All';
      const folderLabel = folder || '(root)';
      if (data.updated === 0 && data.skipped > 0) {
        toast(`No editable skills in ${folderLabel}/ — tracked-repo skills are read-only`, 'error');
      } else {
        toast(`${data.updated} skill${data.updated !== 1 ? 's' : ''} in ${folderLabel}/ now available in ${label}`, 'success');
      }
    },
    onError: (err: Error, _, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(queryKeys.skills.all, ctx.previous);
      toast(err.message, 'error');
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.skills.all }),
  });

  const pendingFolder = batchMutation.isPending ? (batchMutation.variables?.folder ?? null) : null;

  const [confirmUninstall, setConfirmUninstall] = useState<{
    flatName: string;
    name: string;
    kind: Skill['kind'];
  } | null>(null);

  const tree = useMemo(() => buildTree(skills), [skills]);

  const rows = useMemo(
    () => flattenTree(tree, collapsed, isSearching),
    [tree, collapsed, isSearching],
  );

  const folderCount = useMemo(() => {
    let count = 0;
    for (const r of rows) if (r.type === 'folder') count++;
    return count;
  }, [rows]);

  // Track scroll to find which folder should be sticky.
  // Uses DOM positions to find the row index at the toolbar edge,
  // then walks backwards in the rows DATA array (not DOM) to find
  // the nearest folder — works even if Virtuoso unmounted that folder row.
  useEffect(() => {
    let ticking = false;
    const onScroll = () => {
      if (ticking) return;
      ticking = true;
      requestAnimationFrame(() => {
        ticking = false;
        const allEls = document.querySelectorAll<HTMLElement>('[data-tree-idx]');
        if (allEls.length === 0) { setStickyFolder(null); return; }

        // Find the index of the first row at or below the toolbar bottom
        let edgeIdx = -1;
        for (const el of allEls) {
          if (el.getBoundingClientRect().top >= stickyTop) {
            edgeIdx = parseInt(el.dataset.treeIdx!, 10);
            break;
          }
        }
        // All rendered rows are above toolbar — use the last one's index + 1
        if (edgeIdx < 0) {
          const lastEl = allEls[allEls.length - 1];
          edgeIdx = parseInt(lastEl.dataset.treeIdx!, 10) + 1;
        }
        if (edgeIdx <= 0) { setStickyFolder(null); return; }

        // Walk backwards in rows DATA to find nearest folder above the edge
        for (let i = edgeIdx - 1; i >= 0; i--) {
          if (rows[i]?.type === 'folder') {
            setStickyFolder({ node: rows[i], index: i });
            return;
          }
        }
        setStickyFolder(null);
      });
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener('scroll', onScroll);
  }, [rows, stickyTop]);

  const toggleFolder = useCallback((path: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      saveCollapsed(next);
      return next;
    });
  }, []);

  const expandAll = useCallback(() => {
    setCollapsed(new Set());
    saveCollapsed(new Set());
  }, []);

  const collapseAll = useCallback(() => {
    const all = new Set(collectAllFolderPaths(tree));
    setCollapsed(all);
    saveCollapsed(all);
  }, [tree]);

  const renderItem = useCallback((index: number): ReactElement => {
    const node = rows[index];
    const indentGuides = node.depth > 0 ? (
      Array.from({ length: node.depth }, (_, i) => (
        <span
          key={i}
          className="absolute top-0 bottom-0 border-l border-muted/40"
          style={{ left: i * INDENT_PX + 14 }}
        />
      ))
    ) : null;

    if (node.type === 'folder') {
      const isFolderCollapsed = !isSearching && collapsed.has(node.path);
      return (
        <div
          data-tree-idx={index}
          className={`relative flex items-center gap-1.5 py-1.5 px-1 cursor-pointer select-none hover:bg-muted/50 transition-colors${node.isRoot ? ' border-t border-muted/60 mt-2 pt-3' : ''}${contextMenu?.mode === 'folder' && contextMenu.folderPath === node.path ? ' bg-muted/50' : ''}`}
          style={{ paddingLeft: node.depth * INDENT_PX + 4 }}
          onClick={() => toggleFolder(node.path)}
          onContextMenu={(e) => {
            e.preventDefault();
            if (resourceKind === 'agent' || batchMutation.isPending) return;
            setContextMenu({
              point: { x: e.clientX, y: e.clientY },
              mode: 'folder',
              folderPath: node.path,
              currentTargets: node.targetSummary?.targets ?? null,
              isUniform: node.targetSummary?.isUniform ?? true,
            });
          }}
          role="treeitem"
          aria-expanded={!isFolderCollapsed}
        >
          {indentGuides}
          {isFolderCollapsed
            ? <ChevronRight size={14} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            : <ChevronDown size={14} strokeWidth={2.5} className="text-pencil-light shrink-0" />
          }
          {node.name.startsWith('_')
            ? <GitBranch size={16} strokeWidth={2.5} className="text-pencil shrink-0" />
            : isFolderCollapsed
              ? <Folder size={16} strokeWidth={2.5} className="text-pencil shrink-0" />
              : <FolderOpen size={16} strokeWidth={2.5} className="text-pencil shrink-0" />
          }
          <span className={`font-bold text-pencil shrink-0${node.isRoot ? ' text-pencil-light font-semibold' : ''}`}>
            {node.name.startsWith('_') ? formatTrackedRepoName(node.name) : formatSkillDisplayName(node.name)}
          </span>
          <span
            className="text-[11px] text-pencil-light px-1.5 py-0 bg-muted shrink-0 ml-1.5"
            style={{ borderRadius: radius.sm }}
          >
            {node.childCount}
          </span>
          {resourceKind === 'skill' && node.targetSummary && (
            <span className="ml-auto shrink-0 flex items-center gap-1.5">
              {pendingFolder === node.path && <Spinner size="sm" />}
              <Tooltip content={
                node.targetSummary.display === 'Mixed'
                  ? `Some: All targets · Others: ${node.targetSummary.targets.join(', ')}`
                  : node.targetSummary.targets.length > 0
                    ? node.targetSummary.targets.join(', ')
                    : 'All targets'
              }>
                <Badge variant={node.targetSummary.isUniform ? 'default' : 'warning'}>
                  <Target size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                  {node.targetSummary.display}
                </Badge>
              </Tooltip>
            </span>
          )}
        </div>
      );
    }

    const skill = node.skill!;
    const nt = normalizeTargets(skill.targets);
    const skillTargetLabel = nt.length > 0 ? nt.join(', ') : 'All';
    const agentTargetInfo = skill.kind === 'agent' ? summarizeAgentTargets(treeGetSkillTargets(skill.flatName)) : null;
    const tooltipContent = (
      <div>
        <div>{skill.relPath}</div>
        {(skill.source || skill.installedAt) && (
          <>
            <hr className="border-paper/30 my-1" />
            {skill.source && <div>Source: {shortSource(skill.source)}</div>}
            {skill.installedAt && <div>Installed: {new Date(skill.installedAt).toLocaleDateString()}</div>}
          </>
        )}
      </div>
    );

    return (
      <div
        data-tree-idx={index}
        onContextMenu={(e) => {
          e.preventDefault();
          if (batchMutation.isPending) return;
          setContextMenu({
            point: { x: e.clientX, y: e.clientY },
            mode: 'skill',
            skillFlatName: skill.flatName,
            skillName: skill.name,
            kind: skill.kind,
            relPath: skill.relPath,
            disabled: !!skill.disabled,
            isInRepo: !!skill.isInRepo,
            currentTargets: skill.targets ?? null,
            isUniform: true,
          });
        }}
      >
        <Tooltip content={tooltipContent} followCursor delay={1000}>
          <Link
            to={resourceDetailHref(skill)}
            className={`relative flex items-center gap-1.5 py-1 px-1 hover:bg-muted/50 transition-colors no-underline${skill.disabled ? ' opacity-40' : ''}${contextMenu?.mode === 'skill' && contextMenu.skillFlatName === skill.flatName ? ' bg-muted/50' : ''}`}
            style={{ paddingLeft: node.depth * INDENT_PX + 4 }}
          >
            {indentGuides}
            <span style={{ width: 14 }} className="shrink-0" />
            {skill.kind === 'agent'
              ? <FileText size={14} strokeWidth={2} className="text-pencil-light/60 shrink-0" />
              : <Puzzle size={14} strokeWidth={2} className="text-pencil-light/60 shrink-0" />
            }
            <span className="text-sm text-pencil truncate">{skill.name}</span>
            <span className="ml-auto shrink-0 flex items-center gap-1">
              {skill.disabled && <Badge variant="danger">Disabled</Badge>}
              <SourceBadge type={skill.type} isInRepo={skill.isInRepo} />
              {skill.branch && (
                <Badge variant="default">
                  <GitBranch size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                  {skill.branch}
                </Badge>
              )}
              {resourceKind === 'skill' && (
                <Tooltip content={skillTargetLabel === 'All' ? 'All targets' : skillTargetLabel}>
                  <Badge variant="default">
                    <Target size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                    {skillTargetLabel}
                  </Badge>
                </Tooltip>
              )}
              {resourceKind === 'agent' && agentTargetInfo && (
                <Tooltip content={agentTargetInfo.title}>
                  <Badge variant="default">
                    <Target size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                    {agentTargetInfo.label}
                  </Badge>
                </Tooltip>
              )}
            </span>
          </Link>
        </Tooltip>
      </div>
    );
  }, [rows, collapsed, isSearching, toggleFolder, contextMenu, pendingFolder, resourceKind, batchMutation.isPending]);

  return (
    <div>
      {/* Toolbar: stats + Expand/Collapse All */}
      <div className="flex items-center gap-2 mb-3 flex-wrap">
        <span className="text-sm text-pencil-light">
          {isSearching ? (
            <>
              Showing {skills.length} of {totalCount} {resourceKind === 'agent' ? 'agents' : 'skills'}
              {onClearFilters && (
                <>
                  {' '}&middot;{' '}
                  <Button variant="link" onClick={onClearFilters}>Clear filters</Button>
                </>
              )}
            </>
          ) : (
            <>{skills.length} item{skills.length !== 1 ? 's' : ''} in {folderCount} folder{folderCount !== 1 ? 's' : ''}</>
          )}
        </span>
        {folderCount > 1 && (
          <span className="ml-auto flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={expandAll}>
              <ChevronsUpDown size={14} strokeWidth={2.5} /> Expand All
            </Button>
            <Button variant="ghost" size="sm" onClick={collapseAll}>
              <ChevronsDownUp size={14} strokeWidth={2.5} /> Collapse All
            </Button>
          </span>
        )}
      </div>

      {/* Sticky folder header — appears when parent folder scrolls out of view */}
      {stickyFolder && (
        <div className="sticky z-10 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 border-b border-dashed border-muted" style={{ top: stickyTop }}>
          <div
            className="flex items-center gap-1.5 py-1.5 px-1 cursor-pointer select-none"
            style={{ paddingLeft: 4 }}
            onClick={() => {
              const allEls = document.querySelectorAll<HTMLElement>('[data-tree-idx]');
              if (allEls.length < 2) return;
              const firstEl = allEls[0];
              const lastEl = allEls[allEls.length - 1];
              const firstIdx = parseInt(firstEl.dataset.treeIdx!, 10);
              const lastIdx = parseInt(lastEl.dataset.treeIdx!, 10);
              const avgH = (lastEl.getBoundingClientRect().top - firstEl.getBoundingClientRect().top) / (lastIdx - firstIdx);
              // Estimated viewport position of the folder - desired position (toolbar bottom)
              const offset = firstEl.getBoundingClientRect().top + (stickyFolder.index - firstIdx) * avgH - stickyTop;
              window.scrollBy({ top: offset, behavior: 'smooth' });
            }}
          >
            {stickyFolder.node.path?.startsWith('_')
              ? <GitBranch size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
              : <FolderOpen size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            }
            <span className={`font-semibold text-sm${stickyFolder.node.isRoot ? ' text-pencil-light' : ' text-pencil'}`}>
              {stickyFolder.node.path
                ? (stickyFolder.node.path.startsWith('_')
                    ? formatTrackedRepoName(stickyFolder.node.path)
                    : formatSkillDisplayName(stickyFolder.node.path))
                : '(root)'}
            </span>
            <span
              className="text-xs text-pencil-light px-1.5 py-0 bg-muted shrink-0 ml-1"
              style={{ borderRadius: radius.sm }}
            >
              {stickyFolder.node.childCount}
            </span>
            {resourceKind === 'skill' && stickyFolder.node.targetSummary && (
              <span className="ml-auto shrink-0 flex items-center gap-1">
                <Tooltip content={
                  stickyFolder.node.targetSummary.targets.length > 0
                    ? stickyFolder.node.targetSummary.targets.join(', ')
                    : 'All targets'
                }>
                  <Badge variant={stickyFolder.node.targetSummary.isUniform ? 'default' : 'warning'}>
                    <Target size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                    {stickyFolder.node.targetSummary.display}
                  </Badge>
                </Tooltip>
              </span>
            )}
          </div>
        </div>
      )}

      {/* Virtualized tree */}
      <Virtuoso
        useWindowScroll
        totalCount={rows.length}
        overscan={600}
        itemContent={renderItem}
      />

      {contextMenu && (
        <TargetMenu
          open={true}
          anchorPoint={contextMenu.point}
          currentTargets={contextMenu.currentTargets}
          isUniform={contextMenu.isUniform}
          label={contextMenu.mode === 'folder' ? 'Folder available in...' : 'Available in...'}
          extraItems={contextMenu.mode === 'skill' ? buildExtraItems(
              {
                flatName: contextMenu.skillFlatName!,
                name: contextMenu.skillName ?? contextMenu.skillFlatName!,
                relPath: contextMenu.relPath ?? '',
                disabled: !!contextMenu.disabled,
                isInRepo: !!contextMenu.isInRepo,
                kind: contextMenu.kind ?? resourceKind,
              },
              () => setConfirmUninstall({
                flatName: contextMenu.skillFlatName!,
                name: contextMenu.skillName ?? contextMenu.skillFlatName!,
                kind: contextMenu.kind ?? resourceKind,
              }),
              (repoName) => { setConfirmUninstallRepo(repoName); setContextMenu(null); },
          ) : undefined}
          onSelect={(target) => {
            if (batchMutation.isPending) return;
            if (contextMenu.mode === 'folder') {
              batchMutation.mutate({ folder: contextMenu.folderPath ?? '', target });
            } else {
              singleMutation.mutate({ name: contextMenu.skillFlatName!, target });
            }
            setContextMenu(null);
          }}
          onClose={() => setContextMenu(null)}
        />
      )}
      <ConfirmDialog
        open={!!confirmUninstall}
        title={`Uninstall ${resourceLabel(confirmUninstall?.kind ?? resourceKind)}`}
        message={<>Are you sure you want to uninstall <strong>{confirmUninstall?.name}</strong>?</>}
        confirmText="Uninstall"
        variant="danger"
        loading={uninstallMutation.isPending}
        onConfirm={() => {
          if (confirmUninstall) uninstallMutation.mutate({ name: confirmUninstall.flatName, kind: confirmUninstall.kind });
          setConfirmUninstall(null);
        }}
        onCancel={() => setConfirmUninstall(null)}
      />
      <ConfirmDialog
        open={!!confirmUninstallRepo}
        title="Uninstall tracked repo"
        message={<>Are you sure you want to uninstall all skills in <strong>{confirmUninstallRepo}</strong>?</>}
        confirmText="Uninstall Repo"
        variant="danger"
        loading={uninstallRepoMutation.isPending}
        onConfirm={() => {
          if (confirmUninstallRepo) uninstallRepoMutation.mutate(confirmUninstallRepo);
          setConfirmUninstallRepo(null);
        }}
        onCancel={() => setConfirmUninstallRepo(null)}
      />
    </div>
  );
}

/* -- Table view with pagination ------------------- */

const TABLE_PAGE_SIZES = [10, 25, 50] as const;

function SkillsTable({ skills, resourceKind }: { skills: Skill[]; resourceKind: Skill['kind'] }) {
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(() => {
    const saved = localStorage.getItem('skillshare:table-page-size');
    const n = saved ? parseInt(saved, 10) : 0;
    return TABLE_PAGE_SIZES.some((size) => size === n) ? n : 10;
  });
  const [prevSkills, setPrevSkills] = useState(skills);
  if (skills !== prevSkills) {
    setPrevSkills(skills);
    setPage(0);
  }

  const [actionMenu, setActionMenu] = useState<{
    point: { x: number; y: number };
    skillFlatName: string;
    skillName: string;
    kind: Skill['kind'];
    relPath: string;
    disabled: boolean;
    isInRepo: boolean;
  } | null>(null);
  const [confirmUninstall, setConfirmUninstall] = useState<{
    flatName: string;
    name: string;
    kind: Skill['kind'];
  } | null>(null);

  const {
    uninstallMutation,
    uninstallRepoMutation: tableUninstallRepoMutation,
    setTargetMutation: targetMutation,
    buildResourceExtraItems: buildTableExtraItems,
  } = useResourceActions();
  const [tableConfirmUninstallRepo, setTableConfirmUninstallRepo] = useState<string | null>(null);
  const { getSkillTargets } = useSyncMatrix();

  // Available targets for the inline Select
  const { data: availableData } = useQuery({
    queryKey: queryKeys.targets.available,
    queryFn: () => api.availableTargets(),
    staleTime: staleTimes.targets,
  });
  const targetOptions: SelectOption[] = useMemo(() => {
    const installed = (availableData?.targets ?? []).filter((t) => t.installed);
    return [
      { value: '__all__', label: 'All' },
      ...installed.map((t) => ({ value: t.name, label: t.name })),
    ];
  }, [availableData]);

  // targetMutation from useSkillActions (optimistic)

  const totalPages = Math.max(1, Math.ceil(skills.length / pageSize));
  const start = page * pageSize;
  const visible = skills.slice(start, start + pageSize);

  // Build action menu items
  const actionItems: ContextMenuItem[] = actionMenu
    ? buildTableExtraItems(
        {
          flatName: actionMenu.skillFlatName,
          name: actionMenu.skillName,
          relPath: actionMenu.relPath,
          disabled: actionMenu.disabled,
          isInRepo: actionMenu.isInRepo,
          kind: actionMenu.kind,
        },
        () => setConfirmUninstall({ flatName: actionMenu.skillFlatName, name: actionMenu.skillName, kind: actionMenu.kind }),
        (repoName) => { setTableConfirmUninstallRepo(repoName); setActionMenu(null); },
      )
    : [];

  return (
    <Card>
      <div className="overflow-auto max-h-[calc(100vh-320px)]">
        <table className="w-full text-left">
          <thead className="sticky top-0 z-10 bg-surface">
            <tr className="border-b-2 border-dashed border-muted-dark">
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium w-0" />
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Name</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Type</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">
                {resourceKind === 'agent' ? 'Synced to' : 'Available in'}
              </th>
              <th className="pb-3 text-pencil-light text-sm font-medium w-10" />
            </tr>
          </thead>
          <tbody>
            {visible.map((skill) => {
              const currentValue = skill.targets?.length === 1 ? skill.targets[0] : '__all__';
              const showPath = skill.relPath !== skill.name;
              const agentTargets = summarizeAgentTargets(getSkillTargets(skill.flatName));
              return (
                <tr
                  key={skill.flatName}
                  className={`border-b border-dashed border-muted hover:bg-paper-warm/60 transition-colors${actionMenu?.skillFlatName === skill.flatName ? ' bg-paper-warm/60' : ''}`}
                >
                  {/* Status stripe */}
                  <td className="py-3.5 pr-0 w-1">
                    <div
                      className="w-1 h-6 rounded-full"
                      style={{
                        backgroundColor: skill.isInRepo
                          ? 'var(--color-pencil-light)'
                          : 'var(--color-muted)',
                      }}
                      title={skill.isInRepo ? 'Tracked' : 'Local'}
                    />
                  </td>
                  {/* Name + path subtitle + source */}
                  <td className="py-3.5 pr-4">
                    <div className="flex items-center gap-2">
                      <div className="min-w-0 flex-1">
                        <Link
                          to={resourceDetailHref(skill)}
                          className="font-medium text-pencil hover:underline block truncate"
                        >
                          {skill.name}
                        </Link>
                        {showPath && (
                          <span className="block text-xs font-mono text-pencil-light/60 truncate mt-0.5">
                            {skill.relPath}
                          </span>
                        )}
                      </div>
                      {skill.source && (() => {
                        const parsed = parseRemoteURL(skill.source);
                        const raw = parsed?.webURL ?? skill.source;
                        const href = raw.startsWith('http') ? raw : `https://${raw}`;
                        return (
                          <Tooltip content={parsed?.ownerRepo ?? skill.source} delay={500}>
                            <a href={href} target="_blank" rel="noopener noreferrer" className="text-pencil-light/40 hover:text-pencil-light shrink-0 transition-colors" onClick={(e) => e.stopPropagation()}>
                              <Globe size={13} strokeWidth={2} />
                            </a>
                          </Tooltip>
                        );
                      })()}
                    </div>
                  </td>
                  {/* Type badges */}
                  <td className="py-3.5 pr-4">
                    <div className="flex items-center gap-1.5 flex-wrap">
                      {skill.disabled && <Badge variant="danger">Disabled</Badge>}
                      <SourceBadge type={skill.type} isInRepo={skill.isInRepo} />
                      {skill.branch && (
                        <Badge variant="default">
                          <GitBranch size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                          {skill.branch}
                        </Badge>
                      )}
                    </div>
                  </td>
                  {/* Available in — inline Select */}
                  <td className="py-3" onClick={(e) => e.stopPropagation()}>
                    {resourceKind === 'agent' ? (
                      <Tooltip content={agentTargets.title}>
                        <span className="inline-flex items-center text-sm text-pencil-light">
                          {agentTargets.label}
                        </span>
                      </Tooltip>
                    ) : (
                      <Select
                        value={currentValue}
                        onChange={(val) => {
                          targetMutation.mutate({
                            name: skill.flatName,
                            target: val === '__all__' ? null : val,
                          });
                        }}
                        options={targetOptions}
                        size="sm"
                        className="min-w-[7rem] max-w-[9rem]"
                      />
                    )}
                  </td>
                  {/* Actions ⋯ */}
                  <td className="py-3.5 w-10">
                    <button
                      className="p-1 rounded hover:bg-muted/60 transition-colors text-pencil-light"
                      style={{ borderRadius: radius.sm }}
                      onClick={(e) => {
                        e.stopPropagation();
                        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
                        setActionMenu({
                          point: { x: rect.right, y: rect.bottom },
                          skillFlatName: skill.flatName,
                          skillName: skill.name,
                          kind: skill.kind,
                          relPath: skill.relPath,
                          disabled: !!skill.disabled,
                          isInRepo: !!skill.isInRepo,
                        });
                      }}
                      title="Actions"
                    >
                      <MoreHorizontal size={16} strokeWidth={2} />
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {skills.length > TABLE_PAGE_SIZES[0] && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onPageChange={(p) => setPage(p)}
          rangeText={`${start + 1}–${Math.min(start + pageSize, skills.length)} of ${skills.length}`}
          pageSize={{
            value: pageSize,
            options: TABLE_PAGE_SIZES,
            onChange: (s) => { setPageSize(s); setPage(0); localStorage.setItem('skillshare:table-page-size', String(s)); },
          }}
        />
      )}
      {actionMenu && (
        <SkillContextMenu
          open={true}
          items={actionItems}
          anchorPoint={actionMenu.point}
          onClose={() => setActionMenu(null)}
        />
      )}
      <ConfirmDialog
        open={!!confirmUninstall}
        title={`Uninstall ${resourceLabel(confirmUninstall?.kind ?? resourceKind)}`}
        message={<>Are you sure you want to uninstall <strong>{confirmUninstall?.name}</strong>?</>}
        confirmText="Uninstall"
        variant="danger"
        loading={uninstallMutation.isPending}
        onConfirm={() => {
          if (confirmUninstall) uninstallMutation.mutate({ name: confirmUninstall.flatName, kind: confirmUninstall.kind });
          setConfirmUninstall(null);
        }}
        onCancel={() => setConfirmUninstall(null)}
      />
      <ConfirmDialog
        open={!!tableConfirmUninstallRepo}
        title="Uninstall tracked repo"
        message={<>Are you sure you want to uninstall all skills in <strong>{tableConfirmUninstallRepo}</strong>?</>}
        confirmText="Uninstall Repo"
        variant="danger"
        loading={tableUninstallRepoMutation.isPending}
        onConfirm={() => {
          if (tableConfirmUninstallRepo) tableUninstallRepoMutation.mutate(tableConfirmUninstallRepo);
          setTableConfirmUninstallRepo(null);
        }}
        onCancel={() => setTableConfirmUninstallRepo(null)}
      />
    </Card>
  );
}
