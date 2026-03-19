import { useState, useMemo, useCallback, useDeferredValue } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Trash2,
  CheckSquare,
  Square,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Loader2,
  Search,
  LayoutGrid,
  Users,
  Globe,
  FolderOpen,
} from 'lucide-react';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { api } from '../api/client';
import type { Skill, BatchUninstallItemResult } from '../api/client';
import Button from '../components/Button';
import Card from '../components/Card';
import Badge from '../components/Badge';
import PageHeader from '../components/PageHeader';
import EmptyState from '../components/EmptyState';
import SegmentedControl from '../components/SegmentedControl';
import ConfirmDialog from '../components/ConfirmDialog';
import DialogShell from '../components/DialogShell';
import { Input, Select } from '../components/Input';
import { Checkbox } from '../components/Checkbox';
import { useToast } from '../components/Toast';
import { PageSkeleton } from '../components/Skeleton';
import { Virtuoso } from 'react-virtuoso';
import { radius } from '../design';

/* ── Glob → Regex (supports * and ? only) ──────────── */

function globToRegex(pattern: string): RegExp {
  const escaped = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.');
  return new RegExp(`^${escaped}$`, 'i');
}

/* ── Types ──────────────────────────────────────────── */

type FilterType = 'all' | 'tracked' | 'github' | 'local';
type Phase = 'selecting' | 'uninstalling' | 'done';

function matchTypeFilter(skill: Skill, filterType: FilterType): boolean {
  switch (filterType) {
    case 'all': return true;
    case 'tracked': return skill.isInRepo;
    case 'github': return (skill.type === 'github' || skill.type === 'github-subdir') && !skill.isInRepo;
    case 'local': return !skill.type && !skill.isInRepo;
  }
}

const typeFilterOptions: { key: FilterType; label: string; icon: React.ReactNode }[] = [
  { key: 'all', label: 'All', icon: <LayoutGrid size={14} strokeWidth={2.5} /> },
  { key: 'tracked', label: 'Tracked', icon: <Users size={14} strokeWidth={2.5} /> },
  { key: 'github', label: 'GitHub', icon: <Globe size={14} strokeWidth={2.5} /> },
  { key: 'local', label: 'Local', icon: <FolderOpen size={14} strokeWidth={2.5} /> },
];

function getTypeLabel(type?: string): string | undefined {
  if (!type) return undefined;
  if (type === 'github-subdir') return 'github';
  return type;
}

/* ── Component ──────────────────────────────────────── */

