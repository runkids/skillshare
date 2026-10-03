import { describe, expect, it } from 'vitest';
import type { MCPPlan } from '../../api/mcp';
import { buildMatrix, denyRemovesAll, describeError, describeMessage, canImportConflict, groupByFile, isResolvable, joinCommand, mcpClient, mcpNotice, parsePiOptions, parseToolNotice, serverCount, setToolChecked, splitCommand, switchTargets, targetLabel, toolRules, toolSummary } from './mcpView';
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

  // Pi's project override has no server to import, so only replace is offered.
  it("offers only replace for Pi's project override", () => {
    const override = change('direct', 'pi', 'conflict', 'existing entry is a Pi project override of a global server; replace it, or remove the override with /mcp in Pi');
    expect([isResolvable(override), canImportConflict(override)]).toEqual([true, false]);
  });

  it('round-trips quoted command arguments', () => {
    const words = splitCommand(`npx -y @scope/server "~/My Notes" --label='a b' ''`);
    expect(words).toEqual(['npx', '-y', '@scope/server', '~/My Notes', '--label=a b', '']);
    expect(splitCommand(joinCommand(words))).toEqual(words);
  });
});

describe('switchTargets', () => {
  const context7 = { command: 'npx', targets: ['claude', 'opencode', 'kilocode', 'pi'] };

  it('turns a global server off only for the Agents the project uses', () => {
    expect(switchTargets(context7, [], ['opencode', 'pi'])).toEqual(['opencode', 'pi']);
  });

  // Pi's switch carries the global server's command or url, so a switch entry alone cannot reach it.
  it('leaves Pi out for a switch entry, which has no command or url', () => {
    expect(switchTargets({ disabled: true }, ['opencode', 'pi'], ['opencode', 'pi'])).toEqual(['opencode']);
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

describe('denyRemovesAll', () => {
  it('matches the server: only named tools that Deny all removes leave nothing', () => {
    expect(denyRemovesAll({ allow: ['delete_issue', 'delete_repo'], deny: ['delete_*'] })).toBe(true);
    expect(denyRemovesAll({ allow: ['delete_issue', 'search'], deny: ['delete_*'] })).toBe(false);
    expect(denyRemovesAll({ allow: ['get_*'], deny: ['get_*'] })).toBe(false);
    expect(denyRemovesAll({ deny: ['*'] })).toBe(false);
  });
});

describe('toolSummary', () => {
  const t = (key: string, params?: Record<string, string | number>) => `${key} ${JSON.stringify(params)}`;

  it('says in words how many tools a policy allows or excludes', () => {
    expect([toolSummary(t, { deny: ['a', 'b'] }), toolSummary(t, { allow: ['a'] }), toolSummary(t, { allow: ['a', 'b', 'c'], deny: ['d'] })]).toEqual([
      'mcp.tools.summaryDeny.other {"count":2}', 'mcp.tools.summaryAllow.one {"count":1}', 'mcp.tools.summaryBoth {"allow":3,"deny":1}',
    ]);
  });
});

describe('tool checklist', () => {
  it('unticking a tool denies it by its exact name', () => {
    expect(setToolChecked({ deny: ['delete_*'] }, 'search', false)).toEqual({ deny: ['delete_*', 'search'] });
  });

  it('ticking a tool again drops its own Deny entry', () => {
    expect(setToolChecked({ deny: ['search', 'delete_*'] }, 'search', true)).toEqual({ deny: ['delete_*'] });
  });

  it('ticking a tool that a non-empty Allow leaves out allows it by name', () => {
    expect(setToolChecked({ allow: ['get_*'] }, 'search', true)).toEqual({ allow: ['get_*', 'search'], deny: [] });
  });

  it('keeps patterns and names the server does not list as rules, not the names a row shows', () => {
    expect(toolRules({ allow: ['get_*', 'search'], deny: ['fetch', 'delete_*', 'gone'] }, ['fetch', 'search'])).toEqual({ allow: ['get_*'], deny: ['delete_*', 'gone'] });
  });
});

describe('parseToolNotice', () => {
  it('takes apart a tool policy notice, keeping a project server with its root', () => {
    expect(parseToolNotice('tool policy not applied for codex: allow patterns, deny (docs, wiki (/work/app))'))
      .toEqual({ target: 'codex', gaps: ['allow patterns', 'deny'], names: ['docs', 'wiki (/work/app)'] });
    expect(parseToolNotice('piExtension is ignored since 0.23.0 (docs)')).toBeUndefined();
  });
});

describe('parsePiOptions', () => {
  it('reads a JSON object and treats an empty box as nothing set', () => {
    expect(parsePiOptions('{"timeout": 120}')).toEqual({ value: { timeout: 120 } });
    expect(parsePiOptions('  ')).toEqual({});
  });

  it('refuses anything that is not a JSON object', () => {
    for (const text of ['{', '["a"]', '3', 'null']) expect(parsePiOptions(text)).toEqual({ invalid: true });
  });

  it('names a field Skillshare writes itself', () => {
    expect(parsePiOptions('{"timeout": 1, "command": "x"}')).toEqual({ taken: 'command' });
  });

  it('names a pi-mcp-adapter field, telling the tool lists apart from the rest', () => {
    expect([parsePiOptions('{"directTools": true}'), parsePiOptions('{"excludeTools": ["a"]}'), parsePiOptions('{"lifecycle": "lazy"}')])
      .toEqual([{ adapterTools: 'directTools' }, { adapterTools: 'excludeTools' }, { adapter: 'lifecycle' }]);
  });

  it("keeps Pi 0.99.2's auth object and refuses the adapter's auth string", () => {
    expect([parsePiOptions('{"auth": {"provider": "github"}, "oauth": {"clientName": "Claude Code"}}'), parsePiOptions('{"auth": "oauth"}'), parsePiOptions('{"auth": {}}'), parsePiOptions('{"oauth": {"clientName": 1}}')])
      .toEqual([{ value: { auth: { provider: 'github' }, oauth: { clientName: 'Claude Code' } } }, { adapter: 'auth' }, { bad: 'auth.provider' }, { bad: 'oauth.clientName' }]);
  });

  it('accepts an https or loopback oauth.authServerMetadataUrl, as Pi 1.0 requires', () => {
    const parse = (url: string) => parsePiOptions(JSON.stringify({ oauth: { authServerMetadataUrl: url } })).bad;
    expect(['https://example.okta.com/.well-known/openid-configuration', 'http://localhost:8080/meta', 'http://[::1]/meta', 'http://example.com/meta', 'https://u:p@example.com/meta', '/meta'].map(parse))
      .toEqual([undefined, undefined, undefined, 'oauth.authServerMetadataUrl', 'oauth.authServerMetadataUrl', 'oauth.authServerMetadataUrl']);
  });

  it("refuses only Pi's toolExposure while the server has a tool policy", () => {
    expect([parsePiOptions('{"toolExposure": {"a": "hidden"}}', true), parsePiOptions('{"exposure": "direct"}', true)])
      .toEqual([{ overlap: 'toolExposure' }, { value: { exposure: 'direct' } }]);
  });

  it('counts the servers an Agent gets, inherited ones too, but not a switch that turns one off', () => {
    const servers = { own: { command: 'a', targets: ['claude'] }, inherited: { command: 'b' }, off: { disabled: true, targets: ['claude'] } };
    expect(serverCount({ source: { servers, targets: ['claude'] }, paths: { claude: '/c.json' } }, 'claude')).toBe(2);
  });
});

describe('mcpNotice', () => {
  const t = (key: string, params?: Record<string, string>) => `[${key}${params ? ' ' + JSON.stringify(params) : ''}]`;

  it('words the plan notices with UI keys, keeping the servers they name', () => {
    expect(mcpNotice(t, "Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP")).toBe('[mcp.notice.piBuiltin]');
    expect(mcpNotice(t, 'Pi now uses its built-in MCP; the next sync updates the config: docs, local (shop)')).toBe('[mcp.notice.piExtension {"names":"docs, local (shop)"}]');
    expect(mcpNotice(t, "Pi's built-in MCP does not read idleTimeout, lifecycle; the next sync removes them: docs")).toBe('[mcp.notice.adapterOptions {"fields":"idleTimeout, lifecycle","names":"docs"}]');
    expect(mcpNotice(t, 'tool policy not applied for opencode: allow (lists)')).toBe('[mcp.notice.toolPolicy {"agent":"OpenCode","parts":"[mcp.tools.gap.allow]","names":"lists"}]');
  });

  it('shows a notice it does not know as the server sent it', () => {
    expect(mcpNotice(t, 'something new: docs')).toBe('something new: docs');
  });
});
