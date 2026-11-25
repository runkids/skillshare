import type { MCPToolCostGroup } from '../../api/mcpCheck';
import { shortenHome } from '../../lib/paths';
import { targetLabel } from '../mcp/mcpView';

/** Thousands collapse to "3.4k" so the column stays one glance wide; smaller counts stay exact. */
export const kTokens = (n: number) =>
  n < 1000
    ? n.toLocaleString()
    : n >= 10000
      ? `${Math.round(n / 1000)}k`
      : `${(n / 1000).toFixed(1)}k`;

/** A group is one MCP target in one place: `root@target` for an mcp.projects root, the target alone for the global list. */
export const groupKey = (g: Pick<MCPToolCostGroup, 'root' | 'target'>) =>
  g.root ? `${g.root}@${g.target}` : g.target;

export const groupLabel = (g: MCPToolCostGroup) =>
  g.root ? `${targetLabel(g.target)} · ${shortenHome(g.root)}` : targetLabel(g.target);