export default function BatchUninstallPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const { data, isPending } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const skills = data?.skills ?? [];

  // Filter state
  const [group, setGroup] = useState('(all)');
  const [pattern, setPattern] = useState('');
  const [typeFilter, setTypeFilter] = useState<FilterType>('all');

  // Selection state
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // Operation state
  const [phase, setPhase] = useState<Phase>('selecting');
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [forceChecked, setForceChecked] = useState(false);
  const [results, setResults] = useState<BatchUninstallItemResult[]>([]);
  const [summary, setSummary] = useState<{ succeeded: number; failed: number } | null>(null);

  // Extract groups
  const groups = useMemo(() => {
    const dirs = new Set<string>();
    skills.forEach((s) => {
      const parts = s.relPath.split('/');
      if (parts.length > 1) dirs.add(parts[0]);
    });
    return ['(all)', '(root)', ...Array.from(dirs).sort()];
  }, [skills]);

  const groupOptions = useMemo(
    () => groups.map((g) => ({
      value: g,
      label: g === '(all)' ? 'All groups' : g === '(root)' ? 'Top-level' : g.replace(/^_/, ''),
    })),
    [groups],
  );

  // Lookup map for O(1) skill access by flatName
  const skillByFlatName = useMemo(() => {
    const map = new Map<string, Skill>();
    skills.forEach((s) => map.set(s.flatName, s));
    return map;
  }, [skills]);

  // Filter counts
  const filterCounts = useMemo(() => {
    const counts: Record<FilterType, number> = { all: skills.length, tracked: 0, github: 0, local: 0 };
    for (const s of skills) {
      if (s.isInRepo) counts.tracked++;
      if ((s.type === 'github' || s.type === 'github-subdir') && !s.isInRepo) counts.github++;
      if (!s.type && !s.isInRepo) counts.local++;
    }
    return counts;
  }, [skills]);

  const deferredPattern = useDeferredValue(pattern);

  // Filter skills
  const filtered = useMemo(() => {
    let list = skills;
    if (deferredPattern.trim()) {
      const regex = globToRegex(deferredPattern.trim());
      list = list.filter(
        (s) => regex.test(s.name) || regex.test(s.relPath) || regex.test(s.flatName),
      );
    } else if (group !== '(all)') {
      if (group === '(root)') {
        list = list.filter((s) => !s.relPath.includes('/'));
      } else {
        list = list.filter((s) => s.relPath.startsWith(group + '/') || s.relPath === group);
      }
    }
    if (typeFilter !== 'all') {
      list = list.filter((s) => matchTypeFilter(s, typeFilter));
    }
    return list.sort((a, b) => a.name.localeCompare(b.name));
  }, [skills, deferredPattern, group, typeFilter]);

  // Helpers
  const getRepoName = useCallback((skill: Skill): string | null => {
    if (!skill.isInRepo) return null;
    return skill.relPath.split('/')[0];
  }, []);

  const getRepoSiblings = useCallback(
    (repoDir: string): Skill[] =>
      skills.filter((s) => s.relPath.startsWith(repoDir + '/') || s.relPath === repoDir),
    [skills],
  );

  const toggleSelect = useCallback(
    (skill: Skill) => {
      setSelected((prev) => {
        const next = new Set(prev);
        const key = skill.flatName;
        if (next.has(key)) {
          next.delete(key);
          const repo = getRepoName(skill);
          if (repo) getRepoSiblings(repo).forEach((s) => next.delete(s.flatName));
        } else {
          next.add(key);
          const repo = getRepoName(skill);
          if (repo) getRepoSiblings(repo).forEach((s) => next.add(s.flatName));
        }
        return next;
      });
    },
    [getRepoName, getRepoSiblings],
  );

  const selectAll = useCallback(() => {
    setSelected((prev) => {
      const next = new Set(prev);
      filtered.forEach((s) => {
        next.add(s.flatName);
        const repo = getRepoName(s);
        if (repo) getRepoSiblings(repo).forEach((sib) => next.add(sib.flatName));
      });
      return next;
    });
  }, [filtered, getRepoName, getRepoSiblings]);

  const deselectAll = useCallback(() => {
    setSelected((prev) => {
      const next = new Set(prev);
      filtered.forEach((s) => next.delete(s.flatName));
      return next;
    });
  }, [filtered]);

  const buildApiNames = useCallback((): string[] => {
    const names = new Set<string>();
    const processedRepos = new Set<string>();
    for (const flatName of selected) {
      const skill = skillByFlatName.get(flatName);
      if (!skill) continue;
      const repo = getRepoName(skill);
      if (repo && !processedRepos.has(repo)) {
        processedRepos.add(repo);
        names.add(repo);
      } else if (!repo) {
        names.add(skill.flatName);
      }
    }
    return Array.from(names);
  }, [selected, skillByFlatName, getRepoName]);

  const hasRepoWarning = useMemo(() => {
    for (const flatName of selected) {
      const skill = skillByFlatName.get(flatName);
      if (skill?.isInRepo) return true;
    }
    return false;
  }, [selected, skillByFlatName]);

  const executeUninstall = useCallback(async () => {
    setConfirmOpen(false);
    setPhase('uninstalling');
    try {
      const apiNames = buildApiNames();
      const res = await api.batchUninstall({ names: apiNames, force: forceChecked });
      setResults(res.results);
      setSummary(res.summary);
      setPhase('done');
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: ['overview'] });
      queryClient.invalidateQueries({ queryKey: ['trash'] });
      if (res.summary.failed === 0) {
        toast(`Successfully removed ${res.summary.succeeded} item(s)`, 'success');
      } else if (res.summary.succeeded > 0) {
        toast(`Removed ${res.summary.succeeded}, failed ${res.summary.failed}. See details below.`, 'warning');
      } else {
        toast(`All ${res.summary.failed} uninstall(s) failed`, 'error');
      }
    } catch (err) {
      setPhase('done');
      toast(`Uninstall failed: ${err instanceof Error ? err.message : String(err)}`, 'error');
    }
  }, [buildApiNames, forceChecked, queryClient, toast]);

  const dismissResults = useCallback(() => {
    setPhase('selecting');
    setResults([]);
    setSummary(null);
    setSelected(new Set());
  }, []);

  const selectedInView = filtered.filter((s) => selected.has(s.flatName)).length;
  const allInViewSelected = filtered.length > 0 && selectedInView === filtered.length;
  const hasActiveFilter = typeFilter !== 'all' || group !== '(all)' || deferredPattern.trim() !== '';

  // ── Render ───────────────────────────────────────────

  if (isPending) return <PageSkeleton />;

  if (skills.length === 0) {
    return (
      <div className="space-y-5 animate-fade-in">
        <PageHeader title="Uninstall Skills" icon={<Trash2 size={24} strokeWidth={2.5} />} />
        <EmptyState
          icon={Trash2}
          title="No skills installed"
          description="Install some skills first, then come back to manage them."
          action={
            <Button variant="secondary" size="sm" onClick={() => navigate('/search')}>
              Search Skills
            </Button>
          }
        />
      </div>
    );
  }

  return (
    <div className="space-y-3 animate-fade-in pb-20">
      <PageHeader
        title="Uninstall Skills"
        icon={<Trash2 size={24} strokeWidth={2.5} />}
        subtitle={`${skills.length} skill${skills.length !== 1 ? 's' : ''} installed`}
        className="mb-2!"
      />

      {/* ── Sticky Toolbar (matches SkillsPage pattern) ── */}
      <div className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 py-2 mb-1 space-y-2">
        {/* Search row — full width */}
        <div className="relative">
          <Search
            size={18}
            strokeWidth={2.5}
            className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
          />
          <Input
            placeholder="Filter by glob pattern… e.g. *react*, frontend/*"
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            className="!pl-11"
          />
        </div>

        {/* Filters row: group dropdown + type tabs + select actions */}
        <div className="flex flex-wrap items-center gap-3">
          <div className="w-44">
            <Select
              value={group}
              onChange={setGroup}
              options={groupOptions}
              size="sm"
              disabled={!!pattern.trim()}
            />
          </div>
          <SegmentedControl
            value={typeFilter}
            onChange={setTypeFilter}
            size="sm"
            options={typeFilterOptions.map((opt) => ({
              value: opt.key,
              label: <span className="inline-flex items-center gap-1.5">{opt.icon}{opt.label}</span>,
              count: filterCounts[opt.key],
            }))}
          />
          <div className="flex items-center gap-2 ml-auto">
            <Button
              variant="ghost"
              size="sm"
              onClick={allInViewSelected ? deselectAll : selectAll}
              disabled={filtered.length === 0 || phase !== 'selecting'}
            >
              {allInViewSelected
                ? <><Square size={14} /> Deselect All</>
                : <><CheckSquare size={14} /> Select All</>
              }
            </Button>
            {selected.size > 0 && (
              <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
                Clear
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* ── Summary line ─────────────────────────────────── */}
      {hasActiveFilter && (
        <p className="text-pencil-light text-sm mb-3">
          Showing {filtered.length} of {skills.length} skills
          {selected.size > 0 && (
            <> &middot; <strong className="text-danger">{selected.size} selected</strong></>
          )}
          {' '}&middot;{' '}
          <Button
            variant="link"
            onClick={() => { setTypeFilter('all'); setGroup('(all)'); setPattern(''); }}
          >
            Clear filters
          </Button>
        </p>
      )}

      {/* ── Skill List ─────────────────────────────────── */}
      {filtered.length === 0 ? (
        <div className="py-12 text-center">
          <p className="text-pencil-light text-sm">No skills match your filter.</p>
        </div>
      ) : (
        <div
          className="border border-muted bg-surface overflow-hidden"
          style={{ borderRadius: radius.md }}
        >
          <Virtuoso
            useWindowScroll
            totalCount={filtered.length}
            overscan={200}
            itemContent={(index) => {
              const skill = filtered[index];
              const isSelected = selected.has(skill.flatName);
              const repo = getRepoName(skill);
              return (
                <button
                  type="button"
                  className={`
                    w-full text-left px-4 py-2.5 flex items-center gap-3
                    transition-colors duration-100 cursor-pointer
                    ${index > 0 ? 'border-t border-muted/40' : ''}
                    ${isSelected ? 'bg-danger/5' : 'hover:bg-muted/15'}
                    ${phase !== 'selecting' ? 'pointer-events-none opacity-60' : ''}
                  `}
                  onClick={() => toggleSelect(skill)}
                  disabled={phase !== 'selecting'}
                >
                  <Checkbox
                    label=""
                    checked={isSelected}
                    onChange={() => toggleSelect(skill)}
                    size="sm"
                    disabled={phase !== 'selecting'}
                  />
                  <div className="flex-1 min-w-0">
                    <span className="font-mono text-sm text-pencil truncate block">{skill.name}</span>
                    {skill.relPath !== skill.name && (
                      <span className="text-xs text-pencil-light truncate block">{skill.relPath}</span>
                    )}
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    {repo && (
                      <Badge variant="info" size="sm">{repo.replace(/^_/, '')}</Badge>
                    )}
                    <Badge variant="default" size="sm">
                      {skill.isInRepo ? 'tracked' : (getTypeLabel(skill.type) ?? 'local')}
                    </Badge>
                  </div>
                </button>
              );
            }}
          />
        </div>
      )}

      {/* ── Results Dialog (after uninstall) ─────────────── */}
      <DialogShell
        open={phase === 'done' && results.length > 0}
        onClose={dismissResults}
        maxWidth="2xl"
      >
        <Card>
          <h3 className="text-lg font-bold text-pencil mb-4 flex items-center gap-2">
            {summary && summary.failed === 0 ? (
              <><CheckCircle size={20} className="text-success" /> All removed</>
            ) : summary && summary.succeeded === 0 ? (
              <><XCircle size={20} className="text-danger" /> All failed</>
            ) : (
              <><AlertTriangle size={20} className="text-warning" /> Partial result</>
            )}
          </h3>
          <div
            className="max-h-64 overflow-y-auto space-y-1 bg-muted/10 p-3"
            style={{ borderRadius: radius.md }}
          >
            {results.map((r) => (
              <div
                key={r.name}
                className={`flex items-center gap-2 text-sm px-2 py-1 ${
                  r.success ? 'text-success' : 'text-danger'
                }`}
                style={{ borderRadius: radius.sm }}
              >
                {r.success ? <CheckCircle size={14} /> : <XCircle size={14} />}
                <span className="font-mono">{r.name}</span>
                {r.error && <span className="text-pencil-light">— {r.error}</span>}
              </div>
            ))}
          </div>
          <div className="mt-5 pt-4 border-dashed border-t border-pencil-light/30 flex flex-wrap items-center gap-3">
            <Badge variant="info" size="sm">Run sync to update targets</Badge>
            <div className="ml-auto flex gap-3">
              <Button variant="secondary" size="md" onClick={dismissResults}>
                Continue
              </Button>
              <Button variant="primary" size="md" onClick={() => navigate('/sync')}>
                Go to Sync
              </Button>
            </div>
          </div>
        </Card>
      </DialogShell>

      {/* ── Bottom Action Bar ──────────────────────────── */}
      {phase === 'selecting' && selected.size > 0 && (
        <div className="fixed bottom-0 right-0 left-60 max-md:left-0 bg-paper/95 backdrop-blur-sm border-t border-muted px-6 py-3 flex items-center justify-between z-30">
          <span className="text-sm text-pencil">
            <strong className="text-danger">{selected.size}</strong> skill{selected.size !== 1 ? 's' : ''} selected for removal
          </span>
          <Button
            variant="danger"
            size="md"
            onClick={() => setConfirmOpen(true)}
          >
            <Trash2 size={16} />
            Uninstall ({selected.size})
          </Button>
        </div>
      )}

      {phase === 'uninstalling' && (
        <div className="fixed bottom-0 right-0 left-60 max-md:left-0 bg-paper/95 backdrop-blur-sm border-t border-muted px-6 py-3 flex items-center justify-center gap-2 z-30">
          <Loader2 size={16} className="animate-spin text-pencil-light" />
          <span className="text-sm text-pencil">Removing {selected.size} item(s)…</span>
        </div>
      )}

      {/* ── Confirmation Dialog ────────────────────────── */}
      <ConfirmDialog
        open={confirmOpen}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={executeUninstall}
        title="Confirm Batch Uninstall"
        variant="danger"
        confirmText={`Uninstall ${selected.size} item(s)`}
        wide
        message={
          <div className="space-y-3">
            <p className="text-pencil-light">
              {selected.size} item(s) will be moved to trash with 7-day retention.
            </p>
            <div
              className="max-h-48 overflow-y-auto bg-muted/10 p-3 space-y-1"
              style={{ borderRadius: radius.md }}
            >
              {buildApiNames().map((name) => (
                <div key={name} className="font-mono text-sm text-pencil">{name}</div>
              ))}
            </div>
            {hasRepoWarning && (
              <div
                className="flex items-start gap-2 p-3 bg-warning/10 text-sm"
                style={{ borderRadius: radius.md }}
              >
                <AlertTriangle size={16} className="text-warning shrink-0 mt-0.5" />
                <div>
                  <p className="font-medium text-pencil">Tracked repos selected</p>
                  <p className="text-pencil-light mt-0.5">
                    Repos with uncommitted changes will fail unless force is enabled.
                  </p>
                  <Checkbox
                    label="Force (ignore uncommitted changes)"
                    checked={forceChecked}
                    onChange={setForceChecked}
                    size="sm"
                    className="mt-2"
                  />
                </div>
              </div>
            )}
          </div>
        }
      />
    </div>
  );
}
