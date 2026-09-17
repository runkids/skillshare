import { describe, expect, it } from 'vitest';
import type { MCPPlan } from '../../api/mcp';
import { buildMatrix, isResolvable, joinCommand, splitCommand, targetLabel } from './mcpView';
import { mcpTargets } from '../../api/mcp';

const change = (name: string, target: string, action: string, message?: string) => ({ name, target, action, message, path: `/${target}.json` });

describe('MCP view helpers', () => {
  it('includes Antigravity in the shared matrix and dialog targets', () => {
    expect(mcpTargets).toContain('antigravity');
    expect(targetLabel('antigravity')).toBe('Antigravity');
  });
  it('keeps source servers and adds rows for entries only the plan removes', () => {
    const plan: MCPPlan = { revision: 'r', sourcePath: '/c.yaml', blocked: false, changes: [change('docs', 'claude', 'unchanged'), change('old', 'codex', 'remove')] };
    const rows = buildMatrix({ docs: { url: 'https://docs.example/mcp' } }, plan);
    expect(rows.map(row => [row.name, Boolean(row.server), Object.keys(row.cells)])).toEqual([['docs', true, ['claude']], ['old', false, ['codex']]]);
  });

  it('only offers import or replace for conflicts the source can take over', () => {
    expect([
      change('a', 'claude', 'conflict', 'Agent configuration changed; import it or explicitly replace this entry'),
      change('a', 'claude', 'conflict', 'existing entry is not managed; import it to explicitly adopt it'),
      change('a', 'claude', 'conflict', 'managed by another Skillshare config'),
    ].map(isResolvable)).toEqual([true, true, false]);
  });

  it('round-trips quoted command arguments', () => {
    const words = splitCommand(`npx -y @scope/server "~/My Notes" --label='a b' ''`);
    expect(words).toEqual(['npx', '-y', '@scope/server', '~/My Notes', '--label=a b', '']);
    expect(splitCommand(joinCommand(words))).toEqual(words);
  });
});
