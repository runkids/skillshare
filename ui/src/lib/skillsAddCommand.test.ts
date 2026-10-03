import { describe, expect, it } from 'vitest';
import { parseSkillsAddCommand, requestedSkills } from './skillsAddCommand';

describe('requestedSkills', () => {
  const items = [{ name: 'Codebase-Design' }, { name: 'code-review' }];

  it('picks named skills regardless of letter case', () => {
    expect(requestedSkills({ source: 'o/r', skills: ['codebase-design'] }, 'o/r', items)).toEqual([{ name: 'Codebase-Design' }]);
  });

  it('keeps the default for another source', () => {
    expect(requestedSkills({ source: 'o/r', skills: ['code-review'] }, 'o/other', items)).toBeNull();
  });

  it('keeps the default when no skill is named', () => {
    expect(requestedSkills(parseSkillsAddCommand(`npx skills add o/r --skill '*'`), 'o/r', items)).toBeNull();
  });

  it('preselects nothing when no named skill is found', () => {
    expect(requestedSkills({ source: 'o/r', skills: ['renamed'] }, 'o/r', items)).toEqual([]);
  });
});

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

  it('splits the owner/repo@skill shorthand', () => {
    expect(parseSkillsAddCommand('npx skills add vercel-labs/agent-skills@react-best-practices -g -y'))
      .toEqual({ source: 'vercel-labs/agent-skills', skills: ['react-best-practices'] });
  });

  it('keeps @ in local paths', () => {
    expect(parseSkillsAddCommand('npx skills add ./repo@draft')).toEqual({ source: './repo@draft', skills: [] });
  });

  it('keeps @ in SSH sources', () => {
    expect(parseSkillsAddCommand('npx skills add git@github.com:owner/repo.git'))
      .toEqual({ source: 'git@github.com:owner/repo.git', skills: [] });
  });

  it('keeps backslash-escaped spaces in a source', () => {
    expect(parseSkillsAddCommand('npx skills add /tmp/my\\ skill')).toEqual({ source: '/tmp/my skill', skills: [] });
  });

  it('keeps backslashes in a Windows path', () => {
    expect(parseSkillsAddCommand('npx skills add C:\\Users\\me\\skills')).toEqual({ source: 'C:\\Users\\me\\skills', skills: [] });
  });

  it('keeps a trailing backslash in a quoted Windows path', () => {
    expect(parseSkillsAddCommand('npx skills add "C:\\Users\\me\\my skill\\"')).toEqual({ source: 'C:\\Users\\me\\my skill\\', skills: [] });
  });

  it('skips values of other options before the source', () => {
    expect(parseSkillsAddCommand(`npx skills add --metadata '{"origin":"docs"}' --subagent a b -g owner/repo`))
      .toEqual({ source: 'owner/repo', skills: [] });
  });

  it('ignores values that are not a skills add command', () => {
    expect(parseSkillsAddCommand('owner/repo')).toBeNull();
  });

  it('ignores a command without a source', () => {
    expect(parseSkillsAddCommand('npx skills add --skill a')).toBeNull();
  });
});
