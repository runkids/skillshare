import { useState, useEffect, useCallback, useRef, useMemo, memo } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
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
import KindBadge from '../components/KindBadge';
import { radius } from '../design';
import { formatPreviewResourceName } from '../lib/resourceNames';
import { syncMatrixReasonText } from '../lib/syncMatrixText';
import { useT } from '../i18n';

type FilterKind = 'skill' | 'agent';

export default function FilterStudioPage() {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const t = useT();

  const kind: FilterKind = searchParams.get('kind') === 'agent' ? 'agent' : 'skill';
  const kindLabel = kind === 'agent' ? 'agents' : 'skills';

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

  // Draft filter state for active kind
  const [include, setInclude] = useState<string[]>([]);
  const [exclude, setExclude] = useState<string[]>([]);
  const [initialized, setInitialized] = useState(false);

  // Initialize draft from target config once loaded
  useEffect(() => {
    if (target && !initialized) {
      if (kind === 'agent') {
        setInclude(target.agentInclude ?? []);
        setExclude(target.agentExclude ?? []);
      } else {
        setInclude(target.include ?? []);
        setExclude(target.exclude ?? []);
      }
      setInitialized(true);
    }
  }, [target, initialized, kind]);

  // Debounced preview
  const [preview, setPreview] = useState<SyncMatrixEntry[]>([]);
  const [previewLoading, setPreviewLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const fetchPreview = useCallback(
    async (inc: string[], exc: string[]) => {
      if (!name) return;
      setPreviewLoading(true);
      try {
        const skillInc = kind === 'skill' ? inc : [];
        const skillExc = kind === 'skill' ? exc : [];
        const agentInc = kind === 'agent' ? inc : [];
        const agentExc = kind === 'agent' ? exc : [];
        const res = await api.previewSyncMatrix(name, skillInc, skillExc, agentInc, agentExc);
        setPreview(res.entries);
      } catch {
        // silently ignore preview errors
      } finally {
        setPreviewLoading(false);
      }
    },
    [name, kind],
  );

  // Trigger debounced preview on filter change
  useEffect(() => {
    if (!initialized) return;
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => fetchPreview(include, exclude), 500);
    return () => clearTimeout(debounceRef.current);
  }, [include, exclude, initialized, fetchPreview]);

  // Filter preview entries to only show the active kind
  const kindPreview = useMemo(() => {
    if (kind === 'agent') return preview.filter((e) => e.kind === 'agent');
    return preview.filter((e) => e.kind !== 'agent');
  }, [preview, kind]);

  // Unsaved changes detection
  const hasChanges = useMemo(() => {
    if (!target) return false;
    const savedInc = kind === 'agent' ? (target.agentInclude ?? []) : (target.include ?? []);
    const savedExc = kind === 'agent' ? (target.agentExclude ?? []) : (target.exclude ?? []);
    return (
      JSON.stringify(include) !== JSON.stringify(savedInc) ||
      JSON.stringify(exclude) !== JSON.stringify(savedExc)
    );
  }, [target, include, exclude, kind]);

  // Save handler
  const [saving, setSaving] = useState(false);

  const handleSave = async (goBack: boolean) => {
    if (!name) return;
    setSaving(true);
    try {
      const payload = kind === 'agent'
        ? { agent_include: include, agent_exclude: exclude }
        : { include, exclude };
      await api.updateTarget(name, payload);
      toast(t('filterStudio.toast.saved', { kind: kind === 'agent' ? 'Agent' : 'Skill', name }), 'success');
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
  const handleToggle = (entry: SyncMatrixEntry) => {
    if (entry.status === 'skill_target_mismatch') return;
    const item = entry.skill;
    if (entry.status === 'synced') {
      setExclude((prev) => prev.includes(item) ? prev : [...prev, item]);
      setInclude((prev) => prev.filter((p) => p !== item));
    } else {
      setInclude((prev) => prev.includes(item) ? prev : [...prev, item]);
      setExclude((prev) => prev.filter((p) => p !== item));
    }
  };

  // Preview search filter
  const [previewSearch, setPreviewSearch] = useState('');
  const filteredPreview = useMemo(() => {
    if (!previewSearch) return kindPreview;
    const q = previewSearch.toLowerCase();
    return kindPreview.filter((e) => {
      const displayName = formatPreviewResourceName(e.skill, kind);
      return e.skill.toLowerCase().includes(q) || displayName.toLowerCase().includes(q);
    });
  }, [kindPreview, previewSearch, kind]);

  // Summary counts (from kind-filtered preview, not search-filtered)
  const { syncedCount, totalCount } = useMemo(() => ({
    syncedCount: kindPreview.filter((e) => e.status === 'synced').length,
    totalCount: kindPreview.length,
  }), [kindPreview]);

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
          title={t('filterStudio.targetNotFound.title', { name: name ?? '' })}
          description={t('filterStudio.targetNotFound.description')}
          action={
            <Button variant="secondary" size="sm" onClick={() => navigate('/targets')}>
              {t('targets.backToPicker')}
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
        title={t('filterStudio.title')}
        subtitle={
          <span className="inline-flex items-center gap-2">
            <KindBadge kind={kind} />
            <span>{t('filterStudio.routeSubtitle', { kindLabel, name: name ?? '' })}</span>
          </span>
        }
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
              {t('filterStudio.save')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => handleSave(true)}
              loading={saving}
              disabled={!hasChanges}
            >
              {t('filterStudio.saveAndBack')}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => navigate('/targets')}>
              {t('filterStudio.cancel')}
            </Button>
            {hasChanges && (
              <span className="text-xs text-warning">{t('filterStudio.hasChanges')}</span>
            )}
          </>
        }
      />

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left column — Filter Rules */}
        <Card>
          <h3 className="font-bold text-pencil mb-4">
            {kind === 'agent' ? t('filterStudio.agentFilterRules') : t('filterStudio.skillFilterRules')}
          </h3>
          <div className="space-y-4">
            <FilterTagInput
              label={t('filterStudio.includePatterns')}
              patterns={include}
              onChange={setInclude}
              color="blue"
            />
            <FilterTagInput
              label={t('filterStudio.excludePatterns')}
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
            <h3 className="font-bold text-pencil">{t('filterStudio.livePreview')}</h3>
            {previewLoading && <Spinner size="sm" />}
          </div>

          {kindPreview.length === 0 && !previewLoading ? (
            <EmptyState
              icon={PackageOpen}
              title={t('filterStudio.noPreview.title', { kindLabel })}
              description={t('filterStudio.noPreview.description', { kindLabel })}
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
                  placeholder={t('filterStudio.searchPlaceholder', { kindLabel })}
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
                    {t('filterStudio.noSearchMatch', { kindLabel, query: previewSearch })}
                  </p>
                ) : (
                  <Virtuoso
                    style={{ height: '28rem' }}
                    totalCount={filteredPreview.length}
                    overscan={200}
                    itemContent={(index) => (
                      <PreviewRow
                        entry={filteredPreview[index]}
                        kind={kind}
                        onClick={() => handleToggle(filteredPreview[index])}
                      />
                    )}
                  />
                )}
              </div>

              <p className="text-sm text-pencil-light mt-3 text-center">
                {t('filterStudio.syncCount', { synced: syncedCount, total: totalCount, kindLabel })}
                {previewSearch && ` ${t('filterStudio.syncCountSearch', { count: filteredPreview.length })}`}
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
  kind,
  onClick,
}: {
  entry: SyncMatrixEntry;
  kind: FilterKind;
  onClick: () => void;
}) {
  const t = useT();
  const isMismatch = entry.status === 'skill_target_mismatch';
  const clickable = !isMismatch;
  const label = kind === 'agent' ? 'agent' : 'skill';
  const displayName = formatPreviewResourceName(entry.skill, kind);

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
          ? `This ${label} declares specific targets: ${syncMatrixReasonText(entry, t)}`
          : entry.status === 'synced'
            ? `Click to exclude this ${label}`
            : `Click to include this ${label}`
      }
    >
      <StatusIcon status={entry.status} />
      <span className="font-mono text-pencil flex-1 min-w-0 truncate">
        {displayName}
      </span>
      {entry.status === 'excluded' && entry.reason && (
        <span className="text-xs text-pencil-light shrink-0">({syncMatrixReasonText(entry, t)})</span>
      )}
      {isMismatch && (
        <span className="flex items-center gap-1 text-xs text-pencil-light shrink-0">
          <Info size={12} strokeWidth={2.5} />
          {syncMatrixReasonText(entry, t)}
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
