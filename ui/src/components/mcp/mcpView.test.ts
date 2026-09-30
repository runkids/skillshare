import { describe, expect, it } from 'vitest';
import type { MCPPlan } from '../../api/mcp';
import { buildMatrix, describeError, describeMessage, canImportConflict, groupByFile, isResolvable, joinCommand, mcpClient, parsePiOptions, serverCount, splitCommand, switchTargets, targetLabel } from './mcpView';
import { mcpTargets } from '../../api/mcp';

const change = (name: string, target: string, action: string, message?: string) => ({ name, target, action, message, path: `/${target}.json` });

describe('MCP view helpers', () => {
  it('gives the agy skills target the MCP file it shares with Antigravity', () => {
    expect(mcpClient('antigravity-cli')).toBe('antigravity');
  });

  it('includes Antigravity in the shared matrix and dialog targets', () => {
    expect(mcpTargets).toContain('antigravity');
    expect(targetLabel('antigravity')).toBe('Antigravity');
  });
  it('keeps source servers and adds rows for entries only the plan removes', () => {
    const plan: MCPPlan = { revision: 'r', sourcePath: '/c.yaml', blocked: false, changes: [change('docs', 'claude', 'unchanged'), change('old', 'codex', 'remove')] };
    const rows = buildMatrix({ docs: { url: 'https://docs.example/mcp' } }, plan);
    expect(rows.map(row => [row.name, Boolean(row.server), Object.keys(row.cells)])).toEqual([['docs', true, ['claude']], ['old', false, ['codex']]]);
  });

  it('keeps a Pi migration pending when its current file is already unchanged', () => {
    const plan: MCPPlan = { revision: 'r', sourcePath: '/c.yaml', blocked: false, changes: [
      { ...change('docs', 'pi', 'remove'), path: '/pi/mcp-adapter.json' },
      { ...change('docs', 'pi', 'unchanged'), path: '/pi/mcp.json' },
    ] };
    expect(buildMatrix({ docs: { command: 'docs' } }, plan)[0].cells.pi.action).toBe('remove');
  });

  // A key that stops short of the whole sentence leaves the rest of the English message
  // appended to its translation, which only shows up in a locale that is not English.
  it('translates all of a conflict message and keeps only the path', () => {
    const t = (key: string) => `[${key}]`;
    expect(describeMessage(t, 'left over from a Skillshare config that was removed; import it or explicitly replace this entry: /gone/config.yaml'))
      .toBe('[mcp.conflictOrphaned]: /gone/config.yaml');
    expect(describeMessage(t, 'managed by another Skillshare config: /live/config.yaml'))
      .toBe('[mcp.conflictOtherConfig]: /live/config.yaml');
  });

  it('translates the Kilo project environment error and preserves its project path', () => {
    const t = (key: string, params?: Record<string, string>) => `[${key}:${params?.name}]`;
    expect(describeError(t, 'D:\\project: Kilo Code MCP mcp-test: Kilo does not allow environment references in project config and ignores the whole file when it finds one; remove fromEnv here or define this server in global mode'))
      .toBe('D:\\project: [mcp.kilocodeProjectEnv:mcp-test]');
  });

  // A pasted snippet is parsed by the server, whose errors are English and talk about a target file.
  it('translates what the server says about a snippet it cannot read', () => {
    const t = (key: string) => `[${key}]`;
    expect(describeError(t, 'invalid JSON/JSONC; target was not changed')).toBe('[mcp.importError.json]');
    expect(describeError(t, 'invalid TOML; target was not changed')).toBe('[mcp.importError.toml]');
    expect(describeError(t, 'no MCP entries found; select the matching client format')).toBe('[mcp.importError.noEntries]');
    expect(describeError(t, 'something else')).toBe('something else');
  });

  // The owner messages end in a path, so matching them needs the same prefix search
  // describeMessage uses. A live owner keeps no buttons: only it can release the entry.
  it('only offers import or replace for conflicts the source can take over', () => {
    expect([
      change('a', 'claude', 'conflict', 'Agent configuration changed; import it or explicitly replace this entry'),
      change('a', 'claude', 'conflict', 'existing entry is not managed; import it to explicitly adopt it'),
      change('a', 'claude', 'conflict', 'left over from a Skillshare config that was removed; import it or explicitly replace this entry: /gone/.skillshare/config.yaml'),
      change('a', 'claude', 'conflict', 'managed by another Skillshare config: /live/.skillshare/config.yaml'),
    ].map(isResolvable)).toEqual([true, true, true, false]);
  });

  it('round-trips quoted command arguments', () => {
    const words = splitCommand(`npx -y @scope/server "~/My Notes" --label='a b' ''`);
    expect(words).toEqual(['npx', '-y', '@scope/server', '~/My Notes', '--label=a b', '']);
    expect(splitCommand(joinCommand(words))).toEqual(words);
  });
});

describe('switchTargets', () => {
  const context7 = { command: 'npx', targets: ['claude', 'opencode', 'kilocode', 'pi'] };

  // Pi has no per-project switch, so it is never among them.
  it('turns a global server off only for the Agents the project uses', () => {
    expect(switchTargets(context7, [], ['opencode', 'pi'])).toEqual(['opencode']);
  });

  it('follows the default targets of a server that names none', () => {
    expect(switchTargets({ command: 'npx' }, ['claude', 'cursor'], ['claude', 'cursor'])).toEqual(['claude']);
  });
});

describe('groupByFile', () => {
  it("keeps a project's Claude off list apart from the servers in the same file", () => {
    const groups = groupByFile([
      { target: 'claude', path: '/home/u/.claude.json', name: 'context7', action: 'unchanged' },
      { target: 'claude', path: '/home/u/.claude.json', name: 'context7', root: '/work/app', action: 'remove' },
      { target: 'opencode', path: '/work/app/opencode.json', name: 'context7', root: '/work/app', action: 'add' },
    ]);
    expect(groups.map((g) => g.offListFor)).toEqual([undefined, '/work/app', undefined]);
  });
});

describe('canImportConflict', () => {
  // Import reads the Agent's global file, which holds the server and never the project's switch.
  it('offers no import for a changed switch, which only a replace can settle', () => {
    const changed = { target: 'opencode', path: '/work/app/opencode.json', name: 'docs', action: 'conflict', message: 'Agent configuration changed; import it or explicitly replace this entry' };
    expect([canImportConflict(changed), canImportConflict({ ...changed, root: '/work/app', switch: true })]).toEqual([true, false]);
  });
});

describe('parsePiOptions', () => {
  it('reads a JSON object and treats an empty box as nothing set', () => {
    expect(parsePiOptions('{"excludeTools": ["a"]}')).toEqual({ value: { excludeTools: ['a'] } });
    expect(parsePiOptions('  ')).toEqual({});
  });

  it('refuses anything that is not a JSON object', () => {
    for (const text of ['{', '["a"]', '3', 'null']) expect(parsePiOptions(text)).toEqual({ invalid: true });
  });

  it('names a field Skillshare writes itself', () => {
    expect(parsePiOptions('{"excludeTools": [], "directTools": true}')).toEqual({ taken: 'directTools' });
  });

  it('counts the servers an Agent gets, inherited ones too, but not a switch that turns one off', () => {
    const servers = { own: { command: 'a', targets: ['claude'] }, inherited: { command: 'b' }, off: { disabled: true, targets: ['claude'] } };
    expect(serverCount({ source: { servers, targets: ['claude'] }, paths: { claude: '/c.json' } }, 'claude')).toBe(2);
  });
});
