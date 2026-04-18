import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  RefreshCw,
  Eye,
  EyeOff,
  Zap,
  ChevronDown,
  ChevronRight,
  CheckCircle,
  AlertCircle,
  Folder,
  ArrowRight,
  Target,
  FileText,
  Info,
} from 'lucide-react';
import { Virtuoso } from 'react-virtuoso';
import Card from '../components/Card';
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import SplitButton from '../components/SplitButton';
import Spinner from '../components/Spinner';
import { useToast } from '../components/Toast';
import { api, type SyncResult, type DiffTarget, type IgnoreSources } from '../api/client';
import { formatSyncToast, invalidateAfterSync } from '../lib/sync';
import StreamProgressBar from '../components/StreamProgressBar';
import SyncResultList from '../components/SyncResultList';
import { radius, shadows } from '../design';
import KindBadge from '../components/KindBadge';
import SegmentedControl from '../components/SegmentedControl';
import { useT } from '../i18n';

function extractIgnoreSources(data: IgnoreSources): IgnoreSources {
  return {
    ignored_count: data.ignored_count,
    ignored_skills: data.ignored_skills ?? [],
    ignore_root: data.ignore_root ?? '',
    ignore_repos: data.ignore_repos ?? [],
    agent_ignore_root: data.agent_ignore_root ?? '',
    agent_ignored_count: data.agent_ignored_count ?? 0,
    agent_ignored_skills: data.agent_ignored_skills ?? [],
  };
}

