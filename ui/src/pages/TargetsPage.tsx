import { useState, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Trash2, Plus, Target, ArrowDownToLine, Search, CircleDot, PenLine, AlertTriangle, X } from 'lucide-react';
import Card from '../components/Card';
import StatusBadge from '../components/StatusBadge';
import Button from '../components/Button';
import IconButton from '../components/IconButton';
import { Input, Select } from '../components/Input';
import EmptyState from '../components/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import DialogShell from '../components/DialogShell';
import { PageSkeleton } from '../components/Skeleton';
import PageHeader from '../components/PageHeader';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { AvailableTarget, Target as TargetType } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { radius, shadows } from '../design';
import { shortenHome } from '../lib/paths';
import { useSyncMatrix } from '../hooks/useSyncMatrix';
import { useT } from '../i18n';

function getSyncModeOptions(t: (key: string) => string) {
  return [
    { value: 'merge', label: t('targets.syncMode.merge'), description: t('targets.syncMode.mergeDescription') },
    { value: 'symlink', label: t('targets.syncMode.symlink'), description: t('targets.syncMode.symlinkDescription') },
    { value: 'copy', label: t('targets.syncMode.copy'), description: t('targets.syncMode.copyDescription') },
  ];
}

function getTargetNamingOptions(t: (key: string) => string) {
  return [
    { value: 'flat', label: t('targets.targetNaming.flat'), description: t('targets.targetNaming.flatDescription') },
    { value: 'standard', label: t('targets.targetNaming.standard'), description: t('targets.targetNaming.standardDescription') },
  ];
}

function getAgentModeOptions(t: (key: string) => string) {
  return [
    { value: 'merge', label: t('targets.agentMode.merge'), description: t('targets.agentMode.mergeDescription') },
    { value: 'symlink', label: t('targets.agentMode.symlink'), description: t('targets.agentMode.symlinkDescription') },
    { value: 'copy', label: t('targets.agentMode.copy'), description: t('targets.agentMode.copyDescription') },
  ];
}

export type CollectScope = 'skill' | 'agent' | 'both';

type TFunc = (key: string, params?: Record<string, string | number | boolean | null | undefined>, fallback?: string) => string;

function pluralize(t: TFunc, count: number, label: string): string {
  if (count === 1) return t('targets.pluralizeSingular', { count, label });
  return t('targets.pluralize', { count, label });
}

export function getTargetCollectScope(target: TargetType): CollectScope | null {
  const hasLocalSkills = target.localCount > 0;
  const hasLocalAgents = (target.agentLocalCount ?? 0) > 0;

  if (hasLocalSkills && hasLocalAgents) return 'both';
  if (hasLocalAgents) return 'agent';
  if (hasLocalSkills) return 'skill';
  return null;
}

export function targetAgentSummary(t: TFunc, target: TargetType): { text: string; hasDrift: boolean } {
  const expected = target.agentExpectedCount ?? 0;
  const linked = target.agentLinkedCount ?? 0;
  const local = target.agentLocalCount ?? 0;
  const label = target.agentMode === 'copy' ? 'managed' : 'linked';
  const hasDrift = expected > 0 && linked !== expected;

  if (expected === 0) {
    const counts = [
      linked > 0 ? pluralize(t, linked, label) : null,
      local > 0 ? pluralize(t, local, 'local') : null,
    ].filter(Boolean).join(', ');
    if (counts) {
      return { text: t('targets.agentsSummary.noSourceAgentsWithCounts', { counts }), hasDrift: false };
    }
    return { text: t('targets.agentsSummary.noSourceAgents'), hasDrift: false };
  }

  const suffix = target.agentMode === 'symlink' ? t('targets.agentsSummary.dirSymlink') : '';
  const localSuffix = local > 0 ? `, ${pluralize(t, local, 'local')}` : '';
  return { text: `${linked}/${expected} ${label}${suffix}${localSuffix}`, hasDrift };
}

export function targetAgentAvailabilityText(t: TFunc, target: TargetType): string {
  const expected = target.agentExpectedCount ?? 0;
  const local = target.agentLocalCount ?? 0;
  const agentFilters = (target.agentInclude?.length ?? 0) + (target.agentExclude?.length ?? 0);

  if (expected === 0) {
    return local > 0 ? pluralize(t, local, 'local agent') : t('targets.availability.noAgents');
  }

  if (!agentFilters) return t('targets.availability.allAgents', { count: expected });
  const linked = target.agentLinkedCount ?? 0;
  return t('targets.availability.agentsCount', { linked, expected });
}

