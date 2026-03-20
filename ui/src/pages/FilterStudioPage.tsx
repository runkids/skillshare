import { useState, useEffect, useCallback, useRef, useMemo, memo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Virtuoso } from 'react-virtuoso';
import { Filter, Check, X, Info, PackageOpen, Search } from 'lucide-react';
import { api } from '../api/client';
import type { SyncMatrixEntry } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useToast } from '../components/Toast';
import Card from '../components/Card';
import Button from '../components/Button';
import Spinner from '../components/Spinner';
import PageHeader from '../components/PageHeader';
import EmptyState from '../components/EmptyState';
import FilterTagInput from '../components/FilterTagInput';
import { radius } from '../design';

export default function FilterStudioPage() {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  // Load current target config
  const targetsQuery = useQuery({
    queryKey: queryKeys.targets.all,
    queryFn: () => api.listTargets(),
    staleTime: staleTimes.targets,
  });

  const target = useMemo(
    () => targetsQuery.data?.targets.find((t) => t.name === name),
    [targetsQuery.data, name],
  );

  // Draft filter state
  const [include, setInclude] = useState<string[]>([]);
  const [exclude, setExclude] = useState<string[]>([]);
  const [initialized, setInitialized] = useState(false);

  // Initialize draft from target config once loaded
  useEffect(() => {
    if (target && !initialized) {
      setInclude(target.include ?? []);
      setExclude(target.exclude ?? []);
      setInitialized(true);
    }
  }, [target, initialized]);

  // Debounced preview
  const [preview, setPreview] = useState<SyncMatrixEntry[]>([]);
  const [previewLoading, setPreviewLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const fetchPreview = useCallback(
    async (inc: string[], exc: string[]) => {
      if (!name) return;
      setPreviewLoading(true);
      try {
        const res = await api.previewSyncMatrix(name, inc, exc);
        setPreview(res.entries);
      } catch {
        // silently ignore preview errors
      } finally {
        setPreviewLoading(false);
      }
    },
    [name],
  );

  // Trigger debounced preview on filter change
  useEffect(() => {
    if (!initialized) return;
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => fetchPreview(include, exclude), 500);
    return () => clearTimeout(debounceRef.current);
  }, [include, exclude, initialized, fetchPreview]);

  // Unsaved changes detection
  const hasChanges = useMemo(() => {
    if (!target) return false;
    const savedInc = target.include ?? [];
    const savedExc = target.exclude ?? [];
    return (
      JSON.stringify(include) !== JSON.stringify(savedInc) ||
      JSON.stringify(exclude) !== JSON.stringify(savedExc)
    );
  }, [target, include, exclude]);

  // Save handler
  const [saving, setSaving] = useState(false);

  const handleSave = async (goBack: boolean) => {
    if (!name) return;
    setSaving(true);
    try {
      await api.updateTarget(name, { include, exclude });
      toast(`Filters for "${name}" saved.`, 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.syncMatrix() });
      if (goBack) navigate('/targets');
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  // Click-to-toggle on preview items
  const handleToggleSkill = (entry: SyncMatrixEntry) => {
    if (entry.status === 'skill_target_mismatch') return;
    const skill = entry.skill;
    if (entry.status === 'synced') {
      // Exclude this skill: add to exclude, remove from include
      setExclude((prev) => prev.includes(skill) ? prev : [...prev, skill]);
      setInclude((prev) => prev.filter((p) => p !== skill));
    } else {
      // Include this skill: add to include, remove from exclude
      setInclude((prev) => prev.includes(skill) ? prev : [...prev, skill]);
      setExclude((prev) => prev.filter((p) => p !== skill));
    }
  };

  // Preview search filter
  const [previewSearch, setPreviewSearch] = useState('');
  const filteredPreview = useMemo(() => {
    if (!previewSearch) return preview;
    const q = previewSearch.toLowerCase();
    return preview.filter((e) => e.skill.toLowerCase().includes(q));
  }, [preview, previewSearch]);

  // Summary counts (always from full preview, not filtered)
  const { syncedCount, totalCount } = useMemo(() => ({
    syncedCount: preview.filter((e) => e.status === 'synced').length,
    totalCount: preview.length,
  }), [preview]);

  if (targetsQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner size="lg" />
      </div>
    );
  }

  if (!target) {
    return (
      <div className="animate-fade-in">
        <EmptyState
          icon={Filter}
          title={`Target "${name}" not found`}
          description="This target may have been removed."
          action={
            <Button variant="secondary" size="sm" onClick={() => navigate('/targets')}>
              Back to Targets
            </Button>
          }
        />
      </div>
    );
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<Filter size={24} strokeWidth={2.5} />}
        title="Filter Studio"
        subtitle={`Route specific skills to ${name}. Use glob patterns like frontend*, _team__*.`}
        backTo="/targets"
        actions={
          <>
            <Button
              variant="primary"
              size="sm"
              onClick={() => handleSave(false)}
              loading={saving}
              disabled={!hasChanges}
            >
              Save
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => handleSave(true)}
              loading={saving}
              disabled={!hasChanges}
            >
              Save & Back
            </Button>
            <Button variant="ghost" size="sm" onClick={() => navigate('/targets')}>
              Cancel
            </Button>
            {hasChanges && (
              <span className="text-xs text-warning">Unsaved changes</span>
            )}
          </>
        }
      />

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left column — Filter Rules */}
        <Card>
          <h3 className="font-bold text-pencil mb-4">Filter Rules</h3>
          <div className="space-y-4">
            <FilterTagInput
              label="Include patterns"
              patterns={include}
              onChange={setInclude}
              color="blue"
            />
            <FilterTagInput
              label="Exclude patterns"
              patterns={exclude}
              onChange={setExclude}
              color="danger"
            />
          </div>
          <p className="text-xs text-pencil-light mt-3">
            Use glob patterns (e.g. <code className="font-mono bg-muted/10 px-1">frontend*</code>, <code className="font-mono bg-muted/10 px-1">_team__*</code>). Press Enter to add.
          </p>
        </Card>

        {/* Right column — Live Preview */}
        <Card>
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-bold text-pencil">Live Preview</h3>
            {previewLoading && <Spinner size="sm" />}
          </div>

          {preview.length === 0 && !previewLoading ? (
            <EmptyState
              icon={PackageOpen}
              title="No skills to preview"
              description="Add some skills to your source first."
            />
          ) : (
            <>
              {/* Search filter */}
              <div className="relative mb-3">
                <Search size={14} strokeWidth={2.5} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-pencil-light" />
                <input
                  type="text"
                  value={previewSearch}
                  onChange={(e) => setPreviewSearch(e.target.value)}
                  placeholder="Filter skills..."
                  className="w-full pl-8 pr-3 py-1.5 text-sm text-pencil bg-surface border-2 border-muted font-mono placeholder:text-muted-dark focus:border-pencil focus:outline-none"
                  style={{ borderRadius: radius.sm }}
                />
              </div>

              <div
                className="border-2 border-dashed border-pencil-light/30"
                style={{ borderRadius: radius.md }}
              >
                {filteredPreview.length === 0 && previewSearch ? (
                  <p className="text-sm text-pencil-light text-center py-6">
                    No skills matching "{previewSearch}"
                  </p>
                ) : (
                  <Virtuoso
                    style={{ height: '28rem' }}
                    totalCount={filteredPreview.length}
                    overscan={200}
                    itemContent={(index) => (
                      <PreviewRow
                        entry={filteredPreview[index]}
                        onClick={() => handleToggleSkill(filteredPreview[index])}
                      />
                    )}
                  />
                )}
              </div>

              <p className="text-sm text-pencil-light mt-3 text-center">
                <span className="font-bold text-success">{syncedCount}</span>
                /{totalCount} skills will sync
                {previewSearch && ` · showing ${filteredPreview.length}`}
              </p>
            </>
          )}
        </Card>
      </div>
    </div>
  );
}

