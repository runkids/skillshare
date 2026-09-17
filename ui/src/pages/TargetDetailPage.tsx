import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDownToLine, ChevronDown, ChevronRight, Folder, Target as TargetIcon } from 'lucide-react';
import { api, type SyncMatrixEntry, type Target } from '../api/client';
import Button from '../components/Button';
import CollectDialog from '../components/CollectDialog';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import SegmentedControl from '../components/SegmentedControl';
import { PageSkeleton } from '../components/Skeleton';
import Spinner from '../components/Spinner';
import { useToast } from '../components/Toast';
import PatternInput from '../components/targets/PatternInput';
import RemoveTargetDialog from '../components/targets/RemoveTargetDialog';
import { patternName, refreshTargets, togglePatterns } from '../components/targets/targetView';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { shortenHome } from '../lib/paths';
import { useT } from '../i18n';

type Kind = 'skill' | 'agent';
const MODES = ['merge', 'copy', 'symlink'] as const;

const draftOf = (target: Target) => ({
  include: target.include ?? [], exclude: target.exclude ?? [], mode: target.mode || 'merge', naming: target.targetNaming || 'flat',
  agentInclude: target.agentInclude ?? [], agentExclude: target.agentExclude ?? [], agentMode: target.agentMode || 'merge',
});
type Draft = ReturnType<typeof draftOf>;
const same = (a: string[], b: string[]) => a.length === b.length && a.every((x, i) => x === b[i]);

export default function TargetDetailPage() {
  const { name = '' } = useParams();
  const t = useT();
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.targets.all, queryFn: () => api.listTargets(), staleTime: staleTimes.targets });
  const target = data?.targets.find((x) => x.name === name);

  if (isPending) return <PageSkeleton />;
  if (error) return <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>;
  if (!target) {
    return (
      <EmptyState
        icon={TargetIcon}
        title={t('targetDetail.notFound', { name })}
        action={<Link to="/targets"><Button variant="secondary">{t('targetDetail.backToTargets')}</Button></Link>}
      />
    );
  }
  return <TargetEditor key={name} target={target} />;
}