export default function TargetsPage() {
  const t = useT();
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
  const [newTarget, setNewTarget] = useState({ name: '', path: '', agentPath: '' });
  const [searchQuery, setSearchQuery] = useState('');
  const [customMode, setCustomMode] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);
  const [collecting, setCollecting] = useState<{ name: string; scope: CollectScope } | null>(null);
  const navigate = useNavigate();
  const { getTargetSummary } = useSyncMatrix();
  const { toast } = useToast();

  const SYNC_MODE_OPTIONS = useMemo(() => getSyncModeOptions(t), [t]);
  const TARGET_NAMING_OPTIONS = useMemo(() => getTargetNamingOptions(t), [t]);
  const AGENT_MODE_OPTIONS = useMemo(() => getAgentModeOptions(t), [t]);

  const updateTargetSetting = async (
    targetName: string,
    payload: Parameters<typeof api.updateTarget>[1],
    label: string,
  ) => {
    // Optimistic update: apply change to cache immediately
    const queryKey = queryKeys.targets.all;
    const previous = queryClient.getQueryData<{ targets: TargetType[]; sourceSkillCount: number }>(queryKey);

    if (previous) {
      queryClient.setQueryData(queryKey, {
        ...previous,
        targets: previous.targets.map((t) =>
          t.name === targetName ? {
            ...t,
            ...payload,
            ...(payload.target_naming != null ? { targetNaming: payload.target_naming } : {}),
            ...(payload.agent_mode != null ? { agentMode: payload.agent_mode } : {}),
          } : t,
        ),
      });
    }

    try {
      await api.updateTarget(targetName, payload);
      queryClient.invalidateQueries({ queryKey });
      queryClient.invalidateQueries({ queryKey: queryKeys.config });
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
      toast(`${targetName}: ${label}`, 'success');
    } catch (e) {
      // Rollback on error
      if (previous) queryClient.setQueryData(queryKey, previous);
      toast((e as Error).message, 'error');
    }
  };

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
          {t('targets.error.failedToLoad')}
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
      const agentPath = newTarget.agentPath || avail?.agentPath || '';
      await api.addTarget(newTarget.name, path, agentPath || undefined);
      setAdding(false);
      setNewTarget({ name: '', path: '', agentPath: '' });
      setSearchQuery('');
      setCustomMode(false);
      toast(t('targets.targetAdded', { name: newTarget.name }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.available });
      queryClient.invalidateQueries({ queryKey: queryKeys.config });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    }
  };

  const handleRemove = async (name: string) => {
    try {
      await api.removeTarget(name);
      toast(t('targets.targetRemoved', { name }), 'success');
      setRemoving(null);
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.config });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.diff() });
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
        title={t('targets.title')}
        subtitle={targets.length !== 1 ? t('targets.subtitlePlural', { count: targets.length }) : t('targets.subtitle', { count: targets.length })}
        actions={
          <Button
            onClick={() => setAdding(true)}
            variant="primary"
            size="sm"
          >
            <Plus size={16} strokeWidth={2.5} />
            {t('targets.addTarget')}
          </Button>
        }
      />

      {/* Add target modal */}
      <DialogShell
        open={adding}
        onClose={() => {
          setAdding(false);
          setNewTarget({ name: '', path: '', agentPath: '' });
          setSearchQuery('');
          setCustomMode(false);
        }}
        maxWidth="xl"
        padding="none"
      >
        <div className="p-5 pb-0 flex items-center justify-between">
          <h3 className="font-bold text-pencil text-lg">{t('targets.addNewTarget')}</h3>
          <button
            onClick={() => {
              setAdding(false);
              setNewTarget({ name: '', path: '', agentPath: '' });
              setSearchQuery('');
              setCustomMode(false);
            }}
            className="w-8 h-8 flex items-center justify-center text-pencil-light hover:text-pencil transition-colors cursor-pointer"
            aria-label={t('targets.close')}
          >
            <X size={18} strokeWidth={2.5} />
          </button>
        </div>

        {/* Selected target preview + path + actions */}
        {newTarget.name && !customMode ? (
          <div className="p-5 space-y-4 animate-fade-in">
            <div
              className="flex items-center gap-3 bg-surface border-2 border-blue px-4 py-3"
              style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
            >
              <Target size={18} strokeWidth={2.5} className="text-blue shrink-0" />
              <div className="min-w-0 flex-1">
                <p className="font-bold text-pencil">
                  {newTarget.name}
                </p>
                {newTarget.agentPath ? (
                  <>
                    <p className="font-mono text-sm text-pencil-light truncate">
                      <span className="text-[10px] font-bold uppercase tracking-wider text-pencil-light/70 mr-1.5">{t('targets.skillsLabel')}</span>
                      {shortenHome(newTarget.path)}
                    </p>
                    <p className="font-mono text-sm text-pencil-light truncate">
                      <span className="text-[10px] font-bold uppercase tracking-wider text-pencil-light/70 mr-1.5">{t('targets.agentsLabel')}</span>
                      {shortenHome(newTarget.agentPath)}
                    </p>
                  </>
                ) : (
                  <p className="font-mono text-sm text-pencil-light truncate">
                    {shortenHome(newTarget.path)}
                  </p>
                )}
              </div>
            </div>

            <Input
              label={newTarget.agentPath ? t('targets.skillsPathLabel') : t('targets.pathLabel')}
              type="text"
              value={newTarget.path}
              onChange={(e) => setNewTarget({ ...newTarget, path: e.target.value })}
              placeholder={t('targets.skillsPathPlaceholder')}
            />
            {newTarget.agentPath && (
              <Input
                label={t('targets.agentPathLabel')}
                type="text"
                value={newTarget.agentPath}
                onChange={(e) => setNewTarget({ ...newTarget, agentPath: e.target.value })}
                placeholder={t('targets.agentsPathPlaceholder')}
              />
            )}

            <div className="flex items-center justify-between pt-2">
              <Button
                onClick={() => setNewTarget({ name: '', path: '', agentPath: '' })}
                variant="ghost"
                size="sm"
              >
                {t('targets.change')}
              </Button>
              <Button onClick={handleAdd} variant="primary" size="sm">
                <Plus size={16} strokeWidth={2.5} />
                {t('targets.addTarget')}
              </Button>
            </div>
          </div>
        ) : customMode ? (
          /* Custom target entry mode */
          <div className="p-5 space-y-4 animate-fade-in">
            <Input
              label={t('targets.customTargetName')}
              type="text"
              value={newTarget.name}
              onChange={(e) => setNewTarget({ ...newTarget, name: e.target.value })}
              placeholder={t('targets.customTargetNamePlaceholder')}
              autoFocus
            />
            <Input
              label={t('targets.customSkillsPath')}
              type="text"
              value={newTarget.path}
              onChange={(e) => setNewTarget({ ...newTarget, path: e.target.value })}
              placeholder={t('targets.skillsPathPlaceholder')}
            />
            <Input
              label={t('targets.customAgentsPath')}
              type="text"
              value={newTarget.agentPath}
              onChange={(e) => setNewTarget({ ...newTarget, agentPath: e.target.value })}
              placeholder={t('targets.agentsPathPlaceholder')}
            />
            <div className="flex items-center justify-between pt-2">
              <Button
                onClick={() => {
                  setCustomMode(false);
                  setNewTarget({ name: '', path: '', agentPath: '' });
                }}
                variant="ghost"
                size="sm"
              >
                {t('targets.backToPicker')}
              </Button>
              <Button onClick={handleAdd} variant="primary" size="sm">
                <Plus size={16} strokeWidth={2.5} />
                {t('targets.addTarget')}
              </Button>
            </div>
          </div>
        ) : (
          /* Target picker mode */
          <div className="p-5 pt-4 space-y-3">
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
                placeholder={t('targets.searchPlaceholder')}
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
              className="max-h-[60vh] overflow-y-auto border-2 border-dashed border-muted-dark bg-surface"
              style={{ borderRadius: radius.md }}
            >
              {/* Detected section */}
              {detected.length > 0 && (
                <div>
                  <div className="px-3 py-2 border-b border-dashed border-muted-dark sticky top-0 z-10 bg-surface relative">
                    <div className="absolute inset-0 bg-success-light pointer-events-none" />
                    <span className="relative text-sm font-bold text-success flex items-center gap-1.5">
                      <CircleDot size={14} strokeWidth={3} />
                      {t('targets.detectedOnSystem')}
                    </span>
                  </div>
                  {detected.map((t) => (
                    <TargetPickerItem
                      key={t.name}
                      target={t}
                      isDetected
                      onSelect={(target) => {
                        setNewTarget({ name: target.name, path: target.path, agentPath: target.agentPath || '' });
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
                      {t('targets.allAvailableTargets')}
                    </span>
                  </div>
                  {others.map((t) => (
                    <TargetPickerItem
                      key={t.name}
                      target={t}
                      onSelect={(target) => {
                        setNewTarget({ name: target.name, path: target.path, agentPath: target.agentPath || '' });
                        setSearchQuery('');
                      }}
                    />
                  ))}
                </div>
              )}

              {/* No results */}
              {detected.length === 0 && others.length === 0 && (
                <div className="px-4 py-8 text-center text-pencil-light">
                  {searchQuery ? t('targets.noTargetsMatching', { query: searchQuery }) : t('targets.noAvailableTargets')}
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
                {t('targets.enterCustomTarget')}
              </Button>
            </div>
          </div>
        )}
      </DialogShell>

      {/* Targets list */}
      {targets.length > 0 ? (
        <div data-tour="targets-grid" className="space-y-4">
          {targets.map((target, i) => {
            const expectedCount = target.expectedSkillCount || sourceSkillCount;
            const isMergeOrCopy = target.mode === 'merge' && target.status === 'merged' || target.mode === 'copy' && target.status === 'copied';
            const hasDrift = isMergeOrCopy && target.linkedCount < expectedCount;
            const agentSummary = targetAgentSummary(t, target);
            const agentFilters = (target.agentInclude?.length ?? 0) + (target.agentExclude?.length ?? 0);
            const collectScope = getTargetCollectScope(target);
            const visibleAgentInclude = (target.agentInclude ?? []).slice(0, 3);
            const visibleAgentExclude = (target.agentExclude ?? []).slice(0, Math.max(0, 3 - visibleAgentInclude.length));
            const overflowAgentFilters = agentFilters - (visibleAgentInclude.length + visibleAgentExclude.length);
            return (
              <Card
                key={target.name}
                className={`!overflow-visible ${i % 2 === 0 ? 'rotate-[-0.15deg]' : 'rotate-[0.15deg]'}`}
                style={{ position: 'relative', zIndex: targets.length - i }}
              >
                {/* Top row: name + action icons */}
                <div className="flex items-center justify-between gap-4">
                  <div className="flex items-center gap-2 min-w-0">
                    <Target size={16} strokeWidth={2.5} className="text-success shrink-0" />
                    <span className="font-bold text-pencil">{target.name}</span>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    {collectScope && (
                      <IconButton
                        icon={<ArrowDownToLine size={16} strokeWidth={2.5} />}
                        label={collectScope === 'both' ? t('targets.collectLabel.both') : collectScope === 'agent' ? t('targets.collectLabel.agent') : t('targets.collectLabel.skill')}
                        size="md"
                        variant="outline"
                        onClick={() => setCollecting({ name: target.name, scope: collectScope })}
                        className="hover:text-blue hover:border-blue"
                      />
                    )}
                    <IconButton
                      icon={<Trash2 size={16} strokeWidth={2.5} />}
                      label={t('targets.removeTarget')}
                      size="md"
                      variant="danger-outline"
                      onClick={() => setRemoving(target.name)}
                    />
                  </div>
                </div>
                {/* ── Skills Section ── */}
                <div
                  className="mt-4 bg-muted/15 border border-pencil-light/15 px-4 py-3"
                  style={{ borderRadius: radius.md }}
                >
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-xs font-bold uppercase tracking-[0.12em] text-pencil shrink-0">
                      {t('targets.skillsLabel')}
                    </span>
                    <StatusBadge status={target.status} />
                  </div>
                  <p className="font-mono text-sm text-pencil-light truncate mb-2">
                    {shortenHome(target.path)}
                  </p>
                  <div className="flex items-center gap-2">
                    <Select
                      value={target.mode || 'merge'}
                      onChange={(mode) => updateTargetSetting(target.name, { mode }, t('targets.syncModeChanged', { mode }))}
                      options={SYNC_MODE_OPTIONS}
                      size="sm"
                      className="w-44"
                    />
                    {target.mode !== 'symlink' && (
                      <Select
                        value={target.targetNaming || 'flat'}
                        onChange={(naming) => updateTargetSetting(target.name, { target_naming: naming }, t('targets.targetNamingChanged', { naming }))}
                        options={TARGET_NAMING_OPTIONS}
                        size="sm"
                        className="w-48"
                      />
                    )}
                    {(target.mode === 'merge' || target.mode === 'copy') && (
                      <span className={`text-sm ml-auto shrink-0 ${hasDrift ? 'text-warning' : 'text-muted-dark'}`}>
                        {hasDrift ? (
                          <span className="flex items-center gap-1">
                            <AlertTriangle size={12} strokeWidth={2.5} />
                            {target.mode === 'copy'
                              ? t('targets.skillsSummary.managedLocal', { linked: target.linkedCount, expected: expectedCount, local: target.localCount })
                              : t('targets.skillsSummary.sharedLocal', { linked: target.linkedCount, expected: expectedCount, local: target.localCount })}
                          </span>
                        ) : (
                          <>{target.mode === 'copy'
                              ? `${t('targets.skillsSummary.managed', { count: target.linkedCount })}, ${t('targets.skillsSummary.local', { count: target.localCount })}`
                              : `${t('targets.skillsSummary.shared', { count: target.linkedCount })}, ${t('targets.skillsSummary.local', { count: target.localCount })}`}</>
                        )}
                      </span>
                    )}
                  </div>
                  {(target.mode === 'merge' || target.mode === 'copy') && (
                    <div {...(i === 0 ? { 'data-tour': 'skill-filters' } : {})} className="mt-2 flex items-center gap-2 flex-wrap">
                      <span className="text-sm text-pencil-light">
                        {(() => {
                          const summary = getTargetSummary(target.name);
                          const hasFilters = target.include?.length || target.exclude?.length;
                          if (summary.total === 0) return t('targets.skillsNone');
                          if (!hasFilters) return t('targets.skillsAll', { count: summary.total });
                          return t('targets.skillsCount', { synced: summary.synced, total: summary.total });
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
                              <span className="text-xs text-pencil-light">{t('targets.filterCount', { count: overflow })}</span>
                            )}
                          </>
                        );
                      })()}
                      <Link
                        to={`/targets/${encodeURIComponent(target.name)}/filters?kind=skill`}
                        className="text-xs font-bold text-blue hover:underline"
                      >
                        {(target.include?.length || target.exclude?.length) ? t('targets.editInFilterStudio') : t('targets.customizeFilters')}
                      </Link>
                    </div>
                  )}
                </div>

                {/* ── Agents Section ── */}
                {target.agentPath && (
                  <div
                    className="mt-3 bg-muted/15 border border-pencil-light/15 px-4 py-3"
                    style={{ borderRadius: radius.md }}
                  >
                    <div className="flex items-center gap-2 mb-2">
                      <span className="text-xs font-bold uppercase tracking-[0.12em] text-pencil shrink-0">
                        {t('targets.agentsLabel')}
                      </span>
                      {(() => {
                        const agentExpected = target.agentExpectedCount ?? 0;
                        const agentLinked = target.agentLinkedCount ?? 0;
                        const mode = target.agentMode || 'merge';
                        if (agentLinked > 0 || agentExpected > 0) {
                          const label = mode === 'merge' ? 'merged' : mode === 'copy' ? 'copied' : 'linked';
                          return <StatusBadge status={label} />;
                        }
                        return null;
                      })()}
                    </div>
                    <p className="font-mono text-sm text-pencil-light truncate mb-2">
                      {shortenHome(target.agentPath)}
                    </p>
                    <div className="flex items-center gap-2">
                      <Select
                        value={target.agentMode || 'merge'}
                        onChange={(mode) => updateTargetSetting(target.name, { agent_mode: mode }, t('targets.agentModeChanged', { mode }))}
                        options={AGENT_MODE_OPTIONS}
                        size="sm"
                        className="w-44"
                      />
                      <span className={`text-sm ml-auto shrink-0 ${agentSummary.hasDrift ? 'text-warning' : 'text-muted-dark'}`}>
                        {agentSummary.hasDrift ? (
                          <span className="flex items-center gap-1">
                            <AlertTriangle size={12} strokeWidth={2.5} />
                            {agentSummary.text}
                          </span>
                        ) : (
                          agentSummary.text
                        )}
                      </span>
                    </div>
                    <div className="mt-2 flex items-center gap-2 flex-wrap">
                      <span className="text-sm text-pencil-light">{targetAgentAvailabilityText(t, target)}</span>
                      {visibleAgentInclude.map((pattern, idx) => (
                        <span
                          key={`agent-inc-${idx}`}
                          className="text-xs font-bold text-blue bg-info-light px-2 py-0.5 border border-blue/30"
                          style={{ borderRadius: radius.sm }}
                        >
                          + {pattern}
                        </span>
                      ))}
                      {visibleAgentExclude.map((pattern, idx) => (
                        <span
                          key={`agent-exc-${idx}`}
                          className="text-xs font-bold text-danger bg-danger-light px-2 py-0.5 border border-danger/30"
                          style={{ borderRadius: radius.sm }}
                        >
                          − {pattern}
                        </span>
                      ))}
                      {overflowAgentFilters > 0 && (
                        <span className="text-xs text-pencil-light">{t('targets.filterCount', { count: overflowAgentFilters })}</span>
                      )}
                      <Link
                        to={`/targets/${encodeURIComponent(target.name)}/filters?kind=agent`}
                        className="text-xs font-bold text-blue hover:underline"
                      >
                        {agentFilters ? t('targets.editInFilterStudio') : t('targets.customizeFilters')}
                      </Link>
                    </div>
                  </div>
                )}
                {(target.skippedSkillCount ?? 0) > 0 && (
                  <p className="mt-1 text-xs text-warning flex items-center gap-1">
                    <AlertTriangle size={11} strokeWidth={2.5} />
                    {t('targets.skippedSkills', { count: target.skippedSkillCount })}
                    {(target.collisionCount ?? 0) > 0 && <> ({t('targets.skippedSkillsCollisions', { count: target.collisionCount })})</>}
                    {' '}{t('targets.skippedSkillsResolution.prefix')} <strong>{t('targets.skippedSkillsResolution.naming')}</strong> {t('targets.skippedSkillsResolution.suffix')}
                  </p>
                )}
              </Card>
            );
          })}
        </div>
      ) : (
        <EmptyState
          icon={Target}
          title={t('targets.emptyTitle')}
          description={t('targets.emptyDescription')}
          action={
            !adding ? (
              <Button onClick={() => setAdding(true)} variant="secondary" size="sm">
                <Plus size={16} strokeWidth={2.5} />
                {t('targets.addYourFirstTarget')}
              </Button>
            ) : undefined
          }
        />
      )}

      {/* Confirm remove dialog */}
      <ConfirmDialog
        open={!!removing}
        title={t('targets.removeConfirm.title')}
        message={t('targets.removeConfirm.message', { name: removing ?? '' })}
        confirmText={t('targets.removeConfirm.remove')}
        variant="danger"
        onConfirm={() => removing && handleRemove(removing)}
        onCancel={() => setRemoving(null)}
      />

      {/* Confirm collect dialog */}
      <ConfirmDialog
        open={!!collecting}
        title={t('targets.collectConfirm.title', { scope: collecting?.scope === 'both' ? t('targets.scopeBoth') : collecting?.scope === 'agent' ? t('targets.scopeAgent') : t('targets.scopeSkill') })}
        message={collecting
          ? t('targets.collectConfirm.message', { name: collecting.name, scope: collecting.scope === 'both' ? t('targets.scopeBoth') : collecting.scope === 'agent' ? t('targets.scopeAgent') : t('targets.scopeSkill') })
          : ''}
        confirmText={t('targets.collectConfirm.scan')}
        onConfirm={() => {
          if (collecting) {
            const params = new URLSearchParams({
              target: collecting.name,
              scope: collecting.scope,
            });
            navigate(`/collect?${params.toString()}`);
          }
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
  const t = useT();
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
        {target.agentPath ? (
          <div className="mt-0.5 space-y-0.5">
            <p className="font-mono text-xs text-pencil-light truncate">
              <span className="text-[10px] font-bold uppercase tracking-wider text-pencil-light/70 mr-1.5">{t('targets.skillsLabel')}</span>
              {shortenHome(target.path)}
            </p>
            <p className="font-mono text-xs text-pencil-light truncate">
              <span className="text-[10px] font-bold uppercase tracking-wider text-pencil-light/70 mr-1.5">{t('targets.agentsLabel')}</span>
              {shortenHome(target.agentPath)}
            </p>
          </div>
        ) : (
          <p className="font-mono text-xs text-pencil-light truncate mt-0.5">
            {shortenHome(target.path)}
          </p>
        )}
      </div>
      {isDetected && (
        <span
          className="text-xs text-success bg-success-light px-2 py-0.5 shrink-0"
          style={{ borderRadius: radius.sm }}
        >
          {t('targets.detected')}
        </span>
      )}
    </button>
  );
}