/** Single preview row with status indicator and click-to-toggle */
const PreviewRow = memo(function PreviewRow({
  entry,
  onClick,
}: {
  entry: SyncMatrixEntry;
  onClick: () => void;
}) {
  const isMismatch = entry.status === 'skill_target_mismatch';
  const clickable = !isMismatch;

  return (
    <div
      role={clickable ? 'button' : undefined}
      tabIndex={clickable ? 0 : undefined}
      onClick={clickable ? onClick : undefined}
      onKeyDown={clickable ? (e) => { if (e.key === 'Enter') onClick(); } : undefined}
      className={`
        flex items-center gap-2 px-3 py-2 border-b border-dashed border-pencil-light/30 text-sm
        ${clickable ? 'cursor-pointer hover:bg-muted/20 transition-all duration-150' : 'cursor-default'}
      `}
      title={
        isMismatch
          ? `This skill declares specific targets: ${entry.reason}`
          : entry.status === 'synced'
            ? 'Click to exclude this skill'
            : 'Click to include this skill'
      }
    >
      <StatusIcon status={entry.status} />
      <span className="font-mono text-pencil flex-1 min-w-0 truncate">
        {entry.skill}
      </span>
      {entry.status === 'excluded' && entry.reason && (
        <span className="text-xs text-pencil-light shrink-0">({entry.reason})</span>
      )}
      {isMismatch && (
        <span className="flex items-center gap-1 text-xs text-pencil-light shrink-0">
          <Info size={12} strokeWidth={2.5} />
          {entry.reason}
        </span>
      )}
    </div>
  );
});

function StatusIcon({ status }: { status: SyncMatrixEntry['status'] }) {
  switch (status) {
    case 'synced':
      return <Check size={14} strokeWidth={3} className="text-success shrink-0" />;
    case 'excluded':
      return <X size={14} strokeWidth={3} className="text-danger shrink-0" />;
    case 'not_included':
      return <X size={14} strokeWidth={3} className="text-warning shrink-0" />;
    case 'skill_target_mismatch':
      return <Info size={14} strokeWidth={2.5} className="text-pencil-light shrink-0" />;
    default:
      return <span className="w-3.5 shrink-0" />;
  }
}