export default function SyncPage() {
  const t = useT();
  const queryClient = useQueryClient();
  const [syncing, setSyncing] = useState(false);
  const [results, setResults] = useState<SyncResult[] | null>(null);
  const [syncWarnings, setSyncWarnings] = useState<string[]>([]);
  const [lastDryRun, setLastDryRun] = useState(false);
  const [ignoreSources, setIgnoreSources] = useState<IgnoreSources | null>(null);
  const [ignoredExpanded, setIgnoredExpanded] = useState(false);
  const { toast } = useToast();
  const [syncScope, setSyncScope] = useState<'skill' | 'agent' | 'both'>('both');
  const toastRef = useRef(toast);
  useEffect(() => { toastRef.current = toast; });

  // Diff state (SSE-based)
  const [diffData, setDiffData] = useState<DiffTarget[] | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffProgress, setDiffProgress] = useState<{ checked: number; total: number } | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);

  useEffect(() => {
    return () => { esRef.current?.close(); };
  }, []);

  const runDiff = useCallback(() => {
    esRef.current?.close();
    setDiffLoading(true);
    setDiffProgress(null);
    setIgnoreSources(null);
    startTimeRef.current = Date.now();

    esRef.current = api.diffStream(
      () => setDiffProgress({ checked: 0, total: 0 }),
      (total) => setDiffProgress({ checked: 0, total }),
      (_diff, checked) => setDiffProgress((p) => p ? { ...p, checked } : null),
      (data) => {
        setDiffData(data.diffs);
        setIgnoreSources(extractIgnoreSources(data));
        setDiffLoading(false);
        setDiffProgress(null);
      },
      (err) => {
        toastRef.current(err.message, 'error');
        setDiffLoading(false);
        setDiffProgress(null);
      },
    );
  }, []);

  useEffect(() => { runDiff(); }, [runDiff]);

  const handleSync = async (opts: { dryRun?: boolean; force?: boolean } = {}) => {
    const dryRun = opts.dryRun ?? false;
    const force = opts.force ?? false;
    setSyncing(true);
    setLastDryRun(dryRun);
    setSyncWarnings([]);
    try {
      const res = await api.sync({
        dryRun,
        force,
        ...(syncScope !== 'both' ? { kind: syncScope } : {}),
      });
      setResults(res.results);
      setSyncWarnings(res.warnings ?? []);
      setIgnoreSources(extractIgnoreSources(res));
      if (dryRun) {
        toast(t('sync.toast.dryRunComplete'), 'info');
      } else {
        toast(formatSyncToast(res.results), 'success');
      }
      runDiff();
      invalidateAfterSync(queryClient);
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSyncing(false);
    }
  };

  // Derived ignored skills/agents list
  const ignoredSkills = ignoreSources?.ignored_skills ?? [];
  const ignoredAgents = ignoreSources?.agent_ignored_skills ?? [];
  const allIgnored = [...ignoredSkills, ...ignoredAgents];

  // Calculate diff summary by kind (single pass)
  const diffs = diffData ?? [];
  const counts = useMemo(() => {
    const c = { skill: { link: 0, update: 0, prune: 0, skip: 0, local: 0 }, agent: { link: 0, update: 0, prune: 0, skip: 0, local: 0 } };
    for (const d of diffs) {
      for (const i of d.items ?? []) {
        const kind = (i.kind ?? 'skill') as 'skill' | 'agent';
        const action = i.action as keyof typeof c.skill;
        if (c[kind] && action in c[kind]) c[kind][action]++;
      }
    }
    return c;
  }, [diffs]);

  const skillSync = counts.skill.link + counts.skill.update + counts.skill.prune + counts.skill.skip;
  const agentSync = counts.agent.link + counts.agent.update + counts.agent.prune + counts.agent.skip;
  const pendingLocal = counts.skill.local + counts.agent.local;
  const syncActions = skillSync + agentSync;
  const totalLink = counts.skill.link + counts.agent.link;
  const totalUpdate = counts.skill.update + counts.agent.update;
  const totalSkip = counts.skill.skip + counts.agent.skip;
  const totalPrune = counts.skill.prune + counts.agent.prune;

  const statParts = [
    totalLink > 0 && { n: totalLink, label: t('sync.stat.toLink'), cls: 'text-success' },
    totalUpdate > 0 && { n: totalUpdate, label: t('sync.stat.toUpdate'), cls: 'text-info' },
    totalSkip > 0 && { n: totalSkip, label: t('sync.stat.skipped'), cls: 'text-warning' },
    totalPrune > 0 && { n: totalPrune, label: t('sync.stat.toPrune'), cls: 'text-danger' },
    pendingLocal > 0 && { n: pendingLocal, label: t('sync.stat.localOnly'), cls: 'text-pencil-light' },
    allIgnored.length > 0 && { n: allIgnored.length, label: t('sync.stat.ignored'), cls: 'text-muted-dark' },
  ].filter((x): x is { n: number; label: string; cls: string } => !!x);

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader icon={<RefreshCw size={24} strokeWidth={2.5} />} title={t('sync.title')} subtitle={t('sync.subtitle')} />

      {/* Visual Pipeline */}
      <div className="hidden md:flex items-center justify-center gap-4">
        <div
          className="flex items-center gap-2 px-4 py-2 bg-paper border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <Folder size={18} strokeWidth={2.5} className="text-warning" />
          <span className="text-base font-medium">
            {t('sync.pipeline.source')}
          </span>
        </div>

        <div className="flex items-center gap-1">
          <svg width="60" height="20" viewBox="0 0 60 20" className="text-pencil-light">
            <path
              d="M0 10 Q15 4 30 10 Q45 16 60 10"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeDasharray="4 4"
              className={syncing ? 'animate-flow' : ''}
            />
          </svg>
        </div>

        <div
          className="flex items-center gap-2 px-4 py-2 bg-info-light border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          {syncing ? (
            <Spinner size="sm" className="text-blue" />
          ) : (
            <RefreshCw size={18} strokeWidth={2.5} className="text-blue" />
          )}
          <span className="text-base font-medium">
            {t('sync.pipeline.syncEngine')}
          </span>
        </div>

        <div className="flex items-center gap-1">
          <svg width="60" height="20" viewBox="0 0 60 20" className="text-pencil-light">
            <path
              d="M0 10 Q15 4 30 10 Q45 16 60 10"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeDasharray="4 4"
              className={syncing ? 'animate-flow' : ''}
            />
          </svg>
        </div>

        <div
          className="flex items-center gap-2 px-4 py-2 bg-success-light border-2 border-pencil"
          style={{ borderRadius: radius.sm, boxShadow: shadows.sm }}
        >
          <Target size={18} strokeWidth={2.5} className="text-success" />
          <span className="text-base font-medium">
            {t('sync.pipeline.targets', { count: diffs.length })}
          </span>
        </div>
      </div>

      {/* Sync control area */}
      <Card className="text-center">
        <div data-tour="sync-actions" className="flex flex-col items-center gap-4">
          {/* Status indicator */}
          {diffLoading ? (
            <p className="text-pencil-light text-base">{t('sync.status.checking')}</p>
          ) : syncActions > 0 ? (
            <div className="space-y-1 text-center">
              <p className="text-sm">
                {statParts.map((p, i) => (
                  <span key={i}>
                    {i > 0 && <span className="text-muted-dark mx-1.5">·</span>}
                    <strong className={p.cls}>{p.n}</strong>{' '}
                    <span className="text-pencil-light">{p.label}</span>
                  </span>
                ))}
              </p>
              <p className="text-xs text-pencil-light/60">
                {[
                  skillSync > 0 && `${skillSync} skill${skillSync !== 1 ? 's' : ''}`,
                  agentSync > 0 && `${agentSync} agent${agentSync !== 1 ? 's' : ''}`,
                ].filter(Boolean).join(' · ')}
              </p>
            </div>
          ) : (
            <div className="flex flex-wrap items-center justify-center gap-2 text-sm">
              <CheckCircle size={16} strokeWidth={2.5} className="text-success" />
              <span className="font-medium text-success">{t('sync.status.allInSync')}</span>
              {pendingLocal > 0 && <><span className="text-muted-dark">·</span><span className="text-pencil-light">{t('sync.status.localOnlyCount', { count: pendingLocal })}</span></>}
              {allIgnored.length > 0 && <><span className="text-muted-dark">·</span><span className="text-pencil-light">{t('sync.badge.ignored', { count: allIgnored.length })}</span></>}
            </div>
          )}

          {/* Action bar */}
          <div className="flex flex-wrap items-center justify-center gap-3">
            <SegmentedControl
              value={syncScope}
              onChange={setSyncScope}
              options={[
                { value: 'skill' as const, label: t('sync.scope.skills') },
                { value: 'agent' as const, label: t('sync.scope.agents') },
                { value: 'both' as const, label: t('sync.scope.both') },
              ]}
              size="sm"
              connected
            />
            <SplitButton
              onClick={() => handleSync()}
              loading={syncing}
              variant="primary"
              size="sm"
              dropdownAlign="right"
              items={[
                {
                  label: syncScope === 'agent' ? t('sync.button.forceSyncAgents') : syncScope === 'skill' ? t('sync.button.forceSyncSkills') : t('sync.button.forceSync'),
                  icon: <Zap size={16} strokeWidth={2.5} />,
                  onClick: () => handleSync({ force: true }),
                  confirm: true,
                },
                {
                  label: t('sync.button.dryRun'),
                  icon: <Eye size={16} strokeWidth={2.5} />,
                  onClick: () => handleSync({ dryRun: true }),
                },
              ]}
            >
              {!syncing && <RefreshCw size={18} strokeWidth={2.5} />}
              {syncing
                ? t('sync.button.syncing')
                : syncScope === 'skill'
                  ? t('sync.button.syncSkills')
                  : syncScope === 'agent'
                    ? t('sync.button.syncAgents')
                    : t('sync.button.syncNow')}
            </SplitButton>
          </div>
        </div>
      </Card>

      {/* Sync warnings */}
      {syncWarnings.length > 0 && (
        <Card className="animate-fade-in">
          <div className="flex items-start gap-2 text-sm text-pencil">
            <AlertCircle size={16} className="mt-0.5 shrink-0 text-warning" />
            <div className="space-y-1">
              {syncWarnings.map((w, i) => <p key={i}>{w}</p>)}
            </div>
          </div>
        </Card>
      )}

      {/* Sync results */}
      {results && results.length > 0 && (
        <div className="space-y-3">
          <h2
            className="text-lg font-bold text-pencil"
          >
            {lastDryRun ? t('sync.results.preview') : t('sync.results.title')}
          </h2>
          <SyncResultList results={results} />
        </div>
      )}

      {/* Ignored skills/agents collapsible card */}
      {allIgnored.length > 0 && (
        <Card>
          <button
            onClick={() => setIgnoredExpanded((prev) => !prev)}
            className="w-full flex items-center gap-3 cursor-pointer"
          >
            {ignoredExpanded ? (
              <ChevronDown size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            ) : (
              <ChevronRight size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            )}
            <EyeOff size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            <span className="font-medium text-pencil-light text-left flex-1">
              {t('sync.ignored.label')}
            </span>
            <Badge variant="default">{allIgnored.length !== 1 ? t('sync.badge.resourceCountPlural', { count: allIgnored.length }) : t('sync.badge.resourceCount', { count: allIgnored.length })}</Badge>
          </button>

          {ignoredExpanded && (() => {
            const hasRoot = !!ignoreSources?.ignore_root;
            const repoCount = ignoreSources?.ignore_repos?.length ?? 0;
            const hasAgentRoot = !!ignoreSources?.agent_ignore_root;
            return (
              <div className="mt-3 pl-8 space-y-3 animate-fade-in">
                {ignoredSkills.length > 0 && (
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2">
                      <KindBadge kind="skill" />
                      <span className="text-xs text-pencil-light/70">{ignoredSkills.length}</span>
                    </div>
                    {ignoredSkills.map((name) => (
                      <div key={name} className="flex items-center gap-2 text-base py-0.5 pl-1">
                        <EyeOff size={12} className="text-pencil-light/50 shrink-0" />
                        <span className="font-mono text-pencil-light text-sm truncate">{name}</span>
                      </div>
                    ))}
                  </div>
                )}
                {ignoredAgents.length > 0 && (
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2">
                      <KindBadge kind="agent" />
                      <span className="text-xs text-pencil-light/70">{ignoredAgents.length}</span>
                    </div>
                    {ignoredAgents.map((name) => (
                      <div key={name} className="flex items-center gap-2 text-base py-0.5 pl-1">
                        <EyeOff size={12} className="text-pencil-light/50 shrink-0" />
                        <span className="font-mono text-pencil-light text-sm truncate">{name}</span>
                      </div>
                    ))}
                  </div>
                )}
                <div className="pt-2 border-t border-dashed border-pencil-light/30 space-y-1">
                  {hasRoot && (
                    <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                      <Info size={12} className="shrink-0" />
                      <span>{t('sync.ignored.skillignoreActive')}</span>
                    </div>
                  )}
                  {repoCount > 0 && (
                    <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                      <Info size={12} className="shrink-0" />
                      <span>{t('sync.ignored.repoFilesActive', { count: repoCount, files: repoCount === 1 ? 'file' : 'files' })}</span>
                    </div>
                  )}
                  {hasAgentRoot && (
                    <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                      <Info size={12} className="shrink-0" />
                      <span>{t('sync.ignored.agentignoreActive')}</span>
                    </div>
                  )}
                  {!hasRoot && repoCount === 0 && !hasAgentRoot && (
                    <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                      <Info size={12} className="shrink-0" />
                      <span>{t('sync.ignored.editSkillignore')}</span>
                    </div>
                  )}
                </div>
              </div>
            );
          })()}
        </Card>
      )}

      {/* Diff preview */}
      <div>
        <h3
          className="text-xl font-bold text-pencil mb-4"
        >
          {t('sync.diff.title')}
        </h3>
        {diffLoading && diffProgress && (
          <StreamProgressBar
            count={diffProgress.checked}
            total={diffProgress.total}
            startTime={startTimeRef.current}
            icon={RefreshCw}
            labelDiscovering={t('sync.diff.discoveringSkills')}
            labelRunning={t('sync.diff.computingDiff')}
            units={t('sync.diff.units')}
          />
        )}
        {!diffLoading && diffData && <DiffView diffs={diffData} />}
      </div>
    </div>
  );
}

