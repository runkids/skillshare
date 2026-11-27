import { useState } from 'react';
import { createPortal } from 'react-dom';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, AlertTriangle, ArrowDown, Plug, RefreshCw } from 'lucide-react';
import { mcpCheckApi } from '../../api/mcpCheck';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import EmptyState from '../EmptyState';
import { Select } from '../Select';
import Spinner from '../Spinner';
import { plural, useT } from '../../i18n';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import { invalidate } from '../../lib/queryEvents';
import { groupKey, groupLabel, kTokens } from './contextCost';

/** Tool definitions each MCP target loads. MCP targets are a list of their own, not the skill targets. The check button and Agent picker go in the page's `actions` slot. */
export default function MCPCostPanel({ actions }: { actions: HTMLElement | null }) {
  const t = useT();
  const queryClient = useQueryClient();
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.mcpCosts,
    queryFn: () => mcpCheckApi.costs(),
    staleTime: staleTimes.check,
  });
  // The Agent lives in the URL so the Sync and MCP pages can deep-link here.
  const [params, setParams] = useSearchParams();
  const wanted = params.get('agent') ?? '';
  const setAgent = (v: string) =>
    setParams(
      (p) => {
        p.set('agent', v);
        return p;
      },
      { replace: true },
    );
  const [checking, setChecking] = useState(false);
  const [checkError, setCheckError] = useState('');
  const checkLive = async () => {
    setChecking(true);
    setCheckError('');
    try {
      await mcpCheckApi.runLive();
      await invalidate(queryClient, 'mcpToolsCounted');
    } catch (e) {
      setCheckError((e as Error).message);
    } finally {
      setChecking(false);
    }
  };

  if (isPending)
    return (
      <div className="ss-note inf !items-center">
        <Spinner size="sm" />
        {t('context.mcp.loading')}
      </div>
    );
  if (error)
    return (
      <div className="ss-note bad">
        <AlertCircle size={16} />
        <span className="flex-1 break-words">{(error as Error).message}</span>
      </div>
    );

  const groups = data?.groups ?? [];
  if (groups.length === 0)
    return (
      <EmptyState
        icon={Plug}
        title={t('context.mcp.empty.title')}
        description={t('context.mcp.empty.description')}
        action={
          <Link to="/mcp" className="ss-btn">
            {t('context.mcp.openMCP')}
          </Link>
        }
      />
    );

  const budget = data?.budget ?? 0;
  const current = groups.find((g) => groupKey(g) === wanted) ?? groups[0];
  const options = groups.map((g) => ({
    value: groupKey(g),
    label: groupLabel(g),
    icon: <AgentIcon target={g.agent} size={15} />,
  }));
  const over = budget > 0 && current.tokens > budget;
  const tools = current.servers.reduce((n, s) => n + (s.checked ? s.tools : 0), 0);
  const uncounted = current.servers.length - current.measured;
  const heaviest = current.servers[0]?.tokens ?? 0;

  return (
    <div className="flex flex-col gap-5">
      {actions &&
        createPortal(
          <>
            <Button
              variant="secondary"
              onClick={() => void checkLive()}
              disabled={checking}
              title={t('context.mcp.checkHint')}
            >
              <RefreshCw size={15} className={checking ? 'animate-spin' : ''} />
              {t(checking ? 'context.mcp.checking' : 'context.mcp.check')}
            </Button>
            <Select
              value={groupKey(current)}
              onChange={setAgent}
              prefix={t('context.mcp.agent')}
              className="w-[260px]"
              options={options}
            />
          </>,
          actions,
        )}

      {checkError && (
        <div className="ss-note bad">
          <AlertCircle size={16} />
          <span className="flex-1 break-words">{checkError}</span>
        </div>
      )}

      <div className="ss-counts !grid-cols-3">
        <div>
          <span className="flex flex-col gap-[3px]">
            <b className={over ? 'text-warn' : ''}>
              {kTokens(current.tokens)}
              {budget > 0 && (
                <span className="text-[15px] font-normal text-ink-3"> / {kTokens(budget)}</span>
              )}
            </b>
            <span className="text-[13px] text-ink-2">{t('context.mcp.stat.tokens')}</span>
          </span>
        </div>
        <div>
          <span className="flex flex-col gap-[3px]">
            <b>{tools}</b>
            <span className="text-[13px] text-ink-2">{t('context.mcp.stat.tools')}</span>
          </span>
        </div>
        <div>
          <span className="flex flex-col gap-[3px]">
            <b className={uncounted > 0 ? 'text-ink-3' : ''}>
              {current.measured} / {current.servers.length}
            </b>
            <span className="text-[13px] text-ink-2">{t('context.mcp.stat.counted')}</span>
          </span>
        </div>
      </div>

      {over && (
        <div className="ss-note warn">
          <AlertTriangle size={16} />
          <span className="flex-1">
            <b>{t('context.mcp.over')}</b>{' '}
            {t('context.mcp.overDetail', {
              actual: kTokens(current.tokens),
              budget: kTokens(budget),
              agent: groupLabel(current),
            })}
          </span>
        </div>
      )}
      {uncounted > 0 && (
        <div className="ss-note inf">
          <span className="flex-1">
            {t(plural('context.mcp.uncounted', uncounted), { count: uncounted })}
          </span>
        </div>
      )}

      <div className="ss-list">
        <div className="ss-lh">
          <span className="w-[34px]" />
          <span className="flex-1">{t('context.mcp.col.server')}</span>
          <span className="w-[110px]">{t('context.mcp.col.tools')}</span>
          <span className="flex w-[240px] items-center gap-1.5">
            {t('context.mcp.col.tokens')}
            <ArrowDown size={13} />
          </span>
        </div>
        {current.servers.map((s) => (
          <div key={`${s.project ?? ''}/${s.name}`} className="ss-r">
            <span className="ss-cat sm">
              <Plug size={14} />
            </span>
            <span className="nm m truncate">{s.name}</span>
            <span className="flex-1" />
            {s.checked ? (
              <>
                <span className="w-[110px] font-mono text-[13px]">{s.tools}</span>
                <span className="flex w-[240px] items-center gap-2.5">
                  <span className="ss-prog flex-1">
                    <i style={{ width: `${heaviest ? (s.tokens / heaviest) * 100 : 0}%` }} />
                  </span>
                  <span className="w-[38px] text-right font-mono text-[13px]">
                    {kTokens(s.tokens)}
                  </span>
                </span>
              </>
            ) : (
              <span className="flex w-[350px] justify-end">
                <span className="ss-tag">{t('context.mcp.notCounted')}</span>
              </span>
            )}
          </div>
        ))}
      </div>

      <p className="text-[13px] leading-relaxed text-ink-3">{t('context.mcp.footnote')}</p>
    </div>
  );
}
