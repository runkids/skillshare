import { describe, expect, it } from 'vitest';
import { parseSkillsAddCommand } from './skillsAddCommand';

describe('parseSkillsAddCommand', () => {
  it('reads the source from a plain command', () => {
    expect(parseSkillsAddCommand('npx skills add mattpocock/skills')).toEqual({ source: 'mattpocock/skills', skills: [] });
  });

  it('reads --skill=name with a versioned package', () => {
    expect(parseSkillsAddCommand('npx skills@latest add mattpocock/skills --skill=codebase-design'))
      .toEqual({ source: 'mattpocock/skills', skills: ['codebase-design'] });
  });

  it('reads repeated and variadic skill flags', () => {
    expect(parseSkillsAddCommand('npx skills add owner/repo --skill a -s b c'))
      .toEqual({ source: 'owner/repo', skills: ['a', 'b', 'c'] });
  });

  it('treats a quoted wildcard as no pick', () => {
    expect(parseSkillsAddCommand(`npx skills add owner/repo --skill '*'`)).toEqual({ source: 'owner/repo', skills: [] });
  });

  it('drops agent names and location flags', () => {
    expect(parseSkillsAddCommand('npx -y skills add owner/repo -g -a claude-code codex -y'))
      .toEqual({ source: 'owner/repo', skills: [] });
  });

  it('accepts other runners and a shell prompt', () => {
    expect(parseSkillsAddCommand('$ pnpm dlx skills add https://github.com/owner/repo'))
      .toEqual({ source: 'https://github.com/owner/repo', skills: [] });
  });

  it('ignores values that are not a skills add command', () => {
    expect(parseSkillsAddCommand('owner/repo')).toBeNull();
  });

  it('ignores a command without a source', () => {
    expect(parseSkillsAddCommand('npx skills add --skill a')).toBeNull();
  });
});