function ActionBadge({ action }: { action: string }) {
  const map: Record<string, { variant: 'success' | 'info' | 'warning' | 'danger' | 'default'; label: string }> = {
    link: { variant: 'success', label: 'link' },
    linked: { variant: 'success', label: 'linked' },
    update: { variant: 'info', label: 'update' },
    updated: { variant: 'info', label: 'updated' },
    skip: { variant: 'warning', label: 'skip' },
    skipped: { variant: 'warning', label: 'skipped' },
    prune: { variant: 'danger', label: 'prune' },
    pruned: { variant: 'danger', label: 'pruned' },
    local: { variant: 'default', label: 'local' },
  };
  const entry = map[action] ?? { variant: 'default' as const, label: action };
  return <Badge variant={entry.variant}>{entry.label}</Badge>;
}

/** Diff preview with expandable targets */
function DiffView({ diffs: rawDiffs }: { diffs: DiffTarget[] }) {
  const t = useT();
  const diffs = rawDiffs ?? [];

  if (diffs.length === 0) {
    return (
      <Card variant="outlined">
        <div className="flex items-center justify-center gap-2 py-4 text-pencil-light">
          <AlertCircle size={18} strokeWidth={2} />
          <span>{t('sync.diff.noTargets')}</span>
        </div>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {diffs.map((d) => (
        <DiffTargetCard key={d.target} diff={d} />
      ))}
    </div>
  );
}

/** Max items before switching from flat list to virtualized scroll */
const VIRTUALIZE_THRESHOLD = 100;
/** Height of the virtualized container */
const VIRTUOSO_HEIGHT = 400;

function DiffTargetCard({ diff }: { diff: DiffTarget }) {
  const t = useT();
  const items = diff.items ?? [];
  const [expanded, setExpanded] = useState(items.length <= VIRTUALIZE_THRESHOLD);
  const localOnly = useMemo(() => items.filter((i) => i.action === 'local'), [items]);
  const syncItems = useMemo(() => items.filter((i) => i.action !== 'local'), [items]);
  const inSync = items.length === 0;
  const onlyLocal = syncItems.length === 0 && localOnly.length > 0;

  const hasSyncable = syncItems.some((i) => ['link', 'update', 'skip'].includes(i.action));
  const hasLocal = localOnly.length > 0;
  const useVirtualized = items.length > VIRTUALIZE_THRESHOLD;

  return (
    <Card>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 cursor-pointer"
      >
        {expanded ? (
          <ChevronDown size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
        ) : (
          <ChevronRight size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
        )}
        <Target size={16} strokeWidth={2.5} className="text-success shrink-0" />
        <h4
          className="font-bold text-pencil text-left flex-1"
        >
          {diff.target}
        </h4>
        {inSync ? (
          <Badge variant="success">{t('sync.badge.inSync')}</Badge>
        ) : onlyLocal ? (
          <Badge variant="default">{t('sync.badge.localOnly', { count: localOnly.length })}</Badge>
        ) : (
          <div className="flex items-center gap-2">
            <Badge variant="info">{t('sync.badge.pendingCount', { count: syncItems.length })}</Badge>
            {localOnly.length > 0 && <Badge variant="default">{t('sync.badge.localCount', { count: localOnly.length })}</Badge>}
          </div>
        )}
      </button>

      {expanded && items.length > 0 && (
        <div className="mt-3 pl-8 animate-fade-in">
          {useVirtualized ? (
            <Virtuoso
              style={{ height: VIRTUOSO_HEIGHT }}
              totalCount={items.length}
              overscan={200}
              itemContent={(i) => <DiffItemRow item={items[i]} />}
            />
          ) : (
            <div className="space-y-1.5">
              {items.map((item) => (
                <DiffItemRow key={`${item.action}:${item.skill}`} item={item} />
              ))}
            </div>
          )}

          {/* Action hints */}
          {(hasSyncable || hasLocal) && (
            <div className="mt-3 pt-2 border-t border-dashed border-pencil-light/30 space-y-1">
              {hasSyncable && (
                <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                  <Info size={12} className="shrink-0" />
                  <span>
                    {t('sync.hint.runSync')}
                  </span>
                </div>
              )}
              {hasLocal && (
                <div className="flex items-center gap-1.5 text-xs text-pencil-light">
                  <FileText size={12} className="shrink-0" />
                  <span>
                    {t('sync.hint.useCollect')}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {expanded && inSync && (
        <div className="mt-2 pl-8">
          <p className="text-base text-pencil-light">
            {t('sync.status.inSyncDetail')}
          </p>
          {(diff.skippedCount ?? 0) > 0 && (
            <p className="text-sm text-warning mt-1">
              {t('sync.warning.skippedNamingConflicts', { count: diff.skippedCount ?? 0 })}
              {(diff.collisionCount ?? 0) > 0 && <> {t('sync.warning.collisionCount', { count: diff.collisionCount ?? 0 })}</>}
              {' '}{t('sync.warning.skippedSwitchToFlat')}
            </p>
          )}
        </div>
      )}
    </Card>
  );
}

function DiffItemRow({ item }: { item: { action: string; skill: string; reason?: string; kind?: 'skill' | 'agent' } }) {
  return (
    <div className="flex items-center gap-2 text-base py-0.5">
      <ActionBadge action={item.action} />
      <ArrowRight size={12} className="text-muted-dark shrink-0" />
      <KindBadge kind={item.kind ?? 'skill'} />
      <span className="font-mono text-pencil-light text-sm truncate">
        {item.skill}
      </span>
      {item.reason && (
        <span className="text-pencil-light/60 text-xs shrink-0">({item.reason})</span>
      )}
    </div>
  );
}
