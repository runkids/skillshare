import { Fragment, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ArrowDownToLine, Bot, ChevronDown, ChevronRight, CircleCheck, CircleMinus, EyeOff, Folder, FolderPlus, Gauge, Globe, Import, Minus, Plug, Plus, Puzzle, RefreshCw, TriangleAlert, Webhook } from 'lucide-react';
import { api, formatTokenK, type SyncResponse } from '../api/client';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import CollectDialog from '../components/CollectDialog';
import { Checkbox } from '../components/Input';
import PageHeader from '../components/PageHeader';
import Spinner from '../components/Spinner';
import Tooltip from '../components/Tooltip';
import { useToast } from '../components/Toast';
import { hookLabel, hookMessage, rootName } from '../components/hooks/hooksView';
import { describeMessage, mcpClient, targetLabel } from '../components/mcp/mcpView';
import { countChanges, countEdited, extraGroups, groupByFolder, groupInSync, HOOKS_CHANGED, hooksGroups, MCP_CHANGED, mcpGroups, otherWarnings, resourceGroups, runSync, type ChangeGroup, type Part, type RowIcon, type SyncFailure } from '../components/sync/syncView';
import SyncResult from '../components/sync/SyncResult';
import SkillsOffDialog from '../components/targets/SkillsOffDialog';
import { joinList, refreshTargets } from '../components/targets/targetView';
import { formatDateTime, formatRelativeTime, useI18n, useT } from '../i18n';
import { shortenHome } from '../lib/paths';
import { formatAgentDisplayName } from '../lib/resourceNames';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useDiffQuery, useHooksQuery, useMcpQuery, useSyncedTargetsQuery } from '../hooks/useSharedQueries';

const ROW_ICON: Record<RowIcon, React.ReactNode> = {
  add: <Plus size={16} className="shrink-0 text-ok" />,
  adopt: <Import size={15} className="shrink-0 text-info" />,
  update: <RefreshCw size={15} className="shrink-0 text-info" />,
  remove: <Minus size={16} className="shrink-0 text-bad" />,
  kept: <CircleMinus size={15} className="shrink-0 text-ink-3" />,
  conflict: <TriangleAlert size={15} className="shrink-0 text-warn" />,
};
const PART_ICON: Record<Part, React.ReactNode> = { skill: <Puzzle size={14} />, agent: <Bot size={14} />, extra: <FolderPlus size={14} />, mcp: <Plug size={14} />, hooks: <Webhook size={14} /> };
const PART_LABEL: Record<Part, string> = { skill: 'Skills', agent: 'Agents', extra: 'Extras', mcp: 'MCP', hooks: 'Hooks' };
const PARTS = Object.keys(PART_LABEL) as Part[];

/** A target in an expanded list: its logo and name. */
function TargetChip({ target, label }: { target: string; label: string }) {
  return (
    <span className="inline-flex h-7 items-center gap-[7px] rounded-[var(--r-ctl)] border border-line bg-surface pl-[5px] pr-2.5 text-[13px]">
      <span className="inline-flex size-5 items-center justify-center rounded-[5px] bg-sunken"><AgentIcon target={target} size={14} /></span>
      {label}
    </span>
  );
}

/** A labelled row of an expanded list: the label on the left, its items on the right. */
function ScopeRow({ label, sub, icon, children }: { label: string; sub?: string; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[96px_minmax(0,1fr)] items-start gap-3">
      <div className={`flex flex-col gap-0.5 ${sub ? 'pt-[3px]' : ''}`}>
        <span className={`flex items-center gap-1.5 text-[12px] font-semibold text-ink-3 ${sub ? '' : 'h-7'}`}>{icon}{label}</span>
        {sub && <span className="font-mono text-[11.5px] text-ink-3">{sub}</span>}
      </div>
      {children}
    </div>
  );
}

