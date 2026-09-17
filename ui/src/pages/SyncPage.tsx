import { Fragment, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ArrowDownToLine, Bot, ChevronDown, ChevronRight, CircleCheck, CircleMinus, EyeOff, FolderPlus, Gauge, Minus, Plug, Plus, Puzzle, RefreshCw, TriangleAlert } from 'lucide-react';
import { api, formatTokenK, type SyncResponse } from '../api/client';
import { mcpApi } from '../api/mcp';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import CollectDialog from '../components/CollectDialog';
import { Checkbox } from '../components/Input';
import PageHeader from '../components/PageHeader';
import Spinner from '../components/Spinner';
import { useToast } from '../components/Toast';
import { describeMessage, targetLabel } from '../components/mcp/mcpView';
import { countChanges, countEdited, extraGroups, MCP_CHANGED, mcpGroups, resourceGroups, runSync, type ChangeGroup, type Part, type RowIcon } from '../components/sync/syncView';
import { refreshTargets } from '../components/targets/targetView';
import { formatDateTime, formatRelativeTime, useI18n, useT } from '../i18n';
import { shortenHome } from '../lib/paths';
import { formatAgentDisplayName } from '../lib/resourceNames';
import { queryKeys, staleTimes } from '../lib/queryKeys';

const ROW_ICON: Record<RowIcon, React.ReactNode> = {
  add: <Plus size={16} className="shrink-0 text-ok" />,
  update: <RefreshCw size={15} className="shrink-0 text-info" />,
  remove: <Minus size={16} className="shrink-0 text-bad" />,
  kept: <CircleMinus size={15} className="shrink-0 text-ink-3" />,
  conflict: <TriangleAlert size={15} className="shrink-0 text-warn" />,
};
const PART_ICON: Record<Part, React.ReactNode> = { skill: <Puzzle size={14} />, agent: <Bot size={14} />, extra: <FolderPlus size={14} />, mcp: <Plug size={14} /> };
const PART_LABEL: Record<Part, string> = { skill: 'Skills', agent: 'Agents', extra: 'Extras', mcp: 'MCP' };
const PARTS = Object.keys(PART_LABEL) as Part[];

