import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Trash2, Plus, Target, ArrowDownToLine, Search, CircleDot, PenLine, AlertTriangle, Filter } from 'lucide-react';
import Card from '../components/Card';
import StatusBadge from '../components/StatusBadge';
import HandButton from '../components/HandButton';
import { HandInput } from '../components/HandInput';
import FilterTagInput from '../components/FilterTagInput';
import EmptyState from '../components/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { AvailableTarget } from '../api/client';
import { useApi } from '../hooks/useApi';
import { wobbly, shadows } from '../design';
import { shortenHome } from '../lib/paths';

export default function TargetsPage() {
  const { data, loading, error, refetch } = useApi(() => api.listTargets());
  const availTargets = useApi(() => api.availableTargets());
  const [adding, setAdding] = useState(false);
  const [newTarget, setNewTarget] = useState({ name: '', path: '' });
  const [searchQuery, setSearchQuery] = useState('');
  const [customMode, setCustomMode] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);
  const [collecting, setCollecting] = useState<string | null>(null);
  const [editingFilter, setEditingFilter] = useState<string | null>(null);
  const [filterDraft, setFilterDraft] = useState<{ include: string[]; exclude: string[] }>({ include: [], exclude: [] });
  const [savingFilter, setSavingFilter] = useState(false);
  const navigate = useNavigate();
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

  if (loading) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load targets
        </p>
        <p className="text-pencil-light text-sm mt-1">{error}</p>
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
      refetch();
      availTargets.refetch();
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    }
  };

  const handleRemove = async (name: string) => {
    try {
      await api.removeTarget(name);
      toast(`Target "${name}" removed.`, 'success');
      setRemoving(null);
      refetch();
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
      setRemoving(null);
    }
  };

  return (
    <div className="animate-sketch-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2
            className="text-3xl md:text-4xl font-bold text-pencil mb-1"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Targets
          </h2>
          <p className="text-pencil-light">
            {targets.length} target{targets.length !== 1 ? 's' : ''} configured
          </p>
        </div>
        <HandButton
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
          variant={adding ? 'ghost' : 'secondary'}
          size="sm"
        >
          <Plus size={16} strokeWidth={2.5} />
          {adding ? 'Cancel' : 'Add Target'}
        </HandButton>
      </div>

      {/* Add target form */}
      {adding && (
        <Card variant="postit" className="mb-6 animate-sketch-in">
          <h3
            className="font-bold text-pencil text-lg mb-4"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Add New Target
          </h3>

          {/* Selected target preview + path + actions */}
          {newTarget.name && !customMode ? (
            <div className="space-y-4 animate-sketch-in">
              <div
                className="flex items-center gap-3 bg-white border-2 border-blue px-4 py-3"
                style={{ borderRadius: wobbly.sm, boxShadow: shadows.sm }}
              >
                <Target size={18} strokeWidth={2.5} className="text-blue shrink-0" />
                <div className="min-w-0 flex-1">
                  <p className="font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                    {newTarget.name}
                  </p>
                  <p
                    className="text-sm text-pencil-light truncate"
                    style={{ fontFamily: "'Courier New', monospace" }}
                  >
                    {shortenHome(newTarget.path)}
                  </p>
                </div>
                <HandButton
                  onClick={() => setNewTarget({ name: '', path: '' })}
                  variant="ghost"
                  size="sm"
                >
                  Change
                </HandButton>
              </div>

              <HandInput
                label="Path (customize if needed)"
                type="text"
                value={newTarget.path}
                onChange={(e) => setNewTarget({ ...newTarget, path: e.target.value })}
                placeholder="/path/to/target"
              />

              <div className="flex gap-3">
                <HandButton onClick={handleAdd} variant="primary" size="sm">
                  <Plus size={16} strokeWidth={2.5} />
                  Add Target
                </HandButton>
                <HandButton
                  onClick={() => {
                    setAdding(false);
                    setNewTarget({ name: '', path: '' });
                    setSearchQuery('');
                    setCustomMode(false);
                  }}
                  variant="ghost"
                  size="sm"
                >
                  Cancel
                </HandButton>
              </div>
            </div>
          ) : customMode ? (
            /* Custom target entry mode */
            <div className="space-y-4 animate-sketch-in">
              <HandInput
                label="Target Name"
                type="text"
                value={newTarget.name}
                onChange={(e) => setNewTarget({ ...newTarget, name: e.target.value })}
                placeholder="my-custom-target"
              />
              <HandInput
                label="Path"
                type="text"
                value={newTarget.path}
                onChange={(e) => setNewTarget({ ...newTarget, path: e.target.value })}
                placeholder="/path/to/target/skills"
              />
              <div className="flex gap-3">
                <HandButton onClick={handleAdd} variant="primary" size="sm">
                  <Plus size={16} strokeWidth={2.5} />
                  Add Target
                </HandButton>
                <HandButton
                  onClick={() => {
                    setCustomMode(false);
                    setNewTarget({ name: '', path: '' });
                  }}
                  variant="ghost"
                  size="sm"
                >
                  Back to picker
                </HandButton>
                <HandButton
                  onClick={() => {
                    setAdding(false);
                    setNewTarget({ name: '', path: '' });
                    setSearchQuery('');
                    setCustomMode(false);
                  }}
                  variant="ghost"
                  size="sm"
                >
                  Cancel
                </HandButton>
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
                  className="w-full pl-10 pr-4 py-2.5 bg-white border-2 border-pencil text-pencil placeholder:text-muted-dark focus:outline-none focus:border-blue focus:ring-2 focus:ring-blue/20 transition-colors"
                  style={{
                    borderRadius: wobbly.sm,
                    fontFamily: 'var(--font-hand)',
                    fontSize: '1rem',
                  }}
                  autoFocus
                />
              </div>

              {/* Scrollable target list */}
              <div
                className="max-h-72 overflow-y-auto border-2 border-dashed border-muted-dark bg-white"
                style={{ borderRadius: wobbly.md }}
              >
                {/* Detected section */}
                {detected.length > 0 && (
                  <div>
                    <div
                      className="px-3 py-2 bg-success-light/50 border-b border-dashed border-muted-dark sticky top-0"
                      style={{ fontFamily: 'var(--font-hand)' }}
                    >
                      <span className="text-sm font-bold text-success flex items-center gap-1.5">
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
                    <div
                      className="px-3 py-2 bg-muted/40 border-b border-dashed border-muted-dark sticky top-0"
                      style={{ fontFamily: 'var(--font-hand)' }}
                    >
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
                  <div className="px-4 py-8 text-center text-pencil-light" style={{ fontFamily: 'var(--font-hand)' }}>
                    {searchQuery ? `No targets matching "${searchQuery}"` : 'No available targets'}
                  </div>
                )}
              </div>

              {/* Custom target link */}
              <div className="flex items-center justify-between">
                <button
                  onClick={() => setCustomMode(true)}
                  className="inline-flex items-center gap-1.5 text-sm text-blue hover:text-pencil transition-colors cursor-pointer"
                  style={{ fontFamily: 'var(--font-hand)' }}
                >
                  <PenLine size={14} strokeWidth={2.5} />
                  Enter custom target
                </button>
                <HandButton
                  onClick={() => {
                    setAdding(false);
                    setNewTarget({ name: '', path: '' });
                    setSearchQuery('');
                    setCustomMode(false);
                  }}
                  variant="ghost"
                  size="sm"
                >
                  Cancel
                </HandButton>
              </div>
            </div>
          )}
        </Card>
      )}

      {/* Targets list */}
      {targets.length > 0 ? (
        <div className="space-y-4">
          {targets.map((target, i) => {
            const expectedCount = target.expectedSkillCount || sourceSkillCount;
            const isMergeOrCopy = target.mode === 'merge' && target.status === 'merged' || target.mode === 'copy' && target.status === 'copied';
            const hasDrift = isMergeOrCopy && target.linkedCount < expectedCount;
            return (
              <Card
                key={target.name}
                className={i % 2 === 0 ? 'rotate-[-0.2deg]' : 'rotate-[0.2deg]'}
              >
                <div className="flex items-center justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1 flex-wrap">
                      <Target size={16} strokeWidth={2.5} className="text-success shrink-0" />
                      <span
                        className="font-bold text-pencil"
                        style={{ fontFamily: 'var(--font-heading)' }}
                      >
                        {target.name}
                      </span>
                      <StatusBadge status={target.status} />
                      <span
                        className="text-sm text-muted-dark border border-dashed border-muted-dark px-1.5 py-0.5"
                        style={{ borderRadius: wobbly.sm }}
                      >
                        {target.mode}
                      </span>
                    </div>
                    <p
                      className="text-sm text-pencil-light truncate"
                      style={{ fontFamily: "'Courier New', monospace" }}
                    >
                      {shortenHome(target.path)}
                    </p>
                    <div className="mt-1 flex flex-wrap items-center gap-1">
                      {target.include?.length > 0 && (
                        <span className="text-xs text-blue bg-info-light px-1.5 py-0.5" style={{ borderRadius: wobbly.sm }}>
                          include: {target.include.join(', ')}
                        </span>
                      )}
                      {target.exclude?.length > 0 && (
                        <span className="text-xs text-danger bg-danger-light px-1.5 py-0.5" style={{ borderRadius: wobbly.sm }}>
                          exclude: {target.exclude.join(', ')}
                        </span>
                      )}
                      {(target.mode === 'merge' || target.mode === 'copy') && (
                        <button
                          onClick={() => {
                            setEditingFilter(target.name);
                            setFilterDraft({
                              include: [...(target.include || [])],
                              exclude: [...(target.exclude || [])],
                            });
                          }}
                          className="w-6 h-6 flex items-center justify-center text-muted-dark hover:text-blue transition-colors cursor-pointer"
                          title="Edit filters"
                        >
                          <Filter size={13} strokeWidth={2.5} />
                        </button>
                      )}
                    </div>
                    {editingFilter === target.name && (
                      <div className="mt-3 p-3 bg-postit/40 border-2 border-dashed border-muted-dark animate-sketch-in" style={{ borderRadius: wobbly.md }}>
                        <div className="space-y-3">
                          <FilterTagInput
                            label="Include patterns"
                            patterns={filterDraft.include}
                            onChange={(p) => setFilterDraft({ ...filterDraft, include: p })}
                            color="blue"
                          />
                          <FilterTagInput
                            label="Exclude patterns"
                            patterns={filterDraft.exclude}
                            onChange={(p) => setFilterDraft({ ...filterDraft, exclude: p })}
                            color="danger"
                          />
                          <div className="flex gap-2">
                            <HandButton
                              onClick={async () => {
                                setSavingFilter(true);
                                try {
                                  await api.updateTarget(target.name, {
                                    include: filterDraft.include,
                                    exclude: filterDraft.exclude,
                                  });
                                  toast('Filters updated. Run sync to apply.', 'success');
                                  setEditingFilter(null);
                                  refetch();
                                } catch (e: unknown) {
                                  toast((e as Error).message, 'error');
                                } finally {
                                  setSavingFilter(false);
                                }
                              }}
                              variant="primary"
                              size="sm"
                              disabled={savingFilter}
                            >
                              {savingFilter ? 'Saving...' : 'Save'}
                            </HandButton>
                            <HandButton
                              onClick={() => setEditingFilter(null)}
                              variant="ghost"
                              size="sm"
                            >
                              Cancel
                            </HandButton>
                          </div>
                        </div>
                      </div>
                    )}
                    {(target.mode === 'merge' || target.mode === 'copy') && (
                      <p className={`text-sm mt-1 ${hasDrift ? 'text-warning' : 'text-muted-dark'}`}>
                        {hasDrift ? (
                          <span className="flex items-center gap-1">
                            <AlertTriangle size={12} strokeWidth={2.5} />
                            {target.linkedCount}/{expectedCount} {target.mode === 'copy' ? 'managed' : 'shared'}, {target.localCount} local
                          </span>
                        ) : (
                          <>{target.linkedCount} {target.mode === 'copy' ? 'managed' : 'shared'}, {target.localCount} local</>
                        )}
                      </p>
                    )}
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    {(target.mode === 'merge' || target.mode === 'copy') && target.localCount > 0 && (
                      <button
                        onClick={() => setCollecting(target.name)}
                        className="w-8 h-8 flex items-center justify-center text-muted-dark hover:text-blue transition-colors cursor-pointer border-2 border-transparent hover:border-blue"
                        style={{ borderRadius: wobbly.sm }}
                        title="Collect local skills"
                      >
                        <ArrowDownToLine size={16} strokeWidth={2.5} />
                      </button>
                    )}
                    <button
                      onClick={() => setRemoving(target.name)}
                      className="w-8 h-8 flex items-center justify-center text-muted-dark hover:text-danger transition-colors cursor-pointer border-2 border-transparent hover:border-danger"
                      style={{ borderRadius: wobbly.sm }}
                      title="Remove target"
                    >
                      <Trash2 size={16} strokeWidth={2.5} />
                    </button>
                  </div>
                </div>
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
              <HandButton onClick={() => setAdding(true)} variant="secondary" size="sm">
                <Plus size={16} strokeWidth={2.5} />
                Add Your First Target
              </HandButton>
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
      className="w-full text-left px-3 py-2.5 flex items-center gap-3 border-b border-muted/60 hover:bg-postit/40 transition-colors cursor-pointer group"
      style={{ fontFamily: 'var(--font-hand)' }}
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
          className="text-xs text-pencil-light truncate mt-0.5"
          style={{ fontFamily: "'Courier New', monospace" }}
        >
          {shortenHome(target.path)}
        </p>
      </div>
      {isDetected && (
        <span
          className="text-xs text-success bg-success-light px-2 py-0.5 shrink-0"
          style={{ borderRadius: wobbly.sm, fontFamily: 'var(--font-hand)' }}
        >
          detected
        </span>
      )}
    </button>
  );
}
