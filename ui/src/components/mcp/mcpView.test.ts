import { describe, expect, it } from 'vitest';
import type { MCPPlan } from '../../api/mcp';
import { buildMatrix, dayLabel, describeCredentials, groupBackupsByDay, isResolvable } from './mcpView';

const change = (name: string, target: string, action: string, message?: string) => ({ name, target, action, message, path: `/${target}.json` });

describe('MCP view helpers', () => {
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

  it('shows credential references without literal values', () => {
    expect(describeCredentials({ command: 'npx', env: { MODE: 'fast', TOKEN: { fromEnv: 'GH_TOKEN' } }, bearerToken: { fromEnv: 'DOCS' } }))
      .toEqual(['Bearer ← $DOCS', 'MODE', 'TOKEN ← $GH_TOKEN']);
  });

  it('groups backups by the day encoded in their nanosecond IDs', () => {
    const at = (iso: string) => `${BigInt(Date.parse(iso)) * 1_000_000n}-abcd1234`;
    const days = groupBackupsByDay([{ id: at('2026-09-17T10:56:00') }, { id: at('2026-09-17T08:55:00') }, { id: at('2026-09-16T18:03:00') }]);
    expect(days.map(day => day.backups.length)).toEqual([2, 1]);
    expect(dayLabel(days[1].date, 'en', new Date('2026-09-17T12:00:00'))).toBe('yesterday · September 16');
  });
});