function TargetEditor({ target }: { target: Target }) {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [params] = useSearchParams();
  const kind: Kind = target.agentPath && params.get('tab') === 'agents' ? 'agent' : 'skill';
  const saved = draftOf(target);
  const [draft, setDraft] = useState<Draft>(saved);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [collecting, setCollecting] = useState(false);
  const [help, setHelp] = useState(false);

  // Preview the draft filters once typing settles.
  const [filters, setFilters] = useState(draft);
  useEffect(() => {
    const id = setTimeout(() => setFilters(draft), 400);
    return () => clearTimeout(id);
  }, [draft]);
  const preview = useQuery({
    queryKey: ['sync-matrix-preview', target.name, filters.include, filters.exclude, filters.agentInclude, filters.agentExclude],
    queryFn: () => api.previewSyncMatrix(target.name, filters.include, filters.exclude, filters.agentInclude, filters.agentExclude),
    placeholderData: keepPreviousData,
  });
  const entriesOf = (k: Kind) => (preview.data?.entries ?? []).filter((e) => (e.kind === 'agent') === (k === 'agent') && e.status !== 'na');
  const entries = entriesOf(kind);

  const payload: Parameters<typeof api.updateTarget>[1] = {
    ...(!same(draft.include, saved.include) && { include: draft.include }),
    ...(!same(draft.exclude, saved.exclude) && { exclude: draft.exclude }),
    ...(draft.mode !== saved.mode && { mode: draft.mode }),
    ...(draft.naming !== saved.naming && { target_naming: draft.naming }),
    ...(!same(draft.agentInclude, saved.agentInclude) && { agent_include: draft.agentInclude }),
    ...(!same(draft.agentExclude, saved.agentExclude) && { agent_exclude: draft.agentExclude }),
    ...(draft.agentMode !== saved.agentMode && { agent_mode: draft.agentMode }),
  };
  const dirty = Object.keys(payload).length > 0;
  const save = async () => {
    setSaving(true);
    try {
      await api.updateTarget(target.name, payload);
      refreshTargets(queryClient);
      toast(t('targetDetail.saved', { name: target.name }), 'success');
    } catch (err) {
      toast((err as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const agent = kind === 'agent';
  const include = agent ? draft.agentInclude : draft.include;
  const exclude = agent ? draft.agentExclude : draft.exclude;
  const mode = agent ? draft.agentMode : draft.mode;
  const setFiltersFor = (next: { include: string[]; exclude: string[] }) =>
    setDraft(agent ? { ...draft, agentInclude: next.include, agentExclude: next.exclude } : { ...draft, ...next });
  const local = agent ? target.agentLocalCount ?? 0 : target.localCount;
  const synced = entries.filter((e) => e.status === 'synced').length;

  const reason = (e: SyncMatrixEntry) => {
    switch (e.status) {
      case 'synced': return t(include.length ? 'targetDetail.reason.included' : 'targetDetail.reason.noFilter');
      case 'excluded': return t('targetDetail.reason.excluded', { pattern: e.reason });
      case 'not_included': return t('targetDetail.reason.notIncluded');
      case 'skill_target_mismatch': return t('targetDetail.reason.declared', { targets: e.reason });
      default: return e.reason;
    }
  };

  const tabCount = (k: Kind) => entriesOf(k).length || null;
  return (
    <div className="animate-fade-in">
      <PageHeader
        crumbs={[{ label: t('targets.title'), to: '/targets' }, { label: target.name }]}
        title={target.name}
        subtitle={<span className="font-mono">{shortenHome(agent ? target.agentPath ?? '' : target.path)}</span>}
        actions={
          <>
            <Button variant="ghost" onClick={() => setRemoving(true)}>{t('targetDetail.remove')}</Button>
            <Button variant="primary" onClick={save} loading={saving} disabled={!dirty}>{t('common.save')}</Button>
          </>
        }
      />

      {target.agentPath && (
        <nav className="ss-tabs mb-7" aria-label={t('targetDetail.tabs')}>
          {(['skill', 'agent'] as const).map((k) => (
            <Link key={k} to={k === 'agent' ? '?tab=agents' : '?'} replace className={kind === k ? 'on' : ''}>
              {k === 'agent' ? 'Agents' : 'Skills'}
              {tabCount(k) !== null && <span className="ss-cnt">{tabCount(k)}</span>}
            </Link>
          ))}
        </nav>
      )}

      <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] items-start gap-12">
        <section className="flex flex-col gap-5">
          <h2 className="ss-h2">{t('targetDetail.whatSyncs')}</h2>
          {mode === 'symlink' ? (
            <div className="ss-note inf"><span className="flex-1">{t('targetDetail.symlinkNoFilters')}</span></div>
          ) : (
            <>
              {preview.data && (
                <p className="text-[13.5px]">
                  {t(`targetDetail.summary.${agent ? 'agents' : 'skills'}.${entries.length === 1 ? 'one' : 'other'}`, { synced, total: entries.length, name: target.name })}
                </p>
              )}
              <div className="ss-fld">
                <label htmlFor="filter-include">{t('targetDetail.include')}</label>
                <PatternInput id="filter-include" patterns={include} onChange={(next) => setFiltersFor({ include: next, exclude })} disabled={saving} />
                <span className="hp">{t(agent ? 'targetDetail.includeHint.agents' : 'targetDetail.includeHint.skills')}</span>
              </div>
              <div className="ss-fld">
                <label htmlFor="filter-exclude">{t('targetDetail.exclude')}</label>
                <PatternInput id="filter-exclude" patterns={exclude} onChange={(next) => setFiltersFor({ include, exclude: next })} disabled={saving} />
                <span className="hp">{t(agent ? 'targetDetail.excludeHint.agents' : 'targetDetail.excludeHint.skills')}</span>
              </div>
              <button type="button" className="ss-disc self-start" aria-expanded={help} onClick={() => setHelp(!help)}>
                {help ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
                {t('targetDetail.patternHelp')}
              </button>
              {help && (
                <ul className="ml-[22px] -mt-2 flex list-disc flex-col gap-1 pl-4 text-[13px] text-ink-2">
                  <li>{t('targetDetail.help.wildcards')}</li>
                  <li>{t('targetDetail.help.nested')}</li>
                  <li>{t('targetDetail.help.order')}</li>
                  <li>{t('targetDetail.help.declared')}</li>
                </ul>
              )}

              {preview.isPending ? (
                <div className="flex items-center gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('targetDetail.loadingPreview')}</div>
              ) : preview.error ? (
                <div className="ss-note bad"><span className="flex-1">{preview.error.message}</span></div>
              ) : entries.length > 0 && (
                <div className="flex flex-col gap-2">
                  <div className="ss-list !shadow-none">
                    <div className="ss-lh">
                      <span className="flex-1">{t('targetDetail.previewCount', { count: entries.length })}</span>
                      <span className="w-[170px]">{t('targetDetail.becauseOf')}</span>
                      <span className="w-[96px]">{t('targetDetail.result')}</span>
                    </div>
                    <div className="max-h-[340px] overflow-y-auto">
                      {entries.map((e) => {
                        const next = togglePatterns(e, include, exclude);
                        const on = e.status === 'synced';
                        const cells = (
                          <>
                            <span className={`min-w-0 flex-1 truncate font-mono text-[13px] font-semibold ${on ? '' : 'text-ink-2'}`}>{patternName(e)}</span>
                            <span className="w-[170px] shrink-0 truncate font-mono text-[12px] text-ink-2" title={reason(e)}>{reason(e)}</span>
                            <span className="w-[96px] shrink-0"><span className={`ss-st ${on ? 'ok' : 'off'}`}>{t(on ? 'targetDetail.synced' : 'targetDetail.notSynced')}</span></span>
                          </>
                        );
                        return next ? (
                          <button key={e.skill} type="button" className="ss-r link !min-h-[42px] w-full text-left" onClick={() => setFiltersFor(next)} title={t(on ? 'targetDetail.clickExclude' : 'targetDetail.clickInclude')} disabled={saving}>
                            {cells}
                          </button>
                        ) : (
                          <div key={e.skill} className="ss-r !min-h-[42px]">{cells}</div>
                        );
                      })}
                    </div>
                  </div>
                  <p className="text-[13px] text-ink-3">{t('targetDetail.clickHint')}</p>
                </div>
              )}
            </>
          )}
        </section>

        <aside className="flex flex-col gap-7">
          {agent && (
            <div className="flex flex-col gap-1.5">
              <span className="text-[13px] font-semibold">{t('targetDetail.agentsFolder')}</span>
              <span className="flex items-center gap-2 font-mono text-[13px]"><Folder size={15} className="shrink-0 text-ink-3" />{shortenHome(target.agentPath ?? '')}</span>
              <span className="text-[12.5px] text-ink-3">{t('targetDetail.agentsFolderHint')}</span>
            </div>
          )}
          <div className="flex flex-col gap-3">
            <h2 className="ss-h2">{t('targetDetail.syncMode')}</h2>
            <div role="radiogroup" aria-label={t('targetDetail.syncMode')} className="flex flex-col gap-2.5">
              {MODES.map((m) => {
                const on = mode === m;
                return (
                  <button
                    key={m}
                    type="button"
                    role="radio"
                    aria-checked={on}
                    className={`ss-pick text-left ${on ? 'on' : ''}`}
                    onClick={() => setDraft(agent ? { ...draft, agentMode: m } : { ...draft, mode: m })}
                    disabled={saving}
                  >
                    <span className={`ss-chk rad ${on ? 'on' : ''}`} />
                    <span className="flex flex-col gap-0.5">
                      <span><span className="font-semibold">{m}</span>{m === 'merge' && <span className="text-ink-3"> · {t('targetDetail.default')}</span>}</span>
                      <span className="text-[13px] text-ink-2">{t(m === 'merge' ? `targetDetail.mode.merge.${kind}` : `targetDetail.mode.${m}`)}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          {!agent && draft.mode !== 'symlink' && (
            <div className="flex flex-col gap-1.5">
              <div className="flex items-center gap-4">
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="text-[13px] font-semibold">{t('targetDetail.naming')}</span>
                  <span className="text-[13px] text-ink-2">{t(draft.naming === 'standard' ? 'targetDetail.namingStandard' : 'targetDetail.namingFlat')}</span>
                </div>
                <SegmentedControl value={draft.naming} onChange={(naming) => setDraft({ ...draft, naming })} options={[{ value: 'flat', label: 'flat' }, { value: 'standard', label: 'standard' }]} />
              </div>
              {saved.naming === 'standard' && (target.skippedSkillCount ?? 0) > 0 && (
                <span className="text-[13px] text-warn">{t(target.skippedSkillCount === 1 ? 'targetDetail.skipped.one' : 'targetDetail.skipped.other', { count: target.skippedSkillCount })}</span>
              )}
            </div>
          )}

          {local > 0 && (
            <div className="ss-box flex flex-col gap-3">
              <span className="flex items-center gap-2 font-semibold"><ArrowDownToLine size={16} />{t('targetDetail.collect')}</span>
              <p className="text-[13px] text-ink-2">{t(`targetDetail.collectHint.${agent ? 'agents' : 'skills'}.${local === 1 ? 'one' : 'other'}`, { count: local })}</p>
              <Button variant="secondary" onClick={() => setCollecting(true)}>
                {t(`collectDialog.run.${kind}.${local === 1 ? 'one' : 'other'}`, { count: local })}
              </Button>
            </div>
          )}
        </aside>
      </div>

      {removing && (
        <RemoveTargetDialog
          target={target}
          onClose={() => setRemoving(false)}
          onRemoved={() => {
            refreshTargets(queryClient);
            toast(t('targets.targetRemoved', { name: target.name }), 'success');
            navigate('/targets');
          }}
        />
      )}
      {collecting && <CollectDialog target={target.name} kind={kind} onClose={() => setCollecting(false)} />}
    </div>
  );
}
