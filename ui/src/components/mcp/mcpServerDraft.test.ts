import { describe, expect, it } from 'vitest';
import { copyableServers, initialServerDraft, serveSkillsCommand, serveSkillsTarget } from './mcpServerDraft';
import { splitCommand } from './mcpView';

describe('MCP server drafts', () => {
  it('starts from the Pi settings of a server being edited', () => {
    expect(JSON.parse(initialServerDraft({ command: 'docs', piOptions: { exposure: 'direct' } }, 'docs', ['pi'], false).piOptions)).toEqual({ exposure: 'direct' });
  });

  it('starts from the tool policy of a server being edited', () => {
    expect(initialServerDraft({ command: 'docs', tools: { deny: ['delete_*'] } }, 'docs', ['pi'], false).tools).toEqual({ deny: ['delete_*'] });
  });

  it('keeps only Agents with a switch in the targets a new off switch starts with', () => {
    expect(initialServerDraft(undefined, '', ['claude', 'cursor', 'pi'], true).targets).toEqual(['claude', 'pi']);
  });

  it("offers other projects' servers without their targets or off switches", () => {
    const projects = {
      '/work/app': { servers: { mine: { command: 'mine' } } },
      '/work/web': { targets: ['cursor'], servers: { docs: { command: 'docs', targets: ['cursor'] }, github: { disabled: true } } },
    };
    expect(copyableServers(projects, '/work/app')).toEqual([{ root: '/work/web', name: 'docs', server: { command: 'docs' } }]);
  });

  it('reads back the skills target of a command it builds, and nothing from other commands', () => {
    expect(serveSkillsTarget(splitCommand(serveSkillsCommand('claude', true)))).toBe('claude');
    expect(serveSkillsTarget(splitCommand(serveSkillsCommand('', false)))).toBe('');
    // A project's name may hold spaces: its target must survive as one word.
    expect(serveSkillsTarget(splitCommand(serveSkillsCommand('my app@claude', false)))).toBe('my app@claude');
    expect(serveSkillsTarget(['skillshare', 'mcp', 'serve', '--http', ':8765'])).toBeUndefined();
    expect(serveSkillsTarget(['npx', 'skillshare', 'mcp', 'serve'])).toBeUndefined();
  });
});
