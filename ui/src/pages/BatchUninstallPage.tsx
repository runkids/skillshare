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
import { Input, Select } from '../components/Input';
import { Checkbox } from '../components/Checkbox';
import { useToast } from '../components/Toast';

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

const typeFilterOptions = [
  { value: 'all' as const, label: 'All' },
  { value: 'tracked' as const, label: 'Tracked' },
  { value: 'github' as const, label: 'GitHub' },
  { value: 'local' as const, label: 'Local' },
];

/* ── Component ──────────────────────────────────────── */

export default function BatchUninstallPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  // Data — api.listSkills() returns { skills: Skill[] }, not Skill[]
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

  // Extract groups from skills
  const groups = useMemo(() => {
    const dirs = new Set<string>();
    skills.forEach((s) => {
      const parts = s.relPath.split('/');
      if (parts.length > 1) dirs.add(parts[0]);
    });
    return ['(all)', '(root)', ...Array.from(dirs).sort()];
  }, [skills]);

  // Select options (Select component uses { value, label }[] — NOT native <option>)
  const groupOptions = useMemo(
    () => groups.map((g) => ({ value: g, label: g })),
    [groups],
  );

  // Deferred pattern for smooth typing (React 19 useDeferredValue)
  const deferredPattern = useDeferredValue(pattern);

  // Filter skills
  const filtered = useMemo(() => {
    let list = skills;

    // Pattern filter (overrides group)
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

    // Type filter
    if (typeFilter !== 'all') {
      list = list.filter((s) => matchTypeFilter(s, typeFilter));
    }

    return list.sort((a, b) => a.name.localeCompare(b.name));
  }, [skills, deferredPattern, group, typeFilter]);

  // Helpers: derive repo name for in-repo skills
  const getRepoName = useCallback((skill: Skill): string | null => {
    if (!skill.isInRepo) return null;
    return skill.relPath.split('/')[0];
  }, []);

  // Get all sibling skills in a tracked repo
  const getRepoSiblings = useCallback(
    (repoDir: string): Skill[] => {
      return skills.filter((s) => s.relPath.startsWith(repoDir + '/') || s.relPath === repoDir);
    },
    [skills],
  );

  // Toggle selection
  const toggleSelect = useCallback(
    (skill: Skill) => {
      setSelected((prev) => {
        const next = new Set(prev);
        const key = skill.flatName;
        if (next.has(key)) {
          next.delete(key);
          const repo = getRepoName(skill);
          if (repo) {
            getRepoSiblings(repo).forEach((s) => next.delete(s.flatName));
          }
        } else {
          next.add(key);
          const repo = getRepoName(skill);
          if (repo) {
            getRepoSiblings(repo).forEach((s) => next.add(s.flatName));
          }
        }
        return next;
      });
    },
    [getRepoName, getRepoSiblings],
  );

  // Select/Deselect all filtered
  const selectAll = useCallback(() => {
    setSelected((prev) => {
      const next = new Set(prev);
      filtered.forEach((s) => {
        next.add(s.flatName);
        const repo = getRepoName(s);
        if (repo) {
          getRepoSiblings(repo).forEach((sib) => next.add(sib.flatName));
        }
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

  // Build the names to send to API (deduplicate repos)
  const buildApiNames = useCallback((): string[] => {
    const names = new Set<string>();
    const processedRepos = new Set<string>();

    for (const flatName of selected) {
      const skill = skills.find((s) => s.flatName === flatName);
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
  }, [selected, skills, getRepoName]);

  // Check if any selected repo has uncommitted changes warning
  const hasRepoWarning = useMemo(() => {
    for (const flatName of selected) {
      const skill = skills.find((s) => s.flatName === flatName);
      if (skill?.isInRepo) return true;
    }
    return false;
  }, [selected, skills]);

  // Execute uninstall
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

      // toast(message, type) — NOT toast.success()
      if (res.summary.failed === 0) {
        toast(`Successfully removed ${res.summary.succeeded} item(s)`, 'success');
      } else if (res.summary.succeeded > 0) {
        toast(
          `Removed ${res.summary.succeeded}, failed ${res.summary.failed}. See details below.`,
          'warning',
        );
      } else {
        toast(`All ${res.summary.failed} uninstall(s) failed`, 'error');
      }
    } catch (err) {
      setPhase('done');
      toast(`Uninstall failed: ${err instanceof Error ? err.message : String(err)}`, 'error');
    }
  }, [buildApiNames, forceChecked, queryClient, toast]);

  // Filtered selection count
  const selectedInView = filtered.filter((s) => selected.has(s.flatName)).length;
  const allInViewSelected = filtered.length > 0 && selectedInView === filtered.length;

  // ── Render ───────────────────────────────────────────

  if (isPending) {
    return (
      <div className="p-6 space-y-4">
        <PageHeader title="Batch Uninstall" icon={<Trash2 size={24} strokeWidth={2.5} />} backTo="/skills" />
        <div className="flex items-center gap-2 text-pencil-light">
          <Loader2 size={16} className="animate-spin" /> Loading skills…
        </div>
      </div>
    );
  }

  if (skills.length === 0) {
    return (
      <div className="p-6 space-y-4">
        <PageHeader title="Batch Uninstall" icon={<Trash2 size={24} strokeWidth={2.5} />} backTo="/skills" />
        {/* EmptyState.icon expects LucideIcon (component ref), NOT rendered JSX */}
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
    <div className="p-6 space-y-4 pb-24">
      <PageHeader title="Batch Uninstall" icon={<Trash2 size={24} strokeWidth={2.5} />} backTo="/skills" />

      {/* ── Filter Toolbar ─────────────────────────────── */}
      <Card>
        <div className="flex flex-wrap items-end gap-3">
          <div className="w-44">
            {/* Select uses options prop + direct onChange(value) — NOT native <select> */}
            <Select
              label="Group"
              value={group}
              onChange={setGroup}
              options={groupOptions}
              disabled={!!pattern.trim()}
            />
          </div>
          <div className="flex-1 min-w-[200px]">
            {/* Input does NOT have an icon prop — just use placeholder for hint */}
            <Input
              label="Pattern"
              placeholder="e.g. *react*, frontend/*, _team*"
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
            />
          </div>
          <div>
            <SegmentedControl
              options={typeFilterOptions}
              value={typeFilter}
              onChange={setTypeFilter}
            />
          </div>
        </div>
        <div className="flex items-center gap-2 mt-3 pt-3 border-t border-muted/40">
          <Button
            variant="ghost"
            size="sm"
            onClick={allInViewSelected ? deselectAll : selectAll}
            disabled={filtered.length === 0 || phase !== 'selecting'}
          >
            {allInViewSelected ? (
              <><Square size={14} /> Deselect All</>
            ) : (
              <><CheckSquare size={14} /> Select All</>
            )}
          </Button>
          <span className="text-sm text-pencil-light">
            {filtered.length} skill{filtered.length !== 1 ? 's' : ''} shown
            {selected.size > 0 && (
              <> · <strong className="text-danger">{selected.size} selected</strong></>
            )}
          </span>
        </div>
      </Card>

      {/* ── Skill List ─────────────────────────────────── */}
      {filtered.length === 0 ? (
        <p className="text-pencil-light text-sm py-4 text-center">
          No skills match your filter.
        </p>
      ) : (
        <Card padding="none" className="divide-y divide-muted/40">
          {filtered.map((skill) => {
            const isSelected = selected.has(skill.flatName);
            const repo = getRepoName(skill);
            return (
              <button
                key={skill.flatName}
                type="button"
                className={`w-full text-left px-4 py-3 flex items-center gap-3 transition-colors cursor-pointer
                  ${isSelected ? 'bg-danger/5' : 'hover:bg-muted/20'}
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
                  <div className="font-mono text-sm text-pencil truncate">{skill.name}</div>
                  {skill.relPath !== skill.name && (
                    <div className="text-xs text-pencil-light truncate">{skill.relPath}</div>
                  )}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {repo && <Badge variant="info" size="sm">repo: {repo.replace(/^_/, '')}</Badge>}
                  {skill.isInRepo ? (
                    <Badge variant="default" size="sm">tracked</Badge>
                  ) : skill.type ? (
                    <Badge variant="default" size="sm">{skill.type === 'github-subdir' ? 'github' : skill.type}</Badge>
                  ) : (
                    <Badge variant="default" size="sm">local</Badge>
                  )}
                </div>
              </button>
            );
          })}
        </Card>
      )}

      {/* ── Results (after uninstall) ──────────────────── */}
      {phase === 'done' && results.length > 0 && (
        <Card>
          <h3 className="font-bold text-pencil mb-3 flex items-center gap-2">
            {summary && summary.failed === 0 ? (
              <><CheckCircle size={16} className="text-success" /> All removed</>
            ) : summary && summary.succeeded === 0 ? (
              <><XCircle size={16} className="text-danger" /> All failed</>
            ) : (
              <><AlertTriangle size={16} className="text-warning" /> Partial result</>
            )}
          </h3>
          <div className="space-y-1">
            {results.map((r) => (
              <div
                key={r.name}
                className={`flex items-center gap-2 text-sm px-2 py-1 rounded ${
                  r.success ? 'text-success' : 'text-danger bg-danger/5'
                }`}
              >
                {r.success ? <CheckCircle size={14} /> : <XCircle size={14} />}
                <span className="font-mono">{r.name}</span>
                {r.error && <span className="text-pencil-light">— {r.error}</span>}
              </div>
            ))}
          </div>
          <div className="mt-4 pt-3 border-t border-muted/40 flex items-center gap-3">
            <Badge variant="info" size="sm">Run sync to update targets</Badge>
            <Button variant="secondary" size="sm" onClick={() => navigate('/skills')}>
              Back to Skills
            </Button>
            <Button variant="secondary" size="sm" onClick={() => navigate('/sync')}>
              Go to Sync
            </Button>
          </div>
        </Card>
      )}

      {/* ── Bottom Action Bar ──────────────────────────── */}
      {phase === 'selecting' && (
        <div className="fixed bottom-0 left-0 right-0 bg-paper border-t-2 border-pencil/20 px-6 py-3 flex items-center justify-between z-40">
          <span className="text-sm text-pencil-light">
            {selected.size > 0 ? (
              <><strong className="text-danger">{selected.size}</strong> skill{selected.size !== 1 ? 's' : ''} selected</>
            ) : (
              'Select skills to uninstall'
            )}
          </span>
          <Button
            variant="danger"
            size="md"
            onClick={() => setConfirmOpen(true)}
            disabled={selected.size === 0}
            loading={phase === 'uninstalling'}
          >
            <Trash2 size={16} />
            Uninstall {selected.size > 0 ? `(${selected.size})` : ''}
          </Button>
        </div>
      )}

      {phase === 'uninstalling' && (
        <div className="fixed bottom-0 left-0 right-0 bg-paper border-t-2 border-pencil/20 px-6 py-3 flex items-center justify-center z-40">
          <Loader2 size={16} className="animate-spin mr-2" />
          <span className="text-sm text-pencil">Uninstalling {selected.size} item(s)…</span>
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
            <p>
              The following {selected.size} item(s) will be moved to trash (7-day retention):
            </p>
            <div className="max-h-48 overflow-y-auto bg-muted/10 rounded p-2 space-y-1">
              {buildApiNames().map((name) => (
                <div key={name} className="font-mono text-sm text-pencil">
                  {name}
                </div>
              ))}
            </div>
            {hasRepoWarning && (
              <div className="flex items-start gap-2 p-2 bg-warning/10 rounded text-sm">
                <AlertTriangle size={16} className="text-warning shrink-0 mt-0.5" />
                <div>
                  <p className="font-medium">Tracked repos selected</p>
                  <p className="text-pencil-light">
                    Tracked repos with uncommitted changes will fail unless force is enabled.
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
