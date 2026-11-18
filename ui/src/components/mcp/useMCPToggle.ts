import { useQueryClient } from '@tanstack/react-query';
import { mcpApi } from '../../api/mcp';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { describeError, reachOf } from './mcpView';
import { invalidate } from '../../lib/queryEvents';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;

/** Selects or clears one Agent for one server. `order` is every target in display order. */
export function useMCPToggle(order: readonly string[]) {
  const t = useT();
  const { toast } = useToast();
  const cache = useQueryClient();
  return async (name: string, target: string, on: boolean) => {
    const prev = cache.getQueryData<MCPList>(queryKeys.mcp);
    if (!prev) return;
    const current = reachOf(prev.source.servers[name], prev.source.targets ?? []);
    const next = order.filter((x) => (x === target ? on : current.includes(x)));
    // A switch-only entry needs an Agent to turn the server off for; a server may have none.
    if (next.length === 0 && prev.source.servers[name].disabled)
      return toast(t('mcp.needTarget'), 'warning');
    const server = { ...prev.source.servers[name], targets: next };
    // Tick right away; saving only touches the source, Sync writes the files
    cache.setQueryData<MCPList>(
      queryKeys.mcp,
      (old) =>
        old && {
          ...old,
          source: { ...old.source, servers: { ...old.source.servers, [name]: server } },
        },
    );
    try {
      await mcpApi.save({ name, server, replace: true });
      // Not blocked, but said out loud: the next sync takes the server out of every Agent.
      if (next.length === 0) toast(t('mcp.noTargetsToast', { name }), 'info');
    } catch (e) {
      cache.setQueryData(queryKeys.mcp, prev);
      toast(describeError(t, (e as Error).message), 'error');
    }
    void invalidate(cache, 'mcpChanged');
  };
}
