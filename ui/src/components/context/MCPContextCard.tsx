import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { mcpCheckApi } from '../../api/mcpCheck';
import AgentIcon from '../AgentIcon';
import { useT } from '../../i18n';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import { groupKey, groupLabel, kTokens } from './contextCost';

/** The MCP page's link to the Context page: the global MCP target whose tool definitions cost most, against the budget, or the heaviest project one when only projects load servers. */
export default function MCPContextCard() {
  const t = useT();
  const { data } = useQuery({
    queryKey: queryKeys.mcpCosts,
    queryFn: () => mcpCheckApi.costs(),
    staleTime: staleTimes.check,
  });
  const groups = data?.groups ?? [];
  const global = groups.filter((g) => !g.root);
  const pool = global.length > 0 ? global : groups;
  if (pool.length === 0) return null;
  const top = pool.reduce((a, b) => (b.tokens > a.tokens ? b : a));
  const budget = data?.budget ?? 0;
  const over = budget > 0 && top.tokens > budget;
  return (
    <div className="ss-box flex flex-col gap-2.5">
      <h3 className="text-[15px] font-semibold">{t('context.mcp.card.title')}</h3>
      {top.measured === 0 ? (
        <p className="text-[13px] text-ink-2">{t('context.mcp.card.notCounted')}</p>
      ) : (
        <>
          <div className="flex items-center gap-2 text-[13px]">
            <AgentIcon target={top.agent} size={15} />
            <span className="flex-1 truncate">{groupLabel(top)}</span>
            <span className={`font-mono ${over ? 'text-warn' : ''}`}>
              {kTokens(top.tokens)}
              {budget > 0 && <span className="text-ink-3"> / {kTokens(budget)}</span>}
            </span>
          </div>
          {budget > 0 && (
            <span className="ss-prog">
              <i style={{ width: `${Math.min(top.tokens / budget, 1) * 100}%` }} />
            </span>
          )}
        </>
      )}
      <Link
        to={`/context?tab=mcp&agent=${encodeURIComponent(groupKey(top))}`}
        className="ss-btn sm self-start"
      >
        {t('context.mcp.card.open')}
      </Link>
    </div>
  );
}