export default function SyncPage() {
  const t = useT();
  const { locale } = useI18n();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const targets = useSyncedTargetsQuery();
  const diff = useDiffQuery();
  const extras = useQuery({ queryKey: queryKeys.extrasDiff(), queryFn: () => api.diffExtras(), staleTime: staleTimes.extras });
  const mcp = useMcpQuery({ staleTime: staleTimes.extras });
  const hooks = useHooksQuery({ staleTime: staleTimes.extras });
  const log = useQuery({ queryKey: queryKeys.log('ops', 20, { cmd: 'sync' }), queryFn: () => api.listLog('ops', 20, { cmd: 'sync' }), staleTime: staleTimes.log });

  const [off, setOff] = useState<Set<Part>>(new Set());
  const [force, setForce] = useState(false);
  const [open, setOpen] = useState<Set<string>>(new Set());
  const [collecting, setCollecting] = useState(false);
  const [running, setRunning] = useState(false);
  const [runError, setRunError] = useState('');
  const [outcome, setOutcome] = useState<SyncResponse | null>(null);
  const [failures, setFailures] = useState<SyncFailure[]>([]);
  const [stopping, setStopping] = useState('');

  const plan = mcp.data?.plan;
  const hooksPlan = hooks.data?.plan;
  const parts = new Set(PARTS.filter((p) => !off.has(p)));
  const diffs = diff.data?.diffs ?? [];
  const resources = resourceGroups(diffs, targets.data?.targets ?? [], parts, force, { skill: diff.data?.ignored_skills, agent: diff.data?.agent_ignored_skills });
  const mcpShown = parts.has('mcp') ? mcpGroups(plan) : [];
  const hooksShown = parts.has('hooks') ? hooksGroups(hooksPlan) : [];
  const groups: ChangeGroup[] = [...resources.groups, ...(parts.has('extra') ? extraGroups(extras.data?.extras ?? [], force) : []), ...mcpShown, ...hooksShown];
  // A blocked plan applies nothing, so its rows are shown but not counted.
  const count = countChanges(groups) - (plan?.blocked ? countChanges(mcpShown) : 0) - (hooksPlan?.blocked ? countChanges(hooksShown) : 0);
  const edited = countEdited(groups);
  const loading = diff.isPending || targets.isPending;

  const localByTarget = diffs
    .map((d) => ({ target: d.target, items: (d.items ?? []).filter((i) => i.action === 'local' && parts.has(i.kind === 'agent' ? 'agent' : 'skill')) }))
    .filter((d) => d.items.length > 0);
  const local = localByTarget.flatMap((d) => d.items);
  const skillIgnored = parts.has('skill') ? diff.data?.ignored_skills ?? [] : [];
  const agentIgnored = parts.has('agent') ? diff.data?.agent_ignored_skills ?? [] : [];
  const ignored = [...skillIgnored, ...agentIgnored];
  const inSync = groupInSync(resources.inSync);
  const skipped = parts.has('skill') ? diffs.filter((d) => (d.skippedCount ?? 0) > 0) : [];
  const last = log.data?.entries.find((e) => !e.args?.dry_run);
  const targetList = targets.data?.targets ?? [];
  const conflicts = diff.data?.folder_conflicts ?? [];
  const stopTarget = targetList.find((x) => x.name === stopping);
  const stopConflict = conflicts.find((c) => c.stop.includes(stopping));

  const toggle = (set: Set<string>, key: string) => { const next = new Set(set); if (next.has(key)) next.delete(key); else next.add(key); return next; };

  const sync = async () => {
    setRunning(true);
    setRunError('');
    setOutcome(null);
    setFailures([]);
    try {
      const { resources: result, failures: failed } = await runSync({
        resources: parts.has('skill') && parts.has('agent') ? 'both' : parts.has('skill') ? 'skill' : parts.has('agent') ? 'agent' : null,
        extras: parts.has('extra') && !!extras.data?.extras.length,
        mcp: parts.has('mcp') && plan && !plan.blocked && (plan.migrates || plan.changes.some((c) => c.action !== 'unchanged')) ? plan : null,
        hooks: parts.has('hooks') && hooksPlan && !hooksPlan.blocked && hooksPlan.changes.some((c) => c.action !== 'unchanged') ? hooksPlan : null,
        force,
      });
      setOutcome(result ?? null);
      setFailures(failed);
      if (failed.length > 0) toast(t(failed.length === 1 ? 'sync.toast.failed.one' : 'sync.toast.failed.other', { count: failed.length }), 'warning');
      else toast(t('sync.toast.done'), 'success');
    } catch (err) {
      const message = (err as Error).message;
      setRunError(message === MCP_CHANGED ? t('sync.mcpChanged') : message === HOOKS_CHANGED ? t('sync.hooksChanged') : message);
    } finally {
      setRunning(false);
      refreshTargets(queryClient);
      for (const queryKey of [queryKeys.extrasDiff(), queryKeys.extras, queryKeys.mcp, queryKeys.hooks, ['log']]) void queryClient.invalidateQueries({ queryKey });
    }
  };

  const failedTargets = new Set(failures.filter((f) => f.part !== 'extra').map((f) => f.target));
  const syncedTargets = new Set((outcome?.results ?? []).map((r) => r.target).filter((name) => !failedTargets.has(name))).size;

  const groupHead = (g: ChangeGroup) => {
    const n = countChanges([g]);
    return (
      <div className="ss-gh">
        {g.part === 'extra' ? <span className="ss-cat sm extra">{PART_ICON.extra}</span> : <span className="ss-at"><AgentIcon target={g.name} size={17} /></span>}
        <span className="font-semibold">{g.part === 'mcp' ? targetLabel(g.name) : g.part === 'hooks' ? hookLabel(g.name) : g.name}</span>
        <span className="ss-tag">{g.part === 'mcp' ? 'MCP' : g.part === 'hooks' ? 'Hooks' : g.mode}</span>
        {g.path && <span className="min-w-0 truncate font-mono text-[12px] text-ink-3" title={g.path}>{shortenHome(g.path)}</span>}
        {g.part === 'target' && failedTargets.has(g.name) && <span className="ss-tag bad shrink-0">{t('sync.result.lastFailed')}</span>}
        {g.project && <span className="ss-tag shrink-0" title={g.project}>{g.part === 'hooks' ? rootName(g.project) : t('sync.mcp.offList', { project: shortenHome(g.project) })}</span>}
        <span className="flex-1" />
        {n > 0 && <span className="shrink-0 text-[12px] text-ink-2">{t(n === 1 ? 'sync.changes.one' : 'sync.changes.other', { count: n })}</span>}
      </div>
    );
  };

  return (
    <div className="animate-fade-in">
      <PageHeader
        title={t('sync.title')}
        subtitle={t('sync.subtitle')}
        actions={
          <span data-tour="sync-actions">
            <Button variant="primary" onClick={sync} loading={running} disabled={loading || parts.size === 0}>
              {!running && <RefreshCw size={16} />}
              {count > 0 ? t(count === 1 ? 'sync.run.one' : 'sync.run.other', { count }) : t('sync.run.none')}
            </Button>
          </span>
        }
      />
      <div className="grid grid-cols-[minmax(0,1fr)_280px] items-start gap-8">
        <div className="flex min-w-0 flex-col gap-4">
          <div className="flex flex-wrap items-center gap-[18px]">
            <span className="text-[13px] text-ink-3">{t('sync.include')}</span>
            {PARTS.map((p) => (
              <Checkbox key={p} size="sm" label={PART_LABEL[p]} checked={parts.has(p)} disabled={running} onChange={() => setOff((s) => toggle(s, p) as Set<Part>)} />
            ))}
            <span className="flex-1" />
            <span className="flex items-center gap-2" title={t('sync.forceHint')}>
              <button type="button" role="switch" aria-checked={force} aria-labelledby="sync-force" aria-describedby="sync-force-hint" className={`ss-sw ${force ? 'on' : ''} disabled:opacity-50`} disabled={running} onClick={() => setForce(!force)}>
                <i />
              </button>
              <span id="sync-force" className="text-[13px] font-semibold">Force</span>
              <span id="sync-force-hint" className="sr-only">{t('sync.forceHint')}</span>
            </span>
          </div>
          {edited > 0 && (
            <div className={`ss-note ${force ? 'warn' : 'inf'}`}>
              {force ? <TriangleAlert size={16} /> : <CircleMinus size={16} />}
              <span className="flex-1">{t(`sync.edited.${force ? 'replace' : 'kept'}.${edited === 1 ? 'one' : 'other'}`, { count: edited })}</span>
            </div>
          )}

          {runError && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{runError}</span></div>}
          {parts.has('mcp') && plan?.blocked && (
            <div className="ss-note warn !items-center">
              <TriangleAlert size={16} />
              <span className="flex-1">{t('sync.mcpBlocked')}</span>
              <Link to="/mcp" className="ss-btn sm">{t('sync.openMcp')}</Link>
            </div>
          )}
          {parts.has('hooks') && hooksPlan?.blocked && (
            <div className="ss-note warn !items-center">
              <TriangleAlert size={16} />
              <span className="flex-1">{t('sync.hooksBlocked')}</span>
              <Link to="/hooks" className="ss-btn sm">{t('sync.openHooks')}</Link>
            </div>
          )}
          {parts.has('hooks') && hooks.data?.previewError && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{hooks.data.previewError}</span></div>}
          {parts.has('mcp') && mcp.data?.previewError && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{mcp.data.previewError}</span></div>}
          <SyncResult failures={failures} warnings={otherWarnings(outcome)} synced={syncedTargets} force={force} onForce={() => setForce(true)} />
          {!!outcome?.path_overlap && (
            <div className="ss-note warn !items-center">
              <TriangleAlert size={16} />
              <span className="flex-1">{t(outcome.path_overlap === 1 ? 'sync.pathOverlap.one' : 'sync.pathOverlap.other', { count: outcome.path_overlap })}</span>
              <Link to="/doctor" className="ss-btn sm">{t('sync.openDoctor')}</Link>
            </div>
          )}
          {conflicts.map((c) => (
            <div key={c.path} className="ss-note warn !items-center">
              <TriangleAlert size={16} />
              <span className="flex-1">{t('sync.folderConflict.text', { names: joinList(c.targets, locale), path: shortenHome(c.path) })}</span>
              {c.stop.map((name) => (
                <Button key={name} variant="secondary" size="sm" onClick={() => setStopping(name)}>
                  {t('sync.folderConflict.stop', { name })}
                </Button>
              ))}
            </div>
          ))}

          <div className="ss-list">
            {loading ? (
              <div className="ss-r gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('sync.checking')}</div>
            ) : diff.error ? (
              <div className="ss-r text-[13px] text-bad"><AlertCircle size={16} />{diff.error.message}</div>
            ) : diffs.length === 0 && groups.length === 0 ? (
              <div className="ss-r text-[13px] text-ink-2">
                <span className="flex-1">{t('sync.noTargets')}</span>
                <Link to="/targets" className="ss-btn sm">{t('sync.addTarget')}</Link>
              </div>
            ) : (
              <>
                {groups.length === 0 && (
                  <div className="ss-r text-[13px]"><CircleCheck size={16} className="text-ok" />{t('sync.nothing')}</div>
                )}
                {groups.map((g) => (
                  <Fragment key={g.key}>
                    {groupHead(g)}
                    {g.rows.map((r) => (
                      <div key={r.key} className="ss-r !min-h-[46px]">
                        {ROW_ICON[r.icon]}
                        <span className={`ss-cat sm ${r.part}`}>{PART_ICON[r.part]}</span>
                        <span className="w-[220px] shrink-0 truncate font-mono text-[13px] font-semibold" title={r.name}>{r.name}</span>
                        {r.part === 'hooks' && r.detail ? (
                          // A hooks reason is worded for this UI and can be long: hover shows all of it.
                          <span className="min-w-0 flex-1">
                            <Tooltip block content={hookMessage(t, r.detail)}>
                              <span className={`block truncate text-[13px] ${r.icon === 'conflict' ? 'text-warn' : 'text-ink-2'}`}>{r.text ? t(r.text) : hookMessage(t, r.detail)}</span>
                            </Tooltip>
                          </span>
                        ) : (
                          <span className={`min-w-0 flex-1 truncate text-[13px] ${r.icon === 'conflict' ? 'text-warn' : 'text-ink-2'}`} title={r.detail}>
                            {r.text ? t(r.text) : r.part === 'mcp' ? describeMessage(t, r.detail) : r.detail}
                          </span>
                        )}
                      </div>
                    ))}
                  </Fragment>
                ))}
                {resources.inSync.length > 0 && (
                  <button type="button" className="ss-gh w-full text-left" aria-expanded={open.has('inSync')} onClick={() => setOpen((s) => toggle(s, 'inSync'))}>
                    {!open.has('inSync') && <span className="ss-stack ml-1.5">{resources.inSync.slice(0, 4).map((name) => <span key={name} className="ss-at"><AgentIcon target={name} size={14} /></span>)}</span>}
                    <span className="font-semibold">{t(resources.inSync.length === 1 ? 'sync.inSync.one' : 'sync.inSync.other', { count: resources.inSync.length })}</span>
                    <span className="flex-1" />
                    {open.has('inSync') ? <ChevronDown size={15} className="text-ink-3" /> : <ChevronRight size={15} className="text-ink-3" />}
                  </button>
                )}
                {open.has('inSync') && (
                  <div className="flex flex-col px-[18px] pb-[18px] pt-4 [border-top:var(--sep)] [&>*+*]:mt-3.5 [&>*+*]:pt-3.5 [&>*+*]:[border-top:var(--sep)]">
                    {inSync.global.length > 0 && (
                      <ScopeRow icon={<Globe size={13} />} label={`${t('resources.targets.global')} · ${inSync.global.length}`}>
                        <div className="flex flex-wrap gap-2">{inSync.global.map((name) => <TargetChip key={name} target={name} label={name} />)}</div>
                      </ScopeRow>
                    )}
                    {inSync.projects.length > 0 && (
                      <ScopeRow icon={<Folder size={13} />} label={`${t('resources.targets.projects')} · ${resources.inSync.length - inSync.global.length}`}>
                        <div className="flex flex-col gap-2">
                          {inSync.projects.map(({ project, tools }) => (
                            <div key={project} className="grid grid-cols-[120px_minmax(0,1fr)] items-center gap-2.5">
                              <span className="truncate font-mono text-[12.5px] text-ink-2" title={project}>{project}</span>
                              <div className="flex flex-wrap gap-2">{tools.map((tool) => <TargetChip key={tool} target={`${project}@${tool}`} label={tool} />)}</div>
                            </div>
                          ))}
                        </div>
                      </ScopeRow>
                    )}
                  </div>
                )}
              </>
            )}
          </div>

          {outcome?.context_cost?.warnings?.map((w) => (
            <div key={`${w.type}/${w.target}`} className="ss-note warn !items-center">
              <Gauge size={16} />
              <span className="flex-1">
                <b>{t(w.type === 'always_loaded' ? 'sync.budget.always' : 'sync.budget.onDemand')}</b>{' '}
                {t('sync.budget.detail', { actual: formatTokenK(w.actual), budget: formatTokenK(w.budget), target: w.target })}{' '}
                {w.top_offenders.length > 0 && t('sync.budget.biggest', { names: w.top_offenders.slice(0, 3).map((o) => `${o.name} ${formatTokenK(o.tokens)}`).join(', ') })}
              </span>
              <Link to="/skills?tab=analyze" className="ss-btn sm">{t('sync.budget.analyze')}</Link>
            </div>
          ))}

          {!loading && (ignored.length > 0 || skipped.length > 0 || local.length > 0) && (
            <div className="ss-list">
              {ignored.length > 0 && (
                <>
                  <div className="ss-r !min-h-11 text-[13px]">
                    <button type="button" className="flex min-w-0 flex-1 items-center gap-2.5 text-left" aria-expanded={open.has('ignored')} onClick={() => setOpen((s) => toggle(s, 'ignored'))}>
                      {open.has('ignored') ? <ChevronDown size={14} className="shrink-0 text-ink-3" /> : <ChevronRight size={14} className="shrink-0 text-ink-3" />}
                      <EyeOff size={15} className="shrink-0 text-ink-3" />
                      <span><b>{t(ignored.length === 1 ? 'sync.ignored.one' : 'sync.ignored.other', { count: ignored.length })}</b>{!open.has('ignored') && <> <span className="text-ink-2">{t('sync.ignored.by')}</span></>}</span>
                    </button>
                    <Link to="/config" className="shrink-0 font-semibold">{t('sync.ignored.edit')}</Link>
                  </div>
                  {open.has('ignored') && (
                    <div className="flex flex-col gap-3.5 px-[18px] pb-[18px] pt-3.5 [border-top:var(--sep)]">
                      {([['Skills', '.skillignore', skillIgnored], ['Agents', '.agentignore', agentIgnored]] as const).filter(([, , names]) => names.length > 0).map(([label, file, names]) => (
                        <ScopeRow key={file} label={`${label} · ${names.length}`} sub={file}>
                          <div className="flex flex-col gap-2">
                            {groupByFolder(names).map(({ folder, items }) => (
                              <div key={folder} className="flex flex-col gap-1.5">
                                {folder && <div className="flex items-center gap-[7px] font-mono text-[12.5px] text-ink-2"><Folder size={14} className="shrink-0 text-ink-3" />{folder}</div>}
                                <div className={`grid grid-cols-2 gap-x-6 gap-y-1.5 font-mono text-[12.5px] ${folder ? 'pl-[21px]' : ''}`}>
                                  {items.map((name) => <span key={name} className="truncate" title={folder + name}>{name}</span>)}
                                </div>
                              </div>
                            ))}
                          </div>
                        </ScopeRow>
                      ))}
                    </div>
                  )}
                </>
              )}
              {skipped.map((d) => (
                <div key={d.target} className="ss-r !min-h-11 text-[13px]">
                  <TriangleAlert size={15} className="ml-6 shrink-0 text-warn" />
                  <span className="min-w-0 flex-1">
                    <b>{t((d.skippedCount ?? 0) === 1 ? 'sync.skipped.one' : 'sync.skipped.other', { count: d.skippedCount ?? 0, name: d.target })}</b>{' '}
                    <span className="text-ink-2">{t('sync.skipped.hint', { name: d.target })}</span>
                  </span>
                  <Link to={`/targets/${encodeURIComponent(d.target)}`} className="shrink-0 font-semibold">{t('sync.skipped.open')}</Link>
                </div>
              ))}
              {local.length > 0 && (
                <>
                  <div className="ss-r !min-h-11 text-[13px]">
                    <button type="button" className="flex min-w-0 flex-1 items-center gap-2.5 text-left" aria-expanded={open.has('local')} onClick={() => setOpen((s) => toggle(s, 'local'))}>
                      {open.has('local') ? <ChevronDown size={14} className="shrink-0 text-ink-3" /> : <ChevronRight size={14} className="shrink-0 text-ink-3" />}
                      <ArrowDownToLine size={15} className="shrink-0 text-ink-3" />
                      <span><b>{t(local.length === 1 ? 'sync.local.one' : 'sync.local.other', { count: local.length })}</b> <span className="text-ink-2">{t('sync.local.hint')}</span></span>
                    </button>
                    <button type="button" className="shrink-0 font-semibold" onClick={() => setCollecting(true)}>{t('sync.local.collect')}</button>
                  </div>
                  {open.has('local') && (
                    <div className="flex flex-col gap-3.5 px-[18px] pb-[18px] pt-3.5 [border-top:var(--sep)]">
                      {localByTarget.map(({ target, items }) => (
                        <div key={target} className="flex flex-col gap-2">
                          <div className="flex items-center gap-2"><TargetChip target={target} label={target} /><span className="text-[12px] text-ink-3">{items.length}</span></div>
                          <div className="grid grid-cols-2 gap-x-6 gap-y-1.5 pl-[5px] font-mono text-[12.5px]">
                            {items.map((i) => {
                              const name = i.kind === 'agent' ? formatAgentDisplayName(i.skill) : i.skill;
                              return <span key={`${i.kind}:${i.skill}`} className="flex min-w-0 items-center gap-1.5" title={name}>{i.kind === 'agent' ? <Bot size={13} className="shrink-0 text-ink-3" /> : <Puzzle size={13} className="shrink-0 text-ink-3" />}<span className="truncate">{name}</span></span>;
                            })}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              )}
            </div>
          )}

        </div>

        <aside className="flex flex-col">
          <div className="ss-box flex flex-col gap-3.5">
            <div className="flex items-center justify-between">
              <h3 className="text-[15px] font-semibold">{t('sync.last.title')}</h3>
              {last && <span className={`ss-st ${last.status === 'ok' ? 'ok' : last.status === 'partial' ? 'warn' : 'bad'}`}>{last.status === 'ok' ? 'OK' : last.status}</span>}
            </div>
            {last ? (
              <dl className="ss-kv !grid-cols-[80px_minmax(0,1fr)]">
                <dt>{t('sync.last.when')}</dt>
                <dd title={formatDateTime(last.ts, locale)}>{formatRelativeTime(last.ts, locale)}</dd>
                {typeof last.args?.targets_total === 'number' && <><dt>{t('sync.last.targets')}</dt><dd>{last.args.targets_total}</dd></>}
                {last.args?.targets_failed > 0 && (
                  <>
                    <dt>{t('sync.last.failed')}</dt>
                    {/* Entries written before failed_targets existed only have the count. */}
                    <dd className="break-words text-bad">{Array.isArray(last.args?.failed_targets) && last.args.failed_targets.length > 0 ? joinList(last.args.failed_targets, locale) : last.args?.targets_failed}</dd>
                  </>
                )}
                {typeof last.ms === 'number' && <><dt>{t('sync.last.took')}</dt><dd className="font-mono">{(last.ms / 1000).toFixed(1)} s</dd></>}
                {last.msg && <><dt>{t('sync.last.error')}</dt><dd className="break-words text-bad">{last.msg}</dd></>}
              </dl>
            ) : (
              <p className="text-[13px] text-ink-2">{t('sync.last.never')}</p>
            )}
            <Link to="/log" className="ss-btn sm">{t('sync.last.openLog')}</Link>
          </div>
          <p className="mt-3.5 px-1 text-[13px] leading-relaxed text-ink-3">{t('sync.backupNote')} <Link to="/backup" className="font-semibold">{t('sync.backupLink')}</Link></p>
        </aside>
      </div>

      {collecting && <CollectDialog onClose={() => setCollecting(false)} />}
      {stopTarget && (
        <SkillsOffDialog
          target={stopTarget}
          readFrom={targetList.find((x) => x.name === stopConflict?.keep)}
          managed={[...(stopTarget.agentPath ? ['Agents'] : []), ...(mcp.data?.paths[mcpClient(stopTarget.name)] ? ['MCP'] : [])]}
          onClose={() => setStopping('')}
          onStopped={(removed) => {
            setStopping('');
            refreshTargets(queryClient);
            toast(t(removed === 1 ? 'targetDetail.skillsOff.stopped.one' : 'targetDetail.skillsOff.stopped.other', { name: stopTarget.name, count: removed }), 'success');
          }}
        />
      )}
    </div>
  );
}
