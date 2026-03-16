import { useState, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Trash2, Plus, Target, ArrowDownToLine, Search, CircleDot, PenLine, AlertTriangle } from 'lucide-react';
import Card from '../components/Card';
import StatusBadge from '../components/StatusBadge';
import Button from '../components/Button';
import IconButton from '../components/IconButton';
import { Input, Select } from '../components/Input';
import EmptyState from '../components/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import PageHeader from '../components/PageHeader';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { AvailableTarget } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { radius, shadows } from '../design';
import { shortenHome } from '../lib/paths';
import { useSyncMatrix } from '../hooks/useSyncMatrix';

const SYNC_MODE_OPTIONS = [
  { value: 'merge', label: 'Merge (default)', description: 'Per-skill symlinks, preserves local skills' },
  { value: 'symlink', label: 'Symlink', description: 'Entire directory symlinked to source' },
  { value: 'copy', label: 'Copy', description: 'Physical file copies instead of symlinks' },
];

export default function TargetsPage() {
  const queryClient = useQueryClient();
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.targets.all,
    queryFn: () => api.listTargets(),
    staleTime: staleTimes.targets,
  });
  const availTargets = useQuery({
    queryKey: queryKeys.targets.available,
    queryFn: () => api.availableTargets(),
    staleTime: staleTimes.targets,
  });
  const [adding, setAdding] = useState(false);
  const [newTarget, setNewTarget] = useState({ name: '', path: '' });
  const [searchQuery, setSearchQuery] = useState('');
  const [customMode, setCustomMode] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);
  const [collecting, setCollecting] = useState<string | null>(null);
  const navigate = useNavigate();
  const { getTargetSummary } = useSyncMatrix();
  const { toast } = useToast();

  // Compute filtered & sectioned available targets
  const { detected, others } = useMemo(() => {
    const all = (availTargets.data?.targets ?? []).filter((t) => !t.installed);
    const q = searchQuery.toLowerCase().trim();
    const filtered = q ? all.filter((t) => t.name.toLowerCase().includes(q)) : all;
    const sorted = [...filtered].sort((a, b) => a.name.localeCompare(b.name));
    return {
      detected: sorted.filter((t) => t.detected),
      others: sorted.filter((t) => !t.detected),
    };
  }, [availTargets.data, searchQuery]);

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          Failed to load targets
        </p>
        <p className="text-pencil-light text-sm mt-1">{error.message}</p>
      </Card>
    );
  }

  const targets = data?.targets ?? [];
  const sourceSkillCount = data?.sourceSkillCount ?? 0;

  const handleAdd = async () => {
    if (!newTarget.name) return;
    try {
      const avail = availTargets.data?.targets.find((t) => t.name === newTarget.name);
      const path = newTarget.path || avail?.path || '';
      if (!path) return;
      await api.addTarget(newTarget.name, path);
      setAdding(false);
      setNewTarget({ name: '', path: '' });
      setSearchQuery('');
      setCustomMode(false);
      toast(`Target "${newTarget.name}" added.`, 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.available });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    }
  };

  const handleRemove = async (name: string) => {
    try {
      await api.removeTarget(name);
      toast(`Target "${name}" removed.`, 'success');
      setRemoving(null);
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setRemoving(null);
    }
  };

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <PageHeader
        icon={<Target size={24} strokeWidth={2.5} />}
        title="Targets"
        subtitle={`${targets.length} target${targets.length !== 1 ? 's' : ''} configured`}
        actions={
          <Button
            onClick={() => {
              if (adding) {
                setAdding(false);
                setNewTarget({ name: '', path: '' });
                setSearchQuery('');
                setCustomMode(false);
              } else {
                setAdding(true);
              }
            }}
            variant={adding ? 'secondary' : 'primary'}
            size="sm"
          >
            {adding ? null : <Plus size={16} strokeWidth={2.5} />}
            {adding ? 'Cancel' : 'Add Target'}
          </Button>
        }
      />

      {/* Add target form */}
      {adding && (
        <Card className="mb-6 animate-fade-in">
          <h3
            className="font-bold text-pencil text-lg mb-4"
          >
            Add New Target
          </h3>

          {/* Selected target preview + path + actions */}
          {newTarget.name && !customMode ? (
            <div className="space-y-4 animate-fade-in">
              <div
                className="flex items-center gap-3 bg-surface border-2 border-blue px-4 py-3"
                style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
              >
                <Target size={18} strokeWidth={2.5} className="text-blue shrink-0" />
                <div className="min-w-0 flex-1">
                  <p className="font-bold text-pencil">
                    {newTarget.name}
                  </p>
                  <p
                    className="font-mono text-sm text-pencil-light truncate"
                  >
                    {shortenHome(newTarget.path)}
                  </p>
                </div>
                <Button
                  onClick={() => setNewTarget({ name: '', path: '' })}
                  variant="ghost"
                  size="sm"
                >
                  Change
                </Button>
              </div>

              <Input
                label="Path (customize if needed)"
                type="text"
                value={newTarget.path}
                onChange={(e) => setNewTarget({ ...newTarget, path: e.target.value })}
                placeholder="/path/to/target"
              />

              <div className="flex gap-3">
                <Button onClick={handleAdd} variant="primary" size="sm">
                  <Plus size={16} strokeWidth={2.5} />
                  Add Target
                </Button>
              </div>
            </div>
          ) : customMode ? (
            /* Custom target entry mode */
            <div className="space-y-4 animate-fade-in">
              <Input
                label="Target Name"
                type="text"
                value={newTarget.name}
                onChange={(e) => setNewTarget({ ...newTarget, name: e.target.value })}
                placeholder="my-custom-target"
              />
              <Input
                label="Path"
                type="text"
                value={newTarget.path}
                onChange={(e) => setNewTarget({ ...newTarget, path: e.target.value })}
                placeholder="/path/to/target/skills"
              />
              <div className="flex gap-3">
                <Button onClick={handleAdd} variant="primary" size="sm">
                  <Plus size={16} strokeWidth={2.5} />
                  Add Target
                </Button>
                <Button
                  onClick={() => {
                    setCustomMode(false);
                    setNewTarget({ name: '', path: '' });
                  }}
                  variant="ghost"
                  size="sm"
                >
                  Back to picker
                </Button>
              </div>
            </div>
          ) : (
            /* Target picker mode */
            <div className="space-y-4">
              {/* Search bar */}
              <div className="relative">
                <Search
                  size={18}
                  strokeWidth={2.5}
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
                />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search targets..."
                  className="w-full pl-10 pr-4 py-2.5 bg-surface border-2 border-muted text-pencil placeholder:text-muted-dark focus:outline-none focus:border-pencil transition-all"
                  style={{
                    borderRadius: radius.sm,
                    fontSize: '1rem',
                  }}
                  autoFocus
                />
              </div>

              {/* Scrollable target list */}
              <div
                className="max-h-72 overflow-y-auto border-2 border-dashed border-muted-dark bg-surface"
                style={{ borderRadius: radius.md }}
              >
                {/* Detected section */}
                {detected.length > 0 && (
                  <div>
                    <div className="px-3 py-2 border-b border-dashed border-muted-dark sticky top-0 z-10 bg-surface relative">
                      <div className="absolute inset-0 bg-success-light pointer-events-none" />
                      <span className="relative text-sm font-bold text-success flex items-center gap-1.5">
                        <CircleDot size={14} strokeWidth={3} />
                        Detected on your system
                      </span>
                    </div>
                    {detected.map((t) => (
                      <TargetPickerItem
                        key={t.name}
                        target={t}
                        isDetected
                        onSelect={(target) => {
                          setNewTarget({ name: target.name, path: target.path });
                          setSearchQuery('');
                        }}
                      />
                    ))}
                  </div>
                )}

                {/* All available section */}
                {others.length > 0 && (
                  <div>
                    <div className="px-3 py-2 border-b border-dashed border-muted-dark sticky top-0 z-10 bg-surface">
                      <span className="text-sm font-bold text-pencil-light">
                        All available targets
                      </span>
                    </div>
                    {others.map((t) => (
                      <TargetPickerItem
                        key={t.name}
                        target={t}
                        onSelect={(target) => {
                          setNewTarget({ name: target.name, path: target.path });
                          setSearchQuery('');
                        }}
                      />
                    ))}
                  </div>
                )}

                {/* No results */}
                {detected.length === 0 && others.length === 0 && (
                  <div className="px-4 py-8 text-center text-pencil-light">
                    {searchQuery ? `No targets matching "${searchQuery}"` : 'No available targets'}
                  </div>
                )}
              </div>

              {/* Custom target link */}
              <div className="flex items-center justify-between">
                <Button
                  variant="link"
                  onClick={() => setCustomMode(true)}
                  className="inline-flex items-center gap-1.5"
                >
                  <PenLine size={14} strokeWidth={2.5} />
                  Enter custom target
                </Button>
              </div>
            </div>
          )}
        </Card>
      )}

      {/* Targets list */}
      {targets.length > 0 ? (
        <div data-tour="targets-grid" className="space-y-4">
          {targets.map((target, i) => {
            const expectedCount = target.expectedSkillCount || sourceSkillCount;
            const isMergeOrCopy = target.mode === 'merge' && target.status === 'merged' || target.mode === 'copy' && target.status === 'copied';
            const hasDrift = isMergeOrCopy && target.linkedCount < expectedCount;
            return (
              <Card
                key={target.name}
                className={`!overflow-visible ${i % 2 === 0 ? 'rotate-[-0.15deg]' : 'rotate-[0.15deg]'}`}
                style={{ position: 'relative', zIndex: targets.length - i }}
              >
                {/* Top row: name + path + action icons */}
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1 flex-wrap">
                      <Target size={16} strokeWidth={2.5} className="text-success shrink-0" />
                      <span className="font-bold text-pencil">{target.name}</span>
                      <StatusBadge status={target.status} />
                    </div>
                    <p className="font-mono text-sm text-pencil-light truncate">
                      {shortenHome(target.path)}
                    </p>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    {(target.mode === 'merge' || target.mode === 'copy') && target.localCount > 0 && (
                      <IconButton
                        icon={<ArrowDownToLine size={16} strokeWidth={2.5} />}
                        label="Collect local skills"
                        size="md"
                        variant="outline"
                        onClick={() => setCollecting(target.name)}
                        className="hover:text-blue hover:border-blue"
                      />
                    )}
                    <IconButton
                      icon={<Trash2 size={16} strokeWidth={2.5} />}
                      label="Remove target"
                      size="md"
                      variant="danger-outline"
                      onClick={() => setRemoving(target.name)}
                    />
                  </div>
                </div>
                {/* Full-width separator + sync controls */}
                <div className="mt-3 pt-3 border-t border-dashed border-pencil-light/30 flex items-center gap-2">
                  <Select
                    value={target.mode || 'merge'}
                    onChange={async (mode) => {
                      try {
                        await api.updateTarget(target.name, { mode });
                        queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
                        toast(`Sync mode for ${target.name} changed to ${mode}`, 'success');
                      } catch (e) {
                        toast((e as Error).message, 'error');
                      }
                    }}
                    options={SYNC_MODE_OPTIONS}
                    size="sm"
                    className="w-44"
                  />
                  {(target.mode === 'merge' || target.mode === 'copy') && (
                    <span className={`text-sm ml-auto ${hasDrift ? 'text-warning' : 'text-muted-dark'}`}>
                      {hasDrift ? (
                        <span className="flex items-center gap-1">
                          <AlertTriangle size={12} strokeWidth={2.5} />
                          {target.linkedCount}/{expectedCount} {target.mode === 'copy' ? 'managed' : 'shared'}, {target.localCount} local
                        </span>
                      ) : (
                        <>{target.linkedCount} {target.mode === 'copy' ? 'managed' : 'shared'}, {target.localCount} local</>
                      )}
                    </span>
                  )}
                </div>
                {/* Filter summary line */}
                {(target.mode === 'merge' || target.mode === 'copy') && (
                  <div className="mt-3 flex items-center gap-2 flex-wrap">
                    <span className="text-sm text-pencil-light">
                      {(() => {
                        const summary = getTargetSummary(target.name);
                        const hasFilters = target.include?.length || target.exclude?.length;
                        if (summary.total === 0) return 'No skills';
                        if (!hasFilters) return `All ${summary.total} skills`;
                        return `${summary.synced}/${summary.total} skills`;
                      })()}
                    </span>
                    {(() => {
                      const inc = target.include ?? [];
                      const exc = target.exclude ?? [];
                      const MAX_TAGS = 3;
                      const visibleInc = inc.slice(0, MAX_TAGS);
                      const visibleExc = exc.slice(0, Math.max(0, MAX_TAGS - visibleInc.length));
                      const overflow = (inc.length + exc.length) - (visibleInc.length + visibleExc.length);
                      return (
                        <>
                          {visibleInc.map((p, pi) => (
                            <span key={`inc-${pi}`} className="text-xs font-bold text-blue bg-info-light px-2 py-0.5 border border-blue/30" style={{ borderRadius: radius.sm }}>
                              + {p}
                            </span>
                          ))}
                          {visibleExc.map((p, pi) => (
                            <span key={`exc-${pi}`} className="text-xs font-bold text-danger bg-danger-light px-2 py-0.5 border border-danger/30" style={{ borderRadius: radius.sm }}>
                              − {p}
                            </span>
                          ))}
                          {overflow > 0 && (
                            <span className="text-xs text-pencil-light">+{overflow} more</span>
                          )}
                        </>
                      );
                    })()}
                    <Link
                      to={`/targets/${encodeURIComponent(target.name)}/filters`}
                      className="text-xs font-bold text-blue hover:underline"
                    >
                      {(target.include?.length || target.exclude?.length) ? 'Edit in Filter Studio →' : 'Customize filters →'}
                    </Link>
                  </div>
                )}
              </Card>
            );
          })}
        </div>
      ) : (
        <EmptyState
          icon={Target}
          title="No targets configured"
          description="Add a target to start syncing your skills."
          action={
            !adding ? (
              <Button onClick={() => setAdding(true)} variant="secondary" size="sm">
                <Plus size={16} strokeWidth={2.5} />
                Add Your First Target
              </Button>
            ) : undefined
          }
        />
      )}

      {/* Confirm remove dialog */}
      <ConfirmDialog
        open={!!removing}
        title="Remove Target"
        message={`Remove target "${removing}"? Skills will no longer sync to it.`}
        confirmText="Remove"
        variant="danger"
        onConfirm={() => removing && handleRemove(removing)}
        onCancel={() => setRemoving(null)}
      />

      {/* Confirm collect dialog */}
      <ConfirmDialog
        open={!!collecting}
        title="Collect Local Skills"
        message={`Scan "${collecting}" for local skills to collect back to source?`}
        confirmText="Scan"
        onConfirm={() => {
          if (collecting) navigate(`/collect?target=${encodeURIComponent(collecting)}`);
          setCollecting(null);
        }}
        onCancel={() => setCollecting(null)}
      />
    </div>
  );
}

/** Clickable row inside the target picker list */
function TargetPickerItem({
  target,
  isDetected,
  onSelect,
}: {
  target: AvailableTarget;
  isDetected?: boolean;
  onSelect: (target: AvailableTarget) => void;
}) {
  return (
    <button
      onClick={() => onSelect(target)}
      className="w-full text-left px-3 py-2.5 flex items-center gap-3 border-b border-muted/60 hover:bg-muted/20 transition-colors cursor-pointer group"
    >
      {isDetected ? (
        <span className="w-2.5 h-2.5 rounded-full bg-success shrink-0" />
      ) : (
        <span className="w-2.5 h-2.5 rounded-full border-2 border-muted-dark shrink-0" />
      )}
      <div className="min-w-0 flex-1">
        <span className="font-bold text-pencil group-hover:text-blue transition-colors">
          {target.name}
        </span>
        <p
          className="font-mono text-xs text-pencil-light truncate mt-0.5"
        >
          {shortenHome(target.path)}
        </p>
      </div>
      {isDetected && (
        <span
          className="text-xs text-success bg-success-light px-2 py-0.5 shrink-0"
          style={{ borderRadius: radius.sm }}
        >
          detected
        </span>
      )}
    </button>
  );
}
