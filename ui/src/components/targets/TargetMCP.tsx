import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { AlertCircle, ArrowRight, ChevronRight, Plug, RefreshCw } from 'lucide-react';
import { mcpOffTargets, type MCPPlan, type MCPServer, type mcpApi } from '../../api/mcp';
import Button from '../Button';
import EmptyState from '../EmptyState';
import { MCPSyncDialog } from '../mcp/MCPSyncBox';
import { describeEndpoint, describeMessage, mcpOrder, reachOf, serverCount, writes } from '../mcp/mcpView';
import { useMCPToggle } from '../mcp/useMCPToggle';
import { useT } from '../../i18n';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;

/** One Agent's column of the MCP page, laid out like the Skills tab: a row per server of the current scope, global or project. */
export default function TargetMCP({ name, data }: { name: string; data: MCPList }) {
  const t = useT();
  const navigate = useNavigate();
  // Each save previews first and sends that revision, so a second click waits for the first.
  const [busy, setBusy] = useState(false);
  // Held while the dialog is open, so the list it confirms stays put when the queries refresh.
  const [reviewing, setReviewing] = useState<{ plan: MCPPlan; shown: number } | null>(null);
  const order = mcpOrder(data.source.accounts);
  const toggle = useMCPToggle(order);
  const plan = data.plan;
  const servers = data.source.servers ?? {};
  const defaults = data.source.targets ?? [];
  // Changes under mcp.projects belong to the Projects page. A project's own Claude off list
  // carries its root too, so only those roots are left out.
  const roots = Object.keys(data.source.projects ?? {});
  const changes = (plan?.changes ?? []).filter((c) => c.target === name && !(c.root && roots.includes(c.root)));
  const conflicts = changes.filter((c) => c.action === 'conflict');
  const mine = changes.filter(writes).length;
  // Sync writes the whole plan, so the button counts every target's changes.
  const all = (plan?.changes ?? []).filter(writes).length;
  // A switch-only entry turns a server off, which only a few Agents can do per project. A server
  // gone from the source keeps its row while the plan still takes it out of this Agent.
  const rows: [string, MCPServer | undefined][] = [
    ...Object.entries(servers).filter(([, s]) => !s.disabled || mcpOffTargets.includes(name)),
    ...[...new Set(changes.map((c) => c.name))].filter((n) => !servers[n]).map((n): [string, undefined] => [n, undefined]),
  ];
  // Switches and servers on their way out write no server here.
  const total = rows.filter(([, s]) => s && !s.disabled).length;

  if (rows.length === 0) {
    return <EmptyState icon={Plug} title={t('targetDetail.mcp.emptyTitle')} description={t('targetDetail.mcp.emptyDescription', { name })} action={<Link to="/mcp" className="ss-btn pri">{t('targetDetail.mcp.open')}</Link>} />;
  }

  const select = (server: string, on: boolean) => {
    setBusy(true);
    void toggle(server, name, on).finally(() => setBusy(false));
  };

  return (
    <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] items-start gap-12">
      <section className="flex flex-col gap-5">
        <h2 className="ss-h2">{t('targetDetail.whatSyncs')}</h2>
        <p className="text-[13.5px]">
          {t(`targetDetail.mcp.summary.${total === 1 ? 'one' : 'other'}`, { synced: serverCount(data, name), total, name })}
        </p>
        {conflicts.map((c) => (
          <div key={`${c.path}:${c.root ?? ''}:${c.name}`} className="ss-note warn">
            <AlertCircle size={16} />
            <span className="flex-1"><span className="font-mono">{c.name}</span>: {describeMessage(t, c.message)}</span>
          </div>
        ))}
        <div className="flex flex-col gap-2">
          <div className="ss-list !shadow-none">
            <div className="ss-lh">
              <span className="flex-1">{t('targetDetail.previewCount', { count: rows.length })}</span>
              <span className="w-[170px]">{t('targetDetail.mcp.endpoint')}</span>
              <span className="w-[96px]">{t('targetDetail.result')}</span>
            </div>
            {rows.map(([server, s]) => {
              // Where a switch that names no targets goes, sync works out partly from the global config
              // this scope cannot read; the plan says it outright.
              const on = !s ? false : s.disabled && !s.targets && plan ? changes.some((c) => c.name === server && c.action !== 'remove') : reachOf(s, defaults).includes(name);
              const http = Boolean(s?.url);
              const endpoint = !s ? t('mcp.removedFromSource') : s.disabled ? t('mcp.offHere') : `${http ? 'http' : 'stdio'} · ${describeEndpoint(s)}`;
              const cells = (
                <>
                  <span className="flex min-w-0 flex-1 items-center gap-2">
                    <span title={server} className={`truncate font-mono text-[13px] font-semibold ${!s ? 'text-ink-3 line-through' : on ? '' : 'text-ink-2'}`}>{server}</span>
                    {changes.some((c) => c.name === server && writes(c)) && <span className="ss-tag warn shrink-0">{t('plugins.pending')}</span>}
                  </span>
                  <span className="w-[170px] shrink-0 truncate font-mono text-[12px] text-ink-2" title={endpoint}>{endpoint}</span>
                  {/* A switch that applies here turns the server off, so it is not a server written in. */}
                  <span className="w-[96px] shrink-0"><span className={`ss-st ${!on ? 'off' : s?.disabled ? 'warn' : 'ok'}`}>{t(on ? 'targetDetail.synced' : 'targetDetail.notSynced')}</span></span>
                </>
              );
              // Claude Desktop reads stdio servers only; a server gone from the source has nothing to select.
              return !s || (name === 'claude-desktop' && http && !on) ? (
                <div key={server} className="ss-r !min-h-[42px]">{cells}</div>
              ) : (
                <button key={server} type="button" role="switch" aria-checked={on} aria-label={server} disabled={busy} className="ss-r link !min-h-[42px] w-full text-left" onClick={() => select(server, !on)}>
                  {cells}
                </button>
              );
            })}
          </div>
          <p className="text-[13px] text-ink-3">{t('targetDetail.mcp.clickHint', { name })}</p>
        </div>
      </section>

      <aside className="flex flex-col gap-7">
        {plan && (
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between gap-2">
              <h2 className="ss-h2">{t('sync.title')}</h2>
              <span className={`ss-st ${mine > 0 || conflicts.length > 0 ? 'warn' : 'ok'}`}>{mine > 0 ? t(mine === 1 ? 'mcp.pending.one' : 'mcp.pending.other', { count: mine }) : conflicts.length > 0 ? t('mcp.status.conflict') : t('targets.state.synced')}</span>
            </div>
            {mine > 0 && (
              <>
                <p className="text-[13px] text-ink-2">{t(all === 1 ? 'targetDetail.mcp.syncHint.one' : 'targetDetail.mcp.syncHint.other', { count: all })}</p>
                {/* A blocked plan applies nothing; the Sync page shows why. */}
                {plan.blocked
                  ? <Button variant="secondary" className="self-start" onClick={() => navigate('/sync')}>{t('mcp.reviewInSync')}<ChevronRight size={15} /></Button>
                  : <Button variant="secondary" className="self-start" onClick={() => setReviewing({ plan, shown: all })}><RefreshCw size={15} />{t('targetDetail.mcp.syncAll')}</Button>}
              </>
            )}
          </div>
        )}
        <div className="flex flex-col gap-3">
          <h2 className="ss-h2">{t('targetDetail.mcp.manage')}</h2>
          <p className="text-[13px] text-ink-2">{t('targetDetail.mcp.manageHint', { name })}</p>
          <Link to="/mcp" className="ss-btn self-start">{t('targetDetail.mcp.open')}<ArrowRight size={14} /></Link>
        </div>
      </aside>

      {reviewing && <MCPSyncDialog {...reviewing} onClose={() => setReviewing(null)} />}
    </div>
  );
}
