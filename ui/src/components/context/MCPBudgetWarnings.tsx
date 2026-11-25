import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Plug } from 'lucide-react';
import { mcpCheckApi } from '../../api/mcpCheck';
import { useT } from '../../i18n';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import { groupKey, groupLabel, kTokens } from './contextCost';

/** One warning per MCP target whose counted tool definitions exceed warn_mcp_tool_tokens. */
export default function MCPBudgetWarnings() {
  const t = useT();
  const { data } = useQuery({
    queryKey: queryKeys.mcpCosts,
    queryFn: () => mcpCheckApi.costs(),
    staleTime: staleTimes.check,
  });
  const budget = data?.budget ?? 0;
  const over = budget > 0 ? (data?.groups ?? []).filter((g) => g.tokens > budget) : [];
  return over.map((g) => (
    <div key={groupKey(g)} className="ss-note warn !items-center">
      <Plug size={16} />
      <span className="flex-1">
        <b>{t('context.mcp.over')}</b>{' '}
        {t('context.mcp.overDetail', {
          actual: kTokens(g.tokens),
          budget: kTokens(budget),
          agent: groupLabel(g),
        })}{' '}
        {t('sync.budget.mcpBiggest', {
          names: g.servers
            .filter((s) => s.checked)
            .slice(0, 3)
            .map((s) => `${s.name} ${kTokens(s.tokens)}`)
            .join(', '),
        })}
      </span>
      <Link to={`/context?tab=mcp&agent=${encodeURIComponent(groupKey(g))}`} className="ss-btn sm">
        {t('sync.budget.analyze')}
      </Link>
    </div>
  ));
}