export default function SyncPage() {
  const t = useT();
  const { locale } = useI18n();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const targets = useQuery({ queryKey: queryKeys.targets.all, queryFn: () => api.listTargets(), staleTime: staleTimes.targets });
  const diff = useQuery({ queryKey: queryKeys.diff(), queryFn: () => api.diff(), staleTime: staleTimes.diff });
  const extras = useQuery({ queryKey: queryKeys.extrasDiff(), queryFn: () => api.diffExtras(), staleTime: staleTimes.extras });
  const mcp = useQuery({ queryKey: queryKeys.mcp, queryFn: () => mcpApi.list(), staleTime: staleTimes.extras });
  const log = useQuery({ queryKey: queryKeys.log('ops', 20, { cmd: 'sync' }), queryFn: () => api.listLog('ops', 20, { cmd: 'sync' }), staleTime: staleTimes.log });

  const [off, setOff] = useState<Set<Part>>(new Set());
  const [force, setForce] = useState(false);
  const [open, setOpen] = useState<Set<string>>(new Set());
  const [collecting, setCollecting] = useState(false);
  const [running, setRunning] = useState(false);
  const [runError, setRunError] = useState('');
  const [outcome, setOutcome] = useState<SyncResponse | null>(null);

  const plan = mcp.data?.plan;
  const parts = new Set(PARTS.filter((p) => !off.has(p)));
  const diffs = diff.data?.diffs ?? [];
  const resources = resourceGroups(diffs, targets.data?.targets ?? [], parts, force, { skill: diff.data?.ignored_skills, agent: diff.data?.agent_ignored_skills });
  const mcpShown = parts.has('mcp') ? mcpGroups(plan) : [];
  const groups: ChangeGroup[] = [...resources.groups, ...(parts.has('extra') ? extraGroups(extras.data?.extras ?? [], force) : []), ...mcpShown];
  // A blocked plan applies nothing, so its rows are shown but not counted.
  const count = countChanges(groups) - (plan?.blocked ? countChanges(mcpShown) : 0);
  const edited = countEdited(groups);
  const loading = diff.isPending || targets.isPending;

  const local = diffs.flatMap((d) => (d.items ?? []).filter((i) => i.action === 'local' && parts.has(i.kind === 'agent' ? 'agent' : 'skill')));
  const ignored = [...(parts.has('skill') ? diff.data?.ignored_skills ?? [] : []), ...(parts.has('agent') ? diff.data?.agent_ignored_skills ?? [] : [])];
  const skipped = parts.has('skill') ? diffs.filter((d) => (d.skippedCount ?? 0) > 0) : [];
  const last = log.data?.entries.find((e) => !e.args?.dry_run);

  const toggle = (set: Set<string>, key: string) => { const next = new Set(set); if (next.has(key)) next.delete(key); else next.add(key); return next; };

  const sync = async () => {
    setRunning(true);
    setRunError('');
    setOutcome(null);
    try {
      const { resources: result } = await runSync({
        resources: parts.has('skill') && parts.has('agent') ? 'both' : parts.has('skill') ? 'skill' : parts.has('agent') ? 'agent' : null,
        extras: parts.has('extra') && !!extras.data?.extras.length,
        mcp: parts.has('mcp') && plan && !plan.blocked && plan.changes.some((c) => c.action !== 'unchanged') ? plan : null,
        force,
      });
      setOutcome(result ?? null);
      toast(t('sync.toast.done'), 'success');
    } catch (err) {
      const message = (err as Error).message;
      setRunError(message === MCP_CHANGED ? t('sync.mcpChanged') : message);
    } finally {
      setRunning(false);
      refreshTargets(queryClient);
      for (const queryKey of [queryKeys.extrasDiff(), queryKeys.extras, queryKeys.mcp, ['log']]) void queryClient.invalidateQueries({ queryKey });
    }
  };

  const groupHead = (g: ChangeGroup) => {
    const n = countChanges([g]);
    return (
      <div className="ss-gh">
        {g.part === 'extra' ? <span className="ss-cat sm extra">{PART_ICON.extra}</span> : <span className="ss-at"><AgentIcon target={g.name} size={17} /></span>}
        <span className="font-semibold">{g.part === 'mcp' ? targetLabel(g.name) : g.name}</span>
        <span className="ss-tag">{g.part === 'mcp' ? 'MCP' : g.mode}</span>
        {g.path && <span className="min-w-0 truncate font-mono text-[12px] text-ink-3" title={g.path}>{shortenHome(g.path)}</span>}
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
          {parts.has('mcp') && mcp.data?.previewError && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{mcp.data.previewError}</span></div>}
          {outcome?.warnings?.map((w) => <div key={w} className="ss-note warn"><TriangleAlert size={16} /><span className="flex-1">{w}</span></div>)}

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
                        <span className={`min-w-0 flex-1 truncate text-[13px] ${r.icon === 'conflict' ? 'text-warn' : 'text-ink-2'}`} title={r.detail}>
                          {r.text ? t(r.text) : r.part === 'mcp' ? describeMessage(t, r.detail) : r.detail}
                        </span>
                      </div>
                    ))}
                  </Fragment>
                ))}
                {resources.inSync.length > 0 && (
                  <button type="button" className="ss-gh w-full text-left" aria-expanded={open.has('inSync')} onClick={() => setOpen((s) => toggle(s, 'inSync'))}>
                    <span className="ss-stack ml-1.5">{resources.inSync.slice(0, 4).map((name) => <span key={name} className="ss-at"><AgentIcon target={name} size={14} /></span>)}</span>
                    <span className="font-semibold">{t(resources.inSync.length === 1 ? 'sync.inSync.one' : 'sync.inSync.other', { count: resources.inSync.length })}</span>
                    <span className="flex-1" />
                    {open.has('inSync') ? <ChevronDown size={15} className="text-ink-3" /> : <ChevronRight size={15} className="text-ink-3" />}
                  </button>
                )}
                {open.has('inSync') && <div className="ss-r text-[13px] text-ink-2">{resources.inSync.join(', ')}</div>}
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
                      <span><b>{t(ignored.length === 1 ? 'sync.ignored.one' : 'sync.ignored.other', { count: ignored.length })}</b> <span className="text-ink-2">{t('sync.ignored.by')}</span></span>
                    </button>
                    <Link to="/config" className="shrink-0 font-semibold">{t('sync.ignored.edit')}</Link>
                  </div>
                  {open.has('ignored') && <div className="ss-r pl-[62px] font-mono text-[12.5px] text-ink-2">{ignored.join(', ')}</div>}
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
                  {open.has('local') && <div className="ss-r pl-[62px] font-mono text-[12.5px] text-ink-2">{[...new Set(local.map((i) => (i.kind === 'agent' ? formatAgentDisplayName(i.skill) : i.skill)))].join(', ')}</div>}
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
                {typeof last.ms === 'number' && <><dt>{t('sync.last.took')}</dt><dd className="font-mono">{(last.ms / 1000).toFixed(1)} s</dd></>}
                {last.msg && <><dt>{t('sync.last.error')}</dt><dd className="break-words text-bad">{last.msg}</dd></>}
              </dl>
            ) : (
              <p className="text-[13px] text-ink-2">{t('sync.last.never')}</p>
            )}
            <Link to="/log" className="ss-btn sm">{t('sync.last.openLog')}</Link>
          </div>
          <p className="mt-3.5 px-1 text-[13px] leading-relaxed text-ink-3">{t('sync.backupNote')} <Link to="/backup" className="font-semibold">{t('sync.backupLink')}</Link></p>
          <p className="ss-hand ss-only-playful ml-2 mt-[18px] max-w-[150px]">{t('sync.handNote')}</p>
        </aside>
      </div>

      {collecting && <CollectDialog onClose={() => setCollecting(false)} />}
    </div>
  );
}
